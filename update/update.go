package update

import (
	"database/sql"
	"github.com/fabric8-services/fabric8-tenant/auth"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/sentry"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/jinzhu/gorm"
	"sync"
	"time"
)

func RetrieveAttrNameMapping() map[string]*VersionWithTypes {
	return map[string]*VersionWithTypes{
		"last_version_fabric8_tenant_user_file": vt(environment.VersionFabric8TenantUserFile, tenant.TypeUser),

		"last_version_fabric8_tenant_che_mt_file": vt(environment.VersionFabric8TenantCheMtFile, tenant.TypeChe),

		"last_version_fabric8_tenant_che_file": vt(environment.VersionFabric8TenantCheFile, tenant.TypeChe),

		"last_version_fabric8_tenant_che_quotas_file": vt(environment.VersionFabric8TenantCheQuotasFile, tenant.TypeChe),

		"last_version_fabric8_tenant_jenkins_file": vt(environment.VersionFabric8TenantJenkinsFile, tenant.TypeJenkins),

		"last_version_fabric8_tenant_jenkins_quotas_file": vt(environment.VersionFabric8TenantJenkinsQuotasFile, tenant.TypeJenkins),

		"last_version_fabric8_tenant_deploy_file": vt(environment.VersionFabric8TenantDeployFile, tenant.TypeRun, tenant.TypeStage),
	}
}

type VersionWithTypes struct {
	Version string
	envType []tenant.NamespaceType
}

func vt(version string, envTypes ...tenant.NamespaceType) *VersionWithTypes {
	return &VersionWithTypes{
		Version: version,
		envType: envTypes,
	}
}

type followUpFunc func() error

func NewTenantsUpdater(db *gorm.DB, config *configuration.Data, authService *auth.Service, clusterService cluster.Service, updateExecutor controller.UpdateExecutor) *TenantsUpdater {
	return &TenantsUpdater{
		db:             db,
		config:         config,
		authService:    authService,
		clusterService: clusterService,
		updateExecutor: updateExecutor,
	}
}

type TenantsUpdater struct {
	db             *gorm.DB
	config         *configuration.Data
	authService    *auth.Service
	clusterService cluster.Service
	updateExecutor controller.UpdateExecutor
}

func (u *TenantsUpdater) UpdateAllTenants() {

	var followUp followUpFunc = func() error { return nil }

	prepareAndAssignStart := func(repo Repository, envTypes []string) error {
		err := repo.PrepareForUpdating()
		if err != nil {
			return err
		}
		followUp = u.updateTenantsForTypes(envTypes)
		return nil
	}

	err := Transaction(u.db.DB(), lock(func(repo Repository) error {

		status, err := repo.GetStatus()
		if err != nil {
			return err
		}
		if status == Finished {
			envTypesToUpdate, err := checkVersions(repo)
			if err != nil {
				return err
			}
			if len(envTypesToUpdate) > 0 {
				return prepareAndAssignStart(repo, envTypesToUpdate)
			}

		} else if status == Failed {
			return prepareAndAssignStart(repo, environment.DefaultEnvTypes)

		} else if status == Updating {
			lastTimeUpdated, err := repo.GetLastTimeUpdated()
			if err != nil {
				return err
			}
			if u.isOlderThanTimeout(lastTimeUpdated) {
				return prepareAndAssignStart(repo, environment.DefaultEnvTypes)
			} else {
				followUp = u.waitAndRecheck
			}
		}

		return nil
	}))

	if err != nil {
		HandleTenantUpdateError(u.db.DB(), err)
		return
	}

	err = followUp()
	if err != nil {
		HandleTenantUpdateError(u.db.DB(), err)
	}
}

func HandleTenantUpdateError(db *sql.DB, err error) {
	sentry.LogError(nil, map[string]interface{}{
		"commit": controller.Commit,
		"err":    err,
	}, err, "automatic tenant update failed")
	err = Transaction(db, lock(func(repo Repository) error {
		return repo.UpdateStatus(Failed)
	}))
	if err != nil {
		sentry.LogError(nil, map[string]interface{}{
			"commit": controller.Commit,
			"err":    err,
		}, err, "unable to set state to failed in tenants_update table")
	}
}

func (u *TenantsUpdater) waitAndRecheck() error {
	time.Sleep(u.config.GetAutomatedUpdateRetrySleep() + u.config.GetAutomatedUpdateRetrySleep()/10)

	followUp := func() error { return nil }

	err := Transaction(u.db.DB(), lock(func(repo Repository) error {

		lastTimeUpdated, err := repo.GetLastTimeUpdated()
		if err != nil {
			return err
		}
		if u.isOlderThanTimeout(lastTimeUpdated) {
			err := repo.PrepareForUpdating()
			if err != nil {
				return err
			}
			followUp = u.updateTenantsForTypes(environment.DefaultEnvTypes)
		}
		return nil
	}))

	if err != nil {
		return err
	}

	return followUp()
}

func (u *TenantsUpdater) isOlderThanTimeout(when time.Time) bool {
	return when.Before(time.Now().Add(-u.config.GetAutomatedUpdateRetrySleep()))
}

func (u *TenantsUpdater) updateTenantsForTypes(envTypes []string) followUpFunc {
	return func() error {
		mappedTemplates := environment.RetrieveMappedTemplates()
		tenantRepo := tenant.NewDBService(u.db)

		typesWithVersion := map[string]string{}

		for _, envType := range envTypes {
			typesWithVersion[envType] = mappedTemplates[envType].ConstructCompleteVersion()
		}

		for {
			toUpdate, err := tenantRepo.GetTenantsToUpdate(typesWithVersion, 100, controller.Commit)
			if err != nil {
				return err
			}
			if len(toUpdate) == 0 {
				break
			}

			u.updateTenants(toUpdate, tenantRepo, envTypes)

			err = Transaction(u.db.DB(), lock(func(repo Repository) error {
				return repo.UpdateLastTimeUpdated()
			}))
			if err != nil {
				return err
			}
		}

		err := Transaction(u.db.DB(), lock(func(repo Repository) error {
			return u.setStatusAndVersionsAfterUpdate(repo)
		}))
		return err
	}
}

func (u *TenantsUpdater) setStatusAndVersionsAfterUpdate(repo Repository) error {
	failedCount, err := repo.GetFailedCount()
	if err != nil {
		return err
	}
	if failedCount > 0 {
		err = repo.UpdateStatus(Failed)
	} else {
		err = repo.UpdateStatus(Finished)
	}
	if err != nil {
		return err
	}
	return repo.UpdateVersionsTo(RetrieveAttrNameMapping())
}

func (u *TenantsUpdater) updateTenants(tenants []*tenant.Tenant, tenantRepo tenant.Service, envTypes []string) {
	numberOfTriggeredUpdates := 0
	var wg sync.WaitGroup

	for _, tnnt := range tenants {
		wg.Add(1)

		go updateTenant(&wg, tnnt, tenantRepo, envTypes, *u)

		numberOfTriggeredUpdates++
		if numberOfTriggeredUpdates == 10 {
			wg.Wait()
			numberOfTriggeredUpdates = 0
		}
	}
	wg.Wait()
}

func updateTenant(wg *sync.WaitGroup, tnnt *tenant.Tenant, tenantRepo tenant.Service, envTypes []string, updater TenantsUpdater) {
	defer wg.Done()

	namespaces, err := tenantRepo.GetNamespaces(tnnt.ID)
	if err != nil {
		sentry.LogError(nil, map[string]interface{}{
			"err":    err,
			"tenant": tnnt.ID,
		}, err, "unable to get tenant namespaces when doing automated update")
		return
	}

	for _, envType := range envTypes {
		var namespace *tenant.Namespace
		for _, ns := range namespaces {
			if string(ns.Type) == envType {
				namespace = ns
				break
			}
		}
		if namespace == nil {
			continue
		}

		userCluster, err := updater.clusterService.GetCluster(nil, namespace.MasterURL)
		if err != nil {
			sentry.LogError(nil, map[string]interface{}{
				"err":         err,
				"cluster_url": namespace.MasterURL,
				"tenant":      tnnt.ID,
				"env_type":    envType,
			}, err, "unable to fetch cluster when doing automated update")
			return
		}

		osConfig := openshift.NewConfig(updater.config, emptyTemplateRepoInfoSetter, userCluster.User, userCluster.Token, userCluster.APIURL)
		err = controller.UpdateTenant(updater.updateExecutor, nil, tenantRepo, osConfig, tnnt, envType)
		if err != nil {
			err = Transaction(updater.db.DB(), lock(func(repo Repository) error {
				return repo.IncrementFailedCount()
			}))
			if err != nil {
				sentry.LogError(nil, map[string]interface{}{}, err, "unable to increment failed_count")
			}
			sentry.LogError(nil, map[string]interface{}{
				"os_user":            tnnt.OSUsername,
				"tenant_id":          tnnt.ID,
				"env_type_to_update": envType,
				"namespace_name":     namespace.Name,
			}, err, "unable to automatically update tenant")
		}
	}
}

var emptyTemplateRepoInfoSetter openshift.TemplateRepoInfoSetter = func(config openshift.Config) openshift.Config {
	return config
}

func checkVersions(repo Repository) ([]string, error) {
	var types []string

	for attrName, versionWithTypes := range RetrieveAttrNameMapping() {
		isVersionSame, err := repo.IsVersionSame(attrName, versionWithTypes.Version)
		if err != nil {
			return nil, err
		}
		if !isVersionSame {
			addIfNotPresent(&types, versionWithTypes.envType)
		}
	}

	return types, nil
}

func addIfNotPresent(types *[]string, toAdd []tenant.NamespaceType) {
	for _, toAddType := range toAdd {
		found := false
		for _, envType := range *types {
			if envType == string(toAddType) {
				found = true
				break
			}
		}
		if !found {
			*types = append(*types, string(toAddType))
		}
	}
}
