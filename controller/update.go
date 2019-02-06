package controller

import (
	"fmt"
	commonauth "github.com/fabric8-services/fabric8-common/auth"
	"github.com/fabric8-services/fabric8-common/convert/ptr"
	"github.com/fabric8-services/fabric8-common/errors"
	"github.com/fabric8-services/fabric8-common/log"
	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/cluster"
	"github.com/fabric8-services/fabric8-tenant/configuration"
	"github.com/fabric8-services/fabric8-tenant/dbsupport"
	"github.com/fabric8-services/fabric8-tenant/environment"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-tenant/tenant"
	"github.com/fabric8-services/fabric8-tenant/update"
	"github.com/goadesign/goa"
	"github.com/jinzhu/gorm"
)

// UpdateController implements the update resource.
type UpdateController struct {
	*goa.Controller
	db             *gorm.DB
	config         *configuration.Data
	clusterService cluster.Service
	updateExecutor update.Executor
}

// NewUpdateController creates a update controller.
func NewUpdateController(service *goa.Service, db *gorm.DB, config *configuration.Data, clusterService cluster.Service, updateExecutor update.Executor) *UpdateController {
	return &UpdateController{
		Controller:     service.NewController("UpdateController"),
		db:             db,
		config:         config,
		clusterService: clusterService,
		updateExecutor: updateExecutor}
}

// Start runs the start action.
func (c *UpdateController) Start(ctx *app.StartUpdateContext) error {
	if !commonauth.IsSpecificServiceAccount(ctx, commonauth.TenantUpdate) {
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

	tenantsUpdate, err := update.NewRepository(c.db).GetTenantsUpdate()
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "retrieval of TenantsUpdate entity failed")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}
	if tenantsUpdate.Status == update.Updating && !update.IsOlderThanTimeout(tenantsUpdate.LastTimeUpdated, c.config) {
		msg := fmt.Sprintf("There is an ongoing update with the last updated timestamp %s. "+
			"To be sure that the update was interupted and a new one can be started, you have to wait %s since that time.",
			tenantsUpdate.LastTimeUpdated, c.config.GetAutomatedUpdateRetrySleep())
		ctx.Conflict(&app.ConflictMsgSingle{
			Data: &app.ConflictMsg{
				ConflictMsg: &msg}})
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
	if !commonauth.IsSpecificServiceAccount(ctx, commonauth.TenantUpdate) {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}

	tenantsUpdate, err := update.NewRepository(c.db).GetTenantsUpdate()
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "retrieval of TenantsUpdate entity failed")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	var envTypes = environment.DefaultEnvTypes
	if value(ctx.EnvType) != "" {
		envTypes = []environment.Type{environment.Type(value(ctx.EnvType))}
		if !isOneOfDefaults(envTypes[0]) {
			return jsonapi.JSONErrorResponse(ctx, errors.NewBadParameterError("env-type", ctx.EnvType))
		}
	}

	mappedTemplates := environment.RetrieveMappedTemplates()
	typesWithVersion := map[environment.Type]string{}
	for _, envType := range envTypes {
		typesWithVersion[envType] = mappedTemplates[envType].ConstructCompleteVersion()
	}

	numberOfOutdated, err := tenant.NewDBService(c.db).GetNumberOfOutdatedTenants(typesWithVersion, configuration.Commit, value(ctx.ClusterURL))
	if err != nil {
		log.Error(ctx, map[string]interface{}{
			"err": err,
		}, "retrieval of number of outdated tenants failed")
		return jsonapi.JSONErrorResponse(ctx, errors.NewInternalError(ctx, err))
	}

	updateData := convert(tenantsUpdate, numberOfOutdated)
	return ctx.OK(&app.UpdateDataSingle{Data: updateData})
}

func convert(tenantsUpdate *update.TenantsUpdate, numberOfOutdated int) *app.UpdateData {
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
		ToUpdate:        &numberOfOutdated,
	}
}

// Stop runs the stop action.
func (c *UpdateController) Stop(ctx *app.StopUpdateContext) error {
	if !commonauth.IsSpecificServiceAccount(ctx, commonauth.TenantUpdate) {
		return jsonapi.JSONErrorResponse(ctx, errors.NewUnauthorizedError("Wrong token"))
	}

	err := dbsupport.Transaction(c.db, func(tx *gorm.DB) error {
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
