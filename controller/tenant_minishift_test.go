package controller_test

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
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"testing"
	"time"
)

type TenantControllerMinishiftTestSuite struct {
	minishift.TestSuite
}

func TestTenantControllerWithMinishift(t *testing.T) {
	suite.Run(t, &TenantControllerMinishiftTestSuite{
		TestSuite: minishift.TestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")}})
}

func (s *TenantControllerMinishiftTestSuite) TestSetupUpdateCleanAndDeleteTenantNamespaces() {
	// given
	testdoubles.SetTemplateSameVersion("1abcd")
	id := uuid.NewV4()
	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantController(svc, tenant.NewDBService(s.DB), s.GetClusterService(), s.GetAuthService(id), s.GetConfig())

	// when setup is called
	goatest.SetupTenantAccepted(s.T(), createUserContext(s.T(), id.String()), svc, ctrl)

	// then
	repo := tenant.NewTenantRepository(s.DB, id)
	tnnt, err := repo.GetTenant()
	assert.NoError(s.T(), err)

	err = test.WaitWithTimeout(30 * time.Second).Until(func() error {
		namespaces, err := repo.GetNamespaces()
		if err != nil {
			return err
		}
		if len(namespaces) != 2 {
			return fmt.Errorf("not all namespaces created. created only: %+v", namespaces)
		}
		return nil
	})
	assert.NoError(s.T(), err)
	s.VerifyObjectsPresence(s.T(), tnnt.NsBaseName, "1abcd", false)

	s.T().Run("update namespaces", func(t *testing.T) {
		// given
		testdoubles.SetTemplateSameVersion("2abcd")

		// when update is called
		goatest.UpdateTenantAccepted(t, createUserContext(t, id.String()), svc, ctrl)

		// then
		namespaces, err := repo.GetNamespaces()
		assert.NoError(t, err)
		assert.Len(t, namespaces, 2)
		s.VerifyObjectsPresence(t, tnnt.NsBaseName, "2abcd", false)
	})

	s.T().Run("update namespaces should fail", func(t *testing.T) {
		// given
		testdoubles.SetTemplateSameVersion("2abcd")
		cls := *s.ClusterService
		cls.APIURL = "123"
		ctrl := controller.NewTenantController(svc, tenant.NewDBService(s.DB), &cls, s.GetAuthService(id), s.GetConfig())

		// when update is called
		goatest.UpdateTenantInternalServerError(t, createUserContext(t, id.String()), svc, ctrl)
	})

	s.T().Run("clean namespaces should fail", func(t *testing.T) {
		// given
		testdoubles.SetTemplateSameVersion("2abcd")
		cls := *s.ClusterService
		cls.APIURL = "123"
		ctrl := controller.NewTenantController(svc, tenant.NewDBService(s.DB), &cls, s.GetAuthService(id), s.GetConfig())

		// when update is called
		goatest.CleanTenantInternalServerError(t, createUserContext(t, id.String()), svc, ctrl, false)
	})

	s.T().Run("only clean namespaces", func(t *testing.T) {

		// when clean is called
		goatest.CleanTenantNoContent(t, createUserContext(t, id.String()), svc, ctrl, false)

		// then
		namespaces, err := repo.GetNamespaces()
		assert.NoError(t, err)
		assert.Len(t, namespaces, 2)
	})

	s.T().Run("remove namespaces and tenant", func(t *testing.T) {

		// when delete is called
		goatest.CleanTenantNoContent(t, createUserContext(t, id.String()), svc, ctrl, true)
		// then
		namespaces, err := repo.GetNamespaces()
		assert.NoError(t, err)
		assert.Len(t, namespaces, 0)
	})
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
