package eventbus

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

// Event envelope constants
const (
	// EventEnvelopeContentType is the default content type for event envelopes
	EventEnvelopeContentType = "application/json"
)

// DataClass identifies the PII sensitivity of an event.
type DataClass string

// Data classification constants
const (
	// DataClassPublic indicates publicly shareable data
	DataClassPublic DataClass = "public"
	// DataClassInternal indicates internal-only data
	DataClassInternal DataClass = "internal"
	// DataClassRestricted indicates restricted/sensitive data
	DataClassRestricted DataClass = "restricted"
)

// ActorType identifies who triggered the event.
type ActorType string

// Actor type constants
const (
	// ActorTypeUser indicates a user triggered the event
	ActorTypeUser ActorType = "user"
	// ActorTypeService indicates a service triggered the event
	ActorTypeService ActorType = "service"
)

// EventEnvelope defines a generic, structured event envelope.
//
// Core required fields (17):
// id, type, domain, event_name, version, tenant_id, aggregate_id,
// aggregate_type, aggregate_version, producer, partition_key,
// correlation_id, actor_id, actor_type, occurred_at, payload, data_class.
//
// Optional fields: causation_id, schema_id, schema_hash, metadata.
type EventEnvelope struct {
	ID               string            `json:"id"`
	Type             string            `json:"type"`
	Domain           string            `json:"domain"`
	EventName        string            `json:"event_name"`
	Version          string            `json:"version"`
	TenantID         string            `json:"tenant_id"`
	AggregateID      string            `json:"aggregate_id"`
	AggregateType    string            `json:"aggregate_type"`
	AggregateVersion int64             `json:"aggregate_version"`
	Producer         string            `json:"producer"`
	PartitionKey     string            `json:"partition_key"`
	CorrelationID    string            `json:"correlation_id"`
	CausationID      *string           `json:"causation_id,omitempty"`
	ActorID          string            `json:"actor_id"`
	ActorType        ActorType         `json:"actor_type"`
	OccurredAt       time.Time         `json:"occurred_at"`
	Payload          json.RawMessage   `json:"payload"`
	SchemaID         string            `json:"schema_id,omitempty"`
	SchemaHash       string            `json:"schema_hash,omitempty"`
	DataClass        DataClass         `json:"data_class"`
	Metadata         map[string]string `json:"metadata,omitempty"`
}

// PartitionKey returns the stable partition key for ordering guarantees.
func PartitionKey(tenantID, aggregateID string) string {
	return fmt.Sprintf("%s:%s", tenantID, aggregateID)
}

// EnsurePartitionKey sets partition_key to "{tenant_id}:{aggregate_id}" if empty.
func (e *EventEnvelope) EnsurePartitionKey() {
	if e == nil {
		return
	}
	if strings.TrimSpace(e.PartitionKey) == "" {
		e.PartitionKey = PartitionKey(e.TenantID, e.AggregateID)
	}
}

// Validate checks whether the envelope contains all required core fields.
func (e *EventEnvelope) Validate() error {
	if e == nil {
		return errors.New("event envelope is nil")
	}

	trim := func(v string) string { return strings.TrimSpace(v) }

	required := map[string]string{
		"id":             e.ID,
		"type":           e.Type,
		"domain":         e.Domain,
		"event_name":     e.EventName,
		"version":        e.Version,
		"tenant_id":      e.TenantID,
		"aggregate_id":   e.AggregateID,
		"aggregate_type": e.AggregateType,
		"producer":       e.Producer,
		"partition_key":  e.PartitionKey,
		"correlation_id": e.CorrelationID,
		"actor_id":       e.ActorID,
	}

	for field, value := range required {
		if trim(value) == "" {
			return fmt.Errorf("missing required field: %s", field)
		}
	}

	if e.AggregateVersion < 1 {
		return errors.New("aggregate_version must be >= 1")
	}

	if e.OccurredAt.IsZero() {
		return errors.New("occurred_at is required")
	}

	if len(e.Payload) == 0 {
		return errors.New("payload is required")
	}

	switch e.ActorType {
	case ActorTypeUser, ActorTypeService:
	default:
		return fmt.Errorf("invalid actor_type: %q", e.ActorType)
	}

	switch e.DataClass {
	case DataClassPublic, DataClassInternal, DataClassRestricted:
	default:
		return fmt.Errorf("invalid data_class: %q", e.DataClass)
	}

	expectedPartitionKey := PartitionKey(e.TenantID, e.AggregateID)
	if e.PartitionKey != expectedPartitionKey {
		return fmt.Errorf("invalid partition_key: expected %q, got %q", expectedPartitionKey, e.PartitionKey)
	}

	return nil
}

// Serialize marshals the envelope to JSON.
func (e *EventEnvelope) Serialize() ([]byte, error) {
	if err := e.Validate(); err != nil {
		return nil, err
	}

	data, err := json.Marshal(e)
	if err != nil {
		return nil, fmt.Errorf("failed to serialize event envelope: %w", err)
	}
	return data, nil
}

// DeserializeEventEnvelope unmarshals and validates an envelope from JSON.
func DeserializeEventEnvelope(data []byte) (*EventEnvelope, error) {
	if len(data) == 0 {
		return nil, errors.New("cannot deserialize empty envelope")
	}

	var envelope EventEnvelope
	if err := json.Unmarshal(data, &envelope); err != nil {
		return nil, fmt.Errorf("failed to deserialize event envelope: %w", err)
	}

	envelope.EnsurePartitionKey()
	if err := envelope.Validate(); err != nil {
		return nil, err
	}

	return &envelope, nil
}

// ToMessage converts the envelope into an eventbus message.
func (e *EventEnvelope) ToMessage() (*Message, error) {
	payload, err := e.Serialize()
	if err != nil {
		return nil, err
	}

	return &Message{
		ID:          e.ID,
		Key:         e.PartitionKey,
		Value:       payload,
		ContentType: EventEnvelopeContentType,
		Timestamp:   e.OccurredAt,
		Headers: map[string]string{
			"type":           e.Type,
			"tenant_id":      e.TenantID,
			"correlation_id": e.CorrelationID,
			"data_class":     string(e.DataClass),
		},
	}, nil
}

// EventEnvelopeFromMessage decodes an event envelope from a consumed message.
func EventEnvelopeFromMessage(msg *Message) (*EventEnvelope, error) {
	if msg == nil {
		return nil, errors.New("message is nil")
	}
	if len(msg.Value) == 0 {
		return nil, errors.New("message payload is empty")
	}

	envelope, err := DeserializeEventEnvelope(msg.Value)
	if err != nil {
		return nil, err
	}

	return envelope, nil
}
