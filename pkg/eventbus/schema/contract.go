package schema

import (
	"context"
	"fmt"

	"github.com/nimburion/nimburion/pkg/eventbus"
)

// ValidationMode controls runtime behavior on schema validation failures.
// ValidationMode defines how schema validation failures are handled.
type ValidationMode string

// Schema validation mode constants
const (
	// ValidationModeWarn logs validation errors but allows processing
	ValidationModeWarn ValidationMode = "warn"
	// ValidationModeEnforce rejects messages that fail validation
	ValidationModeEnforce ValidationMode = "enforce"
)

// CompatibilityPolicy defines schema evolution compatibility.
type CompatibilityPolicy string

// Schema compatibility policy constants
const (
	// CompatibilityBackward allows reading old data with new schema
	CompatibilityBackward CompatibilityPolicy = "BACKWARD"
	// CompatibilityFull allows both forward and backward compatibility
	CompatibilityFull CompatibilityPolicy = "FULL"
)

// Descriptor identifies one concrete schema version for an event subject.
type Descriptor struct {
	Subject    string
	Version    string
	SchemaID   string
	SchemaHash string
	Message    string
}

// Registry resolves and validates event schemas using in-process descriptors.
type Registry interface {
	Resolve(subject, version string, headers map[string]string) (*Descriptor, error)
	ValidatePayload(desc *Descriptor, payload []byte) error
	ValidateHeaders(desc *Descriptor, headers map[string]string) error
}

// ProducerValidator validates and enriches outbound messages before publish.
type ProducerValidator interface {
	ValidateBeforePublish(ctx context.Context, topic string, msg *eventbus.Message) error
}

// ConsumerValidator validates inbound messages before business processing.
type ConsumerValidator interface {
	ValidateAfterConsume(ctx context.Context, topic string, msg *eventbus.Message) error
}

// ValidationError represents a stable schema validation failure.
type ValidationError struct {
	Code    string
	Subject string
	Version string
	Details map[string]interface{}
	Cause   error
}

// Error implements the error interface.
func (e *ValidationError) Error() string {
	if e == nil {
		return ""
	}
	if e.Cause != nil {
		return fmt.Sprintf("%s: %v", e.Code, e.Cause)
	}
	return e.Code
}

// Unwrap exposes the wrapped cause.
func (e *ValidationError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}
