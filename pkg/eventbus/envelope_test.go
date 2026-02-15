package eventbus

import (
	"encoding/json"
	"testing"
	"time"
)

func ptrString(v string) *string {
	return &v
}

func validEnvelope(t *testing.T) *EventEnvelope {
	t.Helper()

	payload := json.RawMessage(`{"member_id":"mem_1","status":"In_Regola"}`)

	return &EventEnvelope{
		ID:               "evt_1",
		Type:             "acme.members.user.created.v1",
		Domain:           "members",
		EventName:        "user.created",
		Version:          "1.0.0",
		TenantID:         "tenant_1",
		AggregateID:      "mem_1",
		AggregateType:    "user",
		AggregateVersion: 1,
		Producer:         "users-service@1.0.0",
		PartitionKey:     "tenant_1:mem_1",
		CorrelationID:    "corr_1",
		CausationID:      ptrString("evt_0"),
		ActorID:          "user_1",
		ActorType:        ActorTypeUser,
		OccurredAt:       time.Now().UTC().Truncate(time.Millisecond),
		Payload:          payload,
		SchemaID:         "acme.members.user.created.v1/1",
		SchemaHash:       "sha256:abc",
		DataClass:        DataClassInternal,
		Metadata: map[string]string{
			"traceparent": "00-00000000000000000000000000000000-0000000000000000-01",
		},
	}
}

func TestEventEnvelope_Validate(t *testing.T) {
	e := validEnvelope(t)
	if err := e.Validate(); err != nil {
		t.Fatalf("expected valid envelope, got error: %v", err)
	}
}

func TestEventEnvelope_EnsurePartitionKey(t *testing.T) {
	e := validEnvelope(t)
	e.PartitionKey = ""

	e.EnsurePartitionKey()

	if got, want := e.PartitionKey, "tenant_1:mem_1"; got != want {
		t.Fatalf("unexpected partition key: got %q want %q", got, want)
	}
}

func TestEventEnvelope_ValidateInvalidDataClass(t *testing.T) {
	e := validEnvelope(t)
	e.DataClass = "unknown"

	if err := e.Validate(); err == nil {
		t.Fatalf("expected validation error for invalid data class")
	}
}

func TestEventEnvelope_SerializeDeserialize(t *testing.T) {
	e := validEnvelope(t)

	data, err := e.Serialize()
	if err != nil {
		t.Fatalf("serialize failed: %v", err)
	}

	decoded, err := DeserializeEventEnvelope(data)
	if err != nil {
		t.Fatalf("deserialize failed: %v", err)
	}

	if decoded.ID != e.ID {
		t.Fatalf("id mismatch: got %q want %q", decoded.ID, e.ID)
	}
	if decoded.PartitionKey != e.PartitionKey {
		t.Fatalf("partition_key mismatch: got %q want %q", decoded.PartitionKey, e.PartitionKey)
	}
	if string(decoded.Payload) != string(e.Payload) {
		t.Fatalf("payload mismatch: got %s want %s", string(decoded.Payload), string(e.Payload))
	}
}

func TestEventEnvelope_ToMessage(t *testing.T) {
	e := validEnvelope(t)

	msg, err := e.ToMessage()
	if err != nil {
		t.Fatalf("to message failed: %v", err)
	}

	if msg.Key != e.PartitionKey {
		t.Fatalf("key mismatch: got %q want %q", msg.Key, e.PartitionKey)
	}
	if msg.ContentType != EventEnvelopeContentType {
		t.Fatalf("content type mismatch: got %q", msg.ContentType)
	}
	if msg.Headers["tenant_id"] != e.TenantID {
		t.Fatalf("missing tenant header")
	}

	decoded, err := EventEnvelopeFromMessage(msg)
	if err != nil {
		t.Fatalf("from message failed: %v", err)
	}

	if decoded.ID != e.ID {
		t.Fatalf("decoded id mismatch: got %q want %q", decoded.ID, e.ID)
	}
}
