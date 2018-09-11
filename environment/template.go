package environment

import (
	"fmt"
	authClient "github.com/fabric8-services/fabric8-tenant/auth/client"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/keycloak"
	"gopkg.in/yaml.v2"
	"regexp"
	"strings"
)

const (
	FieldKind            = "kind"
	FieldAPIVersion      = "apiVersion"
	FieldObjects         = "objects"
	FieldSpec            = "spec"
	FieldTemplate        = "templateDef"
	FieldItems           = "items"
	FieldMetadata        = "metadata"
	FieldLabels          = "labels"
	FieldReplicas        = "replicas"
	FieldVersion         = "version"
	FieldNamespace       = "namespace"
	FieldName            = "name"
	FieldStatus          = "status"
	FieldResourceVersion = "resourceVersion"

	ValKindTemplate               = "Template"
	ValKindNamespace              = "Namespace"
	ValKindConfigMap              = "ConfigMap"
	ValKindLimitRange             = "LimitRange"
	ValKindProject                = "Project"
	ValKindProjectRequest         = "ProjectRequest"
	ValKindPersistenceVolumeClaim = "PersistentVolumeClaim"
	ValKindService                = "Service"
	ValKindSecret                 = "Secret"
	ValKindServiceAccount         = "ServiceAccount"
	ValKindRoleBindingRestriction = "RoleBindingRestriction"
	ValKindRoleBinding            = "RoleBinding"
	ValKindRoute                  = "Route"
	ValKindJob                    = "Job"
	ValKindList                   = "List"
	ValKindDeployment             = "Deployment"
	ValKindDeploymentConfig       = "DeploymentConfig"
	ValKindResourceQuota          = "ResourceQuota"

	varUserName              = "USER_NAME"
	varProjectUser           = "PROJECT_USER"
	varProjectRequestingUser = "PROJECT_REQUESTING_USER"
	varProjectAdminUser      = "PROJECT_ADMIN_USER"
	varKeycloakURL           = "KEYCLOAK_URL"
	varCommit                = "COMMIT"
	varKeycloakOsoEndpoint   = "KEYCLOAK_OSO_ENDPOINT"
	varKeycloakGHEndpoint    = "KEYCLOAK_GITHUB_ENDPOINT"
)

var sortOrder = map[string]int{
	"Namespace":              1,
	"ProjectRequest":         1,
	"RoleBindingRestriction": 2,
	"LimitRange":             3,
	"ResourceQuota":          4,
	"Secret":                 5,
	"ServiceAccount":         6,
	"Service":                7,
	"RoleBinding":            8,
	"PersistentVolumeClaim":  9,
	"ConfigMap":              10,
	"DeploymentConfig":       11,
	"Route":                  12,
	"Job":                    13,
}

type Objects []map[interface{}]interface{}
type Object map[interface{}]interface{}

type Template struct {
	Filename      string
	DefaultParams map[string]string
	Content       string
	Version       string
}

var (
	stageParams = map[string]string{"DEPLOY_TYPE": "stage"}
	runParams   = map[string]string{"DEPLOY_TYPE": "run"}
	noParams    map[string]string
)

func newTemplate(filename string, defaultParams map[string]string) *Template {
	return &Template{
		Filename:      filename,
		DefaultParams: defaultParams,
	}
}

func (t *Template) Process(vars map[string]string) (Objects, error) {
	var objects Objects
	templateVars := merge(vars, t.DefaultParams)
	pt, err := t.ReplaceVars(templateVars)
	if err != nil {
		return objects, err
	}
	t.Version = vars[varCommit]
	return ParseObjects(pt)
}

// Process takes a K8/Openshift Template as input and resolves the variable expresions
func (t *Template) ReplaceVars(variables map[string]string) (string, error) {
	reg := regexp.MustCompile(`\${([A-Z_0-9]+)}`)
	return string(reg.ReplaceAllFunc([]byte(t.Content), func(found []byte) []byte {
		variableName := toVariableName(string(found))
		if variable, ok := variables[variableName]; ok {
			return []byte(variable)
		}
		return found
	})), nil
}

func CollectVars(user, masterUser, commit string, config *configuration.Data) map[string]string {
	userName := RetrieveUserName(user)

	vars := map[string]string{
		varUserName:              userName,
		varProjectUser:           user,
		varProjectRequestingUser: user,
		varProjectAdminUser:      masterUser,
		varCommit:                commit,
	}

	return merge(vars, getVariables(config))
}

// RetrieveUserName returns a safe namespace basename based on a username
func RetrieveUserName(username string) string {
	return regexp.MustCompile("[^a-z0-9]").ReplaceAllString(strings.Split(username, "@")[0], "-")
}

func getVariables(config *configuration.Data) map[string]string {
	keycloakConfig := keycloak.Config{
		BaseURL: config.GetKeycloakURL(),
		Realm:   config.GetKeycloakRealm(),
		Broker:  config.GetKeycloakOpenshiftBroker(),
	}

	templateVars, err := config.GetTemplateValues()
	if err != nil {
		panic(err)
	}
	templateVars[varKeycloakURL] = ""
	templateVars[varKeycloakOsoEndpoint] = keycloakConfig.CustomBrokerTokenURL("openshift-v3")
	templateVars[varKeycloakGHEndpoint] = fmt.Sprintf("%s%s?for=https://github.com", config.GetAuthURL(), authClient.RetrieveTokenPath())

	return templateVars
}

func merge(target, second map[string]string) map[string]string {
	if len(second) == 0 {
		return target
	}
	result := clone(second)
	for k, v := range target {
		if _, exist := result[k]; !exist {
			result[k] = v
		}
	}
	return result
}

func clone(maps map[string]string) map[string]string {
	maps2 := make(map[string]string)
	for k2, v2 := range maps {
		maps2[k2] = v2
	}
	return maps2
}

func toVariableName(exp string) string {
	return exp[:len(exp)-1][2:]
}

// ParseObjects return a string yaml and return a array of the objects/items from a Template/List kind
func ParseObjects(source string) (Objects, error) {
	var template Object

	err := yaml.Unmarshal([]byte(source), &template)
	if err != nil {
		return nil, err
	}

	if GetKind(template) == ValKindTemplate || GetKind(template) == ValKindList {
		var ts []interface{}
		if GetKind(template) == ValKindTemplate {
			ts = template[FieldObjects].([]interface{})
		} else if GetKind(template) == ValKindList {
			ts = template[FieldItems].([]interface{})
		}
		var objs Objects
		for _, obj := range ts {
			parsedObj := obj.(Object)
			stringKeys := make(Object, len(parsedObj))
			for key, value := range parsedObj {
				stringKeys[key.(string)] = value
			}
			objs = append(objs, stringKeys)
		}
		return objs, nil
	}

	return Objects{template}, nil
}

func GetName(obj Object) string {
	if meta, metaFound := obj[FieldMetadata].(Object); metaFound {
		if name, nameFound := meta[FieldName].(string); nameFound {
			return name
		}
	}
	return ""
}

func GetNamespace(obj Object) string {
	if meta, metaFound := obj[FieldMetadata].(Object); metaFound {
		if name, nameFound := meta[FieldNamespace].(string); nameFound {
			return name
		}
	}
	return ""
}

func GetKind(obj Object) string {
	if kind, kindFound := obj[FieldKind].(string); kindFound {
		return kind
	}
	return ""
}

func HasValidStatus(obj Object) bool {
	return len(GetStatus(obj)) > 0
}

func GetStatus(obj Object) Object {
	if status, statusFound := obj[FieldStatus].(Object); statusFound {
		return status
	}
	return nil
}

func GetLabelVersion(obj Object) string {
	return GetLabel(obj, FieldVersion)
}

func GetLabel(obj Object, name string) string {
	if meta, metaFound := obj[FieldMetadata].(Object); metaFound {
		if labels, labelsFound := meta[FieldLabels].(Object); labelsFound {
			if version, versionFound := labels[name].(string); versionFound {
				return version
			}
		}
	}
	return ""
}

// ByKind represents a list of Openshift objects sortable by Kind
type ByKind Objects

func (a ByKind) Len() int      { return len(a) }
func (a ByKind) Swap(i, j int) { a[i], a[j] = a[j], a[i] }
func (a ByKind) Less(i, j int) bool {
	iO := 30
	jO := 30

	if val, ok := sortOrder[GetKind(a[i])]; ok {
		iO = val
	}
	if val, ok := sortOrder[GetKind(a[j])]; ok {
		jO = val
	}
	return iO < jO
}
