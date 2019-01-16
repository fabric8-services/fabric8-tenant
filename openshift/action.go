package openshift

import (
	"fmt"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/utils"
	"github.com/pkg/errors"
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
	HealingStrategy() HealingFuncGenerator
	ManageAndUpdateResults(errorChan chan error, envTypes []environment.Type, healing Healing) error
}

type HealingFuncGenerator func(openShiftService *ServiceBuilder) Healing
type Healing func(originalError error) error

type commonNamespaceAction struct {
	method           string
	allowSelfHealing bool
	tenantRepo       tenant.Repository
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

var NoHealing = func(openShiftService *ServiceBuilder) Healing {
	return func(originalError error) error {
		return originalError
	}
}

func (c *commonNamespaceAction) HealingStrategy() HealingFuncGenerator {
	return NoHealing
}

func (c *commonNamespaceAction) ManageAndUpdateResults(errorChan chan error, envTypes []environment.Type, healing Healing) error {
	msg := utils.ListErrorsInMessage(errorChan)
	if len(msg) > 0 {
		err := fmt.Errorf("%s method applied to namespace types %s failed with one or more errors:%s", c.method, envTypes, msg)
		if !c.allowSelfHealing {
			return err
		}
		return healing(err)
	}
	return nil
}

func (c *Create) HealingStrategy() HealingFuncGenerator {
	return func(openShiftService *ServiceBuilder) Healing {
		return func(originalError error) error {
			log.Error(openShiftService.service.context.requestCtx, map[string]interface{}{
				"err":                   originalError,
				"self-healing-strategy": "recreate-with-new-nsBaseName",
			}, "the creation failed, starting self-healing logic")
			openShiftUsername := openShiftService.service.context.openShiftUsername
			tnnt, err := c.tenantRepo.GetTenant()
			errMsgSuffix := fmt.Sprintf("while doing self-healing operations triggered by error: [%s]", originalError)
			if err != nil {
				return errors.Wrapf(err, "unable to get tenant %s", errMsgSuffix)
			}
			namespaces, err := c.tenantRepo.GetNamespaces()
			fmt.Println(namespaces[0].ID)
			if err != nil {
				return errors.Wrapf(err, "unable to get namespaces of tenant %s %s", tnnt.ID, errMsgSuffix)
			}
			err = openShiftService.WithDeleteMethod(namespaces, true, true, true).ApplyAll(environment.DefaultEnvTypes)
			if err != nil {
				return errors.Wrapf(err, "deletion of namespaces failed %s", errMsgSuffix)
			}
			newNsBaseName, err := tenant.ConstructNsBaseName(c.tenantRepo.Service(), environment.RetrieveUserName(openShiftUsername))
			if err != nil {
				return errors.Wrapf(err, "unable to construct namespace base name for user with OSname %s %s", openShiftUsername, errMsgSuffix)
			}
			tnnt.NsBaseName = newNsBaseName
			err = c.tenantRepo.Service().SaveTenant(tnnt)
			if err != nil {
				return errors.Wrapf(err, "unable to update tenant db entity %s", errMsgSuffix)
			}
			openShiftService.service.context.nsBaseName = newNsBaseName
			err = openShiftService.WithPostMethod(false).ApplyAll(environment.DefaultEnvTypes)
			if err != nil {
				return errors.Wrapf(err, "unable to create new namespaces %s", errMsgSuffix)
			}
			return nil
		}
	}
}

func NewCreate(tenantRepo tenant.Repository, allowSelfHealing bool) *Create {
	return &Create{
		commonNamespaceAction: &commonNamespaceAction{
			method:           http.MethodPost,
			tenantRepo:       tenantRepo,
			allowSelfHealing: allowSelfHealing},
	}
}

type Create struct {
	*commonNamespaceAction
}

func (c *Create) GetNamespaceEntity(nsTypeService EnvironmentTypeService) (*tenant.Namespace, error) {
	namespace := c.tenantRepo.NewNamespace(
		nsTypeService.GetType(), nsTypeService.GetNamespaceName(), nsTypeService.GetCluster().APIURL, tenant.Provisioning)
	return c.tenantRepo.CreateNamespace(namespace)
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

func NewDelete(tenantRepo tenant.Repository, removeFromCluster, keepTenant, allowSelfHealing bool, existingNamespaces []*tenant.Namespace) *Delete {
	return &Delete{
		withExistingNamespacesAction: &withExistingNamespacesAction{
			commonNamespaceAction: &commonNamespaceAction{
				method:           http.MethodDelete,
				tenantRepo:       tenantRepo,
				allowSelfHealing: allowSelfHealing,
			},
			existingNamespaces: existingNamespaces,
		},
		removeFromCluster: removeFromCluster,
		keepTenant:        keepTenant,
	}
}

type Delete struct {
	*withExistingNamespacesAction
	removeFromCluster bool
	keepTenant        bool
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
	*commonNamespaceAction
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

func (d *Delete) ManageAndUpdateResults(errorChan chan error, envTypes []environment.Type, healing Healing) error {
	err := d.commonNamespaceAction.ManageAndUpdateResults(errorChan, envTypes, healing)
	if err != nil {
		return err
	}
	namespaces, err := d.tenantRepo.GetNamespaces()
	if err != nil {
		return err
	}
	if d.removeFromCluster {
		var names []string
		for _, ns := range namespaces {
			names = append(names, ns.Name)
		}
		if d.keepTenant {
			if len(namespaces) != 0 {
				return fmt.Errorf("all namespaces of the tenant %s weren't properly removed - some namespaces %s still exist", namespaces[0].TenantID, names)
			}
		} else {
			if len(namespaces) == 0 {
				return d.tenantRepo.DeleteTenant()
			}
			return fmt.Errorf("cannot remove tenant %s from DB - some namespaces %s still exist", namespaces[0].TenantID, names)
		}
	}
	return nil
}

func (d *Delete) HealingStrategy() HealingFuncGenerator {
	return d.redoStrategy(func(openShiftService *ServiceBuilder, existingNamespaces []*tenant.Namespace) *WithActionBuilder {
		return openShiftService.WithDeleteMethod(existingNamespaces, d.removeFromCluster, false, false)
	})
}

func NewUpdate(tenantRepo tenant.Repository, existingNamespaces []*tenant.Namespace, allowSelfHealing bool) *Update {
	return &Update{
		withExistingNamespacesAction: &withExistingNamespacesAction{
			commonNamespaceAction: &commonNamespaceAction{
				method:           http.MethodPatch,
				tenantRepo:       tenantRepo,
				allowSelfHealing: allowSelfHealing},
			existingNamespaces: existingNamespaces,
		},
		allowSelfHealing: allowSelfHealing,
	}
}

type Update struct {
	*withExistingNamespacesAction
	allowSelfHealing bool
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

func (u *Update) Filter() FilterFunc {
	return isNotOfKind(environment.ValKindProjectRequest)
}

func (u *Update) HealingStrategy() HealingFuncGenerator {
	return u.redoStrategy(func(openShiftService *ServiceBuilder, existingNamespaces []*tenant.Namespace) *WithActionBuilder {
		return openShiftService.WithPatchMethod(existingNamespaces, false)
	})
}

func (w *withExistingNamespacesAction) redoStrategy(
	toRedo func(openShiftService *ServiceBuilder, existingNamespaces []*tenant.Namespace) *WithActionBuilder) HealingFuncGenerator {

	return func(openShiftService *ServiceBuilder) Healing {
		return func(originalError error) error {
			errMsgSuffix := fmt.Sprintf("while doing self-healing operations triggered by error: [%s]", originalError)
			namespaces, err := w.tenantRepo.GetNamespaces()
			if err != nil {
				return errors.Wrapf(err, "unable to get namespaces %s", errMsgSuffix)
			}
			err = toRedo(openShiftService, namespaces).ApplyAll(environment.DefaultEnvTypes)
			if err != nil {
				return errors.Wrapf(err, "unable to redo the given action for the existing namespaces %s", errMsgSuffix)
			}
			return nil
		}
	}
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
