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
	"github.com/pkg/errors"
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

func NewService(context *ServiceContext, repo tenant.Repository, envService *environment.Service) *ServiceBuilder {
	transport := http.DefaultTransport
	if context.config.APIServerUseTLS() {
		transport = &http.Transport{
			TLSClientConfig: &tls.Config{
				InsecureSkipVerify: context.config.APIServerInsecureSkipTLSVerify(),
			},
		}
	}
	return NewBuilderWithTransport(context, repo, transport, envService)
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

func (b *ServiceBuilder) Create(nsTypes []environment.Type, actionOpts *ActionOptions) error {
	return b.service.processAndApplyAll(nsTypes, NewCreateAction(b.service.tenantRepository, actionOpts))
}

func (b *ServiceBuilder) Update(nsTypes []environment.Type, existingNamespaces []*tenant.Namespace, actionOpts *ActionOptions) error {
	return b.service.processAndApplyAll(nsTypes, NewUpdateAction(b.service.tenantRepository, existingNamespaces, actionOpts))
}

func (b *ServiceBuilder) Delete(nsTypes []environment.Type, existingNamespaces []*tenant.Namespace, deleteOpts *DeleteActionOption) error {
	return b.service.processAndApplyAll(nsTypes, NewDeleteAction(b.service.tenantRepository, existingNamespaces, deleteOpts))
}

func (s *Service) processAndApplyAll(nsTypes []environment.Type, action NamespaceAction) error {
	var nsTypesWait sync.WaitGroup
	nsTypesWait.Add(len(nsTypes))

	errorChan := make(chan error, len(nsTypes)*2)
	for _, nsType := range nsTypes {
		nsTypeService := NewEnvironmentTypeService(nsType, s.context, s.envService)
		go processAndApplyNs(&nsTypesWait, nsTypeService, action, s.httpTransport, errorChan)
	}
	nsTypesWait.Wait()
	close(errorChan)

	healingStrategy := action.HealingStrategy()
	return action.ManageAndUpdateResults(errorChan, nsTypes, healingStrategy(&ServiceBuilder{service: s}))

}

type ObjectChecker func(object environment.Object) bool
type OperationSet map[string]environment.Objects

func processAndApplyNs(nsTypeWait *sync.WaitGroup, nsTypeService EnvironmentTypeService, action NamespaceAction, transport http.RoundTripper, errorChan chan error) {
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

	cluster := nsTypeService.GetCluster()
	client := NewClient(transport, cluster.APIURL, nsTypeService.GetTokenProducer(action.ForceMasterTokenGlobally()))

	failed := false
	operationSets, err := action.GetOperationSets(objects, *client, nsTypeService.GetNamespaceName())
	if err != nil {
		errorChan <- errors.Wrapf(err, "for the namespace [%s] the method %s failed for the cluster %s with following error while getting list of objects to apply",
			nsTypeService.GetNamespaceName(), action.MethodName(), cluster.APIURL)
		failed = true
	} else {
		for opAction, objectsToProcess := range operationSets {
			for _, object := range objectsToProcess {
				_, err := Apply(*client, opAction, object)
				if err != nil {
					errorChan <- errors.Wrapf(err, "for the namespace [%s] the method %s failed for the cluster %s with following error",
						nsTypeService.GetNamespaceName(), opAction, cluster.APIURL)
					failed = true
					break
				}
			}
		}
	}

	err = nsTypeService.AfterCallback(client, action.MethodName())
	if err != nil {
		errorChan <- errors.Wrapf(err, "the after callback of a namespace %s failed for the type %s", action.MethodName(), nsTypeService.GetNamespaceName())
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
