package testfixture

import (
	"context"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/controller"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/test/doubles"
	"github.com/fabric8-services/fabric8-wit/resource"
	"github.com/jinzhu/gorm"
	errs "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"time"
)

// A TestFixture object is the result of a call to
//  NewFixture()
// or
//  NewFixtureIsolated()
//
// Don't create one on your own!
type TestFixture struct {
	info             map[kind]*createInfo
	db               *gorm.DB
	isolatedCreation bool
	ctx              context.Context
	checkFuncs       []func() error

	Tenants    []*tenant.Tenant    // Tenants (if any) that were created for this test fixture.
	Namespaces []*tenant.Namespace // Namespaces (if any) that were created for this test fixture.
}

// NewFixture will create a test fixture by executing the recipies from the
// given recipe functions. If recipeFuncs is empty, nothing will happen.
//
// For example
//     NewFixture(db, Comments(100))
// will create a work item (and everything required in order to create it) and
// author 100 comments for it. They will all be created by the same user if you
// don't tell the system to do it differently. For example, to create 100
// comments from 100 different users we can do the following:
//      NewFixture(db, Tenants(100), Namespaces(100, func(fxt *TestFixture, idx int) error{
//          fxt.Namespaces[idx].TenantID = fxt.Tenants[idx].ID
//          return nil
//      }))
// That will create 100 tenants and 100 namspaces and for each namespace we're
// using the ID of one of the tenants that have been created earlier. There's
// one important observation to make with this example: there's an order to how
// entities get created in the test fixture. That order is basically defined by
// the number of dependencies that each entity has. For example a tenant has
// no dependency, so it will be created first and then can be accessed safely by
// any of the other entity creation functions. A namespace for example depends on
// a tenant. The NewFixture function does take care of recursively resolving those
// dependencies first.
//
func NewFixture(db *gorm.DB, recipeFuncs ...RecipeFunction) (*TestFixture, error) {
	return newFixture(db, false, recipeFuncs...)
}

// NewTestFixture does the same as NewFixture except that it automatically
// fails the given test if the fixture could not be created correctly.
func NewTestFixture(t testing.TB, db *gorm.DB, recipeFuncs ...RecipeFunction) *TestFixture {
	resource.Require(t, resource.Database)

	tc, err := NewFixture(db, recipeFuncs...)
	require.Nil(t, err)
	require.NotNil(t, tc)
	return tc
}

// NewFixtureIsolated will create a test fixture by executing the recipies from
// the given recipe functions. If recipeFuncs is empty, nothing will happen.
//
// The difference to the normal NewFixture function is that we will only create
// those object that where specified in the recipeFuncs. We will not create any
// object that is normally demanded by an object. For example, if you call
//     NewFixture(t, db, Namespaces(1))
// you would (apart from other objects) get at least one namespace AND a tenant
// because that is needed to create a namespace. With
//     NewFixtureIsolated(t, db, Namespaces(2), Tenants(1))
// on the other hand, we will only create a tenant, two namespaces for it, and
// nothing more.
func NewFixtureIsolated(db *gorm.DB, setupFuncs ...RecipeFunction) (*TestFixture, error) {
	return newFixture(db, true, setupFuncs...)
}

// Check runs all check functions that each recipe-function has registered to
// check that the amount of objects has been created that were demanded in the
// recipe function.
//
// In this example
//     fxt, _:= NewFixture(db, Namespaces(2))
//     err = fxt.Check()
// err will only be nil if at least namespaces have been created and all of
// the dependencies that a namespace requires. Look into the documentation of
// each recipe-function to find out what dependencies each entity has.
//
// Notice, that check is called at the end of NewFixture() and its derivatives,
// so if you don't mess with the fixture after it was created, there's no need
// to call Check() again.
func (fxt *TestFixture) Check() error {
	for _, fn := range fxt.checkFuncs {
		if err := fn(); err != nil {
			return errs.Wrap(err, "check function failed")
		}
	}
	return nil
}

type kind string

const (
	kindTenants    kind = "tenant"
	kindNamespaces kind = "namespace"
)

type createInfo struct {
	numInstances         int
	customizeEntityFuncs []CustomizeEntityFunc
}

func (fxt *TestFixture) runCustomizeEntityFuncs(idx int, k kind) error {
	if fxt.info[k] == nil {
		return errs.Errorf("the creation info for kind %s is nil (this should not happen)", k)
	}
	for _, dfn := range fxt.info[k].customizeEntityFuncs {
		if err := dfn(fxt, idx); err != nil {
			return errs.Wrapf(err, "failed to run customize-entity-callbacks for kind %s", k)
		}
	}
	return nil
}

func (fxt *TestFixture) setupInfo(n int, k kind, fns ...CustomizeEntityFunc) error {
	if n <= 0 {
		return errs.Errorf("the number of objects to create must always be greater than zero: %d", n)
	}
	if _, ok := fxt.info[k]; !ok {
		fxt.info[k] = &createInfo{}
	}
	maxN := n
	if maxN < fxt.info[k].numInstances {
		maxN = fxt.info[k].numInstances
	}
	fxt.info[k].numInstances = maxN
	fxt.info[k].customizeEntityFuncs = append(fxt.info[k].customizeEntityFuncs, fns...)
	return nil
}

func newFixture(db *gorm.DB, isolatedCreation bool, recipeFuncs ...RecipeFunction) (*TestFixture, error) {
	fxt := TestFixture{
		checkFuncs:       []func() error{},
		info:             map[kind]*createInfo{},
		db:               db,
		isolatedCreation: isolatedCreation,
		ctx:              context.Background(),
	}
	for _, fn := range recipeFuncs {
		if err := fn(&fxt); err != nil {
			return nil, errs.Wrap(err, "failed to execute recipe function")
		}
	}
	makeFuncs := []func(fxt *TestFixture) error{
		// make the objects that DON'T have any dependency
		makeTenants,
		// actually make the objects that DO have dependencies
		makeNamespaces,
	}
	for _, fn := range makeFuncs {
		if err := fn(&fxt); err != nil {
			return nil, errs.Wrap(err, "failed to make objects")
		}
	}
	if err := fxt.Check(); err != nil {
		return nil, errs.Wrap(err, "test fixture did not pass checks")
	}
	return &fxt, nil
}

func FillDB(t *testing.T, db *gorm.DB, numberOfTenants int, upToDate bool, state tenant.NamespaceState, envTypes ...string) *TestFixture {
	mappedVersions := testdoubles.GetMappedVersions(envTypes...)
	return NewTestFixture(t, db, Tenants(numberOfTenants),
		Namespaces(numberOfTenants*len(envTypes), func(fxt *TestFixture, idx int) error {
			fxt.Namespaces[idx].TenantID = fxt.Tenants[int(idx/len(envTypes))].ID
			fxt.Namespaces[idx].Type = tenant.NamespaceType(envTypes[idx%len(envTypes)])
			fxt.Namespaces[idx].MasterURL = "http://api.cluster1/"
			fxt.Namespaces[idx].UpdatedAt = time.Now()
			fxt.Namespaces[idx].UpdatedBy = controller.Commit
			fxt.Namespaces[idx].State = state
			if upToDate {
				fxt.Namespaces[idx].Version = mappedVersions[string(fxt.Namespaces[idx].Type)]
			} else {
				fxt.Namespaces[idx].Version = "0000"
			}
			return nil
		}))
}
