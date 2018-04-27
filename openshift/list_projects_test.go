package openshift_test

import (
	"context"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListProjects(t *testing.T) {
	// given
	clusterURL := "https://openshift.test"
	r, err := recorder.New("../test/data/openshift/list_projects", recorder.WithJWTMatcher())
	require.NoError(t, err)
	defer r.Stop()

	t.Run("no project", func(t *testing.T) {
		// given
		token, err := testsupport.NewToken(
			map[string]interface{}{
				"sub": "user_no_project",
			},
			"../test/private_key.pem",
		)
		require.NoError(t, err)
		// when
		projectNames, err := openshift.ListProjects(context.Background(), clusterURL, token.Raw, configuration.WithRoundTripper(r))
		// then
		require.NoError(t, err)
		assert.Empty(t, projectNames)

	})

	t.Run("single project", func(t *testing.T) {
		// given
		token, err := testsupport.NewToken(
			map[string]interface{}{
				"sub": "user_single_project",
			},
			"../test/private_key.pem",
		)
		require.NoError(t, err)
		// when
		projectNames, err := openshift.ListProjects(context.Background(), clusterURL, token.Raw, configuration.WithRoundTripper(r))
		// then
		require.NoError(t, err)
		require.Len(t, projectNames, 1)
		assert.Equal(t, "foo", projectNames[0])
	})

	t.Run("multiple projects", func(t *testing.T) {
		// given
		token, err := testsupport.NewToken(
			map[string]interface{}{
				"sub": "user_multi_projects",
			},
			"../test/private_key.pem",
		)
		require.NoError(t, err)
		// when
		projectNames, err := openshift.ListProjects(context.Background(), clusterURL, token.Raw, configuration.WithRoundTripper(r))
		// then
		require.NoError(t, err)
		require.Len(t, projectNames, 2)
		assert.Equal(t, "foo1", projectNames[0])
		assert.Equal(t, "foo2", projectNames[1])

	})
}
