package design

import (
	d "github.com/goadesign/goa/design"
	a "github.com/goadesign/goa/design/apidsl"
)

var updateData = a.Type("UpdateData", func() {
	a.Description(`JSONAPI for the update info object. See also http://jsonapi.org/format/#document-resource-object`)
	a.Attribute("status", d.String, "The update status", func() {
		a.Enum("finished", "updating", "failed", "killed", "incomplete")
	})
	a.Attribute("last-time-updated", d.DateTime, "When an update of the last batch of tenants was finished", func() {
		a.Example("2016-11-29T23:18:14Z")
	})
	a.Attribute("failed-count", d.Integer, "The number of failed tenant updates", func() {
	})
	a.Attribute("file-versions", a.ArrayOf(fileWithVersion), "Lis of files and their versions used for the last finished run", func() {
	})
	a.Attribute("links", genericLinks)
})

var fileWithVersion = a.Type("FileWithVersion", func() {
	a.Attribute("file-name", d.String, "Name of a file")
	a.Attribute("version", d.String, "Version of the file that was set when the last update was finished")
})

var updateInfo = JSONSingle(
	"UpdateData", "Holds information about last/ongoing update",
	updateData,
	nil)

var _ = a.Resource("update", func() {
	a.BasePath("/api/update")
	a.Action("start", func() {
		a.Security("jwt")
		a.Routing(
			a.POST(""),
		)
		a.Params(func() {
			a.Param("cluster_url", d.String, "the URL of the OSO cluster the update should be limited to")
			a.Param("env_type", d.String, "environment type the update should be executed for", func() {
				a.Enum("user", "che", "jenkins", "stage", "run")
			})
		})

		a.Description("Start new cluster-wide update.")
		a.Response(d.Accepted)
		a.Response(d.BadRequest, JSONAPIErrors)
		a.Response(d.InternalServerError, JSONAPIErrors)
		a.Response(d.Unauthorized, JSONAPIErrors)
	})

	a.Action("show", func() {
		a.Security("jwt")
		a.Routing(
			a.GET(""),
		)

		a.Description("Get information about last/ongoing update.")
		a.Response(d.OK, updateInfo)
		a.Response(d.InternalServerError, JSONAPIErrors)
		a.Response(d.Unauthorized, JSONAPIErrors)
	})
	a.Action("stop", func() {
		a.Security("jwt")
		a.Routing(
			a.DELETE(""),
		)

		a.Description("Stops an ongoing cluster-wide update.")
		a.Response(d.Accepted)
		a.Response(d.InternalServerError, JSONAPIErrors)
		a.Response(d.Unauthorized, JSONAPIErrors)
	})
})
