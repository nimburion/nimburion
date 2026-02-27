package schema

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protodesc"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
	"google.golang.org/protobuf/types/descriptorpb"
	"google.golang.org/protobuf/types/dynamicpb"
)

const headerSchemaMessage = "schema_message"

// LocalRegistry provides in-process schema resolution from a protobuf descriptor set.
type LocalRegistry struct {
	files        *protoregistry.Files
	byMessageKey map[string]protoreflect.MessageDescriptor
}

// NewLocalRegistry builds a schema registry from a descriptor-set file.
func NewLocalRegistry(descriptorPath string) (*LocalRegistry, error) {
	descriptorPath = strings.TrimSpace(descriptorPath)
	if descriptorPath == "" {
		return nil, errors.New("descriptor path is required")
	}
	raw, err := os.ReadFile(descriptorPath)
	if err != nil {
		return nil, fmt.Errorf("read descriptor set: %w", err)
	}

	var set descriptorpb.FileDescriptorSet
	if unmarshalErr := proto.Unmarshal(raw, &set); unmarshalErr != nil {
		return nil, fmt.Errorf("decode descriptor set: %w", unmarshalErr)
	}

	files, err := protodesc.NewFiles(&set)
	if err != nil {
		return nil, fmt.Errorf("build descriptor registry: %w", err)
	}

	reg := &LocalRegistry{
		files:        files,
		byMessageKey: make(map[string]protoreflect.MessageDescriptor),
	}
	files.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		registerMessages(fd.Messages(), reg.byMessageKey)
		return true
	})

	if len(reg.byMessageKey) == 0 {
		return nil, errors.New("descriptor set does not contain message descriptors")
	}
	return reg, nil
}

// Resolve selects a descriptor by subject/version and optional headers.
func (r *LocalRegistry) Resolve(subject, version string, headers map[string]string) (*Descriptor, error) {
	if r == nil {
		return nil, errors.New("schema registry is nil")
	}
	subject = strings.TrimSpace(subject)
	version = strings.TrimSpace(version)
	if subject == "" {
		return nil, &ValidationError{Code: "validation.kafka.missing_subject"}
	}
	if version == "" {
		version = "v1"
	}

	candidates := []string{}
	if messageHeader := strings.TrimSpace(headers[headerSchemaMessage]); messageHeader != "" {
		candidates = append(candidates, messageHeader)
	}
	candidates = append(candidates, subject)

	var md protoreflect.MessageDescriptor
	for _, candidate := range candidates {
		if resolved, ok := r.byMessageKey[normalizeKey(candidate)]; ok {
			md = resolved
			break
		}
	}
	if md == nil {
		return nil, &ValidationError{
			Code:    "validation.kafka.schema_not_found",
			Subject: subject,
			Version: version,
		}
	}

	messageName := string(md.FullName())
	schemaID := subject + ":" + version + ":" + messageName
	hash := sha256.Sum256([]byte(schemaID + "|" + messageName))

	return &Descriptor{
		Subject:    subject,
		Version:    version,
		SchemaID:   schemaID,
		SchemaHash: hex.EncodeToString(hash[:]),
		Message:    messageName,
	}, nil
}

// ValidatePayload checks whether payload can be decoded as protobuf (binary or JSON) for the descriptor message.
func (r *LocalRegistry) ValidatePayload(desc *Descriptor, payload []byte) error {
	if r == nil {
		return errors.New("schema registry is nil")
	}
	if desc == nil {
		return errors.New("descriptor is nil")
	}
	if len(payload) == 0 {
		return &ValidationError{
			Code:    "validation.kafka.payload_empty",
			Subject: desc.Subject,
			Version: desc.Version,
		}
	}

	md, ok := r.byMessageKey[normalizeKey(desc.Message)]
	if !ok {
		return &ValidationError{
			Code:    "validation.kafka.message_not_found",
			Subject: desc.Subject,
			Version: desc.Version,
			Details: map[string]interface{}{"message": desc.Message},
		}
	}

	msg := dynamicpb.NewMessage(md)
	if err := proto.Unmarshal(payload, msg); err == nil {
		return nil
	}
	msg = dynamicpb.NewMessage(md)
	if err := protojson.Unmarshal(payload, msg); err == nil {
		return nil
	}

	return &ValidationError{
		Code:    "validation.kafka.payload_invalid",
		Subject: desc.Subject,
		Version: desc.Version,
		Details: map[string]interface{}{"message": desc.Message},
	}
}

// ValidateHeaders checks schema-related header consistency.
func (r *LocalRegistry) ValidateHeaders(desc *Descriptor, headers map[string]string) error {
	if desc == nil {
		return errors.New("descriptor is nil")
	}
	if len(headers) == 0 {
		return nil
	}
	if schemaID := strings.TrimSpace(headers["schema_id"]); schemaID != "" && schemaID != desc.SchemaID {
		return &ValidationError{
			Code:    "validation.kafka.schema_id_mismatch",
			Subject: desc.Subject,
			Version: desc.Version,
			Details: map[string]interface{}{"expected": desc.SchemaID, "actual": schemaID},
		}
	}
	if schemaHash := strings.TrimSpace(headers["schema_hash"]); schemaHash != "" && schemaHash != desc.SchemaHash {
		return &ValidationError{
			Code:    "validation.kafka.schema_hash_mismatch",
			Subject: desc.Subject,
			Version: desc.Version,
			Details: map[string]interface{}{"expected": desc.SchemaHash, "actual": schemaHash},
		}
	}
	return nil
}

func registerMessages(list protoreflect.MessageDescriptors, registry map[string]protoreflect.MessageDescriptor) {
	for idx := 0; idx < list.Len(); idx++ {
		md := list.Get(idx)
		full := string(md.FullName())
		registry[normalizeKey(full)] = md
		registry[normalizeKey(string(md.Name()))] = md
		registerMessages(md.Messages(), registry)
	}
}

func normalizeKey(v string) string {
	return strings.ToLower(strings.TrimSpace(v))
}
