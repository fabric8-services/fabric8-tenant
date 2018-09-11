package test

import (
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-common/log"
)

// NewToken creates a new JWT using the given sub claim and signed with the private key in the given filename
func NewToken(claims map[string]interface{}, privatekeyFilename string) (*jwt.Token, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, jwt.MapClaims(claims))
	// use the test private key to sign the token
	key, err := PrivateKey(privatekeyFilename)
	if err != nil {
		return nil, err
	}
	signed, err := token.SignedString(key)
	if err != nil {
		return nil, err
	}
	token.Raw = signed
	log.Debug(nil, map[string]interface{}{"signed_token": signed, "claims": claims}, "generated test token with custom sub")
	return token, nil
}
