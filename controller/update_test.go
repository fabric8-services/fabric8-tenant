package controller_test

import (
	"context"
	"fmt"
	"github.com/fabric8-services/fabric8-common/convert/ptr"
	goatest "github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/fabric8-services/fabric8-tenant/test/update"
	"github.com/fabric8-services/fabric8-tenant/update"
	"github.com/goadesign/goa"
	"github.com/jinzhu/gorm"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	"testing"
	"time"
)

type UpdateControllerTestSuite struct {
	gormsupport.DBTestSuite
}

func TestUpdateController(t *testing.T) {
	suite.Run(t, &UpdateControllerTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *UpdateControllerTestSuite) TestStartUpdateFailures() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
	svc, ctrl, reset := s.newUpdateController(testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration), 9*time.Minute)
	defer reset()

	s.T().Run("Unauhorized - no token", func(t *testing.T) {
		// when/then
		goatest.StartUpdateUnauthorized(t, context.Background(), svc, ctrl, nil, nil)
	})

	s.T().Run("Unauhorized - no SA token", func(t *testing.T) {
		// when/then
		goatest.StartUpdateUnauthorized(t, createInvalidSAContext(), svc, ctrl, nil, nil)
	})

	s.T().Run("Unauhorized - wrong SA token", func(t *testing.T) {
		// when/then
		goatest.StartUpdateUnauthorized(t, createValidSAContext("other service account"), svc, ctrl, nil, nil)
	})

	s.T().Run("Not found", func(t *testing.T) {
		// when/then
		goatest.StartUpdateUnauthorized(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl, nil, nil)
	})

	s.T().Run("Bad parameter", func(t *testing.T) {
		// expect
		defer func() {
			if r := recover(); r == nil {
				t.Errorf("The code did not panic because of wrong parameter")
			}
		}()
		// when
		goatest.StartUpdateBadRequest(t, createValidSAContext("fabric8-tenant-update"), svc, ctrl, nil, ptr.String("wrong"))
	})

	s.T().Run("Conflict", func(t *testing.T) {
		// expect
		testupdate.Tx(t, s.DB, func(repo update.Repository) error {
			return repo.PrepareForUpdating()
		})
		// when
		_, msg := goatest.StartUpdateConflict(t, createValidSAContext("fabric8-tenant-update"), svc, ctrl, nil, nil)
		require.NotNil(t, msg)
		assert.Contains(t, *msg.Data.ConflictMsg, "There is an ongoing update with the last updated timestamp")
		assert.Contains(t, *msg.Data.ConflictMsg, "9m")
	})
}

func (s *UpdateControllerTestSuite) TestStartUpdateOk() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1", "http://api.cluster2")
	updateExecutor := testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration)
	svc, ctrl, reset := s.newUpdateController(updateExecutor, 0)
	defer reset()
	testdoubles.SetTemplateVersions()

	s.T().Run("without parameter", func(t *testing.T) {
		testdoubles.MockPatchRequestsToOS(ptr.Int(0), "http://api.cluster1/")
		testdoubles.MockPatchRequestsToOS(ptr.Int(0), "http://api.cluster2/")
		fxt1 := tf.FillDB(t, s.DB, tf.AddTenants(6), false, tf.AddDefaultNamespaces().State(tenant.Ready).MasterURL("http://api.cluster1"))
		fxt2 := tf.FillDB(t, s.DB, tf.AddTenants(6), false, tf.AddDefaultNamespaces().State(tenant.Ready).MasterURL("http://api.cluster2"))
		configuration.Commit = "124abcd"
		before := time.Now()

		testupdate.Tx(t, s.DB, func(repo update.Repository) error {
			if err := testupdate.UpdateVersionsTo(repo, "0xy"); err != nil {
				return err
			}
			return repo.UpdateStatus(update.Status(update.Finished))
		})

		// when
		goatest.StartUpdateAccepted(t, createValidSAContext("fabric8-tenant-update"), svc, ctrl, nil, nil)

		// then
		err := test.WaitWithTimeout(10 * time.Second).Until(func() error {
			if int(*updateExecutor.NumberOfCalls) != 12 {
				return fmt.Errorf("expeced number of calls 60 wasn't fullfiled - actual: %d", int(*updateExecutor.NumberOfCalls))
			}
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 12, int(*updateExecutor.NumberOfCalls))
		testupdate.AssertStatusAndAllVersionAreUpToDate(t, s.DB, update.Finished, update.AllTypes)
		for _, tnnts := range [][]*tenant.Tenant{fxt1.Tenants, fxt2.Tenants} {
			for _, tnnt := range tnnts {
				namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
				assert.NoError(t, err)
				for _, ns := range namespaces {
					assert.Equal(t, environment.RetrieveMappedTemplates()[ns.Type].ConstructCompleteVersion(), ns.Version)
					assert.Equal(t, "124abcd", ns.UpdatedBy)
					assert.Equal(t, tenant.Ready.String(), ns.State.String())
					assert.True(t, before.Before(ns.UpdatedAt))
				}
			}
		}
	})

	s.T().Run("with parameters", func(t *testing.T) {
		testdoubles.MockPatchRequestsToOS(ptr.Int(0), "http://api.cluster1/")
		updateExecutor.NumberOfCalls = ptr.Uint64(0)
		fxt1 := tf.FillDB(t, s.DB, tf.AddTenants(6), false, tf.AddDefaultNamespaces().State(tenant.Ready).MasterURL("http://api.cluster1/"))
		fxt2 := tf.FillDB(t, s.DB, tf.AddTenants(6), false, tf.AddDefaultNamespaces().State(tenant.Ready).MasterURL("http://api.cluster2/"))
		configuration.Commit = "xyz"
		before := time.Now()

		testupdate.Tx(t, s.DB, func(repo update.Repository) error {
			return repo.UpdateStatus(update.Status(update.Failed))
		})

		// when
		goatest.StartUpdateAccepted(t, createValidSAContext("fabric8-tenant-update"), svc, ctrl, ptr.String("http://api.cluster1/"), ptr.String("jenkins"))

		// then
		err := test.WaitWithTimeout(10 * time.Second).Until(func() error {
			if int(*updateExecutor.NumberOfCalls) != 6 {
				return fmt.Errorf("expeced number of calls 6 wasn't fullfiled - actual: %d", int(*updateExecutor.NumberOfCalls))
			}
			return nil
		})
		assert.NoError(t, err)
		assert.Equal(t, 6, int(*updateExecutor.NumberOfCalls))
		testupdate.AssertStatusAndAllVersionAreUpToDate(t, s.DB, update.Incomplete, update.AllTypes)
		for _, tnnts := range [][]*tenant.Tenant{fxt1.Tenants, fxt2.Tenants} {
			for _, tnnt := range tnnts {
				namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
				assert.NoError(t, err)
				for _, ns := range namespaces {
					assert.Equal(t, tenant.Ready.String(), ns.State.String())
					if ns.MasterURL == "http://api.cluster1/" && ns.Type == environment.TypeJenkins {
						assert.Equal(t, environment.RetrieveMappedTemplates()[ns.Type].ConstructCompleteVersion(), ns.Version)
						assert.Equal(t, "xyz", ns.UpdatedBy)
						assert.True(t, before.Before(ns.UpdatedAt))
					} else {
						assert.Equal(t, "0000", ns.Version)
						assert.Equal(t, "124abcd", ns.UpdatedBy)
						assert.True(t, before.After(ns.UpdatedAt))
					}
				}
			}
		}
	})
}

func (s *UpdateControllerTestSuite) TestShowUpdateFailures() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
	svc, ctrl, reset := s.newUpdateController(testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration), 0)
	defer reset()

	s.T().Run("Unauhorized - no token", func(t *testing.T) {
		// when/then
		goatest.ShowUpdateUnauthorized(t, context.Background(), svc, ctrl)
	})

	s.T().Run("Unauhorized - no SA token", func(t *testing.T) {
		// when/then
		goatest.ShowUpdateUnauthorized(t, createInvalidSAContext(), svc, ctrl)
	})

	s.T().Run("Unauhorized - wrong SA token", func(t *testing.T) {
		// when/then
		goatest.ShowUpdateUnauthorized(t, createValidSAContext("other service account"), svc, ctrl)
	})

	s.T().Run("Not found", func(t *testing.T) {
		// when/then
		goatest.ShowUpdateUnauthorized(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl)
	})
}

func (s *UpdateControllerTestSuite) TestShowUpdateOk() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
	svc, ctrl, reset := s.newUpdateController(testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration), 0)
	defer reset()
	testdoubles.SetTemplateVersions()
	versionManagers := update.RetrieveVersionManagers()

	for _, status := range []string{"finished", "updating", "failed", "killed", "incomplete"} {
		s.T().Run("with status "+status, func(t *testing.T) {
			// given
			testupdate.Tx(t, s.DB, func(repo update.Repository) error {
				tenantsUpdate, err := repo.GetTenantsUpdate()
				if err != nil {
					return err
				}
				for _, versionManager := range versionManagers {
					versionManager.SetCurrentVersion(tenantsUpdate)
				}
				tenantsUpdate.Status = update.Status(status)
				tenantsUpdate.FailedCount = 10
				return repo.SaveTenantsUpdate(tenantsUpdate)
			})
			after := time.Now()

			// when
			_, updateData := goatest.ShowUpdateOK(t, createValidSAContext("fabric8-tenant-update"), svc, ctrl)

			// then
			assert.Equal(t, status, *updateData.Data.Status)
			assert.Equal(t, 10, *updateData.Data.FailedCount)
			assert.True(t, after.After(*updateData.Data.LastTimeUpdated))
			assert.Len(t, updateData.Data.FileVersions, len(versionManagers))

			for _, fileVersion := range updateData.Data.FileVersions {
				found := false
				for _, vm := range versionManagers {
					if vm.FileName == *fileVersion.FileName {
						found = true
						assert.Equal(t, vm.Version, *fileVersion.Version)
						break
					}
				}
				assert.True(t, found)
			}
		})
	}
}

func (s *UpdateControllerTestSuite) TestStopUpdateFailures() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
	svc, ctrl, reset := s.newUpdateController(testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration), 0)
	defer reset()

	s.T().Run("Unauhorized - no token", func(t *testing.T) {
		// when/then
		goatest.StopUpdateUnauthorized(t, context.Background(), svc, ctrl)
	})

	s.T().Run("Unauhorized - no SA token", func(t *testing.T) {
		// when/then
		goatest.StopUpdateUnauthorized(t, createInvalidSAContext(), svc, ctrl)
	})

	s.T().Run("Unauhorized - wrong SA token", func(t *testing.T) {
		// when/then
		goatest.StopUpdateUnauthorized(t, createValidSAContext("other service account"), svc, ctrl)
	})

	s.T().Run("Not found", func(t *testing.T) {
		// when/then
		goatest.StopUpdateUnauthorized(t, createValidSAContext("fabric8-jenkins-idler"), svc, ctrl)
	})
}

func (s *UpdateControllerTestSuite) TestStopUpdateOk() {
	// given
	defer gock.Off()
	testdoubles.MockCommunicationWithAuth("http://api.cluster1")
	updateExecutor := testupdate.NewDummyUpdateExecutor(s.DB, s.Configuration)
	updateExecutor.TimeToSleep = time.Second
	svc, ctrl, reset := s.newUpdateController(updateExecutor, 0)
	defer reset()
	testdoubles.SetTemplateVersions()

	tf.FillDB(s.T(), s.DB, tf.AddTenants(50), false, tf.AddDefaultNamespaces().State(tenant.Ready))
	configuration.Commit = "124abcd"

	testupdate.Tx(s.T(), s.DB, func(repo update.Repository) error {
		if err := testupdate.UpdateVersionsTo(repo, "0xy"); err != nil {
			return err
		}
		return repo.UpdateStatus(update.Status(update.Finished))
	})

	// when
	goatest.StartUpdateAccepted(s.T(), createValidSAContext("fabric8-tenant-update"), svc, ctrl, nil, nil)

	// then
	test.WaitWithTimeout(5 * time.Second).Until(func() error {
		var tenantsUpdate *update.TenantsUpdate
		testupdate.Tx(s.T(), s.DB, func(repo update.Repository) error {
			var err error
			tenantsUpdate, err = repo.GetTenantsUpdate()
			return err
		})
		if tenantsUpdate.Status != update.Updating {
			return fmt.Errorf("updating process hasn't started")
		}
		return nil
	})
	testupdate.Tx(s.T(), s.DB, func(repo update.Repository) error {
		return repo.Stop()
	})

	var tenantsUpdate *update.TenantsUpdate
	err := test.WaitWithTimeout(10 * time.Second).Until(func() error {
		err := update.Transaction(s.DB, func(tx *gorm.DB) error {
			var err error
			tenantsUpdate, err = update.NewRepository(tx).GetTenantsUpdate()
			return err
		})
		if err != nil {
			return err
		}
		if update.Killed != tenantsUpdate.Status {
			return fmt.Errorf("the status wasn't changed to killed")
		}
		return nil
	})
	assert.NoError(s.T(), err)
	assert.Equal(s.T(), update.Killed.String(), tenantsUpdate.Status.String())
	assert.NotZero(s.T(), updateExecutor.NumberOfCalls)
	assert.NotEqual(s.T(), 250, updateExecutor.NumberOfCalls)
}

func (s *UpdateControllerTestSuite) newUpdateController(executor *testupdate.DummyUpdateExecutor, timeout time.Duration) (*goa.Service, *controller.UpdateController, func()) {
	resetEnvs := test.SetEnvironments(
		test.Env("F8_AUTH_TOKEN_KEY", "foo"),
		test.Env("F8_AUTOMATED_UPDATE_RETRY_SLEEP", timeout.String()),
		test.Env("F8_API_SERVER_USE_TLS", "false"))
	clusterService, _, config, reset := prepareConfigClusterAndAuthService(s.T())
	svc := goa.New("Tenants-service")
	executor.ClusterService = clusterService
	return svc, controller.NewUpdateController(svc, s.DB, config, clusterService, executor), func() {
		resetEnvs()
		reset()
	}
}
