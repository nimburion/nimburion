package eventbus

import (
	"encoding/json"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// TestProperty_EventEnvelopeCompleteness validates Property 2: Event Envelope Completeness.
//
// For any published event, all 17 core required envelope fields must be present and valid.
//
// **Validates: Requirements 18.1**
func TestProperty_EventEnvelopeCompleteness(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	properties.Property("serialized envelopes preserve all required core fields", prop.ForAll(
		func(
			id string,
			domain string,
			eventName string,
			tenantID string,
			aggregateID string,
			aggregateType string,
			producer string,
			correlationID string,
			actorID string,
			versionMajor uint8,
			aggregateVersion uint16,
			payloadValue string,
		) bool {
			trimmed := []string{
				strings.TrimSpace(id),
				strings.TrimSpace(domain),
				strings.TrimSpace(eventName),
				strings.TrimSpace(tenantID),
				strings.TrimSpace(aggregateID),
				strings.TrimSpace(aggregateType),
				strings.TrimSpace(producer),
				strings.TrimSpace(correlationID),
				strings.TrimSpace(actorID),
			}
			for _, v := range trimmed {
				if v == "" {
					return true
				}
			}

			e := &EventEnvelope{
				ID:               id,
				Type:             fmt.Sprintf("acme.%s.%s.v%d", domain, strings.ReplaceAll(eventName, "_", "."), versionMajor+1),
				Domain:           domain,
				EventName:        eventName,
				Version:          fmt.Sprintf("%d.0.0", versionMajor+1),
				TenantID:         tenantID,
				AggregateID:      aggregateID,
				AggregateType:    aggregateType,
				AggregateVersion: int64(aggregateVersion%1000 + 1),
				Producer:         producer,
				CorrelationID:    correlationID,
				ActorID:          actorID,
				ActorType:        ActorTypeUser,
				OccurredAt:       time.Now().UTC(),
				DataClass:        DataClassInternal,
				Payload:          json.RawMessage(fmt.Sprintf(`{"value":%q}`, payloadValue)),
			}
			e.EnsurePartitionKey()

			encoded, err := e.Serialize()
			if err != nil {
				t.Logf("serialize failed: %v", err)
				return false
			}

			decoded, err := DeserializeEventEnvelope(encoded)
			if err != nil {
				t.Logf("deserialize failed: %v", err)
				return false
			}

			return decoded.ID != "" &&
				decoded.Type != "" &&
				decoded.Domain != "" &&
				decoded.EventName != "" &&
				decoded.Version != "" &&
				decoded.TenantID != "" &&
				decoded.AggregateID != "" &&
				decoded.AggregateType != "" &&
				decoded.AggregateVersion >= 1 &&
				decoded.Producer != "" &&
				decoded.PartitionKey == PartitionKey(decoded.TenantID, decoded.AggregateID) &&
				decoded.CorrelationID != "" &&
				decoded.ActorID != "" &&
				decoded.ActorType == ActorTypeUser &&
				!decoded.OccurredAt.IsZero() &&
				len(decoded.Payload) > 0 &&
				decoded.DataClass == DataClassInternal
		},
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.Identifier(),
		gen.UInt8(),
		gen.UInt16(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
