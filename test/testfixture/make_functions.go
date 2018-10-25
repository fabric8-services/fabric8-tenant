package testfixture

import (
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	errs "github.com/pkg/errors"
	uuid "github.com/satori/go.uuid"
)

func makeTenants(fxt *TestFixture) error {
	if fxt.info[kindTenants] == nil {
		return nil
	}
	fxt.Tenants = make([]*tenant.Tenant, fxt.info[kindTenants].numInstances)
	for i := range fxt.Tenants {
		fxt.Tenants[i] = &tenant.Tenant{
			ID:      uuid.NewV4(),
			Email:   createRandomEmailAddress(),
			Profile: "free",
		}
		if err := fxt.runCustomizeEntityFuncs(i, kindTenants); err != nil {
			return errs.WithStack(err)
		}
		err := fxt.tenantService.SaveTenant(fxt.Tenants[i])
		if err != nil {
			return errs.Wrapf(err, "failed to create tenant: %+v", fxt.Tenants[i])
		}
	}
	return nil
}

func makeNamespaces(fxt *TestFixture) error {
	if fxt.info[kindNamespaces] == nil {
		return nil
	}
	fxt.Namespaces = make([]*tenant.Namespace, fxt.info[kindNamespaces].numInstances)
	for i := range fxt.Namespaces {
		fxt.Namespaces[i] = &tenant.Namespace{
			Type:      environment.TypeChe,
			Name:      createRandomNamespaceName(),
			MasterURL: "some.cluster.url",
		}
		if !fxt.isolatedCreation {
			fxt.Namespaces[i].TenantID = fxt.Tenants[0].ID
		}
		if err := fxt.runCustomizeEntityFuncs(i, kindNamespaces); err != nil {
			return errs.WithStack(err)
		}
		if fxt.isolatedCreation {
			if fxt.Namespaces[i].TenantID == uuid.Nil {
				return errs.New("you must specify a tenant ID for each namespace")
			}
		}
		err := fxt.tenantService.SaveNamespace(fxt.Namespaces[i])
		if err != nil {
			return errs.Wrapf(err, "failed to create namespace: %+v", fxt.Namespaces[i])
		}
	}
	return nil
}

func createRandomEmailAddress() string {
	return "johndoe-" + uuid.NewV4().String() + "@foo.com"
}

func createRandomNamespaceName() string {
	return "ns-" + uuid.NewV4().String()
}
