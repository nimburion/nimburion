package schema

import (
	"context"
	"strings"

	"github.com/nimburion/nimburion/pkg/eventbus"
)

// MessageProducerValidator validates outbound messages and enriches schema headers.
type MessageProducerValidator struct {
	Registry       Registry
	Mode           ValidationMode
	DefaultVersion string
}

// NewMessageProducerValidator creates a producer validator instance.
func NewMessageProducerValidator(registry Registry, mode ValidationMode) *MessageProducerValidator {
	return &MessageProducerValidator{
		Registry:       registry,
		Mode:           normalizeMode(mode),
		DefaultVersion: "v1",
	}
}

// ValidateBeforePublish validates outbound messages before broker publish.
func (v *MessageProducerValidator) ValidateBeforePublish(ctx context.Context, topic string, msg *eventbus.Message) error {
	_ = ctx
	if msg == nil || v == nil || v.Registry == nil {
		return nil
	}
	if msg.Headers == nil {
		msg.Headers = map[string]string{}
	}

	subject := strings.TrimSpace(msg.Headers["schema_subject"])
	if subject == "" {
		subject = strings.TrimSpace(topic)
	}
	version := strings.TrimSpace(msg.Headers["schema_version"])
	if version == "" {
		version = v.defaultVersion()
	}

	desc, err := v.Registry.Resolve(subject, version, msg.Headers)
	if err != nil {
		return v.handleFailure(msg, err)
	}
	if err := v.Registry.ValidateHeaders(desc, msg.Headers); err != nil {
		return v.handleFailure(msg, err)
	}
	if err := v.Registry.ValidatePayload(desc, msg.Value); err != nil {
		return v.handleFailure(msg, err)
	}

	msg.Headers["schema_subject"] = desc.Subject
	msg.Headers["schema_version"] = desc.Version
	msg.Headers["schema_id"] = desc.SchemaID
	msg.Headers["schema_hash"] = desc.SchemaHash
	msg.Headers["schema_message"] = desc.Message
	return nil
}

// MessageConsumerValidator validates inbound messages before business handling.
type MessageConsumerValidator struct {
	Registry       Registry
	Mode           ValidationMode
	DefaultVersion string
}

// NewMessageConsumerValidator creates a consumer validator instance.
func NewMessageConsumerValidator(registry Registry, mode ValidationMode) *MessageConsumerValidator {
	return &MessageConsumerValidator{
		Registry:       registry,
		Mode:           normalizeMode(mode),
		DefaultVersion: "v1",
	}
}

// ValidateAfterConsume validates consumed messages before handler execution.
func (v *MessageConsumerValidator) ValidateAfterConsume(ctx context.Context, topic string, msg *eventbus.Message) error {
	_ = ctx
	if msg == nil || v == nil || v.Registry == nil {
		return nil
	}
	headers := msg.Headers
	if headers == nil {
		headers = map[string]string{}
		msg.Headers = headers
	}
	subject := strings.TrimSpace(headers["schema_subject"])
	if subject == "" {
		subject = strings.TrimSpace(topic)
	}
	version := strings.TrimSpace(headers["schema_version"])
	if version == "" {
		version = v.defaultVersion()
	}

	desc, err := v.Registry.Resolve(subject, version, headers)
	if err != nil {
		return v.handleFailure(msg, err)
	}
	if err := v.Registry.ValidateHeaders(desc, headers); err != nil {
		return v.handleFailure(msg, err)
	}
	if err := v.Registry.ValidatePayload(desc, msg.Value); err != nil {
		return v.handleFailure(msg, err)
	}
	return nil
}

func normalizeMode(mode ValidationMode) ValidationMode {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case string(ValidationModeWarn):
		return ValidationModeWarn
	default:
		return ValidationModeEnforce
	}
}

func (v *MessageProducerValidator) defaultVersion() string {
	defaultVersion := strings.TrimSpace(v.DefaultVersion)
	if defaultVersion == "" {
		return "v1"
	}
	return defaultVersion
}

func (v *MessageConsumerValidator) defaultVersion() string {
	defaultVersion := strings.TrimSpace(v.DefaultVersion)
	if defaultVersion == "" {
		return "v1"
	}
	return defaultVersion
}

func (v *MessageProducerValidator) handleFailure(msg *eventbus.Message, err error) error {
	if v.Mode == ValidationModeWarn {
		if msg.Headers == nil {
			msg.Headers = map[string]string{}
		}
		msg.Headers["schema_validation_warn"] = err.Error()
		return nil
	}
	return err
}

func (v *MessageConsumerValidator) handleFailure(msg *eventbus.Message, err error) error {
	if v.Mode == ValidationModeWarn {
		if msg.Headers == nil {
			msg.Headers = map[string]string{}
		}
		msg.Headers["schema_validation_warn"] = err.Error()
		return nil
	}
	return err
}
