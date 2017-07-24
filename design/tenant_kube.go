package design

import (
	d "github.com/goadesign/goa/design"
	a "github.com/goadesign/goa/design/apidsl"
)

var _ = a.Resource("tenantKube", func() {
	a.BasePath("/api/tenant/kubeconnect")
	a.Action("kubeConnected", func() {
		a.Security("jwt")
		a.Routing(
			a.GET(""),
		)

		a.Description("Checks if the kubernetes tenant is connected with KeyCloak.")
		a.Response(d.Accepted)
		a.Response(d.Conflict)
		a.Response(d.OK, tenantSingle)
		a.Response(d.InternalServerError, JSONAPIErrors)
	})
})
