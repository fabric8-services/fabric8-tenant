package update_test

import (
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/dbsupport"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
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
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
	updateExecutor := testupdate.NewDummyUpdateExecutor()
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0, update.AllTypes, "")
	defer reset()
	testdoubles.SetTemplateVersions()

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
			assert.Equal(t, 95, int(*updateExecutor.NumberOfCalls))
			s.assertStatusAndAllVersionAreUpToDate(t, update.Finished)
			for _, tnnt := range fxt.Tenants {
				namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
				assert.NoError(t, err)
				for _, ns := range namespaces {
					assert.Equal(t, environment.RetrieveMappedTemplates()[ns.Type].ConstructCompleteVersion(), ns.Version)
					assert.Equal(t, "124abcd", ns.UpdatedBy)
					assert.Equal(t, tenant.Ready, ns.State)
					assert.True(t, before.Before(ns.UpdatedAt))
				}
			}
		})
	}
}

func (s *TenantsUpdaterTestSuite) TestUpdateOnlyOutdatedNamespacesForAllStatuses() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
	updateExecutor := testupdate.NewDummyUpdateExecutor()
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0, update.AllTypes, "")
	defer reset()
	testdoubles.SetTemplateVersions()

	for _, status := range []string{"finished", "updating", "failed", "killed", "incomplete"} {
		s.T().Run(fmt.Sprintf("running automated update process whould pass when status %s is set", status), func(t *testing.T) {
			configuration.Commit = "xyz"
			*updateExecutor.NumberOfCalls = 0
			fxt := tf.FillDB(t, s.DB, tf.AddTenants(5),
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
			assert.Equal(t, 10, int(*updateExecutor.NumberOfCalls))
			s.assertStatusAndAllVersionAreUpToDate(t, update.Finished)
			for _, tnnt := range fxt.Tenants {
				namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
				assert.NoError(t, err)
				for _, ns := range namespaces {
					assert.Equal(t, environment.RetrieveMappedTemplates()[ns.Type].ConstructCompleteVersion(), ns.Version)
					assert.Equal(t, tenant.Ready, ns.State)
					if ns.Type == environment.TypeChe || ns.Type == environment.TypeJenkins {
						assert.Equal(t, "124abcd", ns.UpdatedBy)
						assert.True(t, before.Before(ns.UpdatedAt))
					} else {
						assert.Equal(t, "xyz", ns.UpdatedBy)
						assert.True(t, before.After(ns.UpdatedAt))
					}
				}
			}
		})
	}
}

func (s *TenantsUpdaterTestSuite) TestHandleTenantUpdateError() {
	// given
	defer gock.Off()

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
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
	updateExecutor := testupdate.NewDummyUpdateExecutor()
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
				namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
				assert.NoError(t, err)
				for _, ns := range namespaces {
					assert.Equal(t, "124abcd", ns.UpdatedBy)
					assert.Equal(t, tenant.Ready, ns.State)
					assert.Equal(t, environment.RetrieveMappedTemplates()[ns.Type].ConstructCompleteVersion(), ns.Version)
					assert.True(t, after.After(ns.UpdatedAt))
				}
			}
		})
	}
}

func (s *TenantsUpdaterTestSuite) TestWhenExecutorFailsThenStatusFailed() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
	updateExecutor := testupdate.NewDummyUpdateExecutor()
	updateExecutor.ShouldFail = true
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
		namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
		assert.NoError(s.T(), err)
		for _, ns := range namespaces {
			assert.Equal(s.T(), "xyz", ns.UpdatedBy)
			assert.Equal(s.T(), tenant.Failed.String(), ns.State.String())
			assert.Equal(s.T(), "0000", ns.Version)
			assert.True(s.T(), before.Before(ns.UpdatedAt))
		}
	}
}

func (s *TenantsUpdaterTestSuite) TestUpdateFilteredForSpecificCluster() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1", "http://api.cluster2")
	updateExecutor := testupdate.NewDummyUpdateExecutor()
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0, update.AllTypes, "http://api.cluster2")
	defer reset()

	testdoubles.SetTemplateVersions()
	configuration.Commit = "124abcd"
	fxt1 := tf.FillDB(s.T(), s.DB, tf.AddTenants(2),
		tf.AddDefaultNamespaces().State(tenant.Ready).MasterURL("http://api.cluster1").Outdated())
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
		namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
		assert.NoError(s.T(), err)
		for _, ns := range namespaces {
			assert.Equal(s.T(), "124abcd", ns.UpdatedBy)
			assert.Equal(s.T(), tenant.Ready, ns.State)
			assert.Equal(s.T(), "0000", ns.Version)
			assert.True(s.T(), before.After(ns.UpdatedAt))
		}
	}
	for _, tnnt := range fxt2.Tenants {
		namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
		assert.NoError(s.T(), err)
		for _, ns := range namespaces {
			assert.Equal(s.T(), "xyz", ns.UpdatedBy)
			assert.Equal(s.T(), tenant.Ready, ns.State)
			assert.Equal(s.T(), environment.RetrieveMappedTemplates()[ns.Type].ConstructCompleteVersion(), ns.Version)
			assert.True(s.T(), before.Before(ns.UpdatedAt))
		}
	}
}

func (s *TenantsUpdaterTestSuite) TestUpdateFilteredForSpecificEnvType() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
	updateExecutor := testupdate.NewDummyUpdateExecutor()
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0, update.OneType(environment.TypeJenkins), "")
	defer reset()

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
		namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
		assert.NoError(s.T(), err)
		for _, ns := range namespaces {
			assert.Equal(s.T(), tenant.Ready, ns.State)
			if ns.Type == environment.TypeJenkins {
				assert.Equal(s.T(), "xyz", ns.UpdatedBy)
				assert.Equal(s.T(), environment.RetrieveMappedTemplates()[ns.Type].ConstructCompleteVersion(), ns.Version)
				assert.True(s.T(), before.Before(ns.UpdatedAt))
			} else {
				assert.Equal(s.T(), "124abcd", ns.UpdatedBy)
				assert.Equal(s.T(), "0000", ns.Version)
				assert.True(s.T(), before.After(ns.UpdatedAt))
			}
		}
	}
}

func (s *TenantsUpdaterTestSuite) TestWhenStopIsCalledThenNothingIsUpdatedAndStatusIsKilled() {
	//given
	goroutinesCanContinue, goroutinesFinished, updateExecs := s.prepareForParallelTest(50, 1, 0, time.Second)

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
	goroutinesCanContinue, goroutinesFinished, updateExecs := s.prepareForParallelTest(5, 10, 3*time.Second, 10*time.Millisecond)

	// when
	goroutinesCanContinue.Done()
	goroutinesFinished.Wait()

	// then
	executorFound := false
	for _, exec := range updateExecs {
		if *exec.NumberOfCalls != 0 {
			assert.False(s.T(), executorFound)
			executorFound = true
			assert.Equal(s.T(), uint64(25), *exec.NumberOfCalls)
		}
	}
	assert.True(s.T(), executorFound)
}

func (s *TenantsUpdaterTestSuite) TestTwoExecutorsDoUpdateBecauseOfLowerWaitTimeout() {
	//given
	goroutinesCanContinue, goroutinesFinished, updateExecs := s.prepareForParallelTest(5, 2, time.Millisecond, 10*time.Millisecond)

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

	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
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
		updateExecutor := testupdate.NewDummyUpdateExecutor()
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

func (s *TenantsUpdaterTestSuite) newTenantsUpdater(updateExecutor openshift.UpdateExecutor, timeout time.Duration,
	filterEnvType update.FilterEnvType, limitToCluster string) (*update.TenantsUpdater, func()) {
	reset := test.SetEnvironments(
		test.Env("F8_AUTH_TOKEN_KEY", "foo"),
		test.Env("F8_AUTOMATED_UPDATE_RETRY_SLEEP", timeout.String()),
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
	config, reset := test.LoadTestConfig(s.T())
	return update.NewTenantsUpdater(s.DB, config, clusterService, updateExecutor, filterEnvType, limitToCluster), func() {
		cleanup()
		reset()
		clusterService.Stop()
	}
}
