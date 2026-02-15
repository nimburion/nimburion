package eventbus

import "errors"

// Common serialization errors
var (
	// ErrInvalidData is returned when the data cannot be serialized or deserialized
	ErrInvalidData = errors.New("invalid data for serialization")

	// ErrUnsupportedType is returned when the type is not supported by the serializer
	ErrUnsupportedType = errors.New("unsupported type for serialization")
)

// Serializer defines the interface for message serialization.
// Implementations must support serializing Go objects to bytes and deserializing bytes back to objects.
type Serializer interface {
	// Serialize converts a Go object to bytes.
	// Returns the serialized bytes and an error if serialization fails.
	Serialize(v interface{}) ([]byte, error)

	// Deserialize converts bytes back to a Go object.
	// The target parameter must be a pointer to the destination object.
	// Returns an error if deserialization fails.
	Deserialize(data []byte, target interface{}) error

	// ContentType returns the MIME type for this serializer.
	// Examples: "application/json", "application/protobuf", "application/avro"
	ContentType() string
}
