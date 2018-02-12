package openshift

import (
	"context"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	jwt "github.com/dgrijalva/jwt-go"
	"github.com/fabric8-services/fabric8-tenant/toggles"
	"github.com/fabric8-services/fabric8-wit/log"
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
