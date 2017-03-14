package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strings"

	"github.com/fabric8io/fabric8-init-tenant/keycloak"
	"github.com/fabric8io/fabric8-init-tenant/openshift"
)

const (
	headerAuthorization = "Authorization"
	headerContentType   = "Content-Type"
)

type errorResponse struct {
	Msg string `json:"msg"`
}

type okResponse struct {
}

var (
	namespaces = []string{
		"%s-development",
		"%s-testing",
		"%s-staging",
		"%s-runtime",
		"%s-dsaas-jenkins",
		"%s-dsaas-che",
	}
)

func createTenant(keycloakConfig keycloak.Config, openshiftConfig openshift.Config) func(http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set(headerContentType, "application/json")
		if r.Method != http.MethodPost {
			w.WriteHeader(http.StatusMethodNotAllowed)
			json.NewEncoder(w).Encode(&errorResponse{Msg: "Only POST allowed"})
			return
		}
		authorization := r.Header.Get(headerAuthorization)
		if authorization == "" {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(&errorResponse{Msg: "require '" + headerAuthorization + "' header"})
			return
		}
		authorization = strings.Replace(authorization, "Bearer ", "", -1)
		openshiftUserToken, err := keycloak.OpenshiftToken(keycloakConfig, authorization)
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(&errorResponse{Msg: "require openshift token"})
			return
		}
		openshiftUser, err := openshift.WhoAmI(openshift.Config{MasterURL: openshiftConfig.MasterURL, Token: openshiftUserToken})
		if err != nil {
			w.WriteHeader(http.StatusUnauthorized)
			json.NewEncoder(w).Encode(&errorResponse{Msg: "unknown/unauthorized openshift user"})
			return
		}

		err = openshift.InitTenant(openshiftConfig, openshiftUser)
		if err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			json.NewEncoder(w).Encode(&errorResponse{Msg: err.Error()})
		}
	}

}

func main() {

	kcBaseURL := os.Getenv("KEYCLOAK_BASE_URL")
	if kcBaseURL == "" {
		kcBaseURL = "http://sso.prod-preview.openshift.io"
	}
	kcc := keycloak.Config{
		BaseURL: kcBaseURL,
		Realm:   "fabric8",
		Broker:  "openshift-v3",
	}

	osURL := os.Getenv("OPENSHIFT_URL")
	if osURL == "" {
		osURL = "https://tsrv.devshift.net:8443"
	}
	osServiceToken := os.Getenv("OPENSHIFT_SERVICE_TOKEN")
	if osServiceToken == "" {
		panic("Missing env variable OPENSHIFT_SERVICE_TOKEN")
	}

	osc := openshift.Config{
		MasterURL: osURL,
		Token:     osServiceToken,
	}

	host := ":8080"

	http.HandleFunc("/init", createTenant(kcc, osc))
	log.Println("Started listening on ", host)
	log.Fatal(http.ListenAndServe(host, nil))
}
