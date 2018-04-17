package errors

import (
	"context"
	"fmt"
)

const (
	stBadParameterErrorMsg         = "Bad value for parameter '%s': '%v'"
	stBadParameterErrorExpectedMsg = "Bad value for parameter '%s': '%v' (expected: '%v')"
	stTenantRecordNotFoundErrorMsg = "%s with id '%s' not found"
)

// Constants that can be used to identify internal server errors
const (
	ErrInternalDatabase = "database_error"
)

type simpleError struct {
	message string
}

func (err simpleError) Error() string {
	return err.message
}

// NewInternalError returns the custom defined error of type InternalError.
func NewInternalError(ctx context.Context, err error) InternalError {
	return InternalError{err}
}

// InternalError means that the operation failed for some internal, unexpected reason
type InternalError struct {
	Err error
}

func (ie InternalError) Error() string {
	return ie.Err.Error()
}

// NewUnauthorizedError returns the custom defined error of type UnauthorizedError.
func NewUnauthorizedError(msg string) UnauthorizedError {
	return UnauthorizedError{simpleError{msg}}
}

// UnauthorizedError means that the operation is unauthorized
type UnauthorizedError struct {
	simpleError
}

// NewForbiddenError returns the custom defined error of type ForbiddenError.
func NewForbiddenError(msg string) ForbiddenError {
	return ForbiddenError{
		simpleError: simpleError{msg},
	}
}

// ForbiddenError means that the operation is forbidden
type ForbiddenError struct {
	simpleError
	Namespace string
}

// QuotaExceedError means that the operation is forbidden because of exceeded quota
type QuotaExceedError struct {
	simpleError
	Namespaces []string
}

// NewQuotaExceedError returns the custom defined error of type QuotaExceedError.
func NewQuotaExceedError(msg string) QuotaExceedError {
	return QuotaExceedError{
		simpleError: simpleError{msg},
	}
}

// NamespaceConflictError means that the version was not as expected in an update operation
type NamespaceConflictError struct {
	simpleError
	Namespace string
}

// NewNamespaceConflictError returns the custom defined error of type NamespaceConflictError.
func NewNamespaceConflictError(msg string) NamespaceConflictError {
	return NamespaceConflictError{
		simpleError: simpleError{msg},
	}
}

// DataConflictError means that the version was not as expected in an update operation
type DataConflictError struct {
	simpleError
	Namespace string
}

// NewDataConflictError returns the custom defined error of type DataConflictError.
func NewDataConflictError(msg string) DataConflictError {
	return DataConflictError{
		simpleError: simpleError{msg},
	}
}

// BadParameterError means that a parameter was not as required
type BadParameterError struct {
	parameter        string
	value            interface{}
	expectedValue    interface{}
	hasExpectedValue bool
}

// Error implements the error interface
func (err BadParameterError) Error() string {
	if err.hasExpectedValue {
		return fmt.Sprintf(stBadParameterErrorExpectedMsg, err.parameter, err.value, err.expectedValue)
	}
	return fmt.Sprintf(stBadParameterErrorMsg, err.parameter, err.value)

}

// Expected sets the optional expectedValue parameter on the BadParameterError
func (err BadParameterError) Expected(expexcted interface{}) BadParameterError {
	err.expectedValue = expexcted
	err.hasExpectedValue = true
	return err
}

// NewBadParameterError returns the custom defined error of type NewBadParameterError.
func NewBadParameterError(param string, actual interface{}) BadParameterError {
	return BadParameterError{parameter: param, value: actual}
}

// TenantRecordNotFoundError means the tenant record specified for the operation does not exist
type TenantRecordNotFoundError struct {
	Entity string
	ID     string
}

func (err TenantRecordNotFoundError) Error() string {
	return fmt.Sprintf(stTenantRecordNotFoundErrorMsg, err.Entity, err.ID)
}

// NewTenantRecordNotFoundError returns the custom defined error of type TenantRecordNotFoundError.
func NewTenantRecordNotFoundError(entity string, id string) TenantRecordNotFoundError {
	return TenantRecordNotFoundError{Entity: entity, ID: id}
}

// OpenShiftObjectNotFoundError means the requested Openshift object does not exist
type OpenShiftObjectNotFoundError struct {
	ObjectURL string
	Message   string
}

// NewOpenShiftObjectNotFoundError returns the custom defined error of type OpenShiftObjectNotFoundError.
func NewOpenShiftObjectNotFoundError(objectURL, message string) OpenShiftObjectNotFoundError {
	return OpenShiftObjectNotFoundError{ObjectURL: objectURL, Message: message}
}

func (err OpenShiftObjectNotFoundError) Error() string {
	return err.Message
}

// OpenShiftObjectConflictError means the requested on an Openshift object conflicts with another resource or operation in progress
type OpenShiftObjectConflictError struct {
	message string
}

// NewOpenShiftObjectConflictError returns the custom defined error of type OpenShiftObjectConflictError.
func NewOpenShiftObjectConflictError(message string) OpenShiftObjectConflictError {
	return OpenShiftObjectConflictError{message: message}
}

func (err OpenShiftObjectConflictError) Error() string {
	return err.message
}
