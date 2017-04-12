package design

import (
	d "github.com/goadesign/goa/design"
	a "github.com/goadesign/goa/design/apidsl"
)

var _ = a.Resource("tenant", func() {
	a.BasePath("/tenant")
	a.Action("setup", func() {
		a.Security("jwt")
		a.Routing(
			a.POST(""),
		)

		a.Description("Initialize new tenant environment.")
		a.Response(d.Created, "/tenant/.*")
		a.Response(d.BadRequest, JSONAPIErrors)
		a.Response(d.NotFound, JSONAPIErrors)
		a.Response(d.InternalServerError, JSONAPIErrors)
		a.Response(d.Unauthorized, JSONAPIErrors)
	})
})
