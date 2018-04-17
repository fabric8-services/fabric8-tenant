package jsonapi

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strconv"

	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/errors"
	"github.com/fabric8-services/fabric8-wit/log"

	"github.com/goadesign/goa"
	errs "github.com/pkg/errors"
)

const (
	ErrorCodeNotFound          = "not_found"
	ErrorCodeBadParameter      = "bad_parameter"
	ErrorCodeConflict          = "conflict"
	ErrorCodeProjectConflict   = "project_conflict"
	ErrorCodeUnknownError      = "unknown_error"
	ErrorCodeConversionError   = "conversion_error"
	ErrorCodeInternalError     = "internal_error"
	ErrorCodeUnauthorizedError = "unauthorized_error"
	ErrorCodeForbiddenError    = "forbidden_error"
	ErrorCodeQuotaExceedError  = "quota_exceeded_error"
	ErrorCodeJWTSecurityError  = "jwt_security_error"
)

// ErrorToJSONAPIError returns the JSONAPI representation
// of an error and the HTTP status code that will be associated with it.
// This function knows about the models package and the errors from there
// as well as goa error classes.
func ErrorToJSONAPIError(ctx context.Context, err error) (app.JSONAPIError, int) {
	cause := errs.Cause(err)
	detail := cause.Error()
	var title, code string
	var statusCode int
	var id *string
	links := make(map[string]*app.JSONAPILink, 0)
	log.Error(ctx, map[string]interface{}{"err": cause, "error_message": cause.Error()}, "an error occurred in our api")
	switch cause := cause.(type) {
	case errors.TenantRecordNotFoundError:
		code = ErrorCodeNotFound
		title = "Tenant record not found error"
		statusCode = http.StatusNotFound
		id = &cause.ID
	case errors.OpenShiftObjectNotFoundError:
		code = ErrorCodeNotFound
		title = "OpenShift object not found error"
		statusCode = http.StatusNotFound
		id = &cause.ObjectURL // pass the object URL that could not be located as the ID in the JSON-API error
	case errors.BadParameterError:
		code = ErrorCodeBadParameter
		title = "Bad parameter error"
		statusCode = http.StatusBadRequest
	case errors.NamespaceConflictError:
		code = ErrorCodeConflict
		title = "Namespace conflict error"
		statusCode = http.StatusConflict
		if ctx, ok := ctx.(app.AbsoluteURL); ok {
			log.Debug(ctx, map[string]interface{}{"namespace": cause.Namespace}, "adding a link to remove the namespace")
			if cause.Namespace != "" {
				deleteNamespaceURL := ctx.AbsoluteURL(fmt.Sprintf("%s/namespaces/%s", app.TenantHref(), cause.Namespace))
				links[cause.Namespace] = &app.JSONAPILink{
					Href: &deleteNamespaceURL,
				}
			}
		}
	case errors.DataConflictError:
		code = ErrorCodeConflict
		title = "Data conflict error"
		statusCode = http.StatusConflict
	case errors.OpenShiftObjectConflictError:
		code = ErrorCodeConflict
		title = "OpenShifr object conflict error"
		statusCode = http.StatusConflict
	case errors.InternalError:
		code = ErrorCodeInternalError
		title = "Internal error"
		statusCode = http.StatusInternalServerError
	case errors.UnauthorizedError:
		code = ErrorCodeUnauthorizedError
		title = "Unauthorized error"
		statusCode = http.StatusUnauthorized
	case errors.QuotaExceedError:
		code = ErrorCodeQuotaExceedError
		title = "Quota exceeded error"
		statusCode = http.StatusForbidden
		if ctx, ok := ctx.(app.AbsoluteURL); ok {
			for _, n := range cause.Namespaces {
				log.Debug(ctx, map[string]interface{}{"namespace": n}, "adding a link to remove the namespace")
				if n != "" {
					deleteNamespaceURL := ctx.AbsoluteURL(fmt.Sprintf("%s/namespaces/%s", app.TenantHref(), n))
					links[n] = &app.JSONAPILink{
						Href: &deleteNamespaceURL,
					}
				}
			}
		}

	case errors.ForbiddenError:
		code = ErrorCodeForbiddenError
		title = "Forbidden error"
		statusCode = http.StatusForbidden
	default:
		code = ErrorCodeUnknownError
		title = "Unknown error"
		statusCode = http.StatusInternalServerError
		cause = errs.Cause(err)
		if err, ok := cause.(goa.ServiceError); ok {
			statusCode = err.ResponseStatus()
			idStr := err.Token()
			id = &idStr
			title = http.StatusText(statusCode)
		}
		if errResp, ok := cause.(*goa.ErrorResponse); ok {
			code = errResp.Code
			detail = errResp.Detail
		}
	}
	statusCodeStr := strconv.Itoa(statusCode)
	jerr := app.JSONAPIError{
		ID:     id,
		Code:   &code,
		Status: &statusCodeStr,
		Title:  &title,
		Detail: detail,
		Links:  links,
	}
	log.Debug(ctx, map[string]interface{}{
		"code":   code,
		"status": statusCodeStr,
		"title":  title,
		"detail": detail,
	}, "converted error to JSON Error")
	return jerr, statusCode
}

// ErrorToJSONAPIErrors is a convenience function if you
// just want to return one error from the models package as a JSONAPI errors
// array.
func ErrorToJSONAPIErrors(ctx context.Context, err error) (*app.JSONAPIErrors, int) {
	jerr, httpStatusCode := ErrorToJSONAPIError(ctx, err)
	jerrors := app.JSONAPIErrors{
		Errors: []*app.JSONAPIError{&jerr},
	}
	return &jerrors, httpStatusCode
}

// BadRequest represent a Context that can return a BadRequest HTTP status
type BadRequest interface {
	BadRequest(*app.JSONAPIErrors) error
}

// InternalServerError represent a Context that can return a InternalServerError HTTP status
type InternalServerError interface {
	InternalServerError(*app.JSONAPIErrors) error
}

// NotFound represent a Context that can return a NotFound HTTP status
type NotFound interface {
	NotFound(*app.JSONAPIErrors) error
}

// Unauthorized represent a Context that can return a Unauthorized HTTP status
type Unauthorized interface {
	Unauthorized(*app.JSONAPIErrors) error
}

// Forbidden represent a Context that can return a Unauthorized HTTP status
type Forbidden interface {
	Forbidden(*app.JSONAPIErrors) error
}

// Conflict represent a Context that can return a Conflict HTTP status
type Conflict interface {
	Conflict(*app.JSONAPIErrors) error
}

// JSONErrorResponse auto maps the provided error to the correct response type
// If all else fails, InternalServerError is returned
func JSONErrorResponse(ctx context.Context, err error) error {
	jsonErr, status := ErrorToJSONAPIErrors(ctx, err)
	log.Debug(ctx, map[string]interface{}{"status": status, "jsonErr type": reflect.TypeOf(jsonErr)}, "processing JSON error")
	switch status {
	case http.StatusBadRequest:
		if ctx, ok := ctx.(BadRequest); ok {
			return errs.WithStack(ctx.BadRequest(jsonErr))
		}
	case http.StatusNotFound:
		if ctx, ok := ctx.(NotFound); ok {
			return errs.WithStack(ctx.NotFound(jsonErr))
		}
	case http.StatusUnauthorized:
		if ctx, ok := ctx.(Unauthorized); ok {
			return errs.WithStack(ctx.Unauthorized(jsonErr))
		}
	case http.StatusForbidden:
		if ctx, ok := ctx.(Forbidden); ok {
			return errs.WithStack(ctx.Forbidden(jsonErr))
		}
	case http.StatusConflict:
		if ctx, ok := ctx.(Conflict); ok {
			return errs.WithStack(ctx.Conflict(jsonErr))
		}
		log.Debug(ctx, nil, "CANNOT convert context to Conflict")
	default:
		return errs.WithStack(ctx.(InternalServerError).InternalServerError(jsonErr))
	}
	return nil
}
