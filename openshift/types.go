package openshift

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"net/http"
	"os"
	"strings"
	"sync"
)

// EnvironmentTypeService represents service operating with information related to environment types(template, objects, cluster,...).
// It is responsible for getting, sorting and filtering objects to be applied, provides information (needed token) specific
// for the particular type and performs after-apply-callback (needed by user namespace)
type EnvironmentTypeService interface {
	GetName() environment.Type
	GetNamespaceName() string
	GetEnvData() (*environment.EnvData, error)
	GetAndSortObjects(env *environment.EnvData, action NamespaceAction) (environment.Objects, error)
	GetCluster() cluster.Cluster
	AfterCallback(client *Client, action string) error
	GetTokenProducer(forceMasterTokenGlobally bool) TokenProducer
	GetRequestsContext() context.Context
}

func NewEnvironmentTypeService(envType environment.Type, context *ServiceContext, envService *environment.Service) EnvironmentTypeService {
	service := &CommonEnvTypeService{
		name:       envType,
		context:    context,
		envService: envService,
	}
	if envType == environment.TypeUser {
		return &UserNamespaceTypeService{CommonEnvTypeService: service}
	}
	return service
}

type CommonEnvTypeService struct {
	name       environment.Type
	context    *ServiceContext
	envService *environment.Service
}

func (t *CommonEnvTypeService) GetRequestsContext() context.Context {
	return t.context.requestCtx
}

func (t *CommonEnvTypeService) GetName() environment.Type {
	return t.name
}

func (t *CommonEnvTypeService) GetNamespaceName() string {
	return fmt.Sprintf("%s-%s", environment.RetrieveUserName(t.context.openShiftUsername), t.name)
}

func (t *CommonEnvTypeService) GetEnvData() (*environment.EnvData, error) {
	return t.envService.GetEnvData(t.context.requestCtx, t.name)
}

func (t *CommonEnvTypeService) getObjects(env *environment.EnvData, filter FilterFunc) (environment.Objects, error) {
	var objs environment.Objects
	cluster := t.context.clusterForType(t.name)
	vars := environment.CollectVars(t.context.openShiftUsername, cluster.User, t.context.config)

	for _, template := range env.Templates {
		if os.Getenv("DISABLE_OSO_QUOTAS") == "true" && strings.Contains(template.Filename, "quotas") {
			continue
		}

		objects, err := template.Process(vars)
		if err != nil {
			return nil, err
		}
		for _, obj := range objects {
			if filter(obj) {
				objs = append(objs, obj)
			}
		}
	}
	return objs, nil
}

func (t *CommonEnvTypeService) GetAndSortObjects(env *environment.EnvData, action NamespaceAction) (environment.Objects, error) {
	objects, err := t.getObjects(env, action.filter())
	if err != nil {
		return nil, err
	}

	action.sort(environment.ByKind(objects))
	return objects, nil
}

func (t *CommonEnvTypeService) GetCluster() cluster.Cluster {
	return t.context.clusterForType(t.name)
}

func (t *CommonEnvTypeService) AfterCallback(client *Client, action string) error {
	return nil
}

func (t *CommonEnvTypeService) GetTokenProducer(forceMasterTokenGlobally bool) TokenProducer {
	return func(forceMasterToken bool) string {
		return t.GetCluster().Token
	}
}

type UserNamespaceTypeService struct {
	*CommonEnvTypeService
}

func (t *UserNamespaceTypeService) GetAndSortObjects(env *environment.EnvData, action NamespaceAction) (environment.Objects, error) {
	objects, err := t.getObjects(env, action.filter())
	if err != nil {
		return nil, err
	}

	action.sort(environment.UserNsByKind(objects))
	return objects, nil
}

func (t *UserNamespaceTypeService) AfterCallback(client *Client, action string) error {
	if action != http.MethodPost {
		return nil
	}
	var removeRoleWait sync.WaitGroup
	removeRoleWait.Add(1)
	adminRoleBinding := CreateAdminRoleBinding(environment.RetrieveUserName(t.context.openShiftUsername))
	objErrs := sync.Map{}
	apply(&removeRoleWait, *client, http.MethodDelete, adminRoleBinding, &objErrs)

	if err, found := objErrs.Load(adminRoleBinding); found {
		return fmt.Errorf("unable to remove adminrolebinding in %s namespace because of the error: %s", t.GetNamespace(), err)
	}
	return nil
}

func (t *UserNamespaceTypeService) GetTokenProducer(forceMasterTokenGlobally bool) TokenProducer {
	return func(forceMasterToken bool) string {
		if forceMasterTokenGlobally || forceMasterToken {
			return t.GetCluster().Token
		}
		return t.context.openShiftUserToken
	}
}

func (t *UserNamespaceTypeService) GetNamespace() string {
	return string(t.GetName())
}

func (t *UserNamespaceTypeService) GetNamespaceName() string {
	return environment.RetrieveUserName(t.context.openShiftUsername)
}

func CreateAdminRoleBinding(namespace string) environment.Object {
	objs, err := environment.ParseObjects(adminRole)
	if err == nil {
		obj := objs[0]
		if val, ok := obj[environment.FieldMetadata].(environment.Object); ok {
			val[environment.FieldNamespace] = namespace
		}
		return obj
	}
	return environment.Object{}
}
