package testdoubles

import (
	"github.com/fabric8-services/fabric8-tenant/configuration"
)

func LoadTestConfig() (*configuration.Data, error) {
	data, err := configuration.GetData()
	data.Set("template.recommender.external.name", "recommender.api.prod-preview.openshift.io")
	data.Set("template.recommender.api.token", "xxxx")
	data.Set("template.domain", "d800.free-int.openshiftapps.com")
	return data, err
}
