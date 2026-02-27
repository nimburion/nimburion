package opensearch

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
)

type mockLogger struct{}

func (m *mockLogger) Debug(string, ...any)                      {}
func (m *mockLogger) Info(string, ...any)                       {}
func (m *mockLogger) Warn(string, ...any)                       {}
func (m *mockLogger) Error(string, ...any)                      {}
func (m *mockLogger) With(...any) logger.Logger                 { return m }
func (m *mockLogger) WithContext(context.Context) logger.Logger { return m }

func TestNewAdapter_EmptyURL(t *testing.T) {
	_, err := NewAdapter(Config{}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error for empty URL")
	}
	if !strings.Contains(err.Error(), "opensearch URL is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestNewAdapter_PingFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusInternalServerError)
	}))
	defer srv.Close()

	_, err := NewAdapter(Config{
		URL:              srv.URL,
		MaxConns:         2,
		OperationTimeout: time.Second,
	}, &mockLogger{})
	if err == nil {
		t.Fatal("expected error when ping fails")
	}
}

func TestIndexDocument_UsesAPIKeyAuth(t *testing.T) {
	var gotAuth string
	var gotPath string
	var gotMethod string

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth = r.Header.Get("Authorization")
		gotPath = r.URL.Path
		gotMethod = r.Method

		if gotPath == "/" && gotMethod == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}

		if gotPath == "/products/_doc/42" && gotMethod == http.MethodPut {
			w.WriteHeader(http.StatusCreated)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	adapter, err := NewAdapter(Config{
		URL:              srv.URL,
		APIKey:           "api-key",
		MaxConns:         2,
		OperationTimeout: time.Second,
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("unexpected setup error: %v", err)
	}
	defer adapter.Close()

	if err := adapter.IndexDocument(context.Background(), "products", "42", map[string]any{"name": "book"}); err != nil {
		t.Fatalf("IndexDocument failed: %v", err)
	}
	if gotAuth != "ApiKey api-key" {
		t.Fatalf("expected ApiKey auth header, got %q", gotAuth)
	}
}

func TestSearch_ReturnsRawJSON(t *testing.T) {
	const body = `{"hits":{"total":{"value":1}}}`

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/products/_search" && r.Method == http.MethodPost {
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(body))
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	adapter, err := NewAdapter(Config{
		URL:              srv.URL,
		MaxConns:         2,
		OperationTimeout: time.Second,
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("unexpected setup error: %v", err)
	}
	defer adapter.Close()

	out, err := adapter.Search(context.Background(), "products", map[string]any{
		"query": map[string]any{
			"match_all": map[string]any{},
		},
	})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}

	var decoded map[string]any
	if err := json.Unmarshal(out, &decoded); err != nil {
		t.Fatalf("Search output is not valid JSON: %v", err)
	}
}

func TestDeleteDocument_NotFoundIsNotError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/" && r.Method == http.MethodGet {
			w.WriteHeader(http.StatusOK)
			return
		}
		if r.URL.Path == "/products/_doc/missing" && r.Method == http.MethodDelete {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		http.NotFound(w, r)
	}))
	defer srv.Close()

	adapter, err := NewAdapter(Config{
		URL:              srv.URL,
		MaxConns:         2,
		OperationTimeout: time.Second,
	}, &mockLogger{})
	if err != nil {
		t.Fatalf("unexpected setup error: %v", err)
	}
	defer adapter.Close()

	if err := adapter.DeleteDocument(context.Background(), "products", "missing"); err != nil {
		t.Fatalf("DeleteDocument should not fail on 404: %v", err)
	}
}

func TestParseBaseURLs_FromURLs(t *testing.T) {
	urls, err := parseBaseURLs(Config{
		URLs: []string{
			"http://node-1:9200",
			"http://node-2:9200",
			"http://node-1:9200",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(urls) != 2 {
		t.Fatalf("expected 2 unique URLs, got %d", len(urls))
	}
}

func TestParseBaseURLs_Empty(t *testing.T) {
	_, err := parseBaseURLs(Config{})
	if err == nil {
		t.Fatal("expected error for empty URL and URLs")
	}
	if !strings.Contains(err.Error(), "opensearch URL is required") {
		t.Fatalf("unexpected error: %v", err)
	}
}
