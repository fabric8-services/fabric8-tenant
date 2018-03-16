package controller

import (
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/goadesign/goa"
)

// NamespacesController implements the namespaces resource.
type NamespacesController struct {
	*goa.Controller
}

// NewNamespacesController creates a namespaces controller.
func NewNamespacesController(service *goa.Service) *NamespacesController {
	return &NamespacesController{Controller: service.NewController("NamespacesController")}
}

// Delete runs the delete action.
func (c *NamespacesController) Delete(ctx *app.DeleteNamespacesContext) error {
	// verify that the user identitied by the JWT has
	return nil
}
