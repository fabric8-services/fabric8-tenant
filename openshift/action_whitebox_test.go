package openshift

import (
	"testing"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"context"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-common/errors"
)

func TestCreateAction(t *testing.T) {
	// given
	id := uuid.NewV4()
	repoService, _ := gormsupport.NewDBServiceStub(&tenant.Tenant{ID: id}, []*tenant.Namespace{})
	repo := repoService.NewTenantRepository(id)
	config, reset := test.LoadTestConfig(t)
	defer reset()

	// when
	create := NewCreate(repo)

	// then
	t.Run("method name should match", func(t *testing.T) {
		assert.Equal(t, "POST", create.methodName())
	})

	t.Run("filter method should always return true", func(t *testing.T) {
		for _, obj := range getObjectsOfAllKinds() {
			assert.True(t, create.filter()(obj))
		}
	})

	t.Run("sort method should sort the objects", func(t *testing.T) {
		// given
		toSort := getObjectsOfAllKinds()
		// when
		create.sort(environment.ByKind(toSort))
		// then
		assert.Equal(t, environment.ValKindProjectRequest, environment.GetKind(toSort[0]))
		assert.Equal(t, environment.ValKindRole, environment.GetKind(toSort[1]))
		assert.Equal(t, environment.ValKindRoleBindingRestriction, environment.GetKind(toSort[2]))
	})

	t.Run("it should not require master token globally", func(t *testing.T) {
		assert.False(t, create.forceMasterTokenGlobally())
	})

	for idx, envType := range environment.DefaultEnvTypes {
		// given
		envService, envData := gewEnvServiceWithData(t, envType, config)
		// when
		namespace, err := create.getNamespaceEntity(envService)
		// then
		assert.NoError(t, err)
		t.Run("verify new namespace was created", func(t *testing.T) {
			assert.NotNil(t, getNs(t, repo, envType))
			assert.NotEmpty(t, namespace.ID)
			assert.Equal(t, envType, namespace.Type)
			assert.Equal(t, tenant.Provisioning, namespace.State)
			namespaces, err := repo.GetNamespaces()
			assert.NoError(t, err)
			assert.Len(t, namespaces, idx+1)
		})

		t.Run("update namespace to ready", func(t *testing.T) {
			// when
			create.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			ns := getNs(t, repo, envType)
			assert.NotNil(t, ns)
			assert.Equal(t, tenant.Ready, ns.State)
			assert.Equal(t, "my-cluster.com", ns.MasterURL)
			// todo name
		})

		t.Run("update namespace to failed", func(t *testing.T) {
			// when
			create.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, true)
			// then
			ns := getNs(t, repo, envType)
			assert.NotNil(t, ns)
			assert.Equal(t, tenant.Failed, ns.State)
			assert.Equal(t, "my-cluster.com", ns.MasterURL)
			// todo name
		})
	}

	t.Run("update tenant should do nothing", func(t *testing.T) {
		// when
		assert.NoError(t, create.updateTenant())
		// then
		tnnt, err := repoService.GetTenant(id)
		assert.NoError(t, err)
		assert.NotNil(t, tnnt)
	})
}

func TestDeleteAction(t *testing.T) {
	// given
	id := uuid.NewV4()

	namespaces := []*tenant.Namespace{newNs("che", "ready", id), newNs("jenkins", "ready", id)}
	repoService, _ := gormsupport.NewDBServiceStub(&tenant.Tenant{ID: id}, namespaces)
	repo := repoService.NewTenantRepository(id)
	config, reset := test.LoadTestConfig(t)
	defer reset()

	// when
	delete := NewDelete(repo, false, namespaces)
	deleteFromCluster := NewDelete(repo, true, namespaces)

	// then
	t.Run("method name should match", func(t *testing.T) {
		assert.Equal(t, "DELETE", delete.methodName())
	})

	t.Run("verify filter method", func(t *testing.T) {
		for _, obj := range getObjectsOfAllKinds() {
			if environment.GetKind(obj) == "ProjectRequest" {
				assert.False(t, delete.filter()(obj), obj.ToString())
				assert.True(t, deleteFromCluster.filter()(obj), obj.ToString())
			} else {
				assert.False(t, deleteFromCluster.filter()(obj), obj.ToString())
				if environment.GetKind(obj) == "PersistentVolumeClaim" || environment.GetKind(obj) == "ConfigMap" {
					assert.True(t, delete.filter()(obj), obj.ToString())
				} else {
					assert.False(t, delete.filter()(obj), obj.ToString())
				}
			}
		}
	})

	t.Run("sort method should do reverse", func(t *testing.T) {
		// given
		toSort := getObjectsOfAllKinds()
		// when
		delete.sort(environment.ByKind(toSort))
		// then
		length := len(toSort)
		assert.Equal(t, environment.ValKindProjectRequest, environment.GetKind(toSort[length-1]))
		assert.Equal(t, environment.ValKindRole, environment.GetKind(toSort[length-2]))
		assert.Equal(t, environment.ValKindRoleBindingRestriction, environment.GetKind(toSort[length-3]))
	})

	t.Run("it should require master token globally", func(t *testing.T) {
		assert.True(t, delete.forceMasterTokenGlobally())
	})

	for _, envType := range environment.DefaultEnvTypes {
		// given
		envService, envData := gewEnvServiceWithData(t, envType, config)

		// verify getting namespace - it should return only if exists
		namespace, err := delete.getNamespaceEntity(envService)
		assert.NoError(t, err)

		t.Run("verify new namespace is returned only if exists", func(t *testing.T) {
			if envType == environment.TypeChe || envType == environment.TypeJenkins {
				assert.NotEmpty(t, namespace.ID)
				assert.Equal(t, envType, namespace.Type)
				assert.Equal(t, tenant.Ready, namespace.State)
			} else {
				assert.Nil(t, namespace)
			}
		})
		if namespace == nil {
			continue
		}

		t.Run("update namespace does nothing when ns is only cleaned", func(t *testing.T) {
			// when
			delete.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			nsToUpdate := getNs(t, repo, envType)
			assert.NotNil(t, nsToUpdate)
			assert.Equal(t, tenant.Ready, nsToUpdate.State)
			assert.Equal(t, "cluster.com", nsToUpdate.MasterURL)
			// todo name
		})

		t.Run("update namespace set state to failed", func(t *testing.T) {
			// when
			delete.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, true)
			// then
			ns := getNs(t, repo, envType)
			assert.NotNil(t, ns)
			assert.Equal(t, tenant.Failed, ns.State)
		})

		t.Run("update namespace deletes entity when it should be removed from cluster", func(t *testing.T) {
			// when
			deleteFromCluster.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			assert.Nil(t, getNs(t, repo, envType))
		})
	}

	// given
	assert.NoError(t, repo.SaveNamespace(namespaces[0]))

	t.Run("should keep entity when one namespace is present", func(t *testing.T) {
		// when
		err := deleteFromCluster.updateTenant()
		// then
		test.AssertError(t, err, test.HasMessageContaining("cannot remove tenant %s from DB - some namespace still exist", id))

	})

	t.Run("should do nothing when namespace were only cleaned", func(t *testing.T) {
		// when
		err := delete.updateTenant()
		// then
		assert.NoError(t, err)

	})

	t.Run("should keep entity when one namespace is present", func(t *testing.T) {
		// given
		assert.NoError(t, repo.DeleteNamespace(namespaces[0]))
		// when
		err := deleteFromCluster.updateTenant()
		// then
		assert.NoError(t, err)
		tnnt, err := repoService.GetTenant(id)
		test.AssertError(t, err, test.IsOfType(errors.NotFoundError{}))
		assert.Nil(t, tnnt)
	})
}

func TestUpdateAction(t *testing.T) {
	// given
	id := uuid.NewV4()
	namespaces := []*tenant.Namespace{newNs("che", "updating", id), newNs("jenkins", "updating", id)}
	repoService, _ := gormsupport.NewDBServiceStub(&tenant.Tenant{ID: id}, namespaces)
	repo := repoService.NewTenantRepository(id)
	config, reset := test.LoadTestConfig(t)
	defer reset()

	// when
	update := NewUpdate(repo, namespaces)

	// then
	t.Run("method name should match", func(t *testing.T) {
		assert.Equal(t, "PATCH", update.methodName())
	})

	t.Run("filter method should always return true except for project request", func(t *testing.T) {
		for _, obj := range getObjectsOfAllKinds() {
			if environment.GetKind(obj) == "ProjectRequest" {
				assert.False(t, update.filter()(obj))
			} else {
				assert.True(t, update.filter()(obj))
			}

		}
	})

	t.Run("sort method should sort the objects", func(t *testing.T) {
		// given
		toSort := getObjectsOfAllKinds()
		// when
		update.sort(environment.ByKind(toSort))
		// then
		assert.Equal(t, environment.ValKindProjectRequest, environment.GetKind(toSort[0]))
		assert.Equal(t, environment.ValKindRole, environment.GetKind(toSort[1]))
		assert.Equal(t, environment.ValKindRoleBindingRestriction, environment.GetKind(toSort[2]))
	})

	t.Run("it should require master token globally", func(t *testing.T) {
		assert.True(t, update.forceMasterTokenGlobally())
	})

	for _, envType := range environment.DefaultEnvTypes {
		// given
		envService, envData := gewEnvServiceWithData(t, envType, config)

		// verify getting namespace - it should return only if exists
		namespace, err := update.getNamespaceEntity(envService)
		assert.NoError(t, err)

		t.Run("verify new namespace is returned only if exists", func(t *testing.T) {
			if envType == environment.TypeChe || envType == environment.TypeJenkins {
				assert.NotEmpty(t, namespace.ID)
				assert.Equal(t, envType, namespace.Type)
				assert.Equal(t, tenant.Updating, namespace.State)
				namespaces, err := repo.GetNamespaces()
				assert.NoError(t, err)
				assert.Len(t, namespaces, 2)
			} else {
				assert.Nil(t, namespace)
			}
		})
		if namespace == nil {
			continue
		}

		// verify namespace update to ready
		t.Run("update namespace to ready", func(t *testing.T) {
			// when
			update.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			ns := getNs(t, repo, envType)
			assert.NotNil(t, ns)
			assert.Equal(t, tenant.Ready, ns.State)
			assert.Equal(t, "my-cluster.com", ns.MasterURL)
			// todo name
		})

		// verify namespace update to failed
		t.Run("update namespace to failed", func(t *testing.T) {
			// when
			update.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, true)
			// then
			ns := getNs(t, repo, envType)
			assert.NotNil(t, ns)
			assert.Equal(t, tenant.Failed, ns.State)
			assert.Equal(t, "my-cluster.com", ns.MasterURL)
			// todo name
		})
	}

	t.Run("update tenant should do nothing", func(t *testing.T) {
		// when
		assert.NoError(t, update.updateTenant())
		// then
		tnnt, err := repoService.GetTenant(id)
		assert.NoError(t, err)
		assert.NotNil(t, tnnt)
	})
}

func newNs(envType environment.Type, state tenant.NamespaceState, tenantID uuid.UUID) *tenant.Namespace {
	return &tenant.Namespace{
		ID:        uuid.NewV4(),
		TenantID:  tenantID,
		MasterURL: "cluster.com",
		State:     state,
		Type:      envType,
	}
}

func getNs(t *testing.T, repo tenant.Repository, envType environment.Type) *tenant.Namespace {
	namespaces, err := repo.GetNamespaces()
	assert.NoError(t, err)
	for _, ns := range namespaces {
		if ns.Type == envType {
			return ns
		}
	}
	return nil
}

func newOsContext(config *configuration.Data) *ServiceContext {
	clusterMapping := singleClusterMapping("http://starter.com", "clusterUser", "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8")

	return NewServiceContext(context.Background(), config, clusterMapping, "developer", "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8")
}

func gewEnvServiceWithData(t *testing.T, envType environment.Type, config *configuration.Data) (EnvironmentTypeService, *environment.EnvData) {
	service := NewEnvironmentTypeService(envType, newOsContext(config), environment.NewService())
	data, _, err := service.GetEnvDataAndObjects(func(objects environment.Object) bool {
		return true
	})
	assert.NoError(t, err)
	return service, data
}

func singleClusterMapping(url, user, token string) cluster.ForType {
	return func(envType environment.Type) cluster.Cluster {
		return cluster.Cluster{
			APIURL: url,
			User:   user,
			Token:  token,
		}
	}
}

var allKinds = []string{environment.ValKindPersistenceVolumeClaim, environment.ValKindConfigMap,
	environment.ValKindLimitRange, environment.ValKindProject, environment.ValKindProjectRequest, environment.ValKindService,
	environment.ValKindSecret, environment.ValKindServiceAccount, environment.ValKindRoleBindingRestriction,
	environment.ValKindRoleBinding, environment.ValKindRole, environment.ValKindRoute, environment.ValKindJob,
	environment.ValKindList, environment.ValKindDeployment, environment.ValKindDeploymentConfig, environment.ValKindResourceQuota}

func getObjectsOfAllKinds() environment.Objects {
	var objects environment.Objects
	for _, kind := range allKinds {
		obj := map[interface{}]interface{}{"kind": kind}
		objects = append(objects, obj)
	}
	return objects
}
