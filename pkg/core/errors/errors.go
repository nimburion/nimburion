// Package errors defines transport-neutral application error contracts.
package errors

import (
	stderrors "errors"
	"fmt"
	"net/http"
	"sync"
)

// Canonical application error codes shared across runtime and transport layers.
const (
	CodeValidationFailed = "validation.failed"
	CodeInvalidArgument  = "argument.invalid"
	CodeNotFound         = "resource.not_found"
	CodeConflict         = "resource.conflict"
	CodeUnauthorized     = "auth.unauthorized"
	CodeForbidden        = "auth.forbidden"
	CodeTimeout          = "operation.timeout"
	CodeRetryable        = "operation.retryable"
	CodeUnavailable      = "service.unavailable"
	CodeClosed           = "resource.closed"
	CodeNotInitialized   = "resource.not_initialized"
	CodeInternal         = "internal.error"
)

// Params carries structured values associated with one application error code.
type Params map[string]interface{}

// Message is the transport-neutral application message payload.
type Message struct {
	Code   string `json:"code"`
	Params Params `json:"params,omitempty"`
}

// AppError is the core application error contract shared across runtime layers.
type AppError struct {
	Code            string
	FallbackMessage string
	Params          Params
	Details         map[string]interface{}
	HTTPStatus      int
	Cause           error
}

// Canonicalizer lifts package-specific errors into the canonical AppError model.
type Canonicalizer func(error) (*AppError, bool)

var (
	canonicalizersMu sync.RWMutex
	canonicalizers   []Canonicalizer
)

// Error implements the error interface.
func (e *AppError) Error() string {
	if e == nil {
		return ""
	}
	label := e.Code
	if e.FallbackMessage != "" {
		label = e.FallbackMessage
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", label, e.Cause)
	}
	return label
}

// Unwrap exposes the wrapped cause for errors.Is and errors.As.
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Message returns the message payload represented by this error.
func (e *AppError) Message() Message {
	if e == nil {
		return Message{}
	}
	return NewMessage(e.Code, e.Params)
}

// NewMessage creates a transport-neutral application message.
func NewMessage(code string, params Params) Message {
	return Message{
		Code:   code,
		Params: cloneParams(params),
	}
}

// New creates an application error with a stable code and optional cause.
func New(code string, params Params, cause error) *AppError {
	return &AppError{
		Code:   code,
		Params: cloneParams(params),
		Cause:  cause,
	}
}

// WithMessage sets a non-localized fallback message.
func (e *AppError) WithMessage(message string) *AppError {
	if e == nil {
		return nil
	}
	e.FallbackMessage = message
	return e
}

// WithHTTPStatus sets an explicit transport mapping hint.
func (e *AppError) WithHTTPStatus(status int) *AppError {
	if e == nil {
		return nil
	}
	e.HTTPStatus = status
	return e
}

// WithDetails sets structured error details.
func (e *AppError) WithDetails(details map[string]interface{}) *AppError {
	if e == nil {
		return nil
	}
	e.Details = details
	return e
}

// NewValidation creates a validation error with an optional fallback message and details.
func NewValidation(message string, details map[string]interface{}) *AppError {
	return New(CodeValidationFailed, nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusBadRequest).
		WithDetails(details)
}

// NewValidationWithCode creates a localized validation error with a stable code.
func NewValidationWithCode(code, fallbackMessage string, params Params, details map[string]interface{}) *AppError {
	return New(code, params, nil).
		WithMessage(fallbackMessage).
		WithHTTPStatus(http.StatusBadRequest).
		WithDetails(details)
}

// NewInvalidArgument creates an invalid-argument error.
func NewInvalidArgument(message string, cause error) *AppError {
	return New(CodeInvalidArgument, nil, cause).
		WithMessage(message).
		WithHTTPStatus(http.StatusBadRequest)
}

// NewNotFound creates a not-found error.
func NewNotFound(message string) *AppError {
	return New(CodeNotFound, nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusNotFound)
}

// NewConflict creates a conflict error.
func NewConflict(message string, details map[string]interface{}) *AppError {
	return New(CodeConflict, nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusConflict).
		WithDetails(details)
}

// NewUnauthorized creates an unauthorized error.
func NewUnauthorized(message string) *AppError {
	return New(CodeUnauthorized, nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusUnauthorized)
}

// NewForbidden creates a forbidden error.
func NewForbidden(message string) *AppError {
	return New(CodeForbidden, nil, nil).
		WithMessage(message).
		WithHTTPStatus(http.StatusForbidden)
}

// NewTimeout creates a timeout error.
func NewTimeout(message string, cause error) *AppError {
	return New(CodeTimeout, nil, cause).
		WithMessage(message).
		WithHTTPStatus(http.StatusGatewayTimeout)
}

// NewUnavailable creates an unavailable error.
func NewUnavailable(message string, cause error) *AppError {
	return New(CodeUnavailable, nil, cause).
		WithMessage(message).
		WithHTTPStatus(http.StatusServiceUnavailable)
}

// NewRetryable creates a retryable error.
func NewRetryable(message string, cause error) *AppError {
	return New(CodeRetryable, nil, cause).
		WithMessage(message).
		WithHTTPStatus(http.StatusServiceUnavailable)
}

// NewClosed creates a closed-resource error.
func NewClosed(message string, cause error) *AppError {
	return New(CodeClosed, nil, cause).
		WithMessage(message).
		WithHTTPStatus(http.StatusConflict)
}

// NewNotInitialized creates a not-initialized error.
func NewNotInitialized(message string, cause error) *AppError {
	return New(CodeNotInitialized, nil, cause).
		WithMessage(message).
		WithHTTPStatus(http.StatusServiceUnavailable)
}

// NewInternal creates an internal error.
func NewInternal(message string, cause error) *AppError {
	return New(CodeInternal, nil, cause).
		WithMessage(message).
		WithHTTPStatus(http.StatusInternalServerError)
}

// RegisterCanonicalizer adds one family-level canonicalizer to the shared registry.
func RegisterCanonicalizer(c Canonicalizer) {
	if c == nil {
		return
	}
	canonicalizersMu.Lock()
	defer canonicalizersMu.Unlock()
	canonicalizers = append(canonicalizers, c)
}

// Canonicalize returns a canonical application error when the input is recognized.
// When err is already an AppError it is returned unchanged.
func Canonicalize(err error) error {
	if err == nil {
		return nil
	}

	var appErr *AppError
	if stderrors.As(err, &appErr) {
		return err
	}

	canonicalizersMu.RLock()
	registered := append([]Canonicalizer(nil), canonicalizers...)
	canonicalizersMu.RUnlock()

	for _, canonicalizer := range registered {
		if canonical, ok := canonicalizer(err); ok {
			return canonical
		}
	}

	return err
}

// AsAppError returns one canonical AppError when the error is already canonical
// or can be recognized by a registered family-level canonicalizer.
func AsAppError(err error) (*AppError, bool) {
	if err == nil {
		return nil, false
	}
	canonical := Canonicalize(err)
	var appErr *AppError
	if stderrors.As(canonical, &appErr) {
		return appErr, true
	}
	return nil, false
}

func cloneParams(params Params) Params {
	if len(params) == 0 {
		return nil
	}
	out := make(Params, len(params))
	for key, value := range params {
		out[key] = value
	}
	return out
}
