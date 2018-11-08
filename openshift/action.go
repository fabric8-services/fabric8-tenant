package openshift

import (
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"net/http"
	"sort"
	"strings"
)

// NamespaceAction represents the action that should be applied on the namespaces for the particular tenant - [post|update|delete].
// It is mainly responsible for operation on DB and provides additional information specific to the action that is needed by other objects
type NamespaceAction interface {
	methodName() string
	getNamespaceEntity(nsTypeService EnvironmentTypeService) (*tenant.Namespace, error)
	updateNamespace(env *environment.EnvData, cluster *cluster.Cluster, namespace *tenant.Namespace, failed bool)
	sort(toSort environment.ByKind)
	filter() FilterFunc
	forceMasterTokenGlobally() bool
	checkNamespacesAndUpdateTenant(namespaces []*tenant.Namespace, envTypes []environment.Type) error
}

type commonNamespaceAction struct {
	method string
}

func (c *commonNamespaceAction) methodName() string {
	return c.method
}

func (c *commonNamespaceAction) sort(toSort environment.ByKind) {
	sort.Sort(toSort)
}

func (c *commonNamespaceAction) filter() FilterFunc {
	return func(objects environment.Object) bool {
		return true
	}
}

func (c *commonNamespaceAction) forceMasterTokenGlobally() bool {
	return true
}

func (c *commonNamespaceAction) checkNamespacesAndUpdateTenant(namespaces []*tenant.Namespace, envTypes []environment.Type) error {
	var failedNamespaces []string
	for _, ns := range namespaces {
		if ns.State == tenant.Failed {
			for _, envType := range envTypes {
				if ns.Type == envType {
					failedNamespaces = append(failedNamespaces, ns.Name)
				}
			}
		}
	}
	if len(failedNamespaces) > 0 {
		return fmt.Errorf("applying %s action on namespaces [%s] failed", c.method, strings.Join(failedNamespaces, ", "))
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

func (c *Create) getNamespaceEntity(nsTypeService EnvironmentTypeService) (*tenant.Namespace, error) {
	namespace := c.tenantRepo.NewNamespace(nsTypeService.GetType(), nsTypeService.GetNamespaceName(), tenant.Provisioning)
	err := c.tenantRepo.SaveNamespace(namespace)
	return namespace, err
}

func (c *Create) updateNamespace(env *environment.EnvData, cluster *cluster.Cluster, namespace *tenant.Namespace, failed bool) {
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
		}, err, "updating namespace entity failed")
	}
}

func (c *Create) forceMasterTokenGlobally() bool {
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

func (d *Delete) getNamespaceEntity(nsTypeService EnvironmentTypeService) (*tenant.Namespace, error) {
	return d.getNamespaceFor(nsTypeService.GetType()), nil
}

func (d *Delete) updateNamespace(env *environment.EnvData, cluster *cluster.Cluster, namespace *tenant.Namespace, failed bool) {
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

func (d *Delete) filter() FilterFunc {
	if d.removeFromCluster {
		return isOfKind(environment.ValKindProjectRequest)
	}
	return isOfKind(environment.ValKindPersistenceVolumeClaim, environment.ValKindConfigMap)
}

func (d *Delete) sort(toSort environment.ByKind) {
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

func (d Delete) checkNamespacesAndUpdateTenant(namespaces []*tenant.Namespace, envTypes []environment.Type) error {
	if d.removeFromCluster {
		if len(namespaces) == 0 {
			return d.tenantRepo.DeleteTenant()
		}
		return fmt.Errorf("cannot remove tenant %s from DB - some namespace still exists", namespaces[0].TenantID)
	}
	return d.commonNamespaceAction.checkNamespacesAndUpdateTenant(namespaces, envTypes)
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

func (u *Update) getNamespaceEntity(nsTypeService EnvironmentTypeService) (*tenant.Namespace, error) {
	return u.getNamespaceFor(nsTypeService.GetType()), nil
}

func (u *Update) updateNamespace(env *environment.EnvData, cluster *cluster.Cluster, namespace *tenant.Namespace, failed bool) {
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

func (d *Update) filter() FilterFunc {
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
