package update_test

import (
	"context"
	"fmt"
	goatest "github.com/fabric8-services/fabric8-tenant/app/test"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	"github.com/fabric8-services/fabric8-tenant/test/minishift"
	"github.com/fabric8-services/fabric8-tenant/test/update"
	"github.com/fabric8-services/fabric8-tenant/update"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"sync"
	"testing"
	"time"
)

type AutomatedUpdateMinishiftTestSuite struct {
	minishift.TestSuite
}

var numberOfTenants = 11

func TestAutomatedUpdateWithMinishift(t *testing.T) {
	toReset := test.SetEnvironments(
		test.Env("F8_AUTOMATED_UPDATE_RETRY_SLEEP", (time.Duration(numberOfTenants) * 8 * time.Second).String()))
	defer toReset()

	suite.Run(t, &AutomatedUpdateMinishiftTestSuite{
		TestSuite: minishift.TestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")}})
}

func (s *AutomatedUpdateMinishiftTestSuite) TestAutomaticUpdateOfTenantNamespaces() {
	// given
	testdoubles.SetTemplateSameVersion("1abcd")
	svc := goa.New("Tenants-service")
	var tenantIDs []uuid.UUID
	clusterService := s.GetClusterService()
	repo := tenant.NewDBService(s.DB)

	for i := 0; i < numberOfTenants; i++ {
		id := uuid.NewV4()
		tenantIDs = append(tenantIDs, id)
		ctrl := controller.NewTenantController(svc, repo, clusterService, s.GetAuthService(id), s.GetConfig())
		goatest.SetupTenantAccepted(s.T(), createUserContext(s.T(), id.String()), svc, ctrl)
	}

	for _, tenantID := range tenantIDs {
		err := test.WaitWithTimeout(time.Duration(numberOfTenants) * 8 * time.Second).Until(func() error {
			namespaces, err := repo.GetNamespaces(tenantID)
			if err != nil {
				return err
			}
			if len(namespaces) != 5 {
				return fmt.Errorf("not all namespaces created. created only: %+v", namespaces)
			}
			return nil
		})
		require.NoError(s.T(), err)
		tnnt, err := repo.GetTenant(tenantID)
		require.NoError(s.T(), err)
		s.VerifyObjectsPresence(s.T(), tnnt.NsBaseName, "1abcd", true)
	}
	defer s.clean(tenantIDs)

	testupdate.Tx(s.T(), s.DB, func(repo update.Repository) error {
		if err := repo.UpdateStatus(update.Finished); err != nil {
			return err
		}
		return testupdate.UpdateVersionsTo(repo, "1abcd")
	})
	before := time.Now()

	// when
	testdoubles.SetTemplateSameVersion("2abcd")

	var goroutineCanContinue sync.WaitGroup
	goroutineCanContinue.Add(1)
	var goroutineFinished sync.WaitGroup
	updateExec := controller.TenantUpdater{ClusterService: clusterService, TenantRepository: repo, Config: s.Config}
	for i := 0; i < 10; i++ {
		goroutineFinished.Add(1)
		go func(updateExecutor update.UpdateExecutor) {
			defer goroutineFinished.Done()

			goroutineCanContinue.Wait()
			update.NewTenantsUpdater(s.DB, s.Config, clusterService, updateExecutor, update.AllTypes, "").UpdateAllTenants()
		}(updateExec)
	}
	goroutineCanContinue.Done()
	goroutineFinished.Wait()
	// then
	testupdate.AssertStatusAndAllVersionAreUpToDate(s.T(), s.DB, update.Finished, update.AllTypes)
	s.verifyAreUpdated(tenantIDs, before)
}

func (s *AutomatedUpdateMinishiftTestSuite) verifyAreUpdated(tenantIDs []uuid.UUID, wasBefore time.Time) {
	var wg sync.WaitGroup
	for _, tenantID := range tenantIDs {
		wg.Add(1)
		go func(t *testing.T, tenantID uuid.UUID) {
			defer wg.Done()
			repo := tenant.NewDBService(s.DB)
			tnnt, err := repo.GetTenant(tenantID)
			assert.NoError(t, err)
			namespaces, err := repo.GetNamespaces(tenantID)
			assert.NoError(t, err)
			assert.Len(t, namespaces, 5)
			for _, ns := range namespaces {
				assert.True(t, wasBefore.Before(ns.UpdatedAt))
				assert.Contains(t, ns.Version, "2abcd")
				assert.NotContains(t, ns.Version, "1abcd")
				assert.Equal(t, tenant.Ready, ns.State)
			}
			s.VerifyObjectsPresence(s.T(), tnnt.NsBaseName, "2abcd", false)
		}(s.T(), tenantID)
	}
	wg.Wait()
}

func (s *AutomatedUpdateMinishiftTestSuite) clean(toCleanup []uuid.UUID) {
	svc := goa.New("Tenants-service")
	var wg sync.WaitGroup
	for _, tenantID := range toCleanup {
		wg.Add(1)
		go func(tenantID uuid.UUID) {
			defer wg.Done()
			ctrl := controller.NewTenantController(svc, tenant.NewDBService(s.DB), s.GetClusterService(), s.GetAuthService(tenantID), s.GetConfig())
			goatest.CleanTenantNoContent(s.T(), createUserContext(s.T(), tenantID.String()), svc, ctrl, true)
		}(tenantID)
	}
	wg.Wait()
}

func createUserContext(t *testing.T, sub string) context.Context {
	userToken, err := test.NewToken(
		map[string]interface{}{
			"sub":                sub,
			"preferred_username": "developer",
			"email":              "developer@redhat.com",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)

	return goajwt.WithJWT(context.Background(), userToken)
}
