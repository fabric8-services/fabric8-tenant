package token_test

import (
	"context"
	"encoding/json"
	"testing"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-tenant/test/recorder"
	"github.com/fabric8-services/fabric8-tenant/token"
	"github.com/fabric8-services/fabric8-wit/log"
	errs "github.com/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	jose "gopkg.in/square/go-jose.v2"
)

func TestPublicKeys(t *testing.T) {

	t.Run("valid keys", func(t *testing.T) {
		//given
		r, err := recorder.New("../test/data/token/auth_get_keys")
		require.NoError(t, err)
		// when
		result, err := token.GetPublicKeys(context.Background(),
			"http://authservice",
			configuration.WithRoundTripper(r))
		// then
		require.NoError(t, err)
		assert.Len(t, result, 3)
	})

	t.Run("invalid url", func(t *testing.T) {
		// when
		_, err := token.GetPublicKeys(context.Background(), "http://google.com")
		// then
		assert.Error(t, err)
	})
}

func generateRawToken(filename, subject string) (*string, error) {
	claims := jwt.MapClaims{}
	claims["sub"] = subject
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
	// use the test private key to sign the token
	key, err := testsupport.PrivateKey(filename)
	if err != nil {
		return nil, errs.Wrap(err, "failed to generate token")
	}
	token.Header["kid"] = "billythekid"
	signed, err := token.SignedString(key)
	if err != nil {
		return nil, errs.Wrap(err, "failed to generate token")
	}
	log.Debug(nil, map[string]interface{}{"raw_token": signed}, "token generated")
	return &signed, nil
}

// utility function to generate the content to put in the `test/data/token/auth_get_keys.yaml` file
func generateJSONWebKey() (interface{}, error) {
	publickey, err := testsupport.PublicKey("../test/public_key.pem")
	if err != nil {
		return nil, err
	}
	key := token.PublicKey{
		KeyID: "foo",
		Key:   publickey,
	}
	jwk := jose.JSONWebKey{Key: key.Key, KeyID: key.KeyID, Algorithm: "RS256", Use: "sig"}
	keyData, err := jwk.MarshalJSON()
	if err != nil {
		return nil, err
	}
	var raw interface{}
	err = json.Unmarshal(keyData, &raw)
	if err != nil {
		return nil, err
	}
	return raw, nil
}

type config struct {
	authServiceURL string
}

func (c *config) GetAuthURL() string {
	return c.authServiceURL
}
