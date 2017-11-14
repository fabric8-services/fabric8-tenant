package design

import (
	d "github.com/goadesign/goa/design"
	a "github.com/goadesign/goa/design/apidsl"
)

var listNamespacesAll = a.Type("NamespacesAll", func() {
	a.Description(`JSONAPI for the tenant all object. See also http://jsonapi.org/format/#document-resource-object`)
	a.Attribute("namespaces", a.ArrayOf(namespaceAttributes), "The tenant namespaces", func() {})
	a.Required("namespaces")
})

var _ = a.Resource("listNamespaces", func() {
	a.BasePath("/api/namespaces/all")
	a.Action("show", func() {
		a.Routing(
			a.GET(""),
		)

		a.Description("Show all namespaces")
		a.Response(d.OK, listNamespacesAll)
		a.Response(d.BadRequest, JSONAPIErrors)
		a.Response(d.NotFound, JSONAPIErrors)
		a.Response(d.InternalServerError, JSONAPIErrors)
		a.Response(d.Unauthorized, JSONAPIErrors)
	})
})
