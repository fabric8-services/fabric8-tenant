package metric

import (
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/prometheus/client_golang/prometheus"
	"strconv"
)

const (
	provisionedTenantsTotalName = "provisioned_tenants_total"
	cleanedTenantsTotalName     = "cleaned_tenants_total"
	updatedTenantsTotalName     = "updated_tenants_total"
	deletedTenantsTotalName     = "deleted_tenants_total"
)

var (
	ProvisionedTenantsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: provisionedTenantsTotalName,
		Help: "Total number of the provisioned tenants",
	}, []string{"successful"})
	DeletedTenantsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: deletedTenantsTotalName,
		Help: "Total number of the removed tenants",
	}, []string{"successful"})
	CleanedTenantsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: cleanedTenantsTotalName,
		Help: "Total number of cleaned tenants",
	}, []string{"successful", "nsBaseName"})
	UpdatedTenantsCounter = prometheus.NewCounterVec(prometheus.CounterOpts{
		Name: updatedTenantsTotalName,
		Help: "Total number of updated tenants",
	}, []string{"successful", "nsBaseName"})
)

func RegisterMetrics() {
	ProvisionedTenantsCounter = register(ProvisionedTenantsCounter, provisionedTenantsTotalName).(*prometheus.CounterVec)
	DeletedTenantsCounter = register(DeletedTenantsCounter, deletedTenantsTotalName).(*prometheus.CounterVec)
	CleanedTenantsCounter = register(CleanedTenantsCounter, cleanedTenantsTotalName).(*prometheus.CounterVec)
	UpdatedTenantsCounter = register(UpdatedTenantsCounter, updatedTenantsTotalName).(*prometheus.CounterVec)
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
			"metric_name": provisionedTenantsTotalName,
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
			"metric_name": cleanedTenantsTotalName,
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
			"metric_name": updatedTenantsTotalName,
			"successful":  successful,
			"nsBaseName":  nsBaseName,
			"err":         err,
		}, "Failed to get metric")
	} else {
		counter.Inc()
	}
}

func RecordDeletedTenant(successful bool) {
	if counter, err := DeletedTenantsCounter.GetMetricWithLabelValues(strconv.FormatBool(successful)); err != nil {
		log.Error(nil, map[string]interface{}{
			"metric_name": deletedTenantsTotalName,
			"successful":  successful,
			"err":         err,
		}, "Failed to get metric")
	} else {
		counter.Inc()
	}
}
