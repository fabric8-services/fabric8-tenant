package test

import (
	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-wit/log"
)

// NewToken creates a new JWT using the given sub claim and signed with the private key in the given filename
func NewToken(sub string, privatekeyFilename string) (*jwt.Token, error) {
	claims := jwt.MapClaims{}
	claims["sub"] = sub
	token := jwt.NewWithClaims(jwt.SigningMethodRS512, claims)
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
	log.Debug(nil, map[string]interface{}{"signed_token": signed, "sub": sub}, "generated test token with custom sub")
	return token, nil
}
