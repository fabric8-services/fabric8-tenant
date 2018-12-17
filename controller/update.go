package controller

import (
	commonauth "github.com/fabric8-services/fabric8-common/auth"
	"github.com/fabric8-services/fabric8-common/convert/ptr"
	"github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/openshift"
	"github.com/fabric8-services/fabric8-tenant/update"
	"github.com/goadesign/goa"
	"github.com/jinzhu/gorm"
)

var fabric8TenantUpdateSA = "fabric8-tenant-update"

// UpdateController implements the update resource.
type UpdateController struct {
	*goa.Controller
	db             *gorm.DB
	config         *configuration.Data
	clusterService cluster.Service
	updateExecutor openshift.UpdateExecutor
}

// NewUpdateController creates a update controller.
func NewUpdateController(service *goa.Service, db *gorm.DB, config *configuration.Data, clusterService cluster.Service, updateExecutor openshift.UpdateExecutor) *UpdateController {
	return &UpdateController{
		Controller:     service.NewController("UpdateController"),
		db:             db,
		config:         config,
		clusterService: clusterService,
		updateExecutor: updateExecutor}
}

// Start runs the start action.
func (c *UpdateController) Start(ctx *app.StartUpdateContext) error {
	if !commonauth.IsSpecificServiceAccount(ctx, fabric8TenantUpdateSA) {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}

	var envTypesFilter update.FilterEnvType
	if value(ctx.EnvType) != "" {
		envType := environment.Type(value(ctx.EnvType))
		if !isOneOfDefaults(envType) {
			return jsonapi.JSONErrorResponse(ctx, errors.NewBadParameterError("env-type", ctx.EnvType))
		}
		envTypesFilter = update.OneType(envType)
	} else {
		envTypesFilter = update.AllTypes
	}

	go update.NewTenantsUpdater(c.db, c.config, c.clusterService, c.updateExecutor, envTypesFilter, value(ctx.ClusterURL)).UpdateAllTenants()

	return ctx.Accepted()
}

func isOneOfDefaults(envType environment.Type) bool {
	for _, defEnvType := range environment.DefaultEnvTypes {
		if defEnvType == envType {
			return true
		}
	}
	return false
}

func value(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// Show runs the show action.
func (c *UpdateController) Show(ctx *app.ShowUpdateContext) error {
	if !commonauth.IsSpecificServiceAccount(ctx, fabric8TenantUpdateSA) {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}

	var tenantsUpdate *update.TenantsUpdate
	err := update.Transaction(c.db, func(tx *gorm.DB) error {
		var err error
		tenantsUpdate, err = update.NewRepository(tx).GetTenantsUpdate()
		return err
	})
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "retrieval of TenantsUpdate entity failed")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	updateData := convert(tenantsUpdate)
	return ctx.OK(&app.UpdateDataSingle{Data: updateData})
}

func convert(tenantsUpdate *update.TenantsUpdate) *app.UpdateData {
	var fileVersions []*app.FileWithVersion
	for _, verManager := range update.RetrieveVersionManagers() {
		fileVersions = append(fileVersions,
			&app.FileWithVersion{
				FileName: ptr.String(verManager.FileName),
				Version:  ptr.String(verManager.GetStoredVersion(tenantsUpdate)),
			})
	}
	return &app.UpdateData{
		Status:          ptr.String(tenantsUpdate.Status.String()),
		LastTimeUpdated: ptr.Time(tenantsUpdate.LastTimeUpdated),
		FailedCount:     ptr.Int(tenantsUpdate.FailedCount),
		FileVersions:    fileVersions,
	}
}

// Stop runs the stop action.
func (c *UpdateController) Stop(ctx *app.StopUpdateContext) error {
	if !commonauth.IsSpecificServiceAccount(ctx, fabric8TenantUpdateSA) {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}

	err := update.Transaction(c.db, func(tx *gorm.DB) error {
		return update.NewRepository(tx).Stop()
	})
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "stopping of tenants update failed")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	return ctx.Accepted()
}
