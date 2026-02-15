package eventbus

import (
	"encoding/json"
	"fmt"
)

// JSONSerializer implements the Serializer interface using encoding/json.
// It provides JSON serialization for message payloads.
type JSONSerializer struct{}

// NewJSONSerializer creates a new JSON serializer.
func NewJSONSerializer() *JSONSerializer {
	return &JSONSerializer{}
}

// Serialize converts a Go object to JSON bytes.
func (s *JSONSerializer) Serialize(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, fmt.Errorf("%w: cannot serialize nil value", ErrInvalidData)
	}

	data, err := json.Marshal(v)
	if err != nil {
		return nil, fmt.Errorf("json serialization failed: %w", err)
	}

	return data, nil
}

// Deserialize converts JSON bytes back to a Go object.
// The target parameter must be a pointer to the destination object.
func (s *JSONSerializer) Deserialize(data []byte, target interface{}) error {
	if target == nil {
		return fmt.Errorf("%w: target cannot be nil", ErrInvalidData)
	}

	if len(data) == 0 {
		return fmt.Errorf("%w: cannot deserialize empty data", ErrInvalidData)
	}

	if err := json.Unmarshal(data, target); err != nil {
		return fmt.Errorf("json deserialization failed: %w", err)
	}

	return nil
}

// ContentType returns the MIME type for JSON serialization.
func (s *JSONSerializer) ContentType() string {
	return "application/json"
}
