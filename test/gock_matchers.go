package test

import (
	"github.com/dgrijalva/jwt-go"
	jwtrequest "github.com/dgrijalva/jwt-go/request"
	"github.com/fabric8-services/fabric8-common/log"
	"gopkg.in/h2non/gock.v1"
	"net/http"
)

func ExpectRequest(matchers ...RequestMatcher) gock.Matcher {
	return createHeaderMatcher(matchers)
}

type RequestMatcher func(req *http.Request) bool

func createHeaderMatcher(matchers []RequestMatcher) gock.Matcher {
	matcher := gock.NewBasicMatcher()
	matcher.Add(func(req *http.Request, _ *gock.Request) (bool, error) {
		for _, match := range matchers {
			if !match(req) {
				return false, nil
			}
		}
		return true, nil
	})
	return matcher
}

func HasJWTWithSub(sub string) RequestMatcher {
	return func(req *http.Request) bool {
		// look-up the JWT's "sub" claim and compare with the request
		token, err := jwtrequest.ParseFromRequest(req, jwtrequest.AuthorizationHeaderExtractor, func(*jwt.Token) (interface{}, error) {
			return PublicKey("../test/public_key.pem")
		})
		if err != nil {
			log.Error(nil, map[string]interface{}{"error": err.Error(), "request_method": req.Method,
				"request_url": req.URL, "authorization_header": req.Header["Authorization"]}, "failed to parse token from request")
			return false
		}
		claims := token.Claims.(jwt.MapClaims)
		log.Debug(nil, map[string]interface{}{
			"req_method":        req.Method,
			"req_url":           req.URL,
			"req_sub":           req.Header.Get("sub"),
			"request_token_sub": claims["sub"],
		}, "comparing `sub` headers")

		return claims["sub"] == sub
	}
}
