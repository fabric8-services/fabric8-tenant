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
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/resource"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/jinzhu/gorm"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	"os"
	"testing"
)

type ActionTestSuite struct {
	gormsupport.DBTestSuite
}

func TestAction(t *testing.T) {
	os.Setenv(resource.Database, "1")
	suite.Run(t, &ActionTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

var emptyHealing = openshift.NoHealing(nil)
var returnErrHealing openshift.Healing = func(originalError error) error {
	return fmt.Errorf("healing error")
}

func (s *ActionTestSuite) TestCreateAction() {
	// given
	fxt := tf.FillDB(s.T(), s.DB, tf.AddTenants(1), tf.AddNamespaces())
	id := fxt.Tenants[0].ID
	repoService := tenant.NewDBService(s.DB)
	repo := repoService.NewTenantRepository(id)
	config, reset := test.LoadTestConfig(s.T())
	defer reset()

	// when
	create := openshift.NewCreate(repo, true)

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

	s.T().Run("ManageAndUpdateResults should do nothing when err channel is empty", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		assert.NoError(t, create.ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe, environment.TypeUser}, emptyHealing))
		// then
		tnnt, err := repoService.GetTenant(id)
		assert.NoError(t, err)
		assert.NotNil(t, tnnt)
	})

	s.T().Run("ManageAndUpdateResults should return error when err channel is not empty and healing is empty", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		errorChan <- fmt.Errorf("first dummy error")
		errorChan <- fmt.Errorf("second dummy error")
		close(errorChan)
		// when
		err := create.ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe, environment.TypeUser}, emptyHealing)
		// then
		test.AssertError(t, err, test.HasMessageContaining("POST method applied to namespace types [che user] failed with one or more errors"),
			test.HasMessageContaining("#1: first dummy error"),
			test.HasMessageContaining("#2: second dummy error"))
	})

	s.T().Run("HealingStrategy should return healing strategy that re-creates new namespaces (with new base name) when error is not nil", func(t *testing.T) {
		testRecreateHealingStrategy(t, s.DB, config, func(repo tenant.Repository, allowSelfHealing bool) openshift.NamespaceAction {
			return openshift.NewCreate(repo, allowSelfHealing)
		})
	})

	s.T().Run("when there was no error then it should not run healing", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		err := openshift.NewCreate(repo, true).ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe}, returnErrHealing)
		// then
		assert.NoError(t, err)
	})
}

func (s *ActionTestSuite) TestDeleteAction() {
	// given
	fxt := tf.FillDB(s.T(), s.DB, tf.AddTenants(1), tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))
	id := fxt.Tenants[0].ID
	repoService := tenant.NewDBService(s.DB)
	repo := repoService.NewTenantRepository(id)
	config, reset := test.LoadTestConfig(s.T())
	defer reset()

	// when
	delete := openshift.NewDelete(repo, false, true, fxt.Namespaces)
	deleteFromCluster := openshift.NewDelete(repo, true, false, fxt.Namespaces)

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

	s.T().Run("ManageAndUpdateResults should keep entity when one namespace is present", func(t *testing.T) {
		// given
		tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(func(tnnt *tenant.Tenant) {
			tnnt.ID = id
		}), tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))

		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		err := deleteFromCluster.ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe}, emptyHealing)
		// then
		test.AssertError(t, err, test.HasMessageContaining("cannot remove tenant %s from DB - some namespaces", id))

	})

	s.T().Run("ManageAndUpdateResults should do nothing when namespace were only cleaned", func(t *testing.T) {
		// given
		tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(func(tnnt *tenant.Tenant) {
			tnnt.ID = id
		}), tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))
		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		err := delete.ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe}, emptyHealing)
		// then
		assert.NoError(t, err)

	})

	s.T().Run("ManageAndUpdateResults should delete entity when no namespace is present", func(t *testing.T) {
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
		err = deleteFromCluster.ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe}, emptyHealing)
		// then
		assert.NoError(t, err)
		tnnt, err := repoService.GetTenant(id)
		test.AssertError(t, err, test.IsOfType(errors.NotFoundError{}))
		assert.Nil(t, tnnt)
	})

	s.T().Run("HealingStrategy should return healing strategy that does nothing and just return original error", func(t *testing.T) {
		originalErr := fmt.Errorf("some error")
		// when
		err := deleteFromCluster.HealingStrategy()(nil)(originalErr)
		// then
		assert.Equal(t, originalErr, err)
		// and also when
		err = delete.HealingStrategy()(nil)(originalErr)
		// then
		assert.Equal(t, originalErr, err)
	})

	s.T().Run("when there was no error then it should not run healing", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		err := deleteFromCluster.ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe}, returnErrHealing)
		// then
		assert.NoError(t, err)
		// and also when
		err = delete.ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe}, returnErrHealing)
		// then
		assert.NoError(t, err)
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
	update := openshift.NewUpdate(repo, namespaces, true)

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

	s.T().Run("ManageAndUpdateResults should do nothing when err channel is empty", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		assert.NoError(t, update.ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe, environment.TypeUser}, emptyHealing))
		// then
		tnnt, err := repoService.GetTenant(id)
		assert.NoError(t, err)
		assert.NotNil(t, tnnt)
	})

	s.T().Run("ManageAndUpdateResults should return error when err channel is not empty", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		errorChan <- fmt.Errorf("first dummy error")
		errorChan <- fmt.Errorf("second dummy error")
		close(errorChan)
		// when
		err := update.ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe, environment.TypeUser}, emptyHealing)
		// then
		test.AssertError(t, err,
			test.HasMessageContaining("PATCH method applied to namespace types [che user] failed with one or more errors"),
			test.HasMessageContaining("#1: first dummy error"),
			test.HasMessageContaining("#2: second dummy error"))
	})

	s.T().Run("HealingStrategy should return healing strategy that re-creates new namespaces (with new base name) when error is not nil", func(t *testing.T) {
		testRecreateHealingStrategy(t, s.DB, config, func(repo tenant.Repository, allowSelfHealing bool) openshift.NamespaceAction {
			return openshift.NewUpdate(repo, namespaces, allowSelfHealing)
		})
	})

	s.T().Run("when there was no error then it should not run healing", func(t *testing.T) {
		// given
		errorChan := make(chan error, 10)
		close(errorChan)
		// when
		err := openshift.NewUpdate(repo, namespaces,true).ManageAndUpdateResults(errorChan, []environment.Type{environment.TypeChe}, returnErrHealing)
		// then
		assert.NoError(t, err)
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

func gewEnvServiceWithData(t *testing.T, envType environment.Type, config *configuration.Data) (openshift.EnvironmentTypeService, *environment.EnvData) {
	osContext := openshift.NewServiceContext(
		context.Background(), config, testdoubles.DefaultClusterMapping, "developer", "developer1", func(cluster cluster.Cluster) string {
			return "HMs8laMmBSsJi8hpMDOtiglbXJ-2eyymE1X46ax5wX8"
		})
	service := openshift.NewEnvironmentTypeService(envType, osContext, environment.NewService())
	data, _, err := service.GetEnvDataAndObjects(func(objects environment.Object) bool {
		return true
	})
	assert.NoError(t, err)
	return service, data
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


func testRecreateHealingStrategy(t *testing.T, db *gorm.DB, config *configuration.Data,
	actionCreator func(repo tenant.Repository, allowSelfHealing bool) openshift.NamespaceAction) {

	t.Run("when there was an error, then should delete and create with basename developer2", func(t *testing.T) {
		// given
		fxt := tf.FillDB(t, db, tf.AddSpecificTenants(tf.SingleWithName("developer")), tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))
		id := fxt.Tenants[0].ID
		fmt.Println(id)
		repoService := tenant.NewDBService(db)
		repo := repoService.NewTenantRepository(id)

		defer gock.Off()
		deleteCalls := 0
		postCalls := 0
		testdoubles.MockPostRequestsToOS(&postCalls, "http://api.cluster1/")
		testdoubles.MockRemoveRequestsToOS(&deleteCalls, "http://api.cluster1/")
		userModifier := testdoubles.AddUser("developer")
		serviceBuilder := testdoubles.NewOSService(config, userModifier, repo)
		// when
		err := actionCreator(repo, true).HealingStrategy()(serviceBuilder)(fmt.Errorf("some error"))
		// then
		assert.NoError(t, err)
		assert.EqualValues(t, testdoubles.ExpectedNumberOfCallsWhenPost(t, config), postCalls)
		assert.EqualValues(t, 2, deleteCalls)
		tnnt, err := repo.GetTenant()
		assert.NoError(t, err)
		assert.Equal(t, "developer2", tnnt.NsBaseName)
	})

	t.Run("healing should not be executed when disabled", func(t *testing.T) {
		// given
		fxt := tf.FillDB(t, db, tf.AddSpecificTenants(tf.SingleWithName("developer")), tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))
		id := fxt.Tenants[0].ID
		fmt.Println(id)
		repoService := tenant.NewDBService(db)
		repo := repoService.NewTenantRepository(id)
		userModifier := testdoubles.AddUser("developer")
		serviceBuilder := testdoubles.NewOSService(config, userModifier, repo)
		// when
		err := actionCreator(repo, false).HealingStrategy()(serviceBuilder)(fmt.Errorf("some error"))
		// then
		test.AssertError(t, err, test.HasMessage("some error"))
		tnnt, err := repo.GetTenant()
		assert.NoError(t, err)
		assert.Equal(t, "developer", tnnt.NsBaseName)
	})

	t.Run("when there was an error and dev2 already exists then it should create dev3", func(t *testing.T) {
		// given
		fxt := tf.FillDB(t, db, tf.AddSpecificTenants(tf.SingleWithName("dev"), tf.SingleWithName("dev2")),
			tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))
		id := fxt.Tenants[0].ID
		fmt.Println(id)
		repoService := tenant.NewDBService(db)
		repo := repoService.NewTenantRepository(id)

		defer gock.Off()
		deleteCalls := 0
		postCalls := 0
		testdoubles.MockPostRequestsToOS(&postCalls, "http://api.cluster1/")
		testdoubles.MockRemoveRequestsToOS(&deleteCalls, "http://api.cluster1/")
		userModifier := testdoubles.AddUser("dev")
		serviceBuilder := testdoubles.NewOSService(config, userModifier, repo)
		// when
		err := actionCreator(repo, true).HealingStrategy()(serviceBuilder)(fmt.Errorf("some error"))
		// then
		assert.NoError(t, err)
		assert.EqualValues(t, testdoubles.ExpectedNumberOfCallsWhenPost(t, config), postCalls)
		assert.EqualValues(t, 2, deleteCalls)
		tnnt, err := repo.GetTenant()
		assert.NoError(t, err)
		assert.Equal(t, "dev3", tnnt.NsBaseName)
	})

	t.Run("when deletion fails then it should stop recreation and return an error", func(t *testing.T) {
		// given
		fxt := tf.FillDB(t, db, tf.AddSpecificTenants(tf.SingleWithName("developer")), tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))
		id := fxt.Tenants[0].ID
		repoService := tenant.NewDBService(db)
		repo := repoService.NewTenantRepository(id)

		defer gock.Off()
		deleteCalls := 0
		gock.New("http://api.cluster1/").
			Delete(".*/developer-jenkins.*").
			Reply(500).
			BodyString("{}")
		testdoubles.MockRemoveRequestsToOS(&deleteCalls, "http://api.cluster1/")
		userModifier := testdoubles.AddUser("developer")
		serviceBuilder := testdoubles.NewOSService(config, userModifier, repo)
		// when
		err := actionCreator(repo, true).HealingStrategy()(serviceBuilder)(fmt.Errorf("some error"))
		// then
		test.AssertError(t, err,
			test.HasMessageContaining("DELETE method applied to namespace types [che jenkins run stage user] failed"),
			test.HasMessageContaining("server responded with status: 500 for the DELETE request"),
			test.HasMessageContaining("while doing self-healing operations triggered by error: [some error]"))
		assert.EqualValues(t, 1, deleteCalls)
		tnnt, err := repo.GetTenant()
		assert.NoError(t, err)
		assert.Equal(t, "developer", tnnt.NsBaseName)
	})

	t.Run("when recreation fails then it should not do another one and return an error", func(t *testing.T) {
		// given
		fxt := tf.FillDB(t, db, tf.AddSpecificTenants(tf.SingleWithName("anotherdev")), tf.AddNamespaces(environment.TypeJenkins, environment.TypeChe))
		id := fxt.Tenants[0].ID
		repoService := tenant.NewDBService(db)
		repo := repoService.NewTenantRepository(id)

		defer gock.Off()
		deleteCalls := 0
		postCalls := 0
		gock.New("http://api.cluster1/").
			Post(".*/anotherdev-jenkins.*").
			Reply(500).
			BodyString("{}")
		testdoubles.MockPostRequestsToOS(&postCalls, "http://api.cluster1/")
		testdoubles.MockRemoveRequestsToOS(&deleteCalls, "http://api.cluster1/")
		userModifier := testdoubles.AddUser("anotherdev")
		serviceBuilder := testdoubles.NewOSService(config, userModifier, repo)
		// when
		err := actionCreator(repo, true).HealingStrategy()(serviceBuilder)(fmt.Errorf("some error"))
		// then
		test.AssertError(t, err,
			test.HasMessageContaining("POST method applied to namespace types [che jenkins run stage user] failed"),
			test.HasMessageContaining("server responded with status: 500 for the POST request"),
			test.HasMessageContaining("while doing self-healing operations triggered by error: [some error]"))
		assert.EqualValues(t, 2, deleteCalls)
		tnnt, err := repo.GetTenant()
		assert.NoError(t, err)
		assert.Equal(t, "anotherdev2", tnnt.NsBaseName)
	})
}