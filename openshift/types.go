package openshift

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/pkg/errors"
	"net/http"
	"os"
	"strings"
)

// EnvironmentTypeService represents service operating with information related to environment types(template, objects, cluster,...).
// It is responsible for getting, sorting and filtering objects to be applied, provides information (needed token) specific
// for the particular type and performs after-apply-callback (needed by user namespace)
type EnvironmentTypeService interface {
	GetType() environment.Type
	GetNamespaceName() string
	GetEnvDataAndObjects(filter FilterFunc) (*environment.EnvData, environment.Objects, error)
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

func (t *CommonEnvTypeService) GetType() environment.Type {
	return t.name
}

func (t *CommonEnvTypeService) GetNamespaceName() string {
	return fmt.Sprintf("%s-%s", t.context.nsBaseName, t.name)
}

func (t *CommonEnvTypeService) GetEnvDataAndObjects(filter FilterFunc) (*environment.EnvData, environment.Objects, error) {
	envData, err := t.envService.GetEnvData(t.context.requestCtx, t.name)
	if err != nil {
		return nil, nil, err
	}
	objects, err := t.getObjects(envData, filter)
	return envData, objects, err
}

func (t *CommonEnvTypeService) getObjects(env *environment.EnvData, filter FilterFunc) (environment.Objects, error) {
	var objs environment.Objects
	cluster := t.context.clusterForEnvType(t.name)
	vars := environment.CollectVars(t.context.openShiftUsername, t.context.nsBaseName, cluster.User, t.context.config)

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

func (t *CommonEnvTypeService) GetCluster() cluster.Cluster {
	return t.context.clusterForEnvType(t.name)
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

func (t *UserNamespaceTypeService) AfterCallback(client *Client, action string) error {
	if action != http.MethodPost {
		return nil
	}
	adminRoleBinding := CreateAdminRoleBinding(t.context.nsBaseName)
	_, err := Apply(*client, http.MethodDelete, adminRoleBinding)

	if err != nil {
		return errors.Wrapf(err, "unable to remove admin rolebinding in %s namespace", t.GetNamespaceName())
	}
	return nil
}

func (t *UserNamespaceTypeService) GetTokenProducer(forceMasterTokenGlobally bool) TokenProducer {
	return func(forceMasterToken bool) string {
		if forceMasterTokenGlobally || forceMasterToken {
			return t.GetCluster().Token
		}
		return t.context.userTokenResolver(t.GetCluster())
	}
}

func (t *UserNamespaceTypeService) GetNamespaceName() string {
	return t.context.nsBaseName
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
