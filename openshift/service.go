package openshift

import (
	"context"
	"crypto/tls"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/auth"
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
	requestCtx        context.Context
	clusterForType    cluster.ForType
	openShiftUsername string
	userTokenResolver UserTokenResolver
	config            *configuration.Data
	nsBaseName        string
}

func NewServiceContext(callerCtx context.Context, config *configuration.Data, clusterMapping cluster.ForType, openShiftUsername, nsBaseName string, userTokenResolver UserTokenResolver) *ServiceContext {
	return &ServiceContext{
		requestCtx:        callerCtx,
		clusterForType:    clusterMapping,
		openShiftUsername: openShiftUsername,
		userTokenResolver: userTokenResolver,
		config:            config,
		nsBaseName:        nsBaseName,
	}
}

type WithActionBuilder struct {
	service *Service
	action  NamespaceAction
}

func (s *WithActionBuilder) ApplyAll(nsTypes []environment.Type) error {
	return s.service.processAndApplyAll(nsTypes, s.action)
}

func (b *ServiceBuilder) WithPostMethod() *WithActionBuilder {
	return &WithActionBuilder{
		service: b.service,
		action:  NewCreate(b.service.tenantRepository),
	}
}

func (b *ServiceBuilder) WithPatchMethod(existingNamespaces []*tenant.Namespace) *WithActionBuilder {
	return &WithActionBuilder{
		service: b.service,
		action:  NewUpdate(b.service.tenantRepository, existingNamespaces),
	}
}

func (b *ServiceBuilder) WithDeleteMethod(existingNamespaces []*tenant.Namespace, removeFromCluster bool) *WithActionBuilder {
	return &WithActionBuilder{
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
	return action.CheckNamespacesAndUpdateTenant(namespaces, nsTypes)
}

type ObjectChecker func(object environment.Object) bool

func processAndApplyNs(nsTypeWait *sync.WaitGroup, nsTypeService EnvironmentTypeService, action NamespaceAction, transport http.RoundTripper) {
	defer nsTypeWait.Done()

	namespace, err := action.GetNamespaceEntity(nsTypeService)
	if err != nil {
		sentry.LogError(nil, map[string]interface{}{
			"envType":       nsTypeService.GetType(),
			"namespaceName": nsTypeService.GetNamespaceName(),
			"action":        action.MethodName(),
		}, err, "getting the namespace failed")
		return
	}
	if namespace == nil {
		return
	}

	env, objects, err := nsTypeService.GetEnvDataAndObjects(action.Filter())
	if err != nil {
		sentry.LogError(nil, map[string]interface{}{
			"envType":       nsTypeService.GetType(),
			"namespaceName": nsTypeService.GetNamespaceName(),
			"action":        action.MethodName(),
		}, err, "getting environment data and objects failed")
		return
	}
	action.Sort(environment.ByKind(objects))

	cluster := nsTypeService.GetCluster()
	client := NewClient(transport, cluster.APIURL, nsTypeService.GetTokenProducer(action.ForceMasterTokenGlobally()))

	failed := false
	for _, object := range objects {
		_, err := Apply(*client, action.MethodName(), object)
		if err != nil {
			sentry.LogError(nsTypeService.GetRequestsContext(), map[string]interface{}{
				"ns-type": nsTypeService.GetType(),
				"action":  action.MethodName(),
				"cluster": cluster.APIURL,
				"ns-name": nsTypeService.GetNamespaceName(),
			}, err, action.MethodName()+"method applied to the namespace failed")
			failed = true
			break
		}
	}

	err = nsTypeService.AfterCallback(client, action.MethodName())
	if err != nil {
		sentry.LogError(nsTypeService.GetRequestsContext(), map[string]interface{}{
			"ns-type": nsTypeService.GetType(),
			"action":  action.MethodName(),
			"cluster": cluster.APIURL,
			"ns-name": nsTypeService.GetNamespaceName(),
		}, err, "the after callback failed")
	}
	namespace.Version = env.Version()
	action.UpdateNamespace(env, &cluster, namespace, failed || err != nil)
}

func Apply(client Client, action string, object environment.Object) (*Result, error) {

	objectEndpoint, found := AllObjectEndpoints[environment.GetKind(object)]
	if !found {
		err := fmt.Errorf("there is no supported endpoint for the object %s", environment.GetKind(object))
		return nil, err
	}

	result, err := objectEndpoint.Apply(&client, object, action)
	return result, err
}

type UserTokenResolver func(cluster cluster.Cluster) string

func TokenResolverForUser(user *auth.User) UserTokenResolver {
	return func(cluster cluster.Cluster) string {
		return user.OpenShiftUserToken
	}
}

func TokenResolver() UserTokenResolver {
	return func(cluster cluster.Cluster) string {
		return cluster.Token
	}
}
