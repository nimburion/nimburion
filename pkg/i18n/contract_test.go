package i18n

import (
	"errors"
	"testing"
)

func TestNewMessage_ClonesParams(t *testing.T) {
	params := Params{"order_id": "o-1"}
	msg := NewMessage("order.created", params)

	params["order_id"] = "mutated"

	if msg.Params["order_id"] != "o-1" {
		t.Fatalf("expected cloned params, got %v", msg.Params["order_id"])
	}
}

func TestAppError_ImplementsErrorContract(t *testing.T) {
	cause := errors.New("db down")
	err := NewError("order.save_failed", Params{"order_id": "o-1"}, cause)

	if err.Error() != "order.save_failed: db down" {
		t.Fatalf("unexpected error string: %q", err.Error())
	}
	if !errors.Is(err, cause) {
		t.Fatalf("expected wrapped cause to be discoverable via errors.Is")
	}
}

func TestAppError_Message(t *testing.T) {
	err := NewError("payment.declined", Params{"provider": "stripe"}, nil)
	msg := err.Message()

	if msg.Code != "payment.declined" {
		t.Fatalf("unexpected code: %q", msg.Code)
	}
	if msg.Params["provider"] != "stripe" {
		t.Fatalf("unexpected params: %#v", msg.Params)
	}
}

func TestCanonicalParams(t *testing.T) {
	keys := CanonicalParams(Params{
		"b": 1,
		"a": 2,
		"c": 3,
	})

	expected := []string{"a", "b", "c"}
	for idx, key := range expected {
		if keys[idx] != key {
			t.Fatalf("unexpected ordering at index %d: got %q, want %q", idx, keys[idx], key)
		}
	}
}
