package openshift

import (
	"github.com/fabric8-services/fabric8-tenant/environment"
	"sync"
)

var CacheOfRemovedObjects = &RemovedObjectsCache{
	removedObjectsResolvers: make(map[string]map[environment.Type]*removedObjectsResolver),
}

type RemovedObjectsCache struct {
	mux                     sync.Mutex
	removedObjectsResolvers map[string]map[environment.Type]*removedObjectsResolver
}

type removedObjectsResolver struct {
	mux               sync.Mutex
	resolved          bool
	allCurrentObjects environment.Objects
	removedObjects    []removedObject
	envType           environment.Type
	objectsToCompare  objectsToCompareRetrieval
}

type objectsForVersionRetrieval func(version string) (*environment.EnvData, environment.Objects, error)
type objectsToCompareRetrieval func() (*environment.EnvData, environment.Objects, error)

func (c *RemovedObjectsCache) GetResolver(envType environment.Type, version string, objectsForVersion objectsForVersionRetrieval,
	allCurrentObjects environment.Objects) *removedObjectsResolver {

	c.mux.Lock()
	defer c.mux.Unlock()

	if c.removedObjectsResolvers == nil {
		c.removedObjectsResolvers = make(map[string]map[environment.Type]*removedObjectsResolver)
	}

	if forVersion, forVersionExists := c.removedObjectsResolvers[version]; forVersionExists {
		if removedObjectsResolver, forTypeExists := forVersion[envType]; forTypeExists {
			return removedObjectsResolver
		}
	} else {
		c.removedObjectsResolvers[version] = make(map[environment.Type]*removedObjectsResolver)
	}
	resolver := &removedObjectsResolver{
		envType:           envType,
		allCurrentObjects: allCurrentObjects,
		objectsToCompare: func() (data *environment.EnvData, objects environment.Objects, e error) {
			return objectsForVersion(version)
		}}
	c.removedObjectsResolvers[version][envType] = resolver
	return resolver
}

func (c *RemovedObjectsCache) RemoveResolver(envType environment.Type, version string) {

	c.mux.Lock()
	defer c.mux.Unlock()

	if forVersion, forVersionExists := c.removedObjectsResolvers[version]; forVersionExists {
		delete(forVersion, envType)
	}
}

func (r *removedObjectsResolver) Resolve() ([]removedObject, error) {
	r.mux.Lock()
	defer r.mux.Unlock()

	if r.resolved {
		return r.removedObjects, nil
	}

	_, objects, err := r.objectsToCompare()
	if err != nil {
		return nil, err
	}
	var removedObjects []removedObject
	for _, obj := range objects {
		found := false
		for _, currentObj := range r.allCurrentObjects {
			if environment.GetKind(obj) == environment.GetKind(currentObj) &&
				environment.GetName(obj) == environment.GetName(currentObj) &&
				environment.GetNamespace(obj) == environment.GetNamespace(currentObj) {
				found = true
				break
			}
		}
		if !found {
			removedObjects = append(removedObjects, removedObject{
				kind: environment.GetKind(obj),
				name: environment.GetName(obj),
			})
		}
	}

	r.removedObjects = removedObjects
	r.resolved = true

	return removedObjects, nil
}

type removedObject struct {
	kind string
	name string
}

func (o removedObject) toObject(namespaceName string) environment.Object {
	return NewObject(o.kind, namespaceName, o.name)
}
