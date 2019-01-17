package update_test

import (
	"fmt"
	"github.com/fabric8-services/fabric8-common/convert/ptr"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/dbsupport"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/assertion"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/fabric8-services/fabric8-tenant/test/update"
	"github.com/fabric8-services/fabric8-tenant/update"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	"sync"
	"testing"
	"time"
)

type TenantsUpdaterTestSuite struct {
	gormsupport.DBTestSuite
}

func TestUpdaterService(t *testing.T) {
	suite.Run(t, &TenantsUpdaterTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *TenantsUpdaterTestSuite) TestUpdateAllTenantsForAllStatuses() {
	// given
	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	updateExecutor := testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration)
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0, update.AllTypes, "")
	defer reset()
	testdoubles.SetTemplateVersions()
	testdoubles.MockPatchRequestsToOS(ptr.Int(0), test.ClusterURL)

	for _, status := range []string{"finished", "updating", "failed", "killed", "incomplete"} {
		s.T().Run(fmt.Sprintf("running automated update process whould pass when status %s is set", status), func(t *testing.T) {
			*updateExecutor.NumberOfCalls = 0
			fxt := tf.FillDB(t, s.DB, tf.AddTenants(19), tf.AddDefaultNamespaces().State(tenant.Ready).Outdated())
			configuration.Commit = "124abcd"
			before := time.Now()

			s.tx(t, func(repo update.Repository) error {
				return repo.UpdateStatus(update.Status(status))
			})
			s.tx(t, func(repo update.Repository) error {
				return testupdate.UpdateVersionsTo(repo, "0xy")
			})

			// when
			tenantsUpdater.UpdateAllTenants()

			// then
			assert.Equal(t, 19, int(*updateExecutor.NumberOfCalls))
			s.assertStatusAndAllVersionAreUpToDate(t, update.Finished)
			for _, tnnt := range fxt.Tenants {
				assertion.AssertTenantFromDB(t, s.DB, tnnt.ID).
					HasNamespacesThat(func(assertion *assertion.NamespaceAssertion) {
						assertion.
							HasCurrentCompleteVersion().
							HasUpdatedBy("124abcd").
							HasState(tenant.Ready).
							WasUpdatedAfter(before)
					})
			}
		})
	}
}

func (s *TenantsUpdaterTestSuite) TestUpdateOnlyOutdatedNamespacesForAllStatuses() {
	// given
	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	updateExecutor := testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration)
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0, update.AllTypes, "")
	defer reset()
	testdoubles.SetTemplateVersions()

	for _, status := range []string{"finished", "updating", "failed", "killed", "incomplete"} {
		gock.OffAll()
		calls := 0
		testdoubles.MockPatchRequestsToOS(&calls, test.ClusterURL)
		s.T().Run(fmt.Sprintf("running automated update process whould pass when status %s is set", status), func(t *testing.T) {
			configuration.Commit = "xyz"
			*updateExecutor.NumberOfCalls = 0
			fxt := tf.FillDB(t, s.DB, tf.AddTenants(1),
				tf.AddNamespaces(environment.TypeChe).State(tenant.Ready).Outdated(),
				tf.AddNamespaces(environment.TypeJenkins).State(tenant.Failed),
				tf.AddNamespaces(environment.TypeRun, environment.TypeUser, environment.TypeStage).State(tenant.Ready))
			configuration.Commit = "124abcd"
			before := time.Now()

			s.tx(t, func(repo update.Repository) error {
				return repo.UpdateStatus(update.Status(status))
			})
			s.tx(t, func(repo update.Repository) error {
				return testupdate.UpdateVersionsTo(repo, "0xy")
			})

			// when
			tenantsUpdater.UpdateAllTenants()

			// then
			expectedNumberOfCalls := testdoubles.ExpectedNumberOfCallsWhenPatch(t, s.Configuration, environment.TypeChe, environment.TypeJenkins)
			assert.Equal(t, expectedNumberOfCalls, calls)
			s.assertStatusAndAllVersionAreUpToDate(t, update.Finished)
			for _, tnnt := range fxt.Tenants {
				namespaces, err := tenant.NewTenantRepository(s.DB, tnnt.ID).GetNamespaces()
				assert.NoError(t, err)
				for _, ns := range namespaces {
					nsAssertion := assertion.AssertNamespace(t, ns).
						HasCurrentCompleteVersion().
						HasState(tenant.Ready)
					if ns.Type == environment.TypeChe || ns.Type == environment.TypeJenkins {
						nsAssertion.
							HasUpdatedBy("124abcd").
							WasUpdatedAfter(before)
					} else {
						nsAssertion.
							HasUpdatedBy("xyz").
							WasUpdatedBefore(before)
					}
				}
			}
		})
	}
}

func (s *TenantsUpdaterTestSuite) TestHandleTenantUpdateError() {
	// given
	defer gock.OffAll()
	testdoubles.MockPatchRequestsToOS(ptr.Int(0), test.ClusterURL)

	s.tx(s.T(), func(repo update.Repository) error {
		return repo.UpdateStatus(update.Status("updating"))
	})

	// when
	update.HandleTenantUpdateError(s.DB, fmt.Errorf("any error"))

	// then
	var err error
	var tenantsUpdate *update.TenantsUpdate
	err = dbsupport.Transaction(s.DB, func(tx *gorm.DB) error {
		tenantsUpdate, err = update.NewRepository(tx).GetTenantsUpdate()
		return err
	})
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), string(update.Failed), string(tenantsUpdate.Status))
}

func (s *TenantsUpdaterTestSuite) TestDoNotUpdateAnythingWhenAllNamespacesAreUpToDateForAllStatuses() {
	// given
	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	updateExecutor := testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration)
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0, update.AllTypes, "")
	defer reset()
	testdoubles.SetTemplateVersions()
	configuration.Commit = "124abcd"

	for _, status := range []string{"finished", "updating", "failed", "killed", "incomplete"} {

		s.T().Run(fmt.Sprintf("running automated update process should pass (without updating anything) when status %s is set", status), func(t *testing.T) {
			*updateExecutor.NumberOfCalls = 0
			fxt := tf.FillDB(t, s.DB, tf.AddTenants(5), tf.AddDefaultNamespaces().State(tenant.Ready))
			after := time.Now()

			s.tx(t, func(repo update.Repository) error {
				return repo.UpdateStatus(update.Status(status))
			})
			s.tx(t, func(repo update.Repository) error {
				return testupdate.UpdateVersionsTo(repo, "")
			})

			// when
			tenantsUpdater.UpdateAllTenants()

			// then
			assert.Zero(t, *updateExecutor.NumberOfCalls)
			s.assertStatusAndAllVersionAreUpToDate(t, update.Finished)
			for _, tnnt := range fxt.Tenants {
				assertion.AssertTenantFromDB(t, s.DB, tnnt.ID).
					HasNamespacesThat(func(assertion *assertion.NamespaceAssertion) {
						assertion.
							HasCurrentCompleteVersion().
							HasUpdatedBy("124abcd").
							HasState(tenant.Ready).
							WasUpdatedBefore(after)
					})
			}
		})
	}
}

func (s *TenantsUpdaterTestSuite) TestWhenExecutorFailsThenStatusFailed() {
	// given
	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	gock.New(test.ClusterURL).
		Get("").
		Persist().
		Reply(200).
		BodyString(`{"status": {"phase":"Active"}}`)
	updateExecutor := testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration)
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0, update.AllTypes, "")
	defer reset()

	testdoubles.SetTemplateVersions()
	configuration.Commit = "124abcd"
	fxt := tf.FillDB(s.T(), s.DB, tf.AddTenants(1), tf.AddDefaultNamespaces().State(tenant.Ready).Outdated())
	s.tx(s.T(), func(repo update.Repository) error {
		return testupdate.UpdateVersionsTo(repo, "0")
	})

	configuration.Commit = "xyz"
	before := time.Now()

	// when
	tenantsUpdater.UpdateAllTenants()

	// then
	s.assertStatusAndAllVersionAreUpToDate(s.T(), update.Failed)
	for _, tnnt := range fxt.Tenants {
		assertion.AssertTenantFromDB(s.T(), s.DB, tnnt.ID).
			HasNamespacesThat(func(assertion *assertion.NamespaceAssertion) {
				assertion.
					HasCurrentCompleteVersion().
					HasUpdatedBy("xyz").
					HasState(tenant.Failed).
					WasUpdatedAfter(before)
			})
	}
}

func (s *TenantsUpdaterTestSuite) TestUpdateFilteredForSpecificCluster() {
	// given
	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth("http://api.cluster2")
	updateExecutor := testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration)
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0, update.AllTypes, "http://api.cluster2")
	defer reset()

	testdoubles.MockPatchRequestsToOS(ptr.Int(0), "http://api.cluster2/")
	testdoubles.SetTemplateVersions()
	configuration.Commit = "124abcd"
	fxt1 := tf.FillDB(s.T(), s.DB, tf.AddTenants(2),
		tf.AddDefaultNamespaces().State(tenant.Ready).MasterURL(test.ClusterURL).Outdated())
	fxt2 := tf.FillDB(s.T(), s.DB, tf.AddTenants(3),
		tf.AddDefaultNamespaces().State(tenant.Ready).MasterURL("http://api.cluster2").Outdated())
	s.tx(s.T(), func(repo update.Repository) error {
		return testupdate.UpdateVersionsTo(repo, "0")
	})

	configuration.Commit = "xyz"
	before := time.Now()

	// when
	tenantsUpdater.UpdateAllTenants()

	// then
	s.assertStatusAndAllVersionAreUpToDate(s.T(), update.Incomplete)
	for _, tnnt := range fxt1.Tenants {
		assertion.AssertTenantFromDB(s.T(), s.DB, tnnt.ID).
			HasNamespacesThat(func(assertion *assertion.NamespaceAssertion) {
				assertion.
					HasVersion("0000").
					HasUpdatedBy("124abcd").
					HasState(tenant.Ready).
					WasUpdatedBefore(before)
			})
	}
	for _, tnnt := range fxt2.Tenants {
		assertion.AssertTenantFromDB(s.T(), s.DB, tnnt.ID).
			HasNamespacesThat(func(assertion *assertion.NamespaceAssertion) {
				assertion.
					HasCurrentCompleteVersion().
					HasUpdatedBy("xyz").
					HasState(tenant.Ready).
					WasUpdatedAfter(before)
			})
	}
}

func (s *TenantsUpdaterTestSuite) TestUpdateFilteredForSpecificEnvType() {
	// given
	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	updateExecutor := testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration)
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0, update.OneType(environment.TypeJenkins), "")
	defer reset()
	testdoubles.MockPatchRequestsToOS(ptr.Int(0), test.ClusterURL)

	testdoubles.SetTemplateVersions()
	configuration.Commit = "124abcd"
	fxt1 := tf.FillDB(s.T(), s.DB, tf.AddTenants(5), tf.AddDefaultNamespaces().State(tenant.Ready).Outdated())
	s.tx(s.T(), func(repo update.Repository) error {
		return testupdate.UpdateVersionsTo(repo, "0")
	})

	configuration.Commit = "xyz"
	before := time.Now()

	// when
	tenantsUpdater.UpdateAllTenants()

	// then
	testupdate.AssertStatusAndAllVersionAreUpToDate(s.T(), s.DB, update.Incomplete, update.OneType(environment.TypeJenkins))
	for _, tnnt := range fxt1.Tenants {
		namespaces, err := tenant.NewTenantRepository(s.DB, tnnt.ID).GetNamespaces()
		assert.NoError(s.T(), err)
		for _, ns := range namespaces {
			nsAssertion := assertion.AssertNamespace(s.T(), ns).
				HasState(tenant.Ready)
			if ns.Type == environment.TypeJenkins {
				nsAssertion.
					HasUpdatedBy("xyz").
					HasCurrentCompleteVersion().
					WasUpdatedAfter(before)
			} else {
				nsAssertion.
					HasUpdatedBy("124abcd").
					HasVersion("0000").
					WasUpdatedBefore(before)
			}
		}
	}
}

func (s *TenantsUpdaterTestSuite) TestWhenStopIsCalledThenNothingIsUpdatedAndStatusIsKilled() {
	//given
	defer gock.OffAll()
	goroutinesCanContinue, goroutinesFinished, updateExecs := s.prepareForParallelTest(50, 1, 0, time.Second)
	testdoubles.MockPatchRequestsToOS(ptr.Int(0), test.ClusterURL)

	// when
	goroutinesCanContinue.Done()
	test.WaitWithTimeout(5 * time.Second).Until(func() error {
		var tenantsUpdate *update.TenantsUpdate
		s.tx(s.T(), func(repo update.Repository) error {
			var err error
			tenantsUpdate, err = repo.GetTenantsUpdate()
			return err
		})
		if tenantsUpdate.Status != update.Updating {
			return fmt.Errorf("updating process hasn't started")
		}
		return nil
	})
	s.tx(s.T(), func(repo update.Repository) error {
		return repo.Stop()
	})
	goroutinesFinished.Wait()

	// then
	var tenantsUpdate *update.TenantsUpdate
	err := dbsupport.Transaction(s.DB, func(tx *gorm.DB) error {
		var err error
		tenantsUpdate, err = update.NewRepository(tx).GetTenantsUpdate()
		return err
	})
	require.NoError(s.T(), err)
	assert.Equal(s.T(), update.Killed, tenantsUpdate.Status)
	assert.NotZero(s.T(), updateExecs[0].NumberOfCalls)
	assert.NotEqual(s.T(), 250, updateExecs[0].NumberOfCalls)
}

func (s *TenantsUpdaterTestSuite) TestMoreGoroutinesTryingToUpdate() {
	//given
	defer gock.OffAll()
	goroutinesCanContinue, goroutinesFinished, updateExecs := s.prepareForParallelTest(5, 10, 3*time.Second, 10*time.Millisecond)
	testdoubles.MockPatchRequestsToOS(ptr.Int(0), test.ClusterURL)

	// when
	goroutinesCanContinue.Done()
	goroutinesFinished.Wait()

	// then
	executorFound := false
	for _, exec := range updateExecs {
		if *exec.NumberOfCalls != 0 {
			assert.False(s.T(), executorFound)
			executorFound = true
			assert.Equal(s.T(), 5, int(*exec.NumberOfCalls))
		}
	}
	assert.True(s.T(), executorFound)
}

func (s *TenantsUpdaterTestSuite) TestTwoExecutorsDoUpdateBecauseOfLowerWaitTimeout() {
	//given
	defer gock.OffAll()
	goroutinesCanContinue, goroutinesFinished, updateExecs := s.prepareForParallelTest(5, 2, time.Millisecond, 10*time.Millisecond)
	testdoubles.MockPatchRequestsToOS(ptr.Int(0), test.ClusterURL)

	// when
	goroutinesCanContinue.Done()
	goroutinesFinished.Wait()

	// then
	assert.NotZero(s.T(), *updateExecs[0].NumberOfCalls)
	assert.NotZero(s.T(), *updateExecs[1].NumberOfCalls)
}

func (s *TenantsUpdaterTestSuite) prepareForParallelTest(numberOfTnnts, count int, timeToWait, timeToSleep time.Duration) (*sync.WaitGroup, *sync.WaitGroup, []*testupdate.DummyUpdateExecutor) {
	var goroutinesCanContinue sync.WaitGroup
	goroutinesCanContinue.Add(1)
	var goroutinesFinished sync.WaitGroup

	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	testdoubles.SetTemplateVersions()
	tf.FillDB(s.T(), s.DB, tf.AddTenants(numberOfTnnts), tf.AddDefaultNamespaces().State(tenant.Ready).Outdated())
	s.tx(s.T(), func(repo update.Repository) error {
		return testupdate.UpdateVersionsTo(repo, "0")
	})
	s.tx(s.T(), func(repo update.Repository) error {
		return repo.UpdateStatus(update.Status("finished"))
	})

	var updateExecs []*testupdate.DummyUpdateExecutor
	var toReset []func()
	defer func() {
		for _, reset := range toReset {
			reset()
		}
	}()

	for i := 0; i < count; i++ {
		goroutinesFinished.Add(1)
		updateExecutor := testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration)
		updateExecutor.TimeToSleep = timeToSleep
		updateExecs = append(updateExecs, updateExecutor)

		tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, timeToWait, update.AllTypes, "")
		toReset = append(toReset, reset)

		go func(updater *update.TenantsUpdater) {
			defer goroutinesFinished.Done()

			goroutinesCanContinue.Wait()
			updater.UpdateAllTenants()

		}(tenantsUpdater)
	}
	return &goroutinesCanContinue, &goroutinesFinished, updateExecs
}

func (s *TenantsUpdaterTestSuite) tx(t *testing.T, do func(repo update.Repository) error) {
	testupdate.Tx(t, s.DB, do)
}

func (s *TenantsUpdaterTestSuite) assertStatusAndAllVersionAreUpToDate(t *testing.T, st update.Status) {
	testupdate.AssertStatusAndAllVersionAreUpToDate(t, s.DB, st, update.AllTypes)
}

func (s *TenantsUpdaterTestSuite) newTenantsUpdater(updateExecutor *testupdate.DummyUpdateExecutor, timeout time.Duration,
	filterEnvType update.FilterEnvType, limitToCluster string) (*update.TenantsUpdater, func()) {
	reset := test.SetEnvironments(
		test.Env("F8_AUTH_TOKEN_KEY", "foo"),
		test.Env("F8_AUTOMATED_UPDATE_RETRY_SLEEP", timeout.String()),
		test.Env("F8_API_SERVER_USE_TLS", "false"),
		test.Env("F8_AUTOMATED_UPDATE_TIME_GAP", "0"))

	saToken, err := test.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	require.NoError(s.T(), err)
	authService, _, cleanup := testdoubles.NewAuthServiceWithRecorder(s.T(), "", "http://authservice", saToken.Raw)

	clusterService := cluster.NewClusterService(time.Hour, authService)
	err = clusterService.Start()
	require.NoError(s.T(), err)

	updateExecutor.ClusterService = clusterService
	config, reset := test.LoadTestConfig(s.T())
	return update.NewTenantsUpdater(s.DB, config, clusterService, updateExecutor, filterEnvType, limitToCluster), func() {
		cleanup()
		reset()
		clusterService.Stop()
	}
}
