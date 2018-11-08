package test

import (
	"github.com/dgrijalva/jwt-go"
	jwtrequest "github.com/dgrijalva/jwt-go/request"
	"github.com/fabric8-services/fabric8-common/log"
	"gopkg.in/h2non/gock.v1"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"net/http"
	"regexp"
)

func ExpectRequest(matchers ...gock.MatchFunc) gock.Matcher {
	return createReqMatcher(matchers)
}

type RequestMatcher func(req *http.Request) bool

func createReqMatcher(matchers []gock.MatchFunc) gock.Matcher {
	matcher := gock.NewBasicMatcher()
	matcher.Add(func(req *http.Request, gockReq *gock.Request) (bool, error) {
		for _, match := range matchers {
			ok, err := match(req, gockReq)
			if err != nil {
				return ok, err

			}
			if !ok {
				return false, nil
			}
		}
		return true, nil
	})
	return matcher
}

func HasJWTWithSub(sub string) gock.MatchFunc {
	return func(req *http.Request, gockReq *gock.Request) (bool, error) {
		// look-up the JWT's "sub" claim and compare with the request
		token, err := jwtrequest.ParseFromRequest(req, jwtrequest.AuthorizationHeaderExtractor, func(*jwt.Token) (interface{}, error) {
			return PublicKey("../test/public_key.pem")
		})
		if err != nil {
			log.Error(nil, map[string]interface{}{"error": err.Error(), "request_method": req.Method,
				"request_url": req.URL, "authorization_header": req.Header["Authorization"]}, "failed to parse token from request")
			return false, err
		}
		claims := token.Claims.(jwt.MapClaims)
		log.Debug(nil, map[string]interface{}{
			"req_method":        req.Method,
			"req_url":           req.URL,
			"req_sub":           req.Header.Get("sub"),
			"request_token_sub": claims["sub"],
		}, "comparing `sub` headers")

		return claims["sub"] == sub, nil
	}
}

func HasBodyContainingObject(object map[interface{}]interface{}) gock.MatchFunc {
	return func(req *http.Request, gockReq *gock.Request) (bool, error) {
		body, err := ioutil.ReadAll(req.Body)
		if err != nil {
			log.Error(nil, map[string]interface{}{"body": string(body)}, err.Error())
			return false, err
		}
		expBody, err := yaml.Marshal(object)
		if err != nil {
			log.Error(nil, map[string]interface{}{"object": object}, err.Error())
			return false, err
		}
		return string(body) == string(expBody), nil
	}
}

func HasUrlMatching(regExp string) gock.MatchFunc {
	return func(req *http.Request, gockReq *gock.Request) (bool, error) {
		return regexp.MustCompile(regExp).MatchString(req.URL.String()), nil
	}
}

// SpyOnCalls checks the number of calls
func SpyOnCalls(counter *int) gock.Matcher {
	matcher := gock.NewBasicMatcher()
	matcher.Add(SpyOnCallsMatchFunc(counter))
	return matcher
}

func SpyOnCallsMatchFunc(counter *int) gock.MatchFunc {
	return func(req *http.Request, _ *gock.Request) (bool, error) {
		*counter++
		return true, nil
	}
}
