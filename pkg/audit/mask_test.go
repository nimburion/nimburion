package audit

import "testing"

func TestRedactSettings(t *testing.T) {
	settings := map[string]interface{}{
		"public": "visible",
		"secret": "hidden",
		"nested": map[string]interface{}{"password": "secret123"},
	}
	mask := map[string]interface{}{
		"secret": true,
		"nested": map[string]interface{}{"password": true},
	}

	result := RedactSettings(settings, mask)
	if result["public"] != "visible" {
		t.Fatalf("public value should not be redacted")
	}
	if result["secret"] != MaskedValue {
		t.Fatalf("secret should be redacted, got %v", result["secret"])
	}
	nested := result["nested"].(map[string]interface{})
	if nested["password"] != MaskedValue {
		t.Fatalf("nested password should be redacted")
	}
}

func TestShouldRedact(t *testing.T) {
	tests := []struct {
		mask     interface{}
		expected bool
	}{
		{nil, false},
		{"", false},
		{"secret", true},
		{true, true},
		{false, false},
		{0, false},
		{1, true},
		{[]interface{}{}, false},
		{[]interface{}{"x"}, true},
		{map[string]interface{}{}, false},
		{map[string]interface{}{"k": "v"}, true},
	}

	for _, tt := range tests {
		if got := ShouldRedact(tt.mask); got != tt.expected {
			t.Fatalf("ShouldRedact(%v) = %v, want %v", tt.mask, got, tt.expected)
		}
	}
}
