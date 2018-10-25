package openshift

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	log "github.com/sirupsen/logrus"
	"net/http"
	"sync"
)

type ServiceBuilder struct {
	service *Service
}

// Service knowing which action is requested starts for every requested environment type a new goroutine. The goroutine
// gets template objects to be applied and the target cluster; and for every object starts new goroutine that sends a request to the OS cluster.
type Service struct {
	httpTransport    http.RoundTripper
	context          *ServiceContext
	tenantRepository tenant.Repository
	envService       *environment.Service
}

func NewService(context *ServiceContext, namespaceRepository tenant.Repository, envService *environment.Service) *ServiceBuilder {
	transport := http.DefaultTransport
	if context.config.APIServerUseTLS() {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: context.config.APIServerInsecureSkipTLSVerify(),
			},
		}
	}
	return NewBuilderWithTransport(context, namespaceRepository, transport, envService)
}

func NewBuilderWithTransport(context *ServiceContext, namespaceRepository tenant.Repository, transport http.RoundTripper, envService *environment.Service) *ServiceBuilder {
	return &ServiceBuilder{service: &Service{
		httpTransport:    transport,
		context:          context,
		tenantRepository: namespaceRepository,
		envService:       envService,
	}}
}

type ServiceContext struct {
	requestCtx         context.Context
	clusterForType     cluster.ForType
	openShiftUsername  string
	openShiftUserToken string
	config             *configuration.Data
}

func NewServiceContext(callerCtx context.Context, config *configuration.Data, clusterMapping cluster.ForType, openShiftUsername, openShiftUserToken string) *ServiceContext {
	return &ServiceContext{
		requestCtx:         callerCtx,
		clusterForType:     clusterMapping,
		openShiftUsername:  openShiftUsername,
		openShiftUserToken: openShiftUserToken,
		config:             config,
	}
}

type WithExistingNamespaces struct {
	service *Service
	action  NamespaceAction
}

func (s *WithExistingNamespaces) ApplyAll() error {
	return s.service.processAndApplyAll(environment.DefaultEnvTypes, s.action)
}

type WithoutExistingNamespaces struct {
	service *Service
	action  NamespaceAction
}

func (s *WithoutExistingNamespaces) ApplyAll(nsTypes ...environment.Type) error {
	return s.service.processAndApplyAll(nsTypes, s.action)
}

func (b *ServiceBuilder) WithPostMethod() *WithoutExistingNamespaces {
	return &WithoutExistingNamespaces{
		service: b.service,
		action:  NewCreate(b.service.tenantRepository),
	}
}

func (b *ServiceBuilder) WithPatchMethod(existingNamespaces []*tenant.Namespace) *WithExistingNamespaces {
	return &WithExistingNamespaces{
		service: b.service,
		action:  NewUpdate(b.service.tenantRepository, existingNamespaces),
	}
}

func (b *ServiceBuilder) WithDeleteMethod(existingNamespaces []*tenant.Namespace, removeFromCluster bool) *WithExistingNamespaces {
	return &WithExistingNamespaces{
		service: b.service,
		action:  NewDelete(b.service.tenantRepository, removeFromCluster, existingNamespaces),
	}
}

func (s *Service) processAndApplyAll(nsTypes []environment.Type, action NamespaceAction) error {
	var nsTypesWait sync.WaitGroup
	nsTypesWait.Add(len(nsTypes))

	for _, nsType := range nsTypes {
		nsTypeService := NewEnvironmentTypeService(nsType, s.context, s.envService)
		go processAndApplyNs(&nsTypesWait, nsTypeService, action, s.httpTransport)
	}
	nsTypesWait.Wait()

	return action.updateTenant()
}

type ObjectChecker func(object environment.Object) bool

func processAndApplyNs(nsTypeWait *sync.WaitGroup, nsTypeService EnvironmentTypeService, action NamespaceAction, transport http.RoundTripper) {
	defer nsTypeWait.Done()

	namespace, err := action.getNamespaceEntity(nsTypeService)
	if err != nil {
		log.Error(err)
		return
	}
	if namespace == nil {
		return
	}

	env, err := nsTypeService.GetEnvData()
	if err != nil {
		return
	}

	objects, err := nsTypeService.GetAndSortObjects(env, action)

	cluster := nsTypeService.GetCluster()
	client := newClient(transport, cluster.APIURL, nsTypeService.GetTokenProducer(action.forceMasterTokenGlobally()))

	var objectsWait sync.WaitGroup
	objectsWait.Add(len(objects))

	objErrs := sync.Map{}
	for _, object := range objects {
		go apply(&objectsWait, *client, action.methodName(), object, &objErrs)
	}
	objectsWait.Wait()

	errorParamsPerObject := getErrorParamsPerObject(&objErrs)
	if len(errorParamsPerObject) > 0 {
		errorParamsPerObject["ns-type"] = nsTypeService.GetName()
		errorParamsPerObject["action"] = action.methodName()
		errorParamsPerObject["cluster"] = cluster.APIURL
		errorParamsPerObject["ns-type"] = nsTypeService.GetName()
		err = fmt.Errorf("creation of the namespace failed")
		sentry.LogError(nsTypeService.GetRequestsContext(), errorParamsPerObject, err, err.Error())
	}

	err = nsTypeService.AfterCallback(client, action.methodName())
	action.updateNamespace(env, &cluster, namespace, len(errorParamsPerObject) > 0 || err != nil)
}

func getErrorParamsPerObject(errors *sync.Map) map[string]interface{} {
	params := map[string]interface{}{}
	index := 1
	errors.Range(func(object, err interface{}) bool {
		params[fmt.Sprintf("object%d", index)] = object
		params[fmt.Sprintf("err%d", index)] = err
		index++
		return true
	})
	return params
}

func apply(objectsWait *sync.WaitGroup, client Client, action string, object environment.Object, errors *sync.Map) {
	defer objectsWait.Done()

	objectEndpoint, found := objectEndpoints[environment.GetKind(object)]
	if !found {
		log.Error(fmt.Errorf("there is no supported endpoint for the object %s", environment.GetKind(object)))
		return
	}

	_, err := objectEndpoint.Apply(&client, object, action)

	if err != nil {
		errors.Store(object.ToString(), err)
	}
}
