package update

import (
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
)

func RetrieveVersionManagers() []*VersionManager {
	return []*VersionManager{
		versionManager(environment.VersionFabric8TenantUserFile,
			func(tu *TenantsUpdate) string {
				return tu.LastVersionFabric8TenantUserFile
			}, func(tu *TenantsUpdate, version string) {
				tu.LastVersionFabric8TenantUserFile = version
			}, tenant.TypeUser),

		versionManager(environment.VersionFabric8TenantCheMtFile,
			func(tu *TenantsUpdate) string {
				return tu.LastVersionFabric8TenantCheMtFile
			}, func(tu *TenantsUpdate, version string) {
				tu.LastVersionFabric8TenantCheMtFile = version
			}, tenant.TypeChe),

		versionManager(environment.VersionFabric8TenantCheQuotasFile,
			func(tu *TenantsUpdate) string {
				return tu.LastVersionFabric8TenantCheQuotasFile
			}, func(tu *TenantsUpdate, version string) {
				tu.LastVersionFabric8TenantCheQuotasFile = version
			}, tenant.TypeChe),

		versionManager(environment.VersionFabric8TenantJenkinsFile,
			func(tu *TenantsUpdate) string {
				return tu.LastVersionFabric8TenantJenkinsFile
			}, func(tu *TenantsUpdate, version string) {
				tu.LastVersionFabric8TenantJenkinsFile = version
			}, tenant.TypeJenkins),

		versionManager(environment.VersionFabric8TenantJenkinsQuotasFile,
			func(tu *TenantsUpdate) string {
				return tu.LastVersionFabric8TenantJenkinsQuotasFile
			}, func(tu *TenantsUpdate, version string) {
				tu.LastVersionFabric8TenantJenkinsQuotasFile = version
			}, tenant.TypeJenkins),

		versionManager(environment.VersionFabric8TenantDeployFile,
			func(tu *TenantsUpdate) string {
				return tu.LastVersionFabric8TenantDeployFile
			}, func(tu *TenantsUpdate, version string) {
				tu.LastVersionFabric8TenantDeployFile = version
			}, tenant.TypeStage, tenant.TypeRun),
	}
}

type VersionManager struct {
	Version           string
	EnvTypes          []tenant.NamespaceType
	GetStoredVersion  func(tu *TenantsUpdate) string
	setCurrentVersion func(tu *TenantsUpdate, versionToSet string)
}

func versionManager(version string, getStoredVersion func(tu *TenantsUpdate) string, setCurrentVersion func(tu *TenantsUpdate, version string), envTypes ...tenant.NamespaceType) *VersionManager {
	return &VersionManager{
		Version:           version,
		EnvTypes:          envTypes,
		GetStoredVersion:  getStoredVersion,
		setCurrentVersion: setCurrentVersion,
	}
}

func (vm *VersionManager) IsVersionUpToDate(tu *TenantsUpdate) bool {
	return vm.Version == vm.GetStoredVersion(tu)
}

func (vm *VersionManager) SetCurrentVersion(tu *TenantsUpdate) {
	vm.setCurrentVersion(tu, vm.Version)
}
