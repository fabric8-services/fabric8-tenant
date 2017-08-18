package openshift_test

import (
	"testing"

	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/stretchr/testify/assert"
)

var dnsRegExp = "^[a-z0-9]([-a-z0-9]*[a-z0-9])?$"

func TestCreateUsername(t *testing.T) {

	assertName(t, "some", "some@email.com")
	assertName(t, "so-me", "so-me@email.com")
	assertName(t, "some", "some")
	assertName(t, "so-me", "so-me")
	assertName(t, "so-me", "so_me")
	assertName(t, "so-me", "so me")
	assertName(t, "so-me", "so me@email.com")
	assertName(t, "so-me", "so.me")
	assertName(t, "so-me", "so?me")
	assertName(t, "so-me", "so:me")
	assertName(t, "some1", "some1")
	assertName(t, "so1me1", "so1me1")
}

func assertName(t *testing.T, expected, username string) {
	assert.Regexp(t, dnsRegExp, openshift.CreateName(username))
	assert.Equal(t, expected, openshift.CreateName(username))
}
