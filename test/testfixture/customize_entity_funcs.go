package testfixture

// A CustomizeEntityFunc acts as a generic function to the various
// recipe-functions (e.g. Tenants(), Namespaces(), etc.). The current test
// fixture is given with the fxt argument and the position of the object that
// will be created next is indicated by the index idx. That index can be used to
// look up e.g. a space with
//     s := fxt.Tenants[idx]
// That tenant will be a ready-to-create space object on that you can modify to
// your liking.
//

type CustomizeEntityFunc func(fxt *TestFixture, idx int) error
