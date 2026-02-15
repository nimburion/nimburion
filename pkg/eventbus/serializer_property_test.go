package eventbus

import (
	"reflect"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// TestMessage is a simple struct for JSON serialization testing
type TestMessage struct {
	ID      string            `json:"id"`
	Content string            `json:"content"`
	Count   int               `json:"count"`
	Active  bool              `json:"active"`
	Tags    []string          `json:"tags"`
	Meta    map[string]string `json:"meta"`
}

// **Validates: Requirements 27.1, 27.2, 27.3, 27.5**
//
// Property 18: Message Serialization Round-Trip
//
// For any message serializer (JSON, Protobuf) and any valid message object,
// serializing then deserializing should produce an equivalent object with
// the correct content-type metadata.
func TestProperty_MessageSerializationRoundTrip(t *testing.T) {
	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Test JSON serialization round-trip
	properties.Property("JSON serializer round-trip preserves data", prop.ForAll(
		func(id, content string, count int, active bool, tags []string) bool {
			// Create test message
			original := &TestMessage{
				ID:      id,
				Content: content,
				Count:   count,
				Active:  active,
				Tags:    tags,
				Meta:    map[string]string{"key1": "value1", "key2": "value2"},
			}

			serializer := NewJSONSerializer()

			// Verify content type
			if serializer.ContentType() != "application/json" {
				t.Logf("Expected content type 'application/json', got '%s'", serializer.ContentType())
				return false
			}

			// Serialize
			data, err := serializer.Serialize(original)
			if err != nil {
				t.Logf("Serialization failed: %v", err)
				return false
			}

			// Deserialize
			var deserialized TestMessage
			err = serializer.Deserialize(data, &deserialized)
			if err != nil {
				t.Logf("Deserialization failed: %v", err)
				return false
			}

			// Compare
			if !reflect.DeepEqual(original, &deserialized) {
				t.Logf("Round-trip failed: original=%+v, deserialized=%+v", original, deserialized)
				return false
			}

			return true
		},
		gen.Identifier(),
		gen.AlphaString(),
		gen.Int(),
		gen.Bool(),
		gen.SliceOf(gen.AlphaString()),
	))

	// Test Protobuf serialization round-trip
	properties.Property("Protobuf serializer round-trip preserves data", prop.ForAll(
		func(value string) bool {
			// Use wrapperspb.StringValue as a simple protobuf message
			original := wrapperspb.String(value)

			serializer := NewProtobufSerializer()

			// Verify content type
			if serializer.ContentType() != "application/protobuf" {
				t.Logf("Expected content type 'application/protobuf', got '%s'", serializer.ContentType())
				return false
			}

			// Serialize
			data, err := serializer.Serialize(original)
			if err != nil {
				t.Logf("Serialization failed: %v", err)
				return false
			}

			// Deserialize
			deserialized := &wrapperspb.StringValue{}
			err = serializer.Deserialize(data, deserialized)
			if err != nil {
				t.Logf("Deserialization failed: %v", err)
				return false
			}

			// Compare
			if !proto.Equal(original, deserialized) {
				t.Logf("Round-trip failed: original=%+v, deserialized=%+v", original, deserialized)
				return false
			}

			return true
		},
		gen.AlphaString(),
	))

	// Test with nested protobuf messages
	properties.Property("Protobuf serializer handles complex messages", prop.ForAll(
		func(intVal int64, boolVal bool, strVal string) bool {
			// Create a more complex protobuf message using multiple wrapper types
			original := &wrapperspb.Int64Value{Value: intVal}

			serializer := NewProtobufSerializer()

			// Serialize
			data, err := serializer.Serialize(original)
			if err != nil {
				t.Logf("Serialization failed: %v", err)
				return false
			}

			// Deserialize
			deserialized := &wrapperspb.Int64Value{}
			err = serializer.Deserialize(data, deserialized)
			if err != nil {
				t.Logf("Deserialization failed: %v", err)
				return false
			}

			// Compare
			if !proto.Equal(original, deserialized) {
				t.Logf("Round-trip failed: original=%+v, deserialized=%+v", original, deserialized)
				return false
			}

			return true
		},
		gen.Int64(),
		gen.Bool(),
		gen.AlphaString(),
	))

	properties.TestingRun(t)
}
