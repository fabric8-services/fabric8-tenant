package update_test

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/resource"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/fabric8-services/fabric8-tenant/update"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

type TenantsUpdaterTestSuite struct {
	gormsupport.DBTestSuite
}

func TestTenantService(t *testing.T) {
	resource.Require(t, resource.Database)
	suite.Run(t, &TenantsUpdaterTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *TenantsUpdaterTestSuite) TestUpdateAllTenantsForAllStatuses() {
	// given
	defer gock.Off()
	createMocks()
	updateExecutor := &DummyUpdateExecutor{numberOfCalls: Uint64(0)}
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0)
	defer reset()
	testdoubles.SetTemplateVersions()

	for _, status := range []string{"finished", "updating", "failed"} {
		s.T().Run(fmt.Sprintf("running automated update process whould pass when status %s is set", status), func(t *testing.T) {
			*updateExecutor.numberOfCalls = 0
			fxt := tf.FillDB(t, s.DB, 19, false, "ready", environment.DefaultEnvTypes...)
			controller.Commit = "124abcd"
			before := time.Now()

			s.tx(t, func(repo update.Repository) error {
				return repo.UpdateStatus(update.Status(status))
			})
			s.tx(t, func(repo update.Repository) error {
				return repo.UpdateVersionsTo(retrieveMappingWithVersion("0xy"))
			})

			// when
			err := tenantsUpdater.UpdateAllTenants()

			// then
			assert.NoError(t, err)
			assert.Equal(t, uint64(95), *updateExecutor.numberOfCalls)
			s.assertStatus(t, "finished")
			s.assertAllVersionAreUpToDate(t)
			for _, tnnt := range fxt.Tenants {
				namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
				assert.NoError(t, err)
				for _, ns := range namespaces {
					assert.Equal(t, environment.RetrieveMappedTemplates()[string(ns.Type)].ConstructCompleteVersion(), ns.Version)
					assert.Equal(t, "124abcd", ns.UpdatedBy)
					assert.Equal(t, "ready", ns.State)
					assert.True(t, before.Before(ns.UpdatedAt))
				}
			}
		})
	}
}

func (s *TenantsUpdaterTestSuite) TestDoNotUpdateAnythingWhenAllNamespacesAreUpToDateForAllStatuses() {
	// given
	defer gock.Off()
	createMocks()
	updateExecutor := &DummyUpdateExecutor{numberOfCalls: Uint64(0)}
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0)
	defer reset()
	testdoubles.SetTemplateVersions()
	controller.Commit = "124abcd"

	for _, status := range []string{"finished", "updating", "failed"} {

		s.T().Run(fmt.Sprintf("running automated update process should pass (without updating anything) when status %s is set", status), func(t *testing.T) {
			*updateExecutor.numberOfCalls = 0
			fxt := tf.FillDB(t, s.DB, 5, true, "ready", environment.DefaultEnvTypes...)
			//controller.Commit = "xyz"
			after := time.Now()

			s.tx(t, func(repo update.Repository) error {
				return repo.UpdateStatus(update.Status(status))
			})
			s.tx(t, func(repo update.Repository) error {
				return repo.UpdateVersionsTo(update.RetrieveAttrNameMapping())
			})

			// when
			err := tenantsUpdater.UpdateAllTenants()

			// then
			assert.NoError(t, err)
			assert.Zero(t, *updateExecutor.numberOfCalls)
			s.assertStatus(t, update.Finished)
			s.assertAllVersionAreUpToDate(t)
			for _, tnnt := range fxt.Tenants {
				namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
				assert.NoError(t, err)
				for _, ns := range namespaces {
					assert.Equal(t, "124abcd", ns.UpdatedBy)
					assert.Equal(t, "ready", ns.State)
					assert.Equal(t, environment.RetrieveMappedTemplates()[string(ns.Type)].ConstructCompleteVersion(), ns.Version)
					assert.True(t, after.After(ns.UpdatedAt))
				}
			}
		})
	}
}

func (s *TenantsUpdaterTestSuite) TestWhenExecutorFailsThenStatusFailed() {
	// given
	defer gock.Off()
	createMocks()
	updateExecutor := &DummyUpdateExecutor{shouldFail: true, numberOfCalls: Uint64(0)}
	tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, 0)
	defer reset()

	testdoubles.SetTemplateVersions()
	controller.Commit = "124abcd"
	fxt := tf.FillDB(s.T(), s.DB, 1, false, "ready", environment.DefaultEnvTypes...)
	s.tx(s.T(), func(repo update.Repository) error {
		return repo.UpdateVersionsTo(retrieveMappingWithVersion("0"))
	})

	controller.Commit = "xyz"
	before := time.Now()

	// when
	err := tenantsUpdater.UpdateAllTenants()

	// then
	assert.NoError(s.T(), err)
	s.assertStatus(s.T(), update.Failed)
	s.assertAllVersionAreUpToDate(s.T())
	for _, tnnt := range fxt.Tenants {
		namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(tnnt.ID)
		assert.NoError(s.T(), err)
		for _, ns := range namespaces {
			assert.Equal(s.T(), "xyz", ns.UpdatedBy)
			assert.Equal(s.T(), "failed", ns.State)
			assert.Equal(s.T(), "0000", ns.Version)
			assert.True(s.T(), before.Before(ns.UpdatedAt))
		}
	}
}

func (s *TenantsUpdaterTestSuite) TestMoreGoroutinesTryingToUpdate() {
	//given
	waitToContinueGoroutine, waitToFinish, updateExecs := s.prepareForParallelTest(10, 3*time.Second, 10*time.Millisecond)

	// when
	waitToContinueGoroutine.Done()
	waitToFinish.Wait()

	// then
	executorFound := false
	for _, exec := range updateExecs {
		if *exec.numberOfCalls != 0 {
			assert.False(s.T(), executorFound)
			executorFound = true
			assert.Equal(s.T(), uint64(25), *exec.numberOfCalls)
		}
	}
	assert.True(s.T(), executorFound)
}

func (s *TenantsUpdaterTestSuite) TestTwoExecutorsDoUpdateBecauseOfLowerWaitTimeout() {
	//given
	waitToContinueGoroutine, waitToFinish, updateExecs := s.prepareForParallelTest(2, time.Millisecond, 10*time.Millisecond)

	// when
	waitToContinueGoroutine.Done()
	waitToFinish.Wait()

	// then
	assert.NotZero(s.T(), *updateExecs[0].numberOfCalls)
	assert.NotZero(s.T(), *updateExecs[1].numberOfCalls)
}

func (s *TenantsUpdaterTestSuite) prepareForParallelTest(count int, timeToWait, timeToSleep time.Duration) (*sync.WaitGroup, *sync.WaitGroup, []*DummyUpdateExecutor) {
	var waitToContinueGoroutine sync.WaitGroup
	waitToContinueGoroutine.Add(1)
	var waitToFinish sync.WaitGroup

	defer gock.Off()
	createMocks()
	testdoubles.SetTemplateVersions()
	tf.FillDB(s.T(), s.DB, 5, false, "ready", environment.DefaultEnvTypes...)
	s.tx(s.T(), func(repo update.Repository) error {
		return repo.UpdateVersionsTo(retrieveMappingWithVersion("0"))
	})
	s.tx(s.T(), func(repo update.Repository) error {
		return repo.UpdateStatus(update.Status("finished"))
	})

	var updateExecs []*DummyUpdateExecutor
	var toReset []func()
	defer func() {
		for _, reset := range toReset {
			reset()
		}
	}()

	for i := 0; i < count; i++ {
		waitToFinish.Add(1)
		updateExecutor := &DummyUpdateExecutor{timeToSleep: timeToSleep, numberOfCalls: Uint64(0)}
		updateExecs = append(updateExecs, updateExecutor)

		tenantsUpdater, reset := s.newTenantsUpdater(updateExecutor, timeToWait)
		toReset = append(toReset, reset)

		go func(toWait *sync.WaitGroup, toMakeDone *sync.WaitGroup, updater *update.TenantsUpdater) {
			defer toMakeDone.Done()

			toWait.Wait()
			err := updater.UpdateAllTenants()

			assert.NoError(s.T(), err)
		}(&waitToContinueGoroutine, &waitToFinish, tenantsUpdater)
	}
	return &waitToContinueGoroutine, &waitToFinish, updateExecs
}

func retrieveMappingWithVersion(version string) map[string]*update.VersionWithTypes {
	mapping := update.RetrieveAttrNameMapping()
	for _, versionWithTypes := range mapping {
		versionWithTypes.Version = version
	}
	return mapping
}

func (s *TenantsUpdaterTestSuite) tx(t *testing.T, do func(repo update.Repository) error) {
	var tx *sql.Tx
	tx, err := s.DB.DB().Begin()
	require.NoError(t, err)
	repo := update.NewRepository(tx)
	if err := do(repo); err != nil {
		tx.Rollback()
		assert.NoError(t, err)
	}
	tx.Commit()
}

func (s *TenantsUpdaterTestSuite) assertStatus(t *testing.T, st update.Status) {
	var status update.Status
	var err error
	s.tx(t, func(repo update.Repository) error {
		status, err = repo.GetStatus()
		return err
	})
	assert.NoError(t, err)
	assert.Equal(t, string(st), string(status))
}

func (s *TenantsUpdaterTestSuite) assertAllVersionAreUpToDate(t *testing.T) {
	var isVersionSame bool
	var err error
	for attrName, versionWithTypes := range update.RetrieveAttrNameMapping() {
		s.tx(t, func(repo update.Repository) error {
			isVersionSame, err = repo.IsVersionSame(attrName, versionWithTypes.Version)
			return err
		})
		assert.NoError(t, err)
		assert.True(t, isVersionSame)
	}

}

func (s *TenantsUpdaterTestSuite) newTenantsUpdater(updateExecutor controller.UpdateExecutor, timeout time.Duration) (*update.TenantsUpdater, func()) {
	reset := test.SetEnvironments(test.Env("F8_AUTH_TOKEN_KEY", "foo"))
	defer reset()
	authService, _, cleanup := testdoubles.NewAuthServiceWithRecorder(s.T(), "", "http://authservice")
	defer cleanup()

	saToken, err := test.NewToken(
		map[string]interface{}{
			"sub": "tenant_service",
		},
		"../test/private_key.pem",
	)
	require.NoError(s.T(), err)
	authService.SaToken = saToken.Raw

	clusterService := cluster.NewClusterService(time.Hour, authService)
	err = clusterService.Start()

	require.NoError(s.T(), err)
	config, reset := test.LoadTestConfig(s.T())
	return update.NewTenantsUpdater(timeout, s.DB, config, authService, clusterService, updateExecutor), func() {
		cleanup()
		reset()
		clusterService.Stop()
	}
}

type DummyUpdateExecutor struct {
	numberOfCalls *uint64
	timeToSleep   time.Duration
	shouldFail    bool
	wg            *sync.WaitGroup
}

func Uint64(v uint64) *uint64 {
	return &v
}

func (e *DummyUpdateExecutor) Update(ctx context.Context, tenantService tenant.Service, openshiftConfig openshift.Config, t *tenant.Tenant, envTypes []string) (map[string]string, error) {
	atomic.AddUint64(e.numberOfCalls, 1)

	time.Sleep(e.timeToSleep)
	if e.wg != nil {
		e.wg.Wait()
	}
	if e.shouldFail {
		return testdoubles.GetMappedVersions(envTypes...), fmt.Errorf("failing")
	}
	return testdoubles.GetMappedVersions(envTypes...), nil
}

func createMocks() {
	gock.New("http://authservice").
		Get("/api/token").
		Persist().
		MatchParam("for", "http://api.cluster1/").
		MatchParam("force_pull", "false").
		Reply(200).
		BodyString(`{ 
			"access_token": "jA0ECQMCYyjV8Zo7wgNg0sDQAUvut+valbh3k/zKDx+KPXcR7mmt7toLkc9Px7XaVMT6lQ6S7aOl6T8hpoPIWIEJuY33hZmJGmEXKkFzkU4BKcDaMnZXhiuwz4ECxOaeREpsUNCd7KSLayFGwuTuXbVwErmZau12CCCIjvlyJH89dCIkZD2hcElOhY6avEXfxQprtDF9iLddHiT+EOwZCSDOMKQbXVyAKR5FDaW8NXQpr7xsTmbe7dpoeS/uvIe2C5vEAH7dnc/TN5HmWYf0Is4ukfznKYef/+E+oSg3UkAO3i7PTFVsRuJCaN4pTIOcgeWjT7pvB49rb9UAZSfwSLDqbHgEfzjEatlC9PszMDlVckqvzg0Y0vhr+HpcvaJuu1VMy6Y5KH6NT4VlnL8tPFIcEeDJZLOreSmi43gkcl8YgTQp8G9C4h5h2nmS4E+1oU14uoBKwpjlek9r/x/o/hinYUrmSsht9FnQbbJAq7Umm/RbmanE47q86gy59UCTlW+zig8cp02pwQ7BW23YRrpZkiVB2QVmOGqB3+NCmK0pMg==",
			"token_type": "bearer",
			"username": "tenant_service"
    }`)

	gock.New("http://api.cluster1").
		Get("/apis/user.openshift.io/v1/users/~").
		Persist().
		Reply(200).
		BodyString(`{
     "kind":"User",
     "apiVersion":"user.openshift.io/v1",
     "metadata":{
       "name":"tenant_service",
       "selfLink":"/apis/user.openshift.io/v1/users/tenant_service",
       "uid":"bcdd0b29-123d-11e8-a8bc-b69930b94f5c",
       "resourceVersion":"814",
       "creationTimestamp":"2018-02-15T10:48:20Z"
     },
     "identities":[],
     "groups":[]
   }`)

	gock.New("http://authservice").
		Get("/api/clusters/").
		Persist().
		Reply(200).
		BodyString(`{
      "data":[
        {
          "name": "cluster1_name",
          "api-url": "http://api.cluster1/",
          "console-url": "http://console.cluster1/console/",
          "metrics-url": "http://metrics.cluster1/",
          "logging-url": "http://logging.cluster1/",
          "app-dns": "foo",
          "capacity-exhausted": false
        }
      ]
    }`)
}
