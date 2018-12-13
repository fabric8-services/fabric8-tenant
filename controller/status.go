package controller

import (
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/goadesign/goa"
	"github.com/jinzhu/gorm"
)

// StatusController implements the status resource.
type StatusController struct {
	*goa.Controller
	db *gorm.DB
}

// NewStatusController creates a status controller.
func NewStatusController(service *goa.Service, db *gorm.DB) *StatusController {
	return &StatusController{
		Controller: service.NewController("StatusController"),
		db:         db,
	}
}

// Show runs the show action.
func (c *StatusController) Show(ctx *app.ShowStatusContext) error {
	res := &app.Status{}
	res.Commit = configuration.Commit
	res.BuildTime = configuration.BuildTime
	res.StartTime = configuration.StartTime

	_, err := c.db.DB().Exec("select 1")
	if err != nil {
		var message string
		message = err.Error()
		res.Error = &message
		return ctx.ServiceUnavailable(res)
	}
	return ctx.OK(res)
}
