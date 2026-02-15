package eventbus

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

// ProtobufSerializer implements the Serializer interface using Protocol Buffers.
// It provides efficient binary serialization for protobuf message types.
type ProtobufSerializer struct{}

// NewProtobufSerializer creates a new Protocol Buffers serializer.
func NewProtobufSerializer() *ProtobufSerializer {
	return &ProtobufSerializer{}
}

// Serialize converts a protobuf message to bytes.
// The value must implement proto.Message interface.
func (s *ProtobufSerializer) Serialize(v interface{}) ([]byte, error) {
	if v == nil {
		return nil, fmt.Errorf("%w: cannot serialize nil value", ErrInvalidData)
	}

	msg, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("%w: value must implement proto.Message", ErrUnsupportedType)
	}

	data, err := proto.Marshal(msg)
	if err != nil {
		return nil, fmt.Errorf("protobuf serialization failed: %w", err)
	}

	return data, nil
}

// Deserialize converts protobuf bytes back to a message.
// The target parameter must be a pointer to a proto.Message.
func (s *ProtobufSerializer) Deserialize(data []byte, target interface{}) error {
	if target == nil {
		return fmt.Errorf("%w: target cannot be nil", ErrInvalidData)
	}

	msg, ok := target.(proto.Message)
	if !ok {
		return fmt.Errorf("%w: target must implement proto.Message", ErrUnsupportedType)
	}

	// Empty data is valid for protobuf (represents default/zero values)
	if len(data) == 0 {
		// Reset to default state
		proto.Reset(msg)
		return nil
	}

	if err := proto.Unmarshal(data, msg); err != nil {
		return fmt.Errorf("protobuf deserialization failed: %w", err)
	}

	return nil
}

// ContentType returns the MIME type for Protocol Buffers serialization.
func (s *ProtobufSerializer) ContentType() string {
	return "application/protobuf"
}
