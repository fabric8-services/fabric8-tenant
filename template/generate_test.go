package template_test

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/template"
	"github.com/stretchr/testify/assert"
	yaml "gopkg.in/yaml.v2"
)

func TestFoundJenkins(t *testing.T) {
	c, err := template.Asset("template/fabric8-tenant-jenkins-openshift.yml")
	if err != nil {
		t.Fatalf("Asset template/fabric8-tenant-jenkins-openshift.yml not found")
	}

	cs := string(c)
	if !strings.Contains(cs, "jenkins") {
		t.Fatalf("Word jenkins not found in the template")
	}

	var template map[interface{}]interface{}
	err = yaml.Unmarshal(c, &template)
	if err != nil {
		t.Fatalf("Could not parse template as yaml")
	}

	params, ok := template["parameters"].([]interface{})
	if !ok {
		t.Fatalf("parameters not found")
	}

	assert.Equal(t, 6, len(params), "unknown number of parameters")
}
func TestFoundJenkinsKnownTypes(t *testing.T) {
	verifyKindMapping(t, "template/fabric8-tenant-jenkins-openshift.yml")
}

func TestFoundJenkinsQuotasOSO(t *testing.T) {
	c, err := template.Asset("template/fabric8-tenant-jenkins-quotas-oso-openshift.yml")
	if err != nil {
		t.Fatalf("Asset template/fabric8-tenant-jenkins-quotas-oso-openshift.yml not found")
	}

	cs := string(c)
	if !strings.Contains(cs, "Limit") {
		t.Fatalf("Word Limit not found in the resource")
	}

	var template map[interface{}]interface{}
	err = yaml.Unmarshal(c, &template)
	if err != nil {
		t.Fatalf("Could not parse resource as yaml")
	}
}
func TestFoundJenkinsQuotasOSOKnownTypes(t *testing.T) {
	verifyKindMapping(t, "template/fabric8-tenant-jenkins-quotas-oso-openshift.yml")
}

func TestFoundChe(t *testing.T) {
	c, err := template.Asset("template/fabric8-tenant-che-openshift.yml")
	if err != nil {
		t.Fatalf("Asset template/fabric8-tenant-che-openshift.yml not found")
	}

	cs := string(c)
	if !strings.Contains(cs, "che") {
		t.Fatalf("Word che not found in the template")
	}

	var template map[interface{}]interface{}
	err = yaml.Unmarshal(c, &template)
	if err != nil {
		t.Fatalf("Could not parse template as yaml")
	}

	params, ok := template["parameters"].([]interface{})
	if !ok {
		t.Fatalf("parameters not found")
	}

	assert.Equal(t, 10, len(params), "unknown number of parameters")
}

func TestFoundCheKnownTypes(t *testing.T) {
	verifyKindMapping(t, "template/fabric8-tenant-che-openshift.yml")
}

func TestFoundCheMultiTenant(t *testing.T) {
	c, err := template.Asset("template/fabric8-tenant-che-mt-openshift.yml")
	if err != nil {
		t.Fatalf("Asset template/fabric8-tenant-che-mt-openshift.yml not found")
	}

	cs := string(c)
	if !strings.Contains(cs, "claim-che-workspace") {
		t.Fatalf("Word claim-che-workspace not found in the template")
	}

	var template map[interface{}]interface{}
	err = yaml.Unmarshal(c, &template)
	if err != nil {
		t.Fatalf("Could not parse template as yaml")
	}

	params, ok := template["parameters"].([]interface{})
	if !ok {
		t.Fatalf("parameters not found")
	}

	assert.Equal(t, 6, len(params), "unknown number of parameters")
}

func TestFoundCheMultiTenantKnownTypes(t *testing.T) {
	verifyKindMapping(t, "template/fabric8-tenant-che-mt-openshift.yml")
}

func TestFoundCheQuotasOSO(t *testing.T) {
	c, err := template.Asset("template/fabric8-tenant-che-quotas-oso-openshift.yml")
	if err != nil {
		t.Fatalf("Asset template/fabric8-tenant-che-quotas-oso-openshift.yml not found")
	}

	cs := string(c)
	if !strings.Contains(cs, "Limit") {
		t.Fatalf("Word Limit not found in the resource")
	}

	var template map[interface{}]interface{}
	err = yaml.Unmarshal(c, &template)
	if err != nil {
		t.Fatalf("Could not parse resource as yaml")
	}
}

func TestFoundCheQuotasOSOKnownTypes(t *testing.T) {
	verifyKindMapping(t, "template/fabric8-tenant-che-quotas-oso-openshift.yml")
}

func TestFoundTeam(t *testing.T) {
	c, err := template.Asset("template/fabric8-tenant-team-openshift.yml")
	if err != nil {
		t.Fatalf("Asset template/fabric8-tenant-team-openshift.yml not found")
	}

	cs := string(c)
	if !strings.Contains(cs, "team") {
		t.Fatalf("Word team not found in the template")
	}

	var template map[interface{}]interface{}
	err = yaml.Unmarshal(c, &template)
	if err != nil {
		t.Fatalf("Could not parse template as yaml")
	}

	params, ok := template["parameters"].([]interface{})
	if !ok {
		t.Fatalf("parameters not found")
	}
	// 1 parameter not used in Openshift templates but bleed through from k8
	assert.Equal(t, 8, len(params), "unknown number of parameters")
}

func TestFoundTeamKnownTypes(t *testing.T) {
	verifyKindMapping(t, "template/fabric8-tenant-team-openshift.yml")
}

func TestStatusAPIJenkins(t *testing.T) {
	assert.NoError(t,
		contain(templates(t),
			openshift.ValKindDeploymentConfig,
			withSpecLabel("app", "jenkins"),
			withNamespaceLike("-jenkins")))
}

func TestStatusAPIChe(t *testing.T) {
	assert.NoError(t,
		contain(templates(t),
			openshift.ValKindDeploymentConfig,
			withSpecLabel("app", "che"),
			withNamespaceLike("-che")))
}

func templates(t *testing.T) []map[interface{}]interface{} {
	templs, err := openshift.LoadProcessedTemplates(context.Background(), openshift.Config{MasterUser: "master"}, "test", map[string]string{})
	assert.NoError(t, err)
	return templs
}

func contain(templtes []map[interface{}]interface{}, kind string, checks ...func(map[interface{}]interface{}) error) error {
	var err error
	for _, temp := range templtes {
		if openshift.GetKind(temp) == kind {
			err = nil
			for _, check := range checks {
				if e := check(temp); e != nil {
					err = e
				}
			}
			if err == nil {
				return nil
			}
		}
	}
	return fmt.Errorf("No template of kind %v found, cause %v", kind, err)
}

func withSpecLabel(name, value string) func(map[interface{}]interface{}) error {
	return func(temp map[interface{}]interface{}) error {
		val := openshift.GetLabel(openshift.GetTemplate(openshift.GetSpec(temp)), name)
		if val == value {
			return nil
		}
		return fmt.Errorf("No label named %v with value %v found", name, value)
	}
}

func withNamespaceLike(name string) func(map[interface{}]interface{}) error {
	return func(temp map[interface{}]interface{}) error {
		val := openshift.GetNamespace(temp)
		if strings.HasSuffix(val, name) {
			return nil
		}
		return fmt.Errorf("No namespace match for %v found", name)
	}
}

func verifyKindMapping(t *testing.T, templateName string) {
	c, err := template.Asset(templateName)
	if err != nil {
		t.Fatalf("Asset " + templateName + " not found")
	}
	templ, err := openshift.ProcessTemplate(string(c), "test", map[string]string{})
	if err != nil {
		t.Error(err)
	}
	for _, object := range templ {
		var url string
		var err error
		url, err = openshift.CreateURL("http://localhost:8080", "POST", object)
		if err != nil || url == "" {
			t.Error(err, "POST not found for "+openshift.GetKind(object))
		}
		if openshift.GetKind(object) == "ProjectRequest" { // ProjectRequests only support GET
			continue
		}

		url, err = openshift.CreateURL("http://localhost:8080", "GET", object)
		if err != nil || url == "" {
			t.Error(err, "GET not found for "+openshift.GetKind(object))
		}
		url, err = openshift.CreateURL("http://localhost:8080", "PUT", object)
		if err != nil || url == "" {
			t.Error(err, "PUT not found for "+openshift.GetKind(object))
		}
		url, err = openshift.CreateURL("http://localhost:8080", "PATCH", object)
		if err != nil || url == "" {
			t.Error(err, "PATCH not found for "+openshift.GetKind(object))
		}
		url, err = openshift.CreateURL("http://localhost:8080", "DELETE", object)
		if err != nil || url == "" {
			t.Error(err, "DELETE not found for "+openshift.GetKind(object))
		}
	}
}
