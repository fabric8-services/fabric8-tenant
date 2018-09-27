package openshift

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/template"
	"github.com/fabric8-services/fabric8-tenant/toggles"
	goajwt "github.com/goadesign/goa/middleware/security/jwt"
	"github.com/pkg/errors"
)

type FilterFunc func(map[interface{}]interface{}) bool

func Filter(vs []map[interface{}]interface{}, f FilterFunc) []map[interface{}]interface{} {
	vsf := make([]map[interface{}]interface{}, 0)
	for _, v := range vs {
		if f(v) {
			vsf = append(vsf, v)
		}
	}
	return vsf
}

func IsOfKind(kinds ...string) FilterFunc {
	return func(vs map[interface{}]interface{}) bool {
		kind := GetKind(vs)
		for _, k := range kinds {
			if k == kind {
				return true
			}
		}
		return false
	}
}

func IsNotOfKind(kinds ...string) FilterFunc {
	f := IsOfKind(kinds...)
	return func(vs map[interface{}]interface{}) bool {
		return !f(vs)
	}
}

func RemoveReplicas(vs []map[interface{}]interface{}) []map[interface{}]interface{} {
	vsf := make([]map[interface{}]interface{}, 0)
	for _, v := range vs {
		if GetKind(v) == ValKindDeploymentConfig {
			if spec, specFound := v[FieldSpec].(map[interface{}]interface{}); specFound {
				delete(spec, FieldReplicas)
			}
		}
		vsf = append(vsf, v)
	}
	return vsf

}

func ProcessTemplate(template, namespace string, vars map[string]string) ([]map[interface{}]interface{}, error) {
	pt, err := Process(template, vars)
	if err != nil {
		return nil, err
	}
	return ParseObjects(pt, namespace)
}

func LoadProcessedTemplates(ctx context.Context, config Config, username string, templateVars map[string]string) ([]map[interface{}]interface{}, error) {
	var objs []map[interface{}]interface{}
	name := CreateName(username)

	vars := map[string]string{
		varProjectName:           name,
		varProjectTemplateName:   name,
		varProjectDisplayName:    name,
		varProjectDescription:    name,
		varProjectUser:           username,
		varProjectRequestingUser: username,
		varProjectAdminUser:      config.MasterUser,
	}

	for k, v := range templateVars {
		if _, exist := vars[k]; !exist {
			vars[k] = v
		}
	}

	userProjectT, err := loadTemplate(config, "fabric8-tenant-user-project-openshift.yml")
	if err != nil {
		return nil, err
	}

	userProjectRolesT, err := loadTemplate(config, "fabric8-tenant-user-rolebindings.yml")
	if err != nil {
		return nil, err
	}

	userProjectCollabT, err := loadTemplate(config, "fabric8-tenant-user-colaborators.yml")
	if err != nil {
		return nil, err
	}

	projectT, err := loadTemplate(config, "fabric8-tenant-team-openshift.yml")
	if err != nil {
		return nil, err
	}

	jenkinsT, err := loadTemplate(config, "fabric8-tenant-jenkins-openshift.yml")
	if err != nil {
		return nil, err
	}

	cheType := "che"
	if toggles.IsEnabled(ctx, "deploy.che-multi-tenant", false) {
		token := goajwt.ContextJWT(ctx)
		if token != nil {
			vars["OSIO_TOKEN"] = token.Raw
			id := token.Claims.(jwt.MapClaims)["sub"]
			if id == nil {
				return nil, errors.New("Missing sub in JWT token")
			}
			vars["IDENTITY_ID"] = id.(string)
		}
		vars["REQUEST_ID"] = log.ExtractRequestID(ctx)
		unixNano := time.Now().UnixNano()
		vars["JOB_ID"] = strconv.FormatInt(unixNano/1000000, 10)
		cheType = "che-mt"
	}

	cheT, err := loadTemplate(config, fmt.Sprintf("fabric8-tenant-%s-openshift.yml", cheType))
	if err != nil {
		return nil, err
	}

	processed, err := ProcessTemplate(string(userProjectT), name, vars)
	if err != nil {
		return nil, err
	}
	objs = append(objs, processed...)

	{
		processed, err = ProcessTemplate(string(userProjectCollabT), name, vars)
		if err != nil {
			return nil, err
		}
		objs = append(objs, processed...)

		processed, err = ProcessTemplate(string(userProjectRolesT), name, vars)
		if err != nil {
			return nil, err
		}
		objs = append(objs, processed...)
	}

	{
		lvars := clone(vars)
		lvars[varProjectDisplayName] = lvars[varProjectName]

		processed, err = ProcessTemplate(string(projectT), name, lvars)
		if err != nil {
			return nil, err
		}
		objs = append(objs, processed...)
	}

	// Quotas needs to be applied before we attempt to install the resources on OSO
	osoQuotas := true
	disableOsoQuotasFlag := os.Getenv("DISABLE_OSO_QUOTAS")
	if disableOsoQuotasFlag == "true" {
		osoQuotas = false
	}
	if osoQuotas {
		jenkinsQuotasT, err := loadTemplate(config, "fabric8-tenant-jenkins-quotas-oso-openshift.yml")
		if err != nil {
			return nil, err
		}
		cheQuotasT, err := loadTemplate(config, "fabric8-tenant-che-quotas-oso-openshift.yml")
		if err != nil {
			return nil, err
		}

		{
			lvars := clone(vars)
			nsname := fmt.Sprintf("%v-jenkins", name)
			lvars[varProjectNamespace] = vars[varProjectName]
			processed, err = ProcessTemplate(string(jenkinsQuotasT), nsname, lvars)
			if err != nil {
				return nil, err
			}
			objs = append(objs, processed...)
		}
		{
			lvars := clone(vars)
			nsname := fmt.Sprintf("%v-che", name)
			lvars[varProjectNamespace] = vars[varProjectName]
			processed, err = ProcessTemplate(string(cheQuotasT), nsname, lvars)
			if err != nil {
				return nil, err
			}
			objs = append(objs, processed...)
		}
	}

	{
		lvars := clone(vars)
		nsname := fmt.Sprintf("%v-jenkins", name)
		lvars[varProjectNamespace] = vars[varProjectName]
		processed, err = ProcessTemplate(string(jenkinsT), nsname, lvars)
		if err != nil {
			return nil, err
		}
		objs = append(objs, processed...)
	}
	{
		lvars := clone(vars)
		nsname := fmt.Sprintf("%v-che", name)
		lvars[varProjectNamespace] = vars[varProjectName]
		processed, err = ProcessTemplate(string(cheT), nsname, lvars)
		if err != nil {
			return nil, err
		}
		objs = append(objs, processed...)
	}

	return objs, nil
}

func MapByNamespaceAndSort(objs []map[interface{}]interface{}) (map[string][]map[interface{}]interface{}, error) {
	ns := map[string][]map[interface{}]interface{}{}
	for _, obj := range objs {
		namespace := GetNamespace(obj)
		if namespace == "" {
			// ProjectRequests and Namespaces are not bound to a Namespace, as it's a Namespace request
			kind := GetKind(obj)
			if kind == ValKindProjectRequest || kind == ValKindNamespace {
				namespace = GetName(obj)
			} else {
				return nil, fmt.Errorf("Object is missing namespace %v", obj)
			}
		}

		if objects, found := ns[namespace]; found {
			objects = append(objects, obj)
			ns[namespace] = objects
		} else {
			objects = []map[interface{}]interface{}{obj}
			ns[namespace] = objects
		}
	}

	for key, val := range ns {
		sort.Sort(ByKind(val))
		ns[key] = val
	}
	return ns, nil
}

// CreateName returns a safe namespace basename based on a username
func CreateName(username string) string {
	return regexp.MustCompile("[^a-z0-9]").ReplaceAllString(strings.Split(username, "@")[0], "-")
}

// loadTemplate will load the template for a specific version from maven central or from the template directory
// or default to the OOTB template included
func loadTemplate(config Config, name string) ([]byte, error) {
	mavenRepo := config.MavenRepoURL
	if mavenRepo == "" {
		mavenRepo = os.Getenv("YAML_MVN_REPO")
	}
	if mavenRepo == "" {
		mavenRepo = "http://central.maven.org/maven2"
	}
	logCallback := config.GetLogCallback()
	url := ""
	if len(config.CheVersion) > 0 {
		switch name {
		// che-mt
		case "fabric8-tenant-che-mt-openshift.yml":
			url = "$MVN_REPO/io/fabric8/tenant/packages/fabric8-tenant-che-mt/$CHE_VERSION/fabric8-tenant-che-mt-$CHE_VERSION-openshift.yml"
		// che
		case "fabric8-tenant-che-openshift.yml":
			url = "$MVN_REPO/io/fabric8/tenant/packages/fabric8-tenant-che/$CHE_VERSION/fabric8-tenant-che-$CHE_VERSION-openshift.yml"
		case "fabric8-tenant-che-quotas-oso-openshift.yml":
			url = "$MVN_REPO/io/fabric8/tenant/packages/fabric8-tenant-che-quotas-oso/$CHE_VERSION/fabric8-tenant-che-quotas-oso-$CHE_VERSION-openshift.yml"
		}
		if len(url) > 0 {
			return replaceVariablesAndLoadURL(config, url, mavenRepo)
		}
	}

	if len(config.JenkinsVersion) > 0 {
		switch name {
		case "fabric8-tenant-jenkins-openshift.yml":
			url = "$MVN_REPO/io/fabric8/tenant/packages/fabric8-tenant-jenkins/$JENKINS_VERSION/fabric8-tenant-jenkins-$JENKINS_VERSION-openshift.yml"
		case "fabric8-tenant-jenkins-quotas-oso-openshift.yml":
			url = "$MVN_REPO/io/fabric8/tenant/packages/fabric8-tenant-jenkins-quotas-oso/$JENKINS_VERSION/fabric8-tenant-jenkins-quotas-oso-$JENKINS_VERSION-openshift.yml"
		}
		if len(url) > 0 {
			return replaceVariablesAndLoadURL(config, url, mavenRepo)
		}
	}

	if len(config.TeamVersion) > 0 {
		switch name {
		case "fabric8-tenant-team-openshift.yml":
			url = "$MVN_REPO/io/fabric8/tenant/packages/fabric8-tenant-team/$TEAM_VERSION/fabric8-tenant-team-$TEAM_VERSION-openshift.yml"
		}
		if len(url) > 0 {
			return replaceVariablesAndLoadURL(config, url, mavenRepo)
		}
	}
	dir := config.TemplateDir
	if len(dir) > 0 {
		fullName := filepath.Join(dir, name)
		d, err := os.Stat(fullName)
		if err == nil {
			if m := d.Mode(); m.IsRegular() {
				logCallback(fmt.Sprintf("Loading template from file: %s", fullName))
				return ioutil.ReadFile(fullName)
			}
		}
	}
	return template.Asset("template/" + name)
}
func replaceVariablesAndLoadURL(config Config, urlExpression string, mavenRepo string) ([]byte, error) {
	logCallback := config.GetLogCallback()
	cheVersion := config.CheVersion
	jenkinsVersion := config.JenkinsVersion
	teamVersion := config.TeamVersion

	url := strings.Replace(urlExpression, "$MVN_REPO", mavenRepo, -1)
	url = strings.Replace(url, "$CHE_VERSION", cheVersion, -1)
	url = strings.Replace(url, "$JENKINS_VERSION", jenkinsVersion, -1)
	url = strings.Replace(url, "$TEAM_VERSION", teamVersion, -1)
	logCallback(fmt.Sprintf("Loading template from URL: %s", url))
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("Failed to load template from %s due to: %v", url, err)
	}
	defer resp.Body.Close()
	statusCode := resp.StatusCode
	if statusCode >= 300 {
		return nil, fmt.Errorf("Failed to GET template from %s got status code to: %d", url, statusCode)
	}
	return ioutil.ReadAll(resp.Body)
}

func executeProccessedNamespaceCMD(t string, opts ApplyOptions) (string, error) {
	hostVerify := ""
	flag := os.Getenv("KEYCLOAK_SKIP_HOST_VERIFY")
	if strings.ToLower(flag) == "true" {
		hostVerify = " --insecure-skip-tls-verify=true"
	}
	serverFlag := "--server=" + opts.MasterURL + hostVerify
	cmdArgs := []string{"-c", "oc process -f - " + serverFlag + " --token=" + opts.Token + " --namespace=" + opts.Namespace + " | oc apply -f -  --overwrite=true --force=true --server=" + opts.MasterURL + hostVerify + " --token=" + opts.Token + " --namespace=" + opts.Namespace}
	return executeCMD(&t, cmdArgs)
}

func executeCMD(input *string, cmdArgs []string) (string, error) {
	cmdName := "/usr/bin/sh"

	var buf bytes.Buffer
	cmd := exec.Command(cmdName, cmdArgs...)
	cmd.Stdout = &buf
	cmd.Stderr = &buf

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return "", err
	}

	if input != nil {
		go func() {
			defer stdin.Close()
			io.WriteString(stdin, *input)

		}()
	}
	if err := cmd.Start(); err != nil {
		return "", err
	}

	if err := cmd.Wait(); err != nil {
		return buf.String(), err
	}

	return buf.String(), nil
}

func clone(maps map[string]string) map[string]string {
	maps2 := make(map[string]string)
	for k2, v2 := range maps {
		maps2[k2] = v2
	}
	return maps2
}
