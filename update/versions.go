package update

import (
	"github.com/fabric8-services/fabric8-tenant/environment"
)

func RetrieveVersionManagers() []*VersionManager {
	return []*VersionManager{
		versionManager(environment.VersionFabric8TenantUserFile, "fabric8-tenant-user.yml",
			func(tu *TenantsUpdate) string {
				return tu.LastVersionFabric8TenantUserFile
			}, func(tu *TenantsUpdate, version string) {
				tu.LastVersionFabric8TenantUserFile = version
			}, environment.TypeUser),

		versionManager(environment.VersionFabric8TenantCheMtFile, "fabric8-tenant-che-mt.yml",
			func(tu *TenantsUpdate) string {
				return tu.LastVersionFabric8TenantCheMtFile
			}, func(tu *TenantsUpdate, version string) {
				tu.LastVersionFabric8TenantCheMtFile = version
			}, environment.TypeChe),

		versionManager(environment.VersionFabric8TenantCheQuotasFile, "fabric8-tenant-che-quotas.yml",
			func(tu *TenantsUpdate) string {
				return tu.LastVersionFabric8TenantCheQuotasFile
			}, func(tu *TenantsUpdate, version string) {
				tu.LastVersionFabric8TenantCheQuotasFile = version
			}, environment.TypeChe),
	}
}

type VersionManager struct {
	Version           string
	EnvTypes          []environment.Type
	FileName          string
	GetStoredVersion  func(tu *TenantsUpdate) string
	setCurrentVersion func(tu *TenantsUpdate, versionToSet string)
}

func versionManager(
	version string,
	fileName string,
	getStoredVersion func(tu *TenantsUpdate) string,
	setCurrentVersion func(tu *TenantsUpdate, version string),
	envTypes ...environment.Type) *VersionManager {

	return &VersionManager{
		Version:           version,
		FileName:          fileName,
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
