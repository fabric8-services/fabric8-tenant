package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/fabric8io/fabric8-init-tenant/keycloak"
	"github.com/fabric8io/fabric8-init-tenant/openshift"
)

const (
	headerAuthorization = "Authorization"
	headerContentType   = "Content-Type"
)

var (
	// Commit current build commit set by build script
	Commit = "0"
	// BuildTime set by build script in ISO 8601 (UTC) format: YYYY-MM-DDThh:mm:ssTZD (see https://www.w3.org/TR/NOTE-datetime for details)
	BuildTime = "0"
	// StartTime in ISO 8601 (UTC) format
	StartTime = time.Now().UTC().Format("2006-01-02T15:04:05Z")
)

type errorResponse struct {
	Msg string `json:"msg"`
}

type okResponse struct {
}

func status(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)

	type status struct {
		Commit    string `json:"commit"`
		BuildTime string `json:"buildTime"`
		StartTime string `json:"startTime"`
	}
	json.NewEncoder(w).Encode(&status{Commit: Commit, BuildTime: BuildTime, StartTime: StartTime})
}

func createTenant(keycloakConfig keycloak.Config, openshiftConfig openshift.Config) func(http.ResponseWriter, *http.Request) {

	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
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
		w.WriteHeader(http.StatusOK)
		fmt.Println("Execution time", time.Since(start))
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
	http.HandleFunc("/status", status)
	log.Println("Started listening on ", host)
	log.Fatal(http.ListenAndServe(host, nil))
}
