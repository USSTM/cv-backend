package api

import (
	genapi "github.com/USSTM/cv-backend/generated/api"
)

const (
	CodeValidationError   = "VALIDATION_ERROR"
	CodeAuthRequired      = "AUTHENTICATION_REQUIRED"
	CodePermissionDenied  = "PERMISSION_DENIED"
	CodeResourceNotFound  = "RESOURCE_NOT_FOUND"
	CodeInsufficientStock = "INSUFFICIENT_STOCK"
	CodeConflict          = "CONFLICT"
	CodeInternalError     = "INTERNAL_ERROR"
)

type ErrorDetail struct {
	Field   string
	Message string
}

// additional error context
type ErrorContext map[string]interface{}

// builder pattern
type ErrorBuilder struct {
	Code    string
	Message string
	Details []ErrorDetail
	Context ErrorContext
}

func NewError(code, message string) *ErrorBuilder {
	return &ErrorBuilder{Code: code, Message: message}
}

func (e *ErrorBuilder) WithDetails(details []ErrorDetail) *ErrorBuilder {
	e.Details = details
	return e
}

func (e *ErrorBuilder) WithContext(context ErrorContext) *ErrorBuilder {
	e.Context = context
	return e
}

func (e *ErrorBuilder) Create() genapi.Error {
	var details *[]struct {
		Field   string `json:"field"`
		Message string `json:"message"`
	}
	if len(e.Details) > 0 {
		apiDetails := make([]struct {
			Field   string `json:"field"`
			Message string `json:"message"`
		}, len(e.Details))
		for i, d := range e.Details {
			apiDetails[i] = struct {
				Field   string `json:"field"`
				Message string `json:"message"`
			}{
				Field:   d.Field,
				Message: d.Message,
			}
		}
		details = &apiDetails
	}

	var context *map[string]interface{}
	if len(e.Context) > 0 {
		ctx := map[string]interface{}(e.Context)
		context = &ctx
	}

	return genapi.Error{
		Error: struct {
			Code    genapi.ErrorErrorCode   `json:"code"`
			Context *map[string]interface{} `json:"context,omitempty"`
			Details *[]struct {
				Field   string `json:"field"`
				Message string `json:"message"`
			} `json:"details,omitempty"`
			Message string `json:"message"`
		}{
			Code:    genapi.ErrorErrorCode(e.Code),
			Message: e.Message,
			Details: details,
			Context: context,
		},
	}
}

// builder pattern extensions

func Unauthorized(msg string) *ErrorBuilder {
	return NewError(CodeAuthRequired, msg)
}

func PermissionDenied(msg string) *ErrorBuilder {
	return NewError(CodePermissionDenied, msg)
}

func NotFound(resource string) *ErrorBuilder {
	return NewError(CodeResourceNotFound, resource+" not found")
}

func ValidationErr(msg string, details []ErrorDetail) *ErrorBuilder {
	return NewError(CodeValidationError, msg).WithDetails(details)
}

func InsufficientStockErr(itemName string, requested, available int) *ErrorBuilder {
	return NewError(CodeInsufficientStock, "Insufficient stock available").
		WithContext(ErrorContext{
			"item_name": itemName,
			"requested": requested,
			"available": available,
		})
}

func InternalError(msg string) *ErrorBuilder {
	return NewError(CodeInternalError, msg)
}

func ConflictErr(msg string) *ErrorBuilder {
	return NewError(CodeConflict, msg)
}
