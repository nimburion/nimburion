package store

import (
	"context"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/config"
	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type mockLogger struct{}

func (m *mockLogger) Debug(string, ...any)                      {}
func (m *mockLogger) Info(string, ...any)                       {}
func (m *mockLogger) Warn(string, ...any)                       {}
func (m *mockLogger) Error(string, ...any)                      {}
func (m *mockLogger) With(...any) logger.Logger                 { return m }
func (m *mockLogger) WithContext(context.Context) logger.Logger { return m }

func TestNewObjectStorageAdapter_Disabled(t *testing.T) {
	adapter, err := NewObjectStorageAdapter(config.ObjectStorageConfig{Enabled: false}, &mockLogger{})
	if err != nil {
		t.Fatalf("expected no error when disabled, got %v", err)
	}
	if adapter != nil {
		t.Fatalf("expected nil adapter when disabled")
	}
}

func TestNewObjectStorageAdapter_UnsupportedType(t *testing.T) {
	_, err := NewObjectStorageAdapter(config.ObjectStorageConfig{
		Enabled: true,
		Type:    "unknown",
	}, &mockLogger{})
	if err == nil {
		t.Fatal("expected unsupported type error")
	}
	if !strings.Contains(err.Error(), "unsupported object_storage.type") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewObjectStorageAdapter_S3ValidationError(t *testing.T) {
	_, err := NewObjectStorageAdapter(config.ObjectStorageConfig{
		Enabled: true,
		Type:    "s3",
		S3: config.ObjectStorageS3Config{
			Region: "eu-west-1",
		},
	}, &mockLogger{})
	if err == nil {
		t.Fatal("expected s3 validation error")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "bucket") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewStorageAdapter_Disabled(t *testing.T) {
	adapter, err := NewStorageAdapter(config.DatabaseConfig{Type: ""}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error for empty type")
	}
	if adapter != nil {
		t.Fatal("expected nil adapter")
	}
}

func TestNewStorageAdapter_UnsupportedType(t *testing.T) {
	_, err := NewStorageAdapter(config.DatabaseConfig{
		Type: "unknown",
	}, &mockLogger{})
	if err == nil {
		t.Fatal("expected unsupported type error")
	}
}

func TestNewSearchAdapter_Disabled(t *testing.T) {
	adapter, err := NewSearchAdapter(config.SearchConfig{Type: ""}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error for empty type")
	}
	if adapter != nil {
		t.Fatal("expected nil adapter")
	}
}

func TestNewSearchAdapter_UnsupportedType(t *testing.T) {
	_, err := NewSearchAdapter(config.SearchConfig{
		Type: "unknown",
	}, &mockLogger{})
	if err == nil {
		t.Fatal("expected unsupported type error")
	}
}
