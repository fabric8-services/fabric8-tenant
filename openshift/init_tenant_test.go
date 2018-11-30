package openshift_test

import (
	"context"
	"github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/h2non/gock.v1"
	"net/http"
	"testing"
)

var emptyCallback = func(statusCode int, method string, request, response map[interface{}]interface{}, versionMapping map[string]string) (string, map[interface{}]interface{}) {
	return "", nil
}

func TestNumberOfCallsToCluster(t *testing.T) {
	// given
	data, reset := test.LoadTestConfig(t)
	defer func() {
		gock.OffAll()
		reset()
	}()
	calls := 0
	gock.New("http://my.cluster").
		SetMatcher(SpyOnCalls(&calls)).
		Times(78).
		Persist().
		Reply(200).
		BodyString("{}")

	user := &client.UserDataAttributes{}
	config := openshift.NewConfigForUser(data, user, "clusterUser", "clusterToken", "http://my.cluster")
	config.HTTPTransport = http.DefaultTransport
	objectsInTemplates := tmplObjects(t, data)

	// when
	err := openshift.RawInitTenant(context.Background(), config, emptyCallback, "developer", "developer", "12345")

	// then
	require.NoError(t, err)
	// the number of calls should be equal to the number of parsed objects plus one call that removes admin role from user's namespace
	assert.Equal(t, len(objectsInTemplates)+1, calls)
}

// SpyOnCalls checks the number of calls
func SpyOnCalls(counter *int) gock.Matcher {
	matcher := gock.NewBasicMatcher()
	matcher.Add(func(_ *http.Request, _ *gock.Request) (bool, error) {
		*counter++
		return true, nil
	})
	return matcher
}
