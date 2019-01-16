package assertion

import (
	"github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test"
	"github.com/jinzhu/gorm"
	"github.com/satori/go.uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

type TenantRepoAssertion struct {
	t          *testing.T
	repo       tenant.Repository
	tnnt       *tenant.Tenant
	namespaces []*tenant.Namespace
}

func AssertTenant(t *testing.T, repo tenant.Repository) *TenantRepoAssertion {
	return &TenantRepoAssertion{
		t:    t,
		repo: repo,
	}
}

func AssertTenantFromDB(t *testing.T, db *gorm.DB, tenantID uuid.UUID) *TenantRepoAssertion {
	return AssertTenantFromService(t, tenant.NewDBService(db), tenantID)
}

func AssertTenantFromService(t *testing.T, repo tenant.Service, tenantID uuid.UUID) *TenantRepoAssertion {
	return &TenantRepoAssertion{
		t:    t,
		repo: repo.NewTenantRepository(tenantID),
	}
}

func (a *TenantRepoAssertion) Exists() *TenantRepoAssertion {
	a.getTnnt()
	return a
}

func (a *TenantRepoAssertion) DoesNotExist() *TenantRepoAssertion {
	tnnt, err := a.repo.GetTenant()
	test.AssertError(a.t, err, test.IsOfType(errors.NotFoundError{}))
	assert.Nil(a.t, tnnt)
	return a
}

func (a *TenantRepoAssertion) getTnnt() *tenant.Tenant {
	if a.tnnt == nil {
		var err error
		a.tnnt, err = a.repo.GetTenant()
		require.NoError(a.t, err)
		require.NotNil(a.t, a.tnnt)
	}
	return a.tnnt
}

func (a *TenantRepoAssertion) getNamespaces() []*tenant.Namespace {
	if a.namespaces == nil {
		var err error
		a.namespaces, err = a.repo.GetNamespaces()
		require.NoError(a.t, err)
		require.NotNil(a.t, a.namespaces)
	}
	return a.namespaces
}

func (a *TenantRepoAssertion) HasNsBaseName(name string) *TenantRepoAssertion {
	assert.Equal(a.t, name, a.getTnnt().NsBaseName)
	return a
}

func (a *TenantRepoAssertion) HasNoNamespace() *TenantRepoAssertion {
	namespaces, err := a.repo.GetNamespaces()
	require.NoError(a.t, err)
	assert.Empty(a.t, namespaces)
	return a
}

func (a *TenantRepoAssertion) HasNumberOfNamespaces(number int) *TenantRepoAssertion {
	assert.Len(a.t, a.getNamespaces(), number)
	return a
}

func (a *TenantRepoAssertion) HasNotNamespaceOfType(envType environment.Type) *TenantRepoAssertion {
	for _, ns := range a.getNamespaces() {
		if ns.Type == envType {
			require.Fail(a.t, "The namespace of the given type was fond, but should not", envType.String())
		}
	}
	return a
}

func (a *TenantRepoAssertion) HasNamespaceOfTypeThat(envType environment.Type) *NamespaceAssertion {
	for _, ns := range a.getNamespaces() {
		if ns.Type == envType {
			return AssertNamespace(a.t, ns)
		}
	}
	require.Fail(a.t, "There was no namespace fond that would be of type", envType.String())
	return nil
}

func (a *TenantRepoAssertion) HasNamespacesThat(nsAssertion func(assertion *NamespaceAssertion)) *TenantRepoAssertion {
	for _, ns := range a.getNamespaces() {
		nsAssertion(AssertNamespace(a.t, ns))
	}
	return a
}

type NamespaceAssertion struct {
	t         *testing.T
	namespace *tenant.Namespace
}

func AssertNamespace(t *testing.T, namespace *tenant.Namespace) *NamespaceAssertion {
	assert.NotNil(t, namespace)
	return &NamespaceAssertion{
		t:         t,
		namespace: namespace,
	}
}

func (a *NamespaceAssertion) HasName(name string) *NamespaceAssertion {
	assert.Equal(a.t, name, a.namespace.Name)
	return a
}

func (a *NamespaceAssertion) HasNameWithBaseName(baseName string) *NamespaceAssertion {
	expName := baseName
	if a.namespace.Type != environment.TypeUser {
		expName += "-" + a.namespace.Type.String()
	}
	assert.Equal(a.t, expName, a.namespace.Name)
	return a
}

func (a *NamespaceAssertion) IsOFType(envType environment.Type) *NamespaceAssertion {
	assert.Equal(a.t, envType.String(), a.namespace.Type.String())
	return a
}

func (a *NamespaceAssertion) HasState(state tenant.NamespaceState) *NamespaceAssertion {
	assert.Equal(a.t, state.String(), a.namespace.State.String())
	return a
}

func (a *NamespaceAssertion) HasMasterURL(masterURL string) *NamespaceAssertion {
	assert.Equal(a.t, test.Normalize(masterURL), test.Normalize(a.namespace.MasterURL))
	return a
}

func (a *NamespaceAssertion) HasVersion(version string) *NamespaceAssertion {
	assert.Equal(a.t, version, a.namespace.Version)
	return a
}

func (a *NamespaceAssertion) HasCurrentCompleteVersion() *NamespaceAssertion {
	assert.Equal(a.t, environment.RetrieveMappedTemplates()[a.namespace.Type].ConstructCompleteVersion(), a.namespace.Version)
	return a
}

func (a *NamespaceAssertion) HasUpdatedBy(updatedBy string) *NamespaceAssertion {
	assert.Equal(a.t, updatedBy, a.namespace.UpdatedBy)
	return a
}

func (a *NamespaceAssertion) WasUpdatedAfter(before time.Time) *NamespaceAssertion {
	assert.True(a.t, a.namespace.UpdatedAt.After(before))
	return a
}

func (a *NamespaceAssertion) WasUpdatedBefore(after time.Time) *NamespaceAssertion {
	assert.True(a.t, a.namespace.UpdatedAt.Before(after))
	return a
}
