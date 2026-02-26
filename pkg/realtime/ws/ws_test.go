package ws

import (
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/middleware/cors"
)

func TestParseTopics(t *testing.T) {
	tests := []struct {
		name    string
		raw     string
		wantLen int
		wantErr bool
	}{
		{name: "empty", raw: "", wantLen: 0, wantErr: false},
		{name: "spaces and commas", raw: "membership, payments governance", wantLen: 3, wantErr: false},
		{name: "dedupe", raw: "membership,membership", wantLen: 1, wantErr: false},
		{name: "invalid token", raw: "membership,$bad", wantErr: true},
		{name: "too many", raw: "a b c d", wantErr: true},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			topics, err := ParseTopics(test.raw, 3, 64)
			if test.wantErr && err == nil {
				t.Fatalf("expected error, got nil")
			}
			if !test.wantErr && err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !test.wantErr && len(topics) != test.wantLen {
				t.Fatalf("expected %d topics, got %d", test.wantLen, len(topics))
			}
		})
	}
}

func TestIsAllowedOrigin_LegacyAllowAll(t *testing.T) {
	cfg := Config{
		AllowedOrigins: []string{},
	}
	if !isAllowedOrigin("https://app.example.com", cfg) {
		t.Fatalf("expected origin to be allowed with empty legacy allow-list")
	}
}

func TestIsAllowedOrigin_LegacyExactMatch(t *testing.T) {
	cfg := Config{
		AllowedOrigins: []string{"https://app.example.com"},
	}
	if !isAllowedOrigin("https://app.example.com", cfg) {
		t.Fatalf("expected exact origin match to be allowed")
	}
	if isAllowedOrigin("https://other.example.com", cfg) {
		t.Fatalf("expected non matching origin to be denied")
	}
}

func TestIsAllowedOrigin_OriginPolicyWildcard(t *testing.T) {
	cfg := Config{
		OriginPolicy: cors.Config{
			AllowOrigins:  []string{"https://*.example.com"},
			AllowWildcard: true,
		},
	}
	if !isAllowedOrigin("https://tenant.example.com", cfg) {
		t.Fatalf("expected wildcard origin to be allowed")
	}
}

func TestIsAllowedOrigin_OriginPolicyTakesPrecedence(t *testing.T) {
	cfg := Config{
		AllowedOrigins: []string{"https://legacy.example.com"},
		OriginPolicy: cors.Config{
			AllowOrigins: []string{"https://new.example.com"},
		},
	}
	if !isAllowedOrigin("https://new.example.com", cfg) {
		t.Fatalf("expected origin policy to take precedence over legacy allow-list")
	}
	if isAllowedOrigin("https://legacy.example.com", cfg) {
		t.Fatalf("expected legacy origin to be ignored when origin policy is configured")
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.ReadLimit != 4096 {
		t.Errorf("expected ReadLimit=4096, got %d", cfg.ReadLimit)
	}
	if cfg.WriteTimeout != 10*time.Second {
		t.Errorf("expected WriteTimeout=10s, got %v", cfg.WriteTimeout)
	}
	if cfg.MaxTopicCount != 20 {
		t.Errorf("expected MaxTopicCount=20, got %d", cfg.MaxTopicCount)
	}
	if cfg.MaxTopicLength != 64 {
		t.Errorf("expected MaxTopicLength=64, got %d", cfg.MaxTopicLength)
	}
}

func TestParseList(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"", []string{}},
		{"a,b,c", []string{"a", "b", "c"}},
		{"a b c", []string{"a", "b", "c"}},
		{"a, b , c", []string{"a", "b", "c"}},
		{"  a  b  ", []string{"a", "b"}},
	}

	for _, tt := range tests {
		result := parseList(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("parseList(%q) = %v, want %v", tt.input, result, tt.expected)
			continue
		}
		for i := range result {
			if result[i] != tt.expected[i] {
				t.Errorf("parseList(%q)[%d] = %q, want %q", tt.input, i, result[i], tt.expected[i])
			}
		}
	}
}

func TestIsValidTopic(t *testing.T) {
	tests := []struct {
		topic string
		valid bool
	}{
		{"", true}, // Empty string passes character validation
		{"valid-topic", true},
		{"valid_topic", true},
		{"valid.topic", true},
		{"valid123", true},
		{"$invalid", false},
		{"invalid!", false},
		{"a", true},
	}

	for _, tt := range tests {
		result := isValidTopic(tt.topic)
		if result != tt.valid {
			t.Errorf("isValidTopic(%q) = %v, want %v", tt.topic, result, tt.valid)
		}
	}
}
