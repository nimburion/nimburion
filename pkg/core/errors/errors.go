// Package errors defines transport-neutral application error contracts.
package errors

import "fmt"

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
