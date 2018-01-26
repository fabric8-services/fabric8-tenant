package token_test

import (
	"testing"

	"github.com/fabric8-services/fabric8-tenant/token"
)

func TestDecryptSuccess(t *testing.T) {
	txt, err := token.NewGPGDecypter("foo")(testEncryptedMessage)
	if err != nil {
		t.Fatal("Could not decode token", err)
	}
	if txt != "SuperSecret" {
		t.Errorf("Wrong decypted msg, got [%v]", txt)
	}
}

func TestDecryptUnSuccess(t *testing.T) {
	_, err := token.NewGPGDecypter("foo2")(testEncryptedMessage)
	if err == nil {
		t.Fatal("Could decode token", err)
	}
}

var testEncryptedMessage = `jA0ECQMCtCG1bfGEQbxg0kABEQ6nh/A4tMGGkHMHJtLDtFLyXh28IuLvoyGjsZtWPV0LHwN+EEsTtu90BQGbWFdBv+2Wiedk9eE3h08lwA8m`
