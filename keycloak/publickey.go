package keycloak

import (
	"crypto/rsa"
	"fmt"

	authtoken "github.com/fabric8-services/fabric8-auth/token"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
)

// GetPublicKeys returns the known public keys used to sign tokens from the auth service
func GetPublicKeys(authServiceBase string) ([]*rsa.PublicKey, error) {
	keysEndpoint := fmt.Sprintf("%s%s", authServiceBase, authclient.KeysTokenPath())
	keys, err := authtoken.FetchKeys(keysEndpoint)
	if err != nil {
		return nil, err
	}
	rsaKeys := []*rsa.PublicKey{}
	for _, key := range keys {
		rsaKeys = append(rsaKeys, key.Key)
	}
	return rsaKeys, nil
}
