package openshift_test

import (
	"testing"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/errors"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	errs "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDeleteNamespace(t *testing.T) {
	clusterURL := "https://openshift.test"
	r, err := recorder.New("../test/data/openshift/delete_namespace", recorder.WithJWTMatcher())
	require.NoError(t, err)
	defer r.Stop()
	// given
	token, err := testsupport.NewToken(
		map[string]interface{}{
			"sub": "user",
		},
		"../test/private_key.pem",
	)
	require.NoError(t, err)

	t.Run("ok", func(t *testing.T) {
		// when
		err = openshift.DeleteNamespace("ns1", clusterURL, token.Raw, configuration.WithRoundTripper(r))
		// then
		require.NoError(t, err)
	})

	t.Run("fail", func(t *testing.T) {

		t.Run("no namespace", func(t *testing.T) {
			// when
			err = openshift.DeleteNamespace("unknown", clusterURL, token.Raw, configuration.WithRoundTripper(r))
			// then
			require.Error(t, err)
			t.Logf("error: %v", err)
			assert.IsType(t, errors.OpenShiftObjectNotFoundError{}, errs.Cause(err))
		})

		t.Run("conflict", func(t *testing.T) {
			// when
			err = openshift.DeleteNamespace("conflict", clusterURL, token.Raw, configuration.WithRoundTripper(r))
			// then
			require.Error(t, err)
			t.Logf("error: %v", err)
			assert.IsType(t, errors.OpenShiftObjectConflictError{}, errs.Cause(err))
		})
	})
}
