package validation

import (
	"context"
	"errors"
	"fmt"
)

// Layer identifies one validation stage in the gRPC pipeline.
type Layer string

const (
	// LayerTransport identifies transport-level validation.
	LayerTransport Layer = "transport"
	// LayerContract identifies contract-level validation.
	LayerContract Layer = "contract"
	// LayerDomain identifies domain-level validation.
	LayerDomain Layer = "domain"
)

// Validator validates one request payload at a specific layer.
type Validator interface {
	Validate(context.Context, any) error
}

// ValidatorFunc adapts a function to Validator.
type ValidatorFunc func(context.Context, any) error

// Validate implements Validator.
func (fn ValidatorFunc) Validate(ctx context.Context, req any) error {
	return fn(ctx, req)
}

// Violation describes one validation problem.
type Violation struct {
	Field       string
	Description string
}

// Error carries one validation-layer failure.
type Error struct {
	Layer      Layer
	Message    string
	Violations []Violation
	Cause      error
}

// Error implements error.
func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	if e.Message != "" {
		return e.Message
	}
	if e.Cause != nil {
		return e.Cause.Error()
	}
	return fmt.Sprintf("%s validation failed", e.Layer)
}

// Unwrap returns the wrapped cause.
func (e *Error) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

// NewError builds one validation error for a layer.
func NewError(layer Layer, message string, cause error, violations ...Violation) *Error {
	return &Error{
		Layer:      layer,
		Message:    message,
		Cause:      cause,
		Violations: append([]Violation(nil), violations...),
	}
}

// IsLayer reports whether err is a validation error for a specific layer.
func IsLayer(err error, layer Layer) bool {
	var validationErr *Error
	if !errors.As(err, &validationErr) {
		return false
	}
	return validationErr.Layer == layer
}

// Pipeline runs transport, contract, then domain validators in order.
type Pipeline struct {
	Transport []Validator
	Contract  []Validator
	Domain    []Validator
}

// Validate runs validators in layered order and stops at the first error.
func (p Pipeline) Validate(ctx context.Context, req any) error {
	for _, item := range []struct {
		layer      Layer
		validators []Validator
	}{
		{layer: LayerTransport, validators: p.Transport},
		{layer: LayerContract, validators: p.Contract},
		{layer: LayerDomain, validators: p.Domain},
	} {
		for _, validator := range item.validators {
			if err := validator.Validate(ctx, req); err != nil {
				var validationErr *Error
				if errors.As(err, &validationErr) {
					return err
				}
				return NewError(item.layer, fmt.Sprintf("%s validation failed", item.layer), err)
			}
		}
	}
	return nil
}
