package metric

import (
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

var (
	ProvisionedTenantsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "provisioned_tenants",
		Help: "Total number of the provisioned tenants",
	}, []string{"successful"})
	CleanedTenantsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "cleaned_tenants",
		Help: "Total number of cleaned tenants",
	}, []string{"successful", "nsBaseName"})
	UpdatedTenantsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: "updated_tenants",
		Help: "Total number of updated tenants",
	}, []string{"successful", "nsBaseName"})
)

func RegisterMetrics() {
	ProvisionedTenantsCounter = register(ProvisionedTenantsCounter, "provisioned_tenants").(*prometheus.CounterVec)
	CleanedTenantsCounter = register(CleanedTenantsCounter, "cleaned_tenants").(*prometheus.CounterVec)
	UpdatedTenantsCounter = register(UpdatedTenantsCounter, "updated_tenants").(*prometheus.CounterVec)
	log.Info(nil, nil, "metrics registered successfully")
}

func register(collector prometheus.Collector, name string) prometheus.Collector {
	err := prometheus.Register(collector)
	if err != nil {
		if are, ok := err.(prometheus.AlreadyRegisteredError); ok {
			return are.ExistingCollector
		}
		log.Panic(nil, map[string]interface{}{
			"metric_name": name,
			"err":         err,
		}, "failed to register the prometheus metric")
	}
	log.Info(nil, map[string]interface{}{
		"metric_name": name,
	}, "metric registered successfully")
	return collector
}

func RecordProvisionedTenant(successful bool) {
	if counter, err := ProvisionedTenantsCounter.GetMetricWithLabelValues(strconv.FormatBool(successful)); err != nil {
		log.Error(nil, map[string]interface{}{
			"metric_name": "provisioned_tenants",
			"successful":  successful,
			"err":         err,
		}, "Failed to get metric")
	} else {
		counter.Inc()
	}
}

func RecordCleanedTenant(successful bool, nsBaseName string) {
	if counter, err := CleanedTenantsCounter.GetMetricWithLabelValues(strconv.FormatBool(successful), nsBaseName); err != nil {
		log.Error(nil, map[string]interface{}{
			"metric_name": "cleaned_tenants",
			"successful":  successful,
			"nsBaseName":  nsBaseName,
			"err":         err,
		}, "Failed to get metric")
	} else {
		counter.Inc()
	}
}

func RecordUpdatedTenant(successful bool, nsBaseName string) {
	if counter, err := UpdatedTenantsCounter.GetMetricWithLabelValues(strconv.FormatBool(successful), nsBaseName); err != nil {
		log.Error(nil, map[string]interface{}{
			"metric_name": "updated_tenants",
			"successful":  successful,
			"nsBaseName":  nsBaseName,
			"err":         err,
		}, "Failed to get metric")
	} else {
		counter.Inc()
	}
}
