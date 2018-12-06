package openshift

import (
	"context"
	"github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"testing"
)

type ActionWhiteboxTestSuite struct {
	gormsupport.DBTestSuite
}

func TestActionWhitebox(t *testing.T) {
	suite.Run(t, &ActionWhiteboxTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *ActionWhiteboxTestSuite) TestCreateAction() {
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
	create := NewCreate(repo)

	// then
	s.T().Run("method name should match", func(t *testing.T) {
		assert.Equal(s.T(), "POST", create.methodName())
	})

	s.T().Run("filter method should always return true", func(t *testing.T) {
		for _, obj := range getObjectsOfAllKinds() {
			assert.True(s.T(), create.filter()(obj))
		}
	})

	s.T().Run("sort method should sort the objects", func(t *testing.T) {
		// given
		toSort := getObjectsOfAllKinds()
		// when
		create.sort(environment.ByKind(toSort))
		// then
		assert.Equal(s.T(), environment.ValKindProjectRequest, environment.GetKind(toSort[0]))
		assert.Equal(s.T(), environment.ValKindRole, environment.GetKind(toSort[1]))
		assert.Equal(s.T(), environment.ValKindRoleBindingRestriction, environment.GetKind(toSort[2]))
	})

	s.T().Run("it should not require master token globally", func(t *testing.T) {
		assert.False(s.T(), create.forceMasterTokenGlobally())
	})

	for idx, envType := range environment.DefaultEnvTypes {
		// given
		envService, envData := gewEnvServiceWithData(s.T(), envType, config)
		// when
		namespace, err := create.getNamespaceEntity(envService)
		// then
		assert.NoError(s.T(), err)
		s.T().Run("verify new namespace was created", func(t *testing.T) {
			assert.NotNil(s.T(), getNs(s.T(), repo, envType))
			assert.NotEmpty(s.T(), namespace.ID)
			assert.Equal(s.T(), envType, namespace.Type)
			assert.Equal(s.T(), tenant.Provisioning, namespace.State)
			namespaces, err := repo.GetNamespaces()
			assert.NoError(s.T(), err)
			assert.Len(s.T(), namespaces, idx+1)
		})

		s.T().Run("update namespace to ready", func(t *testing.T) {
			// when
			create.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			ns := getNs(s.T(), repo, envType)
			assert.NotNil(s.T(), ns)
			assert.Equal(s.T(), tenant.Ready, ns.State)
			assert.Equal(s.T(), "my-cluster.com", ns.MasterURL)
			expName := "developer1"
			if envType != environment.TypeUser {
				expName += "-" + envType.String()
			}
			assert.Equal(s.T(), expName, ns.Name)
		})

		s.T().Run("update namespace to failed", func(t *testing.T) {
			// when
			create.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, true)
			// then
			ns := getNs(s.T(), repo, envType)
			assert.NotNil(s.T(), ns)
			assert.Equal(s.T(), tenant.Failed, ns.State)
			assert.Equal(s.T(), "my-cluster.com", ns.MasterURL)
			expName := "developer1"
			if envType != environment.TypeUser {
				expName += "-" + envType.String()
			}
			assert.Equal(s.T(), expName, ns.Name)
		})
	}

	s.T().Run("checkNamespacesAndUpdateTenant should do nothing when namespaces are ready", func(t *testing.T) {
		// when
		namespaces := []*tenant.Namespace{{State: tenant.Ready, Type: environment.TypeChe}, {State: tenant.Ready, Type: environment.TypeUser}}
		assert.NoError(s.T(), create.checkNamespacesAndUpdateTenant(namespaces, []environment.Type{environment.TypeChe, environment.TypeUser}))
		// then
		tnnt, err := repoService.GetTenant(id)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), tnnt)
	})

	s.T().Run("checkNamespacesAndUpdateTenant should return error when one namespace is failed", func(t *testing.T) {
		// when
		namespaces := []*tenant.Namespace{
			{Name: "johny-che", State: tenant.Ready, Type: environment.TypeChe},
			{Name: "johny", State: tenant.Failed, Type: environment.TypeUser}}
		err := create.checkNamespacesAndUpdateTenant(namespaces, []environment.Type{environment.TypeChe, environment.TypeUser})
		// then
		test.AssertError(s.T(), err, test.HasMessageContaining("applying POST action on namespaces [johny] failed"))
	})
}

func (s *ActionWhiteboxTestSuite) TestDeleteAction() {
	// given
	id := uuid.NewV4()

	namespaces := []*tenant.Namespace{newNs("che", "ready", id), newNs("jenkins", "ready", id)}
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
	delete := NewDelete(repo, false, namespaces)
	deleteFromCluster := NewDelete(repo, true, namespaces)

	// then
	s.T().Run("method name should match", func(t *testing.T) {
		assert.Equal(s.T(), "DELETE", delete.methodName())
	})

	s.T().Run("verify filter method", func(t *testing.T) {
		for _, obj := range getObjectsOfAllKinds() {
			if environment.GetKind(obj) == "ProjectRequest" {
				assert.False(s.T(), delete.filter()(obj), obj.ToString())
				assert.True(s.T(), deleteFromCluster.filter()(obj), obj.ToString())
			} else {
				assert.False(s.T(), deleteFromCluster.filter()(obj), obj.ToString())
				if environment.GetKind(obj) == "PersistentVolumeClaim" || environment.GetKind(obj) == "ConfigMap" {
					assert.True(s.T(), delete.filter()(obj), obj.ToString())
				} else {
					assert.False(s.T(), delete.filter()(obj), obj.ToString())
				}
			}
		}
	})

	s.T().Run("sort method should do reverse", func(t *testing.T) {
		// given
		toSort := getObjectsOfAllKinds()
		// when
		delete.sort(environment.ByKind(toSort))
		// then
		length := len(toSort)
		assert.Equal(s.T(), environment.ValKindProjectRequest, environment.GetKind(toSort[length-1]))
		assert.Equal(s.T(), environment.ValKindRole, environment.GetKind(toSort[length-2]))
		assert.Equal(s.T(), environment.ValKindRoleBindingRestriction, environment.GetKind(toSort[length-3]))
	})

	s.T().Run("it should require master token globally", func(t *testing.T) {
		assert.True(s.T(), delete.forceMasterTokenGlobally())
	})

	for _, envType := range environment.DefaultEnvTypes {
		// given
		envService, envData := gewEnvServiceWithData(s.T(), envType, config)

		// verify getting namespace - it should return only if exists
		namespace, err := delete.getNamespaceEntity(envService)
		assert.NoError(s.T(), err)

		s.T().Run("verify new namespace is returned only if exists", func(t *testing.T) {
			if envType == environment.TypeChe || envType == environment.TypeJenkins {
				assert.NotEmpty(s.T(), namespace.ID)
				assert.Equal(s.T(), envType, namespace.Type)
				assert.Equal(s.T(), tenant.Ready, namespace.State)
			} else {
				assert.Nil(s.T(), namespace)
			}
		})
		if namespace == nil {
			continue
		}

		s.T().Run("update namespace does nothing when ns is only cleaned", func(t *testing.T) {
			// when
			delete.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			nsToUpdate := getNs(s.T(), repo, envType)
			assert.NotNil(s.T(), nsToUpdate)
			assert.Equal(s.T(), tenant.Ready, nsToUpdate.State)
			assert.Equal(s.T(), "cluster.com", nsToUpdate.MasterURL)
		})

		s.T().Run("update namespace set state to failed", func(t *testing.T) {
			// when
			delete.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, true)
			// then
			ns := getNs(s.T(), repo, envType)
			assert.NotNil(s.T(), ns)
			assert.Equal(s.T(), tenant.Failed, ns.State)
		})

		s.T().Run("update namespace deletes entity when it should be removed from cluster", func(t *testing.T) {
			// when
			deleteFromCluster.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			assert.Nil(s.T(), getNs(s.T(), repo, envType))
		})
	}

	s.T().Run("checkNamespacesAndUpdateTenant should keep entity when one namespace is present", func(t *testing.T) {
		// when
		err := deleteFromCluster.checkNamespacesAndUpdateTenant([]*tenant.Namespace{namespaces[0]}, []environment.Type{environment.TypeChe})
		// then
		test.AssertError(s.T(), err, test.HasMessageContaining("cannot remove tenant %s from DB - some namespace still exist", id))

	})

	s.T().Run("checkNamespacesAndUpdateTenant should do nothing when namespace were only cleaned", func(t *testing.T) {
		// when
		err := delete.checkNamespacesAndUpdateTenant([]*tenant.Namespace{namespaces[0]}, []environment.Type{environment.TypeChe})
		// then
		assert.NoError(s.T(), err)

	})

	s.T().Run("checkNamespacesAndUpdateTenant should delete entity when no namespace is present", func(t *testing.T) {
		// when
		err := deleteFromCluster.checkNamespacesAndUpdateTenant([]*tenant.Namespace{}, []environment.Type{environment.TypeChe})
		// then
		assert.NoError(s.T(), err)
		tnnt, err := repoService.GetTenant(id)
		test.AssertError(s.T(), err, test.IsOfType(errors.NotFoundError{}))
		assert.Nil(s.T(), tnnt)
	})
}

func (s *ActionWhiteboxTestSuite) TestUpdateAction() {
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
	update := NewUpdate(repo, namespaces)

	// then
	s.T().Run("method name should match", func(t *testing.T) {
		assert.Equal(s.T(), "PATCH", update.methodName())
	})

	s.T().Run("filter method should always return true except for project request", func(t *testing.T) {
		for _, obj := range getObjectsOfAllKinds() {
			if environment.GetKind(obj) == "ProjectRequest" {
				assert.False(s.T(), update.filter()(obj))
			} else {
				assert.True(s.T(), update.filter()(obj))
			}

		}
	})

	s.T().Run("sort method should sort the objects", func(t *testing.T) {
		// given
		toSort := getObjectsOfAllKinds()
		// when
		update.sort(environment.ByKind(toSort))
		// then
		assert.Equal(s.T(), environment.ValKindProjectRequest, environment.GetKind(toSort[0]))
		assert.Equal(s.T(), environment.ValKindRole, environment.GetKind(toSort[1]))
		assert.Equal(s.T(), environment.ValKindRoleBindingRestriction, environment.GetKind(toSort[2]))
	})

	s.T().Run("it should require master token globally", func(t *testing.T) {
		assert.True(s.T(), update.forceMasterTokenGlobally())
	})

	for _, envType := range environment.DefaultEnvTypes {
		// given
		envService, envData := gewEnvServiceWithData(s.T(), envType, config)

		// verify getting namespace - it should return only if exists
		namespace, err := update.getNamespaceEntity(envService)
		assert.NoError(s.T(), err)

		s.T().Run("verify new namespace is returned only if exists", func(t *testing.T) {
			if envType == environment.TypeChe || envType == environment.TypeJenkins {
				assert.NotEmpty(s.T(), namespace.ID)
				assert.Equal(s.T(), envType, namespace.Type)
				assert.Equal(s.T(), tenant.Updating, namespace.State)
				namespaces, err := repo.GetNamespaces()
				assert.NoError(s.T(), err)
				assert.Len(s.T(), namespaces, 2)
			} else {
				assert.Nil(s.T(), namespace)
			}
		})
		if namespace == nil {
			continue
		}

		// verify namespace update to ready
		s.T().Run("update namespace to ready", func(t *testing.T) {
			// when
			update.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, false)
			// then
			ns := getNs(s.T(), repo, envType)
			assert.NotNil(s.T(), ns)
			assert.Equal(s.T(), tenant.Ready, ns.State)
			assert.Equal(s.T(), "my-cluster.com", ns.MasterURL)
		})

		// verify namespace update to failed
		s.T().Run("update namespace to failed", func(t *testing.T) {
			// when
			update.updateNamespace(envData, &cluster.Cluster{APIURL: "my-cluster.com"}, namespace, true)
			// then
			ns := getNs(s.T(), repo, envType)
			assert.NotNil(s.T(), ns)
			assert.Equal(s.T(), tenant.Failed, ns.State)
			assert.Equal(s.T(), "my-cluster.com", ns.MasterURL)
		})
	}

	s.T().Run("checkNamespacesAndUpdateTenant should do nothing when namespaces are ready", func(t *testing.T) {
		// when
		namespaces := []*tenant.Namespace{{State: tenant.Ready, Type: environment.TypeChe}, {State: tenant.Ready, Type: environment.TypeUser}}
		assert.NoError(s.T(), update.checkNamespacesAndUpdateTenant(namespaces, []environment.Type{environment.TypeChe, environment.TypeUser}))
		// then
		tnnt, err := repoService.GetTenant(id)
		assert.NoError(s.T(), err)
		assert.NotNil(s.T(), tnnt)
	})

	s.T().Run("checkNamespacesAndUpdateTenant should return error when botch namespaces are failed", func(t *testing.T) {
		// when
		namespaces := []*tenant.Namespace{
			{Name: "johny-che", State: tenant.Failed, Type: environment.TypeChe},
			{Name: "johny", State: tenant.Failed, Type: environment.TypeUser}}
		err := update.checkNamespacesAndUpdateTenant(namespaces, []environment.Type{environment.TypeChe, environment.TypeUser})
		// then
		test.AssertError(s.T(), err, test.HasMessageContaining("applying PATCH action on namespaces [johny-che, johny] failed"))
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

	return NewServiceContext(
		context.Background(), config, clusterMapping, "developer", "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8", "developer1")
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
