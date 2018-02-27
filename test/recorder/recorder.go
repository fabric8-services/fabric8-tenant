package recorder

import (
	"fmt"
	"net/http"
	"os"

	jwt "github.com/dgrijalva/jwt-go"
	jwtrequest "github.com/dgrijalva/jwt-go/request"
	"github.com/dnaeon/go-vcr/cassette"
	"github.com/dnaeon/go-vcr/recorder"
	testsupport "github.com/fabric8-services/fabric8-tenant/test"
	"github.com/fabric8-services/fabric8-wit/log"
	errs "github.com/pkg/errors"
)

// Option an option to customize the recorder to create
type Option func(*recorder.Recorder)

// WithMatcher an option to specify a custom matcher for the recorder
func WithMatcher(matcher cassette.Matcher) Option {
	return func(r *recorder.Recorder) {
		r.SetMatcher(matcher)
	}
}

// WithJWTMatcher an option to specify the JWT matcher for the recorder
func WithJWTMatcher() Option {
	return func(r *recorder.Recorder) {
		r.SetMatcher(JWTMatcher())
	}
}

// New creates a new recorder
func New(cassetteName string, options ...Option) (*recorder.Recorder, error) {
	_, err := os.Stat(fmt.Sprintf("%s.yaml", cassetteName))
	if err != nil {
		return nil, errs.Wrapf(err, "unable to find file '%s.yaml'", cassetteName)
	}
	r, err := recorder.New(cassetteName)
	if err != nil {
		return nil, errs.Wrapf(err, "unable to create recorder from file '%s.yaml'", cassetteName)
	}
	// custom cassette matcher that will compare the HTTP requests' token subject with the `sub` header of the recorded data (the yaml file)
	for _, opt := range options {
		opt(r)
	}
	return r, nil
}

// JWTMatcher a cassette matcher that verifies the request method/URL and the subject of the token in the "Authorization" header.
func JWTMatcher() cassette.Matcher {
	return func(httpRequest *http.Request, cassetteRequest cassette.Request) bool {
		// check the request URI and method
		if httpRequest.Method != cassetteRequest.Method ||
			(httpRequest.URL != nil && httpRequest.URL.String() != cassetteRequest.URL) {
			log.Debug(nil, map[string]interface{}{
				"httpRequest_method":     httpRequest.Method,
				"cassetteRequest_method": cassetteRequest.Method,
				"httpRequest_url":        httpRequest.URL,
				"cassetteRequest_url":    cassetteRequest.URL,
			}, "Cassette method/url doesn't match with the current request")
			return false
		}

		// look-up the JWT's "sub" claim and compare with the request
		token, err := jwtrequest.ParseFromRequest(httpRequest, jwtrequest.AuthorizationHeaderExtractor, func(*jwt.Token) (interface{}, error) {
			return testsupport.PublicKey("../test/public_key.pem")
		})
		if err != nil {
			log.Error(nil, map[string]interface{}{"error": err.Error()}, "failed to parse token from request")
			return false
		}
		claims := token.Claims.(jwt.MapClaims)
		if sub, found := cassetteRequest.Headers["sub"]; found {
			return sub[0] == claims["sub"]
		}
		return false
	}
}
