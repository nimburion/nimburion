package eventbus

import (
	"errors"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

// **Validates: Requirements 27.6**
//
// Property 19: Serialization Error Handling
//
// For any serializer, when given invalid data for deserialization,
// the serializer should return a descriptive error indicating what went wrong.
func TestProperty_SerializationErrorHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping property test in short mode")
	}

	parameters := gopter.DefaultTestParameters()
	parameters.MinSuccessfulTests = 100
	properties := gopter.NewProperties(parameters)

	// Test JSON deserializer with invalid data
	properties.Property("JSON deserializer returns error for invalid data", prop.ForAll(
		func(invalidData []byte) bool {
			serializer := NewJSONSerializer()
			var target TestMessage

			err := serializer.Deserialize(invalidData, &target)
			
			// Should return an error
			if err == nil {
				t.Logf("Expected error for invalid data, got nil")
				return false
			}

			// Error should be descriptive (not empty)
			if err.Error() == "" {
				t.Logf("Error message is empty")
				return false
			}

			return true
		},
		gen.SliceOfN(100, gen.UInt8()), // Generate random byte slices
	))

	// Test JSON deserializer with empty data
	properties.Property("JSON deserializer returns error for empty data", prop.ForAll(
		func() bool {
			serializer := NewJSONSerializer()
			var target TestMessage

			err := serializer.Deserialize([]byte{}, &target)
			
			// Should return an error
			if err == nil {
				t.Logf("Expected error for empty data, got nil")
				return false
			}

			// Error should mention invalid data
			if !errors.Is(err, ErrInvalidData) && err.Error() == "" {
				t.Logf("Error should be descriptive, got: %v", err)
				return false
			}

			return true
		},
	))

	// Test JSON deserializer with nil target
	properties.Property("JSON deserializer returns error for nil target", prop.ForAll(
		func(data []byte) bool {
			serializer := NewJSONSerializer()

			err := serializer.Deserialize(data, nil)
			
			// Should return an error
			if err == nil {
				t.Logf("Expected error for nil target, got nil")
				return false
			}

			// Error should mention invalid data
			if !errors.Is(err, ErrInvalidData) {
				t.Logf("Expected ErrInvalidData, got: %v", err)
				return false
			}

			return true
		},
		gen.SliceOf(gen.UInt8()),
	))

	// Test JSON serializer with nil value
	properties.Property("JSON serializer returns error for nil value", prop.ForAll(
		func() bool {
			serializer := NewJSONSerializer()

			_, err := serializer.Serialize(nil)
			
			// Should return an error
			if err == nil {
				t.Logf("Expected error for nil value, got nil")
				return false
			}

			// Error should mention invalid data
			if !errors.Is(err, ErrInvalidData) {
				t.Logf("Expected ErrInvalidData, got: %v", err)
				return false
			}

			return true
		},
	))

	// Test Protobuf deserializer with invalid data
	// Use data that is definitely invalid: truncated field tags
	properties.Property("Protobuf deserializer returns error for invalid data", prop.ForAll(
		func(seed byte) bool {
			serializer := NewProtobufSerializer()
			target := &wrapperspb.StringValue{}

			// Create invalid protobuf: incomplete varint (field tag without value)
			invalidData := []byte{0x80 | seed} // High bit set but no continuation byte

			err := serializer.Deserialize(invalidData, target)
			
			// Should return an error for truncated data
			if err == nil {
				// Empty or all-zero data might be valid (default values)
				// Only fail if we have actual invalid structure
				if len(invalidData) > 0 && invalidData[0] != 0 {
					t.Logf("Expected error for invalid data, got nil")
					return false
				}
			}

			return true
		},
		gen.UInt8(),
	))

	// Test Protobuf deserializer with empty data
	// Note: Empty data is valid for protobuf (represents default/zero values)
	properties.Property("Protobuf deserializer accepts empty data", prop.ForAll(
		func() bool {
			serializer := NewProtobufSerializer()
			target := &wrapperspb.StringValue{}

			err := serializer.Deserialize([]byte{}, target)
			
			// Should NOT return an error - empty data is valid for protobuf
			if err != nil {
				t.Logf("Expected no error for empty data, got: %v", err)
				return false
			}

			// Target should be reset to default state
			if target.Value != "" {
				t.Logf("Expected empty string value, got: %s", target.Value)
				return false
			}

			return true
		},
	))

	// Test Protobuf deserializer with nil target
	properties.Property("Protobuf deserializer returns error for nil target", prop.ForAll(
		func(data []byte) bool {
			serializer := NewProtobufSerializer()

			err := serializer.Deserialize(data, nil)
			
			// Should return an error
			if err == nil {
				t.Logf("Expected error for nil target, got nil")
				return false
			}

			// Error should mention invalid data
			if !errors.Is(err, ErrInvalidData) {
				t.Logf("Expected ErrInvalidData, got: %v", err)
				return false
			}

			return true
		},
		gen.SliceOf(gen.UInt8()),
	))

	// Test Protobuf serializer with nil value
	properties.Property("Protobuf serializer returns error for nil value", prop.ForAll(
		func() bool {
			serializer := NewProtobufSerializer()

			_, err := serializer.Serialize(nil)
			
			// Should return an error
			if err == nil {
				t.Logf("Expected error for nil value, got nil")
				return false
			}

			// Error should mention invalid data
			if !errors.Is(err, ErrInvalidData) {
				t.Logf("Expected ErrInvalidData, got: %v", err)
				return false
			}

			return true
		},
	))

	// Test Protobuf serializer with non-protobuf type
	properties.Property("Protobuf serializer returns error for non-protobuf type", prop.ForAll(
		func(str string) bool {
			serializer := NewProtobufSerializer()

			// Try to serialize a regular string (not a proto.Message)
			_, err := serializer.Serialize(str)
			
			// Should return an error
			if err == nil {
				t.Logf("Expected error for non-protobuf type, got nil")
				return false
			}

			// Error should mention unsupported type
			if !errors.Is(err, ErrUnsupportedType) {
				t.Logf("Expected ErrUnsupportedType, got: %v", err)
				return false
			}

			return true
		},
		gen.AlphaString(),
	))

	// Test Protobuf deserializer with non-protobuf target
	properties.Property("Protobuf deserializer returns error for non-protobuf target", prop.ForAll(
		func(data []byte) bool {
			serializer := NewProtobufSerializer()
			var target string // Not a proto.Message

			err := serializer.Deserialize(data, &target)
			
			// Should return an error
			if err == nil {
				t.Logf("Expected error for non-protobuf target, got nil")
				return false
			}

			// Error should mention unsupported type
			if !errors.Is(err, ErrUnsupportedType) {
				t.Logf("Expected ErrUnsupportedType, got: %v", err)
				return false
			}

			return true
		},
		gen.SliceOf(gen.UInt8()),
	))

	properties.TestingRun(t)
}
