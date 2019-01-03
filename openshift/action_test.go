package openshift_test

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
)

type ActionTestSuite struct {
	gormsupport.DBTestSuite
}

func TestAction(t *testing.T) {
	suite.Run(t, &ActionTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *ActionTestSuite) TestCreateAction() {
	// given
	id := uuid.NewV4()
	tf.NewTestFixture(s.T(), s.DB, tf.Tenants(1, func(fxt *tf.TestFixture, idx int) error {
		fxt.Tenants[0].ID = id
		return nil
	}))
	repoService := tenant.NewDBService(s.DB)
	repo := repoService.NewTenantRepository(id)
	config, reset := test.LoadTestConfig(s.T())
	defer reset()

	// when
	create := openshift.NewCreate(repo)

	// then
	s.T().Run("method name should match", func(t *testing.T) {
		assert.Equal(t, "POST", create.MethodName())
	})

	s.T().Run("filter method should always return true", func(t *testing.T) {
		for _, obj := range getObjectsOfAllKinds() {
			assert.True(t, create.Filter()(obj))
		}
	})

	s.T().Run("sort method should sort the objects", func(t *testing.T) {
		// given
		toSort := getObjectsOfAllKinds()
		// when
		create.Sort(environment.ByKind(toSort))
		// then
		assert.Equal(t, environment.ValKindProjectRequest, environment.GetKind(toSort[0]))
		assert.Equal(t, environment.ValKindRole, environment.GetKind(toSort[1]))
		assert.Equal(t, environment.ValKindRoleBindingRestriction, environment.GetKind(toSort[2]))
	})

	s.T().Run("it should not require master token globally", func(t *testing.T) {
		assert.False(t, create.ForceMasterTokenGlobally())
	})

	for idx, envType := range environment.DefaultEnvTypes {
		// given
		envService, envData := gewEnvServiceWithData(s.T(), envType, config)
		// when
		namespace, err := create.GetNamespaceEntity(envService)
		// then
		assert.NoError(s.T(), err)
		s.T().Run("verify new namespace was created", func(t *testing.T) {
			assert.NotNil(t, getNs(t, repo, envType))
			assert.NotEmpty(t, namespace.ID)
			assert.Equal(t, envType, namespace.Type)
			assert.Equal(t, tenant.Provisioning, namespace.State)
			namespaces, err := repo.GetNamespaces()
			assert.NoError(t, err)
			assert.Len(t, namespaces, idx+1)
		})

		s.T().Run("update namespace to ready", func(t *testing.T) {
			// when
			create.UpdateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			ns := getNs(t, repo, envType)
			assert.NotNil(t, ns)
			assert.Equal(t, tenant.Ready, ns.State)
			assert.Equal(t, "my-cluster.com", ns.MasterURL)
			expName := "developer1"
			if envType != environment.TypeUser {
				expName += "-" + envType.String()
			}
			assert.Equal(t, expName, ns.Name)
		})

		s.T().Run("update namespace to failed", func(t *testing.T) {
			// when
			create.UpdateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, true)
			// then
			ns := getNs(t, repo, envType)
			assert.NotNil(t, ns)
			assert.Equal(t, tenant.Failed, ns.State)
			assert.Equal(t, "my-cluster.com", ns.MasterURL)
			expName := "developer1"
			if envType != environment.TypeUser {
				expName += "-" + envType.String()
			}
			assert.Equal(t, expName, ns.Name)
		})
	}

	s.T().Run("CheckNamespacesAndUpdateTenant should do nothing when err channel is empty", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		assert.NoError(t, create.CheckNamespacesAndUpdateTenant(errorChan, []environment.Type{environment.TypeChe, environment.TypeUser}))
		// then
		tnnt, err := repoService.GetTenant(id)
		assert.NoError(t, err)
		assert.NotNil(t, tnnt)
	})

	s.T().Run("CheckNamespacesAndUpdateTenant should return error when err channel is not empty", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		errorChan <- fmt.Errorf("first dummy error")
		errorChan <- fmt.Errorf("second dummy error")
		close(errorChan)
		// when
		err := create.CheckNamespacesAndUpdateTenant(errorChan, []environment.Type{environment.TypeChe, environment.TypeUser})
		// then
		test.AssertError(t, err, test.HasMessageContaining("POST method applied to namespace types [che user] failed with one or more errors"),
			test.HasMessageContaining("#1: first dummy error"),
			test.HasMessageContaining("#2: second dummy error"))
	})
}

func (s *ActionTestSuite) TestDeleteAction() {
	// given
	fxt := tf.FillDB(s.T(), s.DB, tf.AddTenants(1), true, tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))
	id := fxt.Tenants[0].ID
	repoService := tenant.NewDBService(s.DB)
	repo := repoService.NewTenantRepository(id)
	config, reset := test.LoadTestConfig(s.T())
	defer reset()

	// when
	delete := openshift.NewDelete(repo, false, fxt.Namespaces)
	deleteFromCluster := openshift.NewDelete(repo, true, fxt.Namespaces)

	// then
	s.T().Run("method name should match", func(t *testing.T) {
		assert.Equal(t, "DELETE", delete.MethodName())
	})

	s.T().Run("verify filter method", func(t *testing.T) {
		for _, obj := range getObjectsOfAllKinds() {
			if environment.GetKind(obj) == "ProjectRequest" {
				assert.False(t, delete.Filter()(obj), obj.ToString())
				assert.True(t, deleteFromCluster.Filter()(obj), obj.ToString())
			} else {
				assert.False(t, deleteFromCluster.Filter()(obj), obj.ToString())
				if environment.GetKind(obj) == "PersistentVolumeClaim" || environment.GetKind(obj) == "ConfigMap" ||
					environment.GetKind(obj) == "Service" || environment.GetKind(obj) == "DeploymentConfig" || environment.GetKind(obj) == "Route" {
					assert.True(t, delete.Filter()(obj), obj.ToString())
				} else {
					assert.False(t, delete.Filter()(obj), obj.ToString())
				}
			}
		}
	})

	s.T().Run("sort method should do reverse", func(t *testing.T) {
		// given
		toSort := getObjectsOfAllKinds()
		// when
		delete.Sort(environment.ByKind(toSort))
		// then
		length := len(toSort)
		assert.Equal(t, environment.ValKindProjectRequest, environment.GetKind(toSort[length-1]))
		assert.Equal(t, environment.ValKindRole, environment.GetKind(toSort[length-2]))
		assert.Equal(t, environment.ValKindRoleBindingRestriction, environment.GetKind(toSort[length-3]))
	})

	s.T().Run("it should require master token globally", func(t *testing.T) {
		assert.True(t, delete.ForceMasterTokenGlobally())
	})

	for _, envType := range environment.DefaultEnvTypes {
		// given
		envService, envData := gewEnvServiceWithData(s.T(), envType, config)

		// verify getting namespace - it should return only if exists
		namespace, err := delete.GetNamespaceEntity(envService)
		assert.NoError(s.T(), err)

		s.T().Run("verify new namespace is returned only if exists", func(t *testing.T) {
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

		s.T().Run("update namespace does nothing when ns is only cleaned", func(t *testing.T) {
			// when
			delete.UpdateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			nsToUpdate := getNs(t, repo, envType)
			assert.NotNil(t, nsToUpdate)
			assert.Equal(t, tenant.Ready, nsToUpdate.State)
			assert.Equal(t, "http://api.cluster1/", nsToUpdate.MasterURL)
		})

		s.T().Run("update namespace set state to failed", func(t *testing.T) {
			// when
			delete.UpdateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, true)
			// then
			ns := getNs(t, repo, envType)
			assert.NotNil(t, ns)
			assert.Equal(t, tenant.Failed, ns.State)
		})

		s.T().Run("update namespace deletes entity when it should be removed from cluster", func(t *testing.T) {
			// when
			deleteFromCluster.UpdateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			assert.Nil(t, getNs(t, repo, envType))
		})
	}

	s.T().Run("CheckNamespacesAndUpdateTenant should keep entity when one namespace is present", func(t *testing.T) {
		// given
		tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(func(tnnt *tenant.Tenant) {
			tnnt.ID = id
		}), true, tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))

		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		err := deleteFromCluster.CheckNamespacesAndUpdateTenant(errorChan, []environment.Type{environment.TypeChe})
		// then
		test.AssertError(t, err, test.HasMessageContaining("cannot remove tenant %s from DB - some namespaces", id))

	})

	s.T().Run("CheckNamespacesAndUpdateTenant should do nothing when namespace were only cleaned", func(t *testing.T) {
		// given
		tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(func(tnnt *tenant.Tenant) {
			tnnt.ID = id
		}), true, tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))
		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		err := delete.CheckNamespacesAndUpdateTenant(errorChan, []environment.Type{environment.TypeChe})
		// then
		assert.NoError(t, err)

	})

	s.T().Run("CheckNamespacesAndUpdateTenant should delete entity when no namespace is present", func(t *testing.T) {
		// given
		repo := tenant.NewDBService(s.DB).NewTenantRepository(id)
		namespaces, err := repo.GetNamespaces()
		require.NoError(t, err)
		for _, ns := range namespaces {
			require.NoError(t, repo.DeleteNamespace(ns))
		}
		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		err = deleteFromCluster.CheckNamespacesAndUpdateTenant(errorChan, []environment.Type{environment.TypeChe})
		// then
		assert.NoError(t, err)
		tnnt, err := repoService.GetTenant(id)
		test.AssertError(t, err, test.IsOfType(errors.NotFoundError{}))
		assert.Nil(t, tnnt)
	})
}

func (s *ActionTestSuite) TestUpdateAction() {
	// given
	id := uuid.NewV4()
	namespaces := []*tenant.Namespace{newNs("che", "updating", id), newNs("jenkins", "updating", id)}
	tf.NewTestFixture(s.T(), s.DB, tf.Tenants(1, func(fxt *tf.TestFixture, idx int) error {
		fxt.Tenants[0].ID = id
		return nil
	}), tf.Namespaces(len(namespaces), func(fxt *tf.TestFixture, idx int) error {
		fxt.Namespaces[idx] = namespaces[idx]
		return nil
	}))
	repoService := tenant.NewDBService(s.DB)
	repo := repoService.NewTenantRepository(id)
	config, reset := test.LoadTestConfig(s.T())
	defer reset()

	// when
	update := openshift.NewUpdate(repo, namespaces)

	// then
	s.T().Run("method name should match", func(t *testing.T) {
		assert.Equal(t, "PATCH", update.MethodName())
	})

	s.T().Run("filter method should always return true except for project request", func(t *testing.T) {
		for _, obj := range getObjectsOfAllKinds() {
			if environment.GetKind(obj) == "ProjectRequest" {
				assert.False(t, update.Filter()(obj))
			} else {
				assert.True(t, update.Filter()(obj))
			}

		}
	})

	s.T().Run("sort method should sort the objects", func(t *testing.T) {
		// given
		toSort := getObjectsOfAllKinds()
		// when
		update.Sort(environment.ByKind(toSort))
		// then
		assert.Equal(t, environment.ValKindProjectRequest, environment.GetKind(toSort[0]))
		assert.Equal(t, environment.ValKindRole, environment.GetKind(toSort[1]))
		assert.Equal(t, environment.ValKindRoleBindingRestriction, environment.GetKind(toSort[2]))
	})

	s.T().Run("it should require master token globally", func(t *testing.T) {
		assert.True(t, update.ForceMasterTokenGlobally())
	})

	for _, envType := range environment.DefaultEnvTypes {
		// given
		envService, envData := gewEnvServiceWithData(s.T(), envType, config)

		// verify getting namespace - it should return only if exists
		namespace, err := update.GetNamespaceEntity(envService)
		assert.NoError(s.T(), err)

		s.T().Run("verify new namespace is returned only if exists", func(t *testing.T) {
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
		s.T().Run("update namespace to ready", func(t *testing.T) {
			// when
			update.UpdateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			ns := getNs(t, repo, envType)
			assert.NotNil(t, ns)
			assert.Equal(t, tenant.Ready, ns.State)
			assert.Equal(t, "my-cluster.com", ns.MasterURL)
		})

		// verify namespace update to failed
		s.T().Run("update namespace to failed", func(t *testing.T) {
			// when
			update.UpdateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, true)
			// then
			ns := getNs(t, repo, envType)
			assert.NotNil(t, ns)
			assert.Equal(t, tenant.Failed, ns.State)
			assert.Equal(t, "my-cluster.com", ns.MasterURL)
		})
	}

	s.T().Run("CheckNamespacesAndUpdateTenant should do nothing when err channel is empty", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		assert.NoError(t, update.CheckNamespacesAndUpdateTenant(errorChan, []environment.Type{environment.TypeChe, environment.TypeUser}))
		// then
		tnnt, err := repoService.GetTenant(id)
		assert.NoError(t, err)
		assert.NotNil(t, tnnt)
	})

	s.T().Run("CheckNamespacesAndUpdateTenant should return error when err channel is not empty", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		errorChan <- fmt.Errorf("first dummy error")
		errorChan <- fmt.Errorf("second dummy error")
		close(errorChan)
		// when
		err := update.CheckNamespacesAndUpdateTenant(errorChan, []environment.Type{environment.TypeChe, environment.TypeUser})
		// then
		test.AssertError(t, err,
			test.HasMessageContaining("PATCH method applied to namespace types [che user] failed with one or more errors"),
			test.HasMessageContaining("#1: first dummy error"),
			test.HasMessageContaining("#2: second dummy error"))
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

func newOsContext(config *configuration.Data) *openshift.ServiceContext {
	clusterMapping := singleClusterMapping("http://starter.com", "clusterUser", "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8")

	return openshift.NewServiceContext(
		context.Background(), config, clusterMapping, "developer", "developer1", func(cluster cluster.Cluster) string {
			return "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8"
		})
}

func gewEnvServiceWithData(t *testing.T, envType environment.Type, config *configuration.Data) (openshift.EnvironmentTypeService, *environment.EnvData) {
	service := openshift.NewEnvironmentTypeService(envType, newOsContext(config), environment.NewService())
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
