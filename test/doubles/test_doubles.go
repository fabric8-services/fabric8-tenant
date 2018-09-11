package testdoubles

import (
	"github.com/fabric8-services/fabric8-tenant/configuration"
)

func LoadTestConfig() (*configuration.Data, error) {
	data, err := configuration.GetData()
	data.Set(configuration.VarTemplateRecommenderExternalName, "recommender.api.prod-preview.openshift.io")
	data.Set(configuration.VarTemplateRecommenderAPIToken, "xxxx")
	data.Set(configuration.VarTemplateDomain, "d800.free-int.openshiftapps.com")
	return data, err
}
