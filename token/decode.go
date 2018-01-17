package token

import (
	"bytes"
	"encoding/base64"
	"errors"
	"io/ioutil"

	"golang.org/x/crypto/openpgp"
)

type Decode func(data string) (string, error)

// PlainTextToken is a Decode function that can be used to fetch tokens that are not encrypted.
// Simply return the same token back
func PlainTextToken(token string) (string, error) {
	return token, nil
}

// NewGPGDecypter takes a passphrase and returns a GPG based Decypter decode function
func NewGPGDecypter(passphrase string) Decode {
	return func(body string) (string, error) {
		return gpgDecyptToken(body, passphrase)
	}
}

// GPGDecyptToken decrypts a Base64 encoded GPG un armored encrypted string
// using provided passphrase.
// echo -n "SuperSecret" | gpg --symmetric --cipher-algo AES256 | base64 -w0
func gpgDecyptToken(base64Body, passphrase string) (string, error) {
	decodedEnc, err := base64.StdEncoding.DecodeString(base64Body)
	if err != nil {
		return "", err
	}
	decbuf := bytes.NewBuffer(decodedEnc)
	firstCall := true
	md, err := openpgp.ReadMessage(decbuf, nil, func(keys []openpgp.Key, symmetric bool) ([]byte, error) {
		if firstCall {
			firstCall = false
			return []byte(passphrase), nil
		}
		return nil, errors.New("unable to decrypt token with given key")

	}, nil)
	if err != nil {
		return "", err
	}
	bytes, err := ioutil.ReadAll(md.UnverifiedBody)
	if err != nil {
		return "", err
	}
	return string(bytes), nil
}
