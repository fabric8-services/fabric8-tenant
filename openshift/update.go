package openshift

import (
	"context"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/pkg/errors"
)

type UpdateExecutor interface {
	Update(ctx context.Context, tenantService tenant.Service, openshiftConfig Config, t *tenant.Tenant, envTypes []environment.Type) (map[environment.Type]string, error)
}

func UpdateTenant(updateExecutor UpdateExecutor, ctx context.Context, tenantService tenant.Service, openshiftConfig Config, t *tenant.Tenant, envTypes ...environment.Type) error {
	versionMapping, err := updateExecutor.Update(ctx, tenantService, openshiftConfig, t, envTypes)
	if err != nil {
		updateNamespaceEntities(tenantService, t, versionMapping, true)
		return err
	}

	return updateNamespaceEntities(tenantService, t, versionMapping, false)
}

func updateNamespaceEntities(tenantService tenant.Service, t *tenant.Tenant, versionMapping map[environment.Type]string, failed bool) error {
	namespaces, err := tenantService.GetNamespaces(t.ID)
	if err != nil {
		return errors.Wrapf(err, "unable to get tenant namespaces")
	}
	var found bool
	var nsVersion string
	for _, ns := range namespaces {
		if nsVersion, found = versionMapping[ns.Type]; found {
			if failed {
				ns.State = tenant.Failed
			} else {
				ns.State = tenant.Ready
				ns.Version = nsVersion
			}
			ns.UpdatedBy = configuration.Commit
			err := tenantService.SaveNamespace(ns)
			if err != nil {
				return errors.Wrapf(err, "unable to save tenant namespace %+v", ns)
			}
		}
	}
	return nil
}
