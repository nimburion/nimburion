package eventbus

import (
	"errors"
	"testing"

	"google.golang.org/protobuf/types/known/wrapperspb"
)

func TestJSONSerializer_Behavior(t *testing.T) {
	s := NewJSONSerializer()

	t.Run("serialize nil", func(t *testing.T) {
		_, err := s.Serialize(nil)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrInvalidData) {
			t.Fatalf("expected ErrInvalidData, got %v", err)
		}
	})

	t.Run("round trip", func(t *testing.T) {
		in := map[string]any{"name": "nimburion", "n": 3}
		data, err := s.Serialize(in)
		if err != nil {
			t.Fatalf("serialize: %v", err)
		}

		var out map[string]any
		if err := s.Deserialize(data, &out); err != nil {
			t.Fatalf("deserialize: %v", err)
		}
		if out["name"] != "nimburion" {
			t.Fatalf("unexpected name: %v", out["name"])
		}
	})

	t.Run("deserialize empty", func(t *testing.T) {
		var out map[string]any
		err := s.Deserialize([]byte{}, &out)
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrInvalidData) {
			t.Fatalf("expected ErrInvalidData, got %v", err)
		}
	})

	if ct := s.ContentType(); ct != "application/json" {
		t.Fatalf("unexpected content type: %s", ct)
	}
}

func TestProtobufSerializer_Behavior(t *testing.T) {
	s := NewProtobufSerializer()

	t.Run("serialize unsupported type", func(t *testing.T) {
		_, err := s.Serialize("not-proto")
		if err == nil {
			t.Fatal("expected error")
		}
		if !errors.Is(err, ErrUnsupportedType) {
			t.Fatalf("expected ErrUnsupportedType, got %v", err)
		}
	})

	t.Run("round trip", func(t *testing.T) {
		in := wrapperspb.String("hello")
		data, err := s.Serialize(in)
		if err != nil {
			t.Fatalf("serialize: %v", err)
		}

		out := &wrapperspb.StringValue{}
		if err := s.Deserialize(data, out); err != nil {
			t.Fatalf("deserialize: %v", err)
		}
		if out.Value != "hello" {
			t.Fatalf("unexpected value: %q", out.Value)
		}
	})

	t.Run("deserialize empty resets message", func(t *testing.T) {
		out := wrapperspb.String("has-value")
		if err := s.Deserialize([]byte{}, out); err != nil {
			t.Fatalf("deserialize empty: %v", err)
		}
		if out.Value != "" {
			t.Fatalf("expected reset message, got %q", out.Value)
		}
	})

	if ct := s.ContentType(); ct != "application/protobuf" {
		t.Fatalf("unexpected content type: %s", ct)
	}
}
