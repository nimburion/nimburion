package i18n

import (
	"fmt"
	"sort"
)

// Params carries dynamic values used to interpolate a localized message template.
type Params map[string]interface{}

// AppMessage is the transport-agnostic contract for application messages.
// Services should return this shape (code + params) instead of localized strings.
type AppMessage struct {
	Code   string `json:"code"`
	Params Params `json:"params,omitempty"`
}

// NewMessage creates an AppMessage with the given code and params.
func NewMessage(code string, params Params) AppMessage {
	return AppMessage{
		Code:   code,
		Params: cloneParams(params),
	}
}

// AppError is an error contract suitable for i18n:
// stable code + params + optional wrapped cause.
type AppError struct {
	Code            string
	FallbackMessage string
	Params          Params
	Details         map[string]interface{}
	HTTPStatus      int
	Cause           error
}

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

// Unwrap exposes the wrapped cause for errors.Is / errors.As.
func (e *AppError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// Message returns the i18n message payload represented by this error.
func (e *AppError) Message() AppMessage {
	if e == nil {
		return AppMessage{}
	}
	return NewMessage(e.Code, e.Params)
}

// NewError creates an AppError with a stable message code.
func NewError(code string, params Params, cause error) *AppError {
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

// WithHTTPStatus sets an explicit HTTP status for this error.
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

// Translator resolves a message key into a localized text.
type Translator interface {
	T(key string, args ...interface{}) string
}

// CanonicalParams returns a deterministic list of keys.
// Useful for tests/logging/telemetry.
func CanonicalParams(params Params) []string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
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
