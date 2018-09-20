package controller

import (
	"context"
	"testing"
	"time"

	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/tenant"

	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/require"
)

var resolveCluster = func(ctx context.Context, target string) (cluster.Cluster, error) {
	return cluster.Cluster{
		AppDNS:            "apps.example.com",
		ConsoleURL:        "https://console.example.com/console",
		MetricsURL:        "https://metrics.example.com",
		LoggingURL:        "https://console.example.com/console", // not a typo; logging and console are on the same host
		CapacityExhausted: true,
	}, nil
}

func namespaces(ns1, ns2 time.Time) []*tenant.Namespace {
	return []*tenant.Namespace{
		{
			CreatedAt: ns1,
			UpdatedAt: ns1,
			MasterURL: "http://test1.org",
			Name:      "test-che",
			Type:      tenant.TypeChe,
			Version:   "1.0",
			State:     "created",
		},
		{
			CreatedAt: ns2,
			UpdatedAt: ns2,
			MasterURL: "http://test2.org",
			Name:      "test-jenkins",
			Type:      tenant.TypeJenkins,
			Version:   "1.0",
			State:     "created",
		},
	}
}

func Test_convertTenant(t *testing.T) {
	ctx := context.Background()

	tenantCreated := time.Now()
	tenantID := uuid.NewV4()
	tenant := &tenant.Tenant{
		ID:        tenantID,
		Email:     "q@x.com",
		Profile:   "Q X",
		CreatedAt: tenantCreated,
	}

	ns1Time, ns2Time := time.Now(), time.Now()
	namespaces := namespaces(ns1Time, ns2Time)
	want := &app.Tenant{
		ID:   &tenantID,
		Type: "tenants",
		Attributes: &app.TenantAttributes{
			CreatedAt: &tenantCreated,
			Email:     strToPtr("q@x.com"),
			Profile:   strToPtr("Q X"),
			Namespaces: []*app.NamespaceAttributes{
				{
					CreatedAt:         &ns1Time,
					UpdatedAt:         &ns1Time,
					ClusterURL:        strToPtr("http://test1.org"),
					ClusterAppDomain:  strToPtr("apps.example.com"),
					ClusterConsoleURL: strToPtr("https://console.example.com/console"),
					ClusterMetricsURL: strToPtr("https://metrics.example.com"),
					ClusterLoggingURL: strToPtr("https://console.example.com/console"),
					Name:              strToPtr("test-che"),
					Type:              strToPtr("che"),
					Version:           strToPtr("1.0"),
					State:             strToPtr("created"),
					ClusterCapacityExhausted: boolToPtr(true),
				},
				{
					CreatedAt:         &ns2Time,
					UpdatedAt:         &ns2Time,
					ClusterURL:        strToPtr("http://test2.org"),
					ClusterAppDomain:  strToPtr("apps.example.com"),
					ClusterConsoleURL: strToPtr("https://console.example.com/console"),
					ClusterMetricsURL: strToPtr("https://metrics.example.com"),
					ClusterLoggingURL: strToPtr("https://console.example.com/console"),
					Name:              strToPtr("test-jenkins"),
					Type:              strToPtr("jenkins"),
					Version:           strToPtr("1.0"),
					State:             strToPtr("created"),
					ClusterCapacityExhausted: boolToPtr(true),
				},
			},
		},
	}

	got := convertTenant(ctx, tenant, namespaces, resolveCluster)
	require.Equal(t, want, got)
}

func strToPtr(s string) *string {
	return &s
}

func boolToPtr(b bool) *bool {
	return &b
}
