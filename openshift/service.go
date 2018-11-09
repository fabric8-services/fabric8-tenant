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
	nsBaseName         string
}

func NewServiceContext(callerCtx context.Context, config *configuration.Data, clusterMapping cluster.ForType, openShiftUsername, openShiftUserToken, nsBaseName string) *ServiceContext {
	return &ServiceContext{
		requestCtx:         callerCtx,
		clusterForType:     clusterMapping,
		openShiftUsername:  openShiftUsername,
		openShiftUserToken: openShiftUserToken,
		config:             config,
		nsBaseName:         nsBaseName,
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

	namespaces, err := s.tenantRepository.GetNamespaces()
	if err != nil {
		return err
	}
	return action.checkNamespacesAndUpdateTenant(namespaces, nsTypes)
}

type ObjectChecker func(object environment.Object) bool

func processAndApplyNs(nsTypeWait *sync.WaitGroup, nsTypeService EnvironmentTypeService, action NamespaceAction, transport http.RoundTripper) {
	defer nsTypeWait.Done()

	namespace, err := action.getNamespaceEntity(nsTypeService)
	if err != nil {
		sentry.LogError(nil, map[string]interface{}{
			"envType":       nsTypeService.GetType(),
			"namespaceName": nsTypeService.GetNamespaceName(),
			"action":        action.methodName(),
		}, err, "getting the namespace failed")
		return
	}
	if namespace == nil {
		return
	}

	env, objects, err := nsTypeService.GetEnvDataAndObjects(action.filter())
	if err != nil {
		sentry.LogError(nil, map[string]interface{}{
			"envType":       nsTypeService.GetType(),
			"namespaceName": nsTypeService.GetNamespaceName(),
			"action":        action.methodName(),
		}, err, "getting environment data and objects failed")
		return
	}
	action.sort(environment.ByKind(objects))

	cluster := nsTypeService.GetCluster()
	client := NewClient(transport, cluster.APIURL, nsTypeService.GetTokenProducer(action.forceMasterTokenGlobally()))

	failed := false
	for _, object := range objects {
		err := apply(*client, action.methodName(), object)
		if err != nil {
			err = fmt.Errorf("%s method applied to the namespace failed", action.methodName())
			sentry.LogError(nsTypeService.GetRequestsContext(), map[string]interface{}{
				"ns-type": nsTypeService.GetType(),
				"action":  action.methodName(),
				"cluster": cluster.APIURL,
				"ns-name": nsTypeService.GetNamespaceName(),
			}, err, err.Error())
			failed = true
			break
		}
	}

	err = nsTypeService.AfterCallback(client, action.methodName())
	action.updateNamespace(env, &cluster, namespace, failed || err != nil)
}

func apply(client Client, action string, object environment.Object) error {

	objectEndpoint, found := AllObjectEndpoints[environment.GetKind(object)]
	if !found {
		err := fmt.Errorf("there is no supported endpoint for the object %s", environment.GetKind(object))
		return err
	}

	_, err := objectEndpoint.Apply(&client, object, action)
	return err
}
