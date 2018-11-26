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
	repo := tenant.NewDBService(s.DB)
	tnnt, err := repo.GetTenant(id)
	assert.NoError(s.T(), err)

	iteration := 0
	for {
		namespaces, err := repo.GetNamespaces(id)
		if err != nil {
			assert.NoError(s.T(), err)
			break
		}
		if len(namespaces) == 5 {
			break
		}
		if iteration == 10 {
			assert.Fail(s.T(), fmt.Sprintf("not all namespaces created. created only: %+v", namespaces))
			break
		}
		iteration++
		time.Sleep(500 * time.Millisecond)
	}
	mappedObjects, masterOpts := s.GetMappedTemplateObjects(tnnt.NsBaseName)
	minishift.VerifyObjectsPresence(s.T(), mappedObjects, masterOpts, "1abcd")

	s.T().Run("update namespaces", func(t *testing.T) {
		// given
		testdoubles.SetTemplateSameVersion("2abcd")

		// when update is called
		goatest.UpdateTenantAccepted(t, createUserContext(t, id.String()), svc, ctrl)

		// then
		namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(id)
		assert.NoError(t, err)
		assert.Len(t, namespaces, 5)
		minishift.VerifyObjectsPresence(t, mappedObjects, masterOpts, "2abcd")
	})

	s.T().Run("only clean namespaces", func(t *testing.T) {

		// when clean is called
		goatest.CleanTenantNoContent(t, createUserContext(t, id.String()), svc, ctrl, false)

		// then
		namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(id)
		assert.NoError(t, err)
		assert.Len(t, namespaces, 5)
	})

	s.T().Run("remove namespaces and tenant", func(t *testing.T) {

		// when delete is called
		goatest.CleanTenantNoContent(t, createUserContext(t, id.String()), svc, ctrl, true)
		// then
		namespaces, err := tenant.NewDBService(s.DB).GetNamespaces(id)
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
