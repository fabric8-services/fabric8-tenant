package testfixture

import (
	errs "github.com/pkg/errors"
)

// A RecipeFunction tells the test fixture to create n objects of a given kind.
// You can pass in customize-entity-functions in order to manipulate the objects
// before they get created.
type RecipeFunction func(fxt *TestFixture) error

const checkStr = "expected at least %d \"%s\" objects but found only %d"

func (fxt *TestFixture) deps(fns ...RecipeFunction) error {
	if !fxt.isolatedCreation {
		for _, fn := range fns {
			if err := fn(fxt); err != nil {
				return errs.Wrap(err, "failed to setup dependency")
			}
		}
	}
	return nil
}

// CustomizeTenantFunc is directly compatible with CustomizeEntityFunc
// but it can only be used for the Tenants() recipe-function.
type CustomizeTenantFunc CustomizeEntityFunc

// Tenants tells the test fixture to create at least n tenant objects.
//
// If called multiple times with differently n's, the biggest n wins. All
// customize-entitiy-functions fns from all calls will be respected when
// creating the test fixture.
//
// Here's an example how you can create 42 tenants and give them a numbered
// user name like "John Doe 0", "John Doe 1", and so forth:
//    Tenants(42, func(fxt *TestFixture, idx int) error{
//        fxt.Tenants[idx].Email = "JaneDoe" + strconv.FormatInt(idx, 10) + "@foo.com"
//        return nil
//    })
// Notice that the index idx goes from 0 to n-1 and that you have to manually
// lookup the object from the test fixture. The Tenant object referenced by
//    fxt.Tenants[idx]
// is guaranteed to be ready to be used for creation. That means, you don't
// necessarily have to touch it to avoid unique key violation for example. This
// is totally optional.
func Tenants(n int, fns ...CustomizeTenantFunc) RecipeFunction {
	return func(fxt *TestFixture) error {
		fxt.checkFuncs = append(fxt.checkFuncs, func() error {
			l := len(fxt.Tenants)
			if l < n {
				return errs.Errorf(checkStr, n, kindTenants, l)
			}
			return nil
		})
		// Convert fns to []CustomizeEntityFunc
		customFuncs := make([]CustomizeEntityFunc, len(fns))
		for idx := range fns {
			customFuncs[idx] = CustomizeEntityFunc(fns[idx])
		}
		return fxt.setupInfo(n, kindTenants, customFuncs...)
	}
}

// CustomizeNamespaceFunc is directly compatible with CustomizeEntityFunc
// but it can only be used for the Namespaces() recipe-function.
type CustomizeNamespaceFunc CustomizeEntityFunc

// Namespaces tells the test fixture to create at least n namespace objects. See also
// the Tenants() function for more general information on n and fns.
//
// When called in NewFixture() this function will call also call
//     Tenants(1)
// but with NewFixtureIsolated(), no other objects will be created.
func Namespaces(n int, fns ...CustomizeNamespaceFunc) RecipeFunction {
	return func(fxt *TestFixture) error {
		fxt.checkFuncs = append(fxt.checkFuncs, func() error {
			l := len(fxt.Namespaces)
			if l < n {
				return errs.Errorf(checkStr, n, kindNamespaces, l)
			}
			return nil
		})
		// Convert fns to []CustomizeEntityFunc
		customFuncs := make([]CustomizeEntityFunc, len(fns))
		for idx := range fns {
			customFuncs[idx] = CustomizeEntityFunc(fns[idx])
		}
		if err := fxt.setupInfo(n, kindNamespaces, customFuncs...); err != nil {
			return err
		}
		return fxt.deps(Tenants(1))
	}
}
