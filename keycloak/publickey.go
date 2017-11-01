package keycloak

import (
	"crypto/rsa"
	"fmt"

	authclient "github.com/fabric8-services/fabric8-auth/token"
	"github.com/fabric8-services/fabric8-tenant/auth"
)

// GetPublicKeys returns the known public keys used to sign tokens from the auth service
func GetPublicKeys(authServiceBase string) ([]*rsa.PublicKey, error) {
	keysEndpoint := fmt.Sprintf("%s%s", authServiceBase, auth.KeysTokenPath())
	keys, err := authclient.FetchKeys(keysEndpoint)
	if err != nil {
		return nil, err
	}
	rsaKeys := []*rsa.PublicKey{}
	for _, key := range keys {
		rsaKeys = append(rsaKeys, key.Key)
	}
	return rsaKeys, nil
}
