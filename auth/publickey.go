package auth

import (
	"context"
	"crypto/rsa"
	"encoding/json"
	"net/http"

	"github.com/fabric8-services/fabric8-common/log"
	authclient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-wit/rest"
	errs "github.com/pkg/errors"
	"gopkg.in/square/go-jose.v2"
)

// GetPublicKeys returns the known public keys used to sign tokens from the auth service
func (s *Service) GetPublicKeys() ([]*rsa.PublicKey, error) {
	client, err := s.newClient("") // no need for a token when calling this endpoint
	if err != nil {
		return nil, errs.Wrapf(err, "unable to retrieve public keys from %s", s.GetAuthURL())
	}
	res, err := client.KeysToken(context.Background(), authclient.KeysTokenPath(), nil)
	if err != nil {
		log.Error(context.Background(), map[string]interface{}{
			"err": err.Error(),
		}, "unable to get public keys from the auth service")
		return nil, errs.Wrap(err, "unable to get public keys from the auth service")
	}
	defer res.Body.Close()
	bodyString := rest.ReadBody(res.Body)
	if res.StatusCode != http.StatusOK {
		if err != nil {
			log.Error(context.Background(), map[string]interface{}{
				"err": err.Error(),
			}, "unable to read public keys from the auth service")
			return nil, errs.Wrap(err, "unable to read public keys from the auth service")
		}
	}
	keys, err := unmarshalKeys([]byte(bodyString))
	if err != nil {
		return nil, errs.Wrapf(err, "unable to load keys from auth service")
	}
	log.Info(nil, map[string]interface{}{
		"url":            authclient.KeysTokenPath(),
		"number_of_keys": len(keys),
	}, "Public keys loaded")
	result := make([]*rsa.PublicKey, 0)
	for _, k := range keys {
		result = append(result, k.Key)
	}
	return result, nil
}

// PublicKey a public key loaded from auth service
type PublicKey struct {
	KeyID string
	Key   *rsa.PublicKey
}

// JSONKeys the JSON structure for unmarshalling the keys
type JSONKeys struct {
	Keys []interface{} `json:"keys"`
}

func unmarshalKeys(jsonData []byte) ([]*PublicKey, error) {
	var keys []*PublicKey
	var raw JSONKeys
	err := json.Unmarshal(jsonData, &raw)
	if err != nil {
		return nil, err
	}
	for _, key := range raw.Keys {
		jsonKeyData, err := json.Marshal(key)
		if err != nil {
			return nil, err
		}
		publicKey, err := unmarshalKey(jsonKeyData)
		if err != nil {
			return nil, err
		}
		keys = append(keys, publicKey)
	}
	return keys, nil
}

func unmarshalKey(jsonData []byte) (*PublicKey, error) {
	var key *jose.JSONWebKey
	key = &jose.JSONWebKey{}
	err := key.UnmarshalJSON(jsonData)
	if err != nil {
		return nil, err
	}
	rsaKey, ok := key.Key.(*rsa.PublicKey)
	if !ok {
		return nil, errs.New("Key is not an *rsa.PublicKey")
	}
	log.Info(nil, map[string]interface{}{"key_id": key.KeyID}, "unmarshalled public key")
	return &PublicKey{
			KeyID: key.KeyID,
			Key:   rsaKey},
		nil
}
