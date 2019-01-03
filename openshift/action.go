package openshift

import (
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"net/http"
	"sort"
)

// NamespaceAction represents the action that should be applied on the namespaces for the particular tenant - [post|update|delete].
// It is mainly responsible for operation on DB and provides additional information specific to the action that is needed by other objects
type NamespaceAction interface {
	MethodName() string
	GetNamespaceEntity(nsTypeService EnvironmentTypeService) (*tenant.Namespace, error)
	UpdateNamespace(env *environment.EnvData, cluster *cluster.Cluster, namespace *tenant.Namespace, failed bool)
	Sort(toSort environment.ByKind)
	Filter() FilterFunc
	ForceMasterTokenGlobally() bool
	CheckNamespacesAndUpdateTenant(errorChan chan error, envTypes []environment.Type) error
}

type commonNamespaceAction struct {
	method string
}

func (c *commonNamespaceAction) MethodName() string {
	return c.method
}

func (c *commonNamespaceAction) Sort(toSort environment.ByKind) {
	sort.Sort(toSort)
}

func (c *commonNamespaceAction) Filter() FilterFunc {
	return func(objects environment.Object) bool {
		return true
	}
}

func (c *commonNamespaceAction) ForceMasterTokenGlobally() bool {
	return true
}

func (c *commonNamespaceAction) CheckNamespacesAndUpdateTenant(errorChan chan error, envTypes []environment.Type) error {
	var msg string
	index := 1
	for er := range errorChan {
		if er != nil {
			msg += fmt.Sprintf("#%d: %s\n", index, er.Error())
			index++
		}
	}
	if len(msg) > 0 {
		return fmt.Errorf("%s method applied to namespace types %s failed with one or more errors:\n%s", c.method, envTypes, msg)
	}
	return nil
}

func NewCreate(tenantRepo tenant.Repository) *Create {
	return &Create{
		commonNamespaceAction: &commonNamespaceAction{method: http.MethodPost},
		tenantRepo:            tenantRepo,
	}
}

type Create struct {
	*commonNamespaceAction
	tenantRepo tenant.Repository
}

func (c *Create) GetNamespaceEntity(nsTypeService EnvironmentTypeService) (*tenant.Namespace, error) {
	namespace := c.tenantRepo.NewNamespace(nsTypeService.GetType(), nsTypeService.GetNamespaceName(), tenant.Provisioning)
	err := c.tenantRepo.SaveNamespace(namespace)
	return namespace, err
}

func (c *Create) UpdateNamespace(env *environment.EnvData, cluster *cluster.Cluster, namespace *tenant.Namespace, failed bool) {
	state := tenant.Ready
	if failed {
		state = tenant.Failed
	}
	namespace.UpdateData(env, cluster, state)
	err := c.tenantRepo.SaveNamespace(namespace)
	if err != nil {
		sentry.LogError(nil, map[string]interface{}{
			"env_type": env.EnvType,
			"cluster":  cluster.APIURL,
			"tenant":   namespace.TenantID,
			"state":    state,
		}, err, "creation of namespace entity failed")
	}
}

func (c *Create) ForceMasterTokenGlobally() bool {
	return false
}

func NewDelete(tenantRepo tenant.Repository, removeFromCluster bool, existingNamespaces []*tenant.Namespace) *Delete {
	return &Delete{
		commonNamespaceAction: &commonNamespaceAction{method: http.MethodDelete},
		withExistingNamespacesAction: &withExistingNamespacesAction{
			existingNamespaces: existingNamespaces,
		},
		tenantRepo:        tenantRepo,
		removeFromCluster: removeFromCluster,
	}
}

type Delete struct {
	*commonNamespaceAction
	*withExistingNamespacesAction
	tenantRepo        tenant.Repository
	removeFromCluster bool
}

func (d *Delete) GetNamespaceEntity(nsTypeService EnvironmentTypeService) (*tenant.Namespace, error) {
	return d.getNamespaceFor(nsTypeService.GetType()), nil
}

func (d *Delete) UpdateNamespace(env *environment.EnvData, cluster *cluster.Cluster, namespace *tenant.Namespace, failed bool) {
	var err error
	if failed {
		namespace.State = tenant.Failed
		err = d.tenantRepo.SaveNamespace(namespace)
	} else if d.removeFromCluster {
		err = d.tenantRepo.DeleteNamespace(namespace)
	}
	if err != nil {
		sentry.LogError(nil, map[string]interface{}{
			"env_type":            env.EnvType,
			"cluster":             cluster.APIURL,
			"tenant":              namespace.TenantID,
			"state":               namespace.State,
			"remove_from_cluster": d.removeFromCluster,
		}, err, "deleting namespace entity failed")
	}
}

func (d *Delete) Filter() FilterFunc {
	if d.removeFromCluster {
		return isOfKind(environment.ValKindProjectRequest)
	}
	return isOfKind(environment.ValKindPersistenceVolumeClaim, environment.ValKindConfigMap, environment.ValKindService,
		environment.ValKindDeploymentConfig, environment.ValKindRoute)
}

func (d *Delete) Sort(toSort environment.ByKind) {
	sort.Sort(sort.Reverse(toSort))
}

type withExistingNamespacesAction struct {
	existingNamespaces []*tenant.Namespace
}

func (a withExistingNamespacesAction) getNamespaceFor(nsType environment.Type) *tenant.Namespace {
	for _, ns := range a.existingNamespaces {
		if ns.Type == nsType {
			return ns
		}
	}
	return nil
}

func (d Delete) CheckNamespacesAndUpdateTenant(errorChan chan error, envTypes []environment.Type) error {
	err := d.commonNamespaceAction.CheckNamespacesAndUpdateTenant(errorChan, envTypes)
	if err != nil {
		return err
	}
	namespaces, err := d.tenantRepo.GetNamespaces()
	if err != nil {
		return err
	}
	if d.removeFromCluster {
		if len(namespaces) == 0 {
			return d.tenantRepo.DeleteTenant()
		}
		var names []string
		for _, ns := range namespaces {
			names = append(names, ns.Name)
		}
		return fmt.Errorf("cannot remove tenant %s from DB - some namespaces %s still exist", namespaces[0].TenantID, names)
	}
	return nil
}

func NewUpdate(tenantRepo tenant.Repository, existingNamespaces []*tenant.Namespace) *Update {
	return &Update{
		commonNamespaceAction: &commonNamespaceAction{method: http.MethodPatch},
		withExistingNamespacesAction: &withExistingNamespacesAction{
			existingNamespaces: existingNamespaces,
		},
		tenantRepo: tenantRepo,
	}
}

type Update struct {
	*commonNamespaceAction
	*withExistingNamespacesAction
	tenantRepo tenant.Repository
}

func (u *Update) GetNamespaceEntity(nsTypeService EnvironmentTypeService) (*tenant.Namespace, error) {
	return u.getNamespaceFor(nsTypeService.GetType()), nil
}

func (u *Update) UpdateNamespace(env *environment.EnvData, cluster *cluster.Cluster, namespace *tenant.Namespace, failed bool) {
	state := tenant.Ready
	if failed {
		state = tenant.Failed
	}
	namespace.UpdateData(env, cluster, state)
	err := u.tenantRepo.SaveNamespace(namespace)
	if err != nil {
		sentry.LogError(nil, map[string]interface{}{
			"env_type": env.EnvType,
			"cluster":  cluster.APIURL,
			"tenant":   namespace.TenantID,
			"state":    state,
		}, err, "updating namespace entity failed")
	}
}

func (d *Update) Filter() FilterFunc {
	return isNotOfKind(environment.ValKindProjectRequest)
}

type FilterFunc func(environment.Object) bool

func isOfKind(kinds ...string) FilterFunc {
	return func(vs environment.Object) bool {
		kind := environment.GetKind(vs)
		for _, k := range kinds {
			if k == kind {
				return true
			}
		}
		return false
	}
}

func isNotOfKind(kinds ...string) FilterFunc {
	f := isOfKind(kinds...)
	return func(vs environment.Object) bool {
		return !f(vs)
	}
}
