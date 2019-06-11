package metric_test

import (
	"context"
	"github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-common/convert/ptr"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/metric"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-tenant/test/gormsupport"
	tf "github.com/fabric8-services/fabric8-tenant/test/testfixture"
	"github.com/goadesign/goa"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gopkg.in/h2non/gock.v1"
	"io/ioutil"
	"net/http/httptest"
	"testing"

	apptest "github.com/fabric8-services/fabric8-tenant/app/test"
	dto "github.com/prometheus/client_model/go"
)

func TestMetricsEndpointExposesTenantSpecificMetrics(t *testing.T) {
	// given
	defer unRegisterAndResetMetrics()
	metric.RegisterMetrics()
	metric.RecordProvisionedTenant(true)
	metric.RecordDeletedTenant(true)
	metric.RecordDeletedTenant(false)
	metric.RecordCleanedTenant(true, "")
	metric.RecordUpdatedTenant(true, "")

	handler := promhttp.Handler()

	request := httptest.NewRequest("GET", "/metrics", nil)
	response := httptest.NewRecorder()

	// when
	handler.ServeHTTP(response, request)

	// then
	body, err := ioutil.ReadAll(response.Body)
	require.NoError(t, err)

	assert.Contains(t, string(body), "provisioned_tenants_total")
	assert.Contains(t, string(body), "deleted_tenants_total")
	assert.Contains(t, string(body), "cleaned_tenants_total")
	assert.Contains(t, string(body), "updated_tenants_total")
}

type MetricTestSuite struct {
	gormsupport.DBTestSuite
}

func TestMetric(t *testing.T) {
	suite.Run(t, &MetricTestSuite{DBTestSuite: gormsupport.NewDBTestSuite("../config.yaml")})
}

func (s *MetricTestSuite) TestFailedProvisionedTenantMetric() {
	// given
	defer unRegisterAndResetMetrics()
	metric.RegisterMetrics()
	defer gock.OffAll()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()
	gock.New(test.ClusterURL).
		Post(".*/rolebindingrestrictions").
		Reply(500)
	testdoubles.MockPostRequestsToOS(ptr.Int(0), test.ClusterURL, environment.DefaultEnvTypes, "johny")

	// when
	apptest.SetupTenantInternalServerError(s.T(), testdoubles.CreateAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)

	// then
	s.verifyCount(metric.ProvisionedTenantsCounter, 1, "false")
	s.verifyCount(metric.ProvisionedTenantsCounter, 0, "true")
}

func (s *MetricTestSuite) TestSuccessfulProvisionedTenantMetric() {
	// given
	defer unRegisterAndResetMetrics()
	metric.RegisterMetrics()
	defer gock.OffAll()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()
	testdoubles.MockPostRequestsToOS(ptr.Int(0), test.ClusterURL, environment.DefaultEnvTypes, "johny")

	// when
	apptest.SetupTenantAccepted(s.T(), testdoubles.CreateAndMockUserAndToken(s.T(), uuid.NewV4().String(), false), svc, ctrl)

	// then
	s.verifyCount(metric.ProvisionedTenantsCounter, 0, "false")
	s.verifyCount(metric.ProvisionedTenantsCounter, 1, "true")
}

func (s *MetricTestSuite) TestFailedUpdatedTenantMetric() {
	// given
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johny")), tf.AddDefaultNamespaces())
	id := fxt.Tenants[0].ID.String()
	defer unRegisterAndResetMetrics()
	metric.RegisterMetrics()
	defer gock.OffAll()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()
	gock.New(test.ClusterURL).
		Get(".*/persistentvolumeclaims/.*").
		Persist().
		Reply(404)
	gock.New(test.ClusterURL).
		Post(".*/persistentvolumeclaims/.*").
		Persist().
		Reply(500)
	testdoubles.MockPatchRequestsToOS(ptr.Int(0), test.ClusterURL)

	// when
	apptest.UpdateTenantInternalServerError(s.T(), testdoubles.CreateAndMockUserAndToken(s.T(), id, false), svc, ctrl)

	// then
	s.verifyCount(metric.UpdatedTenantsCounter, 1, "false", "johny")
	s.verifyCount(metric.UpdatedTenantsCounter, 0, "true", "johny")
}

func (s *MetricTestSuite) TestSuccessfulUpdatedTenantMetric() {
	// given
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johny")), tf.AddDefaultNamespaces())
	id := fxt.Tenants[0].ID.String()
	defer unRegisterAndResetMetrics()
	metric.RegisterMetrics()
	defer gock.OffAll()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()
	testdoubles.MockPatchRequestsToOS(ptr.Int(0), test.ClusterURL)

	// when
	apptest.UpdateTenantAccepted(s.T(), testdoubles.CreateAndMockUserAndToken(s.T(), id, false), svc, ctrl)

	// then
	s.verifyCount(metric.UpdatedTenantsCounter, 0, "false", "johny")
	s.verifyCount(metric.UpdatedTenantsCounter, 1, "true", "johny")
}

func (s *MetricTestSuite) TestFailedCleanedTenantMetric() {
	// given
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johny")), tf.AddDefaultNamespaces())
	id := fxt.Tenants[0].ID.String()
	defer unRegisterAndResetMetrics()
	metric.RegisterMetrics()
	defer gock.OffAll()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()
	gock.New(test.ClusterURL).
		Get(".*-che.*/pods").
		Persist().
		Reply(202).
		BodyString(`{"items": [
        {"metadata": {"name": "workspace"}}]}`)
	gock.New(test.ClusterURL).
		Delete(".*/workspace").
		Persist().
		Reply(500)
	testdoubles.MockPatchRequestsToOS(ptr.Int(0), test.ClusterURL)

	// when
	apptest.CleanTenantInternalServerError(s.T(), testdoubles.CreateAndMockUserAndToken(s.T(), id, false), svc, ctrl, false)

	// then
	s.verifyCount(metric.CleanedTenantsCounter, 1, "false", "johny")
	s.verifyCount(metric.CleanedTenantsCounter, 0, "true", "johny")
}

func (s *MetricTestSuite) TestSuccessfulCleanedTenantMetric() {
	// given
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johny")), tf.AddDefaultNamespaces())
	id := fxt.Tenants[0].ID.String()
	defer unRegisterAndResetMetrics()
	metric.RegisterMetrics()
	defer gock.OffAll()
	svc, ctrl, _, reset := s.newTestTenantController()
	defer reset()
	testdoubles.MockPatchRequestsToOS(ptr.Int(0), test.ClusterURL)

	// when
	apptest.UpdateTenantAccepted(s.T(), testdoubles.CreateAndMockUserAndToken(s.T(), id, false), svc, ctrl)

	// then
	s.verifyCount(metric.UpdatedTenantsCounter, 0, "false", "johny")
	s.verifyCount(metric.UpdatedTenantsCounter, 1, "true", "johny")
}

func (s *MetricTestSuite) TestFailedDeletedTenantMetric() {
	// given
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johny")), tf.AddDefaultNamespaces())
	id := fxt.Tenants[0].ID
	defer unRegisterAndResetMetrics()
	metric.RegisterMetrics()
	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	svc, ctrl, reset := s.newTestTenantsController()
	defer reset()
	gock.New(test.ClusterURL).
		Delete(".*/projects/.*").
		Persist().
		Reply(500)
	testdoubles.MockRemoveRequestsToOS(ptr.Int(0), test.ClusterURL)

	// when
	apptest.DeleteTenantsInternalServerError(s.T(), authContext(), svc, ctrl, id)

	// then
	s.verifyCount(metric.DeletedTenantsCounter, 1, "false")
	s.verifyCount(metric.DeletedTenantsCounter, 0, "true")
}

func (s *MetricTestSuite) TestDeletedProvisionedTenantMetric() {
	// given
	fxt := tf.FillDB(s.T(), s.DB, tf.AddSpecificTenants(tf.SingleWithName("johny")), tf.AddDefaultNamespaces())
	id := fxt.Tenants[0].ID
	defer unRegisterAndResetMetrics()
	metric.RegisterMetrics()
	defer gock.OffAll()
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	svc, ctrl, reset := s.newTestTenantsController()
	defer reset()
	testdoubles.MockRemoveRequestsToOS(ptr.Int(0), test.ClusterURL)

	// when
	apptest.DeleteTenantsNoContent(s.T(), authContext(), svc, ctrl, id)

	// then
	s.verifyCount(metric.DeletedTenantsCounter, 0, "false")
	s.verifyCount(metric.DeletedTenantsCounter, 1, "true")
}

func authContext() context.Context {
	claims := jwt.MapClaims{}
	claims["service_accountname"] = "fabric8-auth"
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	return goajwt.WithJWT(context.Background(), token)
}

func (s *MetricTestSuite) verifyCount(counterVec *prometheus.CounterVec, expected int, labels ...string) {
	counter, err := counterVec.GetMetricWithLabelValues(labels...)
	require.NoError(s.T(), err)
	metric := &dto.Metric{}
	err = counter.Write(metric)
	require.NoError(s.T(), err)
	assert.Equal(s.T(), expected, int(metric.Counter.GetValue()))
}

func (s *MetricTestSuite) newTestTenantController() (*goa.Service, *controller.TenantController, *configuration.Data, func()) {
	testdoubles.MockCommunicationWithAuth(test.ClusterURL)
	clusterService, authService, config, reset := testdoubles.PrepareConfigClusterAndAuthService(s.T())
	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantController(svc, tenant.NewDBService(s.DB), clusterService, authService, config)
	return svc, ctrl, config, reset
}

func (s *MetricTestSuite) newTestTenantsController() (*goa.Service, *controller.TenantsController, func()) {
	clusterService, authService, config, reset := testdoubles.PrepareConfigClusterAndAuthService(s.T())
	svc := goa.New("Tenants-service")
	ctrl := controller.NewTenantsController(svc, tenant.NewDBService(s.DB), clusterService, authService, config)
	return svc, ctrl, reset
}

func unRegisterAndResetMetrics() {
	prometheus.Unregister(metric.ProvisionedTenantsCounter)
	metric.ProvisionedTenantsCounter.Reset()
	prometheus.Unregister(metric.CleanedTenantsCounter)
	metric.CleanedTenantsCounter.Reset()
	prometheus.Unregister(metric.UpdatedTenantsCounter)
	metric.UpdatedTenantsCounter.Reset()
	prometheus.Unregister(metric.DeletedTenantsCounter)
	metric.DeletedTenantsCounter.Reset()
}
