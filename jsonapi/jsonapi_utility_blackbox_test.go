package jsonapi_test

import (
	"context"
	"fmt"
	"net/http"
	"strconv"
	"testing"

	"github.com/fabric8-services/fabric8-tenant/app"
	"github.com/fabric8-services/fabric8-tenant/jsonapi"
	"github.com/fabric8-services/fabric8-common/errors"
	errs "github.com/pkg/errors"
	"github.com/stretchr/testify/require"
	"github.com/fabric8-services/fabric8-tenant/test/resource"
)

func TestErrorToJSONAPIError(t *testing.T) {
	t.Parallel()
	if ready, reason := resource.IsReady(resource.UnitTest); !ready {
		t.Skip(reason)
	}

	var jerr app.JSONAPIError
	var httpStatus int

	// test not found error
	jerr, httpStatus = jsonapi.ErrorToJSONAPIError(nil, errors.NewNotFoundError("foo", "bar"))
	require.Equal(t, http.StatusNotFound, httpStatus)
	require.NotNil(t, jerr.Code)
	require.Equal(t, jsonapi.ErrorCodeNotFound, *jerr.Code)
	require.NotNil(t, jerr.Status)
	require.Equal(t, strconv.Itoa(httpStatus), *jerr.Status)
	require.NotNil(t, jerr.ID)
	require.Equal(t, "bar", *jerr.ID)

	// test not found error
	jerr, httpStatus = jsonapi.ErrorToJSONAPIError(nil, errors.NewConversionError("foo"))
	require.Equal(t, http.StatusBadRequest, httpStatus)
	require.NotNil(t, jerr.Code)
	require.Equal(t, jsonapi.ErrorCodeConversionError, *jerr.Code)
	require.NotNil(t, jerr.Status)
	require.Equal(t, strconv.Itoa(httpStatus), *jerr.Status)

	// test bad parameter error
	jerr, httpStatus = jsonapi.ErrorToJSONAPIError(nil, errors.NewBadParameterError("foo", "bar"))
	require.Equal(t, http.StatusBadRequest, httpStatus)
	require.NotNil(t, jerr.Code)
	require.Equal(t, jsonapi.ErrorCodeBadParameter, *jerr.Code)
	require.NotNil(t, jerr.Status)
	require.Equal(t, strconv.Itoa(httpStatus), *jerr.Status)

	// test internal server error
	jerr, httpStatus = jsonapi.ErrorToJSONAPIError(nil, errors.NewInternalError(context.Background(), errs.New("foo")))
	require.Equal(t, http.StatusInternalServerError, httpStatus)
	require.NotNil(t, jerr.Code)
	require.Equal(t, jsonapi.ErrorCodeInternalError, *jerr.Code)
	require.NotNil(t, jerr.Status)
	require.Equal(t, strconv.Itoa(httpStatus), *jerr.Status)

	// test unauthorized error
	jerr, httpStatus = jsonapi.ErrorToJSONAPIError(nil, errors.NewUnauthorizedError("foo"))
	require.Equal(t, http.StatusUnauthorized, httpStatus)
	require.NotNil(t, jerr.Code)
	require.Equal(t, jsonapi.ErrorCodeUnauthorizedError, *jerr.Code)
	require.NotNil(t, jerr.Status)
	require.Equal(t, strconv.Itoa(httpStatus), *jerr.Status)

	// test forbidden error
	jerr, httpStatus = jsonapi.ErrorToJSONAPIError(nil, errors.NewForbiddenError("foo"))
	require.Equal(t, http.StatusForbidden, httpStatus)
	require.NotNil(t, jerr.Code)
	require.Equal(t, jsonapi.ErrorCodeForbiddenError, *jerr.Code)
	require.NotNil(t, jerr.Status)
	require.Equal(t, strconv.Itoa(httpStatus), *jerr.Status)

	// test unspecified error
	jerr, httpStatus = jsonapi.ErrorToJSONAPIError(nil, fmt.Errorf("foobar"))
	require.Equal(t, http.StatusInternalServerError, httpStatus)
	require.NotNil(t, jerr.Code)
	require.Equal(t, jsonapi.ErrorCodeUnknownError, *jerr.Code)
	require.NotNil(t, jerr.Status)
	require.Equal(t, strconv.Itoa(httpStatus), *jerr.Status)
}
