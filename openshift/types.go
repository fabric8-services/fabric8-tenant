package openshift

import (
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/toggles"
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
	GetEnvDataAndObjects() (*EnvAndObjectsManager, error)
	GetCluster() cluster.Cluster
	AfterCallback(client *Client, action string) error
	GetTokenProducer(forceMasterTokenGlobally bool) TokenProducer
	AdditionalObject() (environment.Object, bool)
}

func NewEnvironmentTypeService(envType environment.Type, context *ServiceContext, envService *environment.Service) EnvironmentTypeService {
	service := &CommonEnvTypeService{
		envType:    envType,
		context:    context,
		envService: envService,
	}
	if envType == environment.TypeUser {
		return &UserNamespaceTypeService{CommonEnvTypeService: service}
	} else if envType == environment.TypeChe {
		return &CheNamespaceTypeService{
			CommonEnvTypeService: service,
			isToggleEnabled:      toggles.IsEnabled,
		}
	}
	return service
}

type CommonEnvTypeService struct {
	envType    environment.Type
	context    *ServiceContext
	envService *environment.Service
}

func (t *CommonEnvTypeService) GetType() environment.Type {
	return t.envType
}

func (t *CommonEnvTypeService) GetNamespaceName() string {
	return fmt.Sprintf("%s-%s", t.context.nsBaseName, t.envType)
}

func (t *CommonEnvTypeService) GetEnvDataAndObjects() (*EnvAndObjectsManager, error) {
	return t.getEnvDataAndObjects(t.GetNamespaceName())
}

func (t *CommonEnvTypeService) getEnvDataAndObjects(namespaceName string) (*EnvAndObjectsManager, error) {
	envData, objects, err := getEnvDataAndObjects(t.envService, t.envType, *t.context)
	envAndObjectsManager := NewEnvAndObjectsManager(envData, objects,
		func(version string) (*environment.EnvData, environment.Objects, error) {
			return getEnvDataAndObjects(environment.NewServiceForBlob(version), t.envType, *t.context)
		}, t.envType, namespaceName,
	)
	return envAndObjectsManager, err
}

func getEnvDataAndObjects(envService *environment.Service, envType environment.Type, context ServiceContext) (*environment.EnvData, environment.Objects, error) {
	envData, err := envService.GetEnvData(context.requestCtx, envType)
	if err != nil {
		return nil, nil, err
	}
	objects, err := getObjects(envData, envType, context)
	return envData, objects, err
}

func getObjects(env *environment.EnvData, envType environment.Type, context ServiceContext) (environment.Objects, error) {
	var objs environment.Objects
	cluster := context.clusterForType(envType)
	vars := environment.CollectVars(context.openShiftUsername, context.nsBaseName, cluster.User, context.config)

	for _, template := range env.Templates {
		if os.Getenv("DISABLE_OSO_QUOTAS") == "true" && strings.Contains(template.Filename, "quotas") {
			continue
		}

		objects, err := template.Process(vars)
		if err != nil {
			return nil, err
		}
		objs = append(objs, objects...)
	}
	return objs, nil
}

func (t *CommonEnvTypeService) GetCluster() cluster.Cluster {
	return t.context.clusterForType(t.envType)
}

func (t *CommonEnvTypeService) AfterCallback(client *Client, action string) error {
	return nil
}

func (t *CommonEnvTypeService) GetTokenProducer(forceMasterTokenGlobally bool) TokenProducer {
	return func(forceMasterToken bool) string {
		return t.GetCluster().Token
	}
}

func (t *CommonEnvTypeService) AdditionalObject() (environment.Object, bool) {
	return environment.Object{}, true
}

type CheNamespaceTypeService struct {
	*CommonEnvTypeService
	isToggleEnabled toggles.IsToggleEnabled
}

func (t *CheNamespaceTypeService) AdditionalObject() (environment.Object, bool) {
	return t.newEditRightsObject(), t.isToggleEnabled(t.context.requestCtx, "che.edit.rights", false)
}

func (t *CheNamespaceTypeService) newEditRightsObject() environment.Object {
	adminRb := NewObject(environment.ValKindRoleBinding, t.GetNamespaceName(), "user-edit")
	adminRb["roleRef"] = environment.Object{"name": "edit"}
	adminRb["subjects"] = environment.Objects{{
		"kind": "User",
		"name": t.context.openShiftUsername}}
	adminRb["userNames"] = []string{t.context.openShiftUsername}
	return adminRb
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

func (t *UserNamespaceTypeService) GetEnvDataAndObjects() (*EnvAndObjectsManager, error) {
	return t.getEnvDataAndObjects(t.GetNamespaceName())
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

type EnvAndObjectsManager struct {
	EnvData           *environment.EnvData
	allObjects        environment.Objects
	objectsForVersion objectsForVersionRetrieval
	envType           environment.Type
	namespaceName     string
}

func NewEnvAndObjectsManager(envData *environment.EnvData, allObjects environment.Objects,
	objectsForVersion objectsForVersionRetrieval, envType environment.Type, namespaceName string) *EnvAndObjectsManager {
	return &EnvAndObjectsManager{
		EnvData:           envData,
		allObjects:        allObjects,
		objectsForVersion: objectsForVersion,
		envType:           envType,
		namespaceName:     namespaceName,
	}
}

func (m *EnvAndObjectsManager) GetMissingObjectsComparingWith(version string) (environment.Objects, error) {
	removedObjects, err := CacheOfRemovedObjects.GetResolver(m.envType, version, m.objectsForVersion, m.allObjects).Resolve()
	if err != nil {
		CacheOfRemovedObjects.RemoveResolver(m.envType, version)
		return nil, err
	}
	var objs environment.Objects
	for _, removedObj := range removedObjects {
		objs = append(objs, removedObj.toObject(m.namespaceName))
	}

	return objs, nil
}

func (m *EnvAndObjectsManager) GetObjects(filterFunc FilterFunc) environment.Objects {
	var filtered environment.Objects
	for _, obj := range m.allObjects {
		if filterFunc(obj) {
			filtered = append(filtered, obj)
		}
	}
	return filtered
}
