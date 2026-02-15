package tracing

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"

	"github.com/nimburion/nimburion/pkg/middleware/requestid"
	"github.com/nimburion/nimburion/pkg/server/router"
)

func hasAttr(attrs []attribute.KeyValue, key string) bool {
	for _, attr := range attrs {
		if string(attr.Key) == key {
			return true
		}
	}
	return false
}

// mockTracingContext implements router.Context for testing
type mockTracingContext struct {
	req    *http.Request
	resp   *mockTracingResponseWriter
	params map[string]string
	values map[string]interface{}
}

func newMockTracingContext(req *http.Request) *mockTracingContext {
	return &mockTracingContext{
		req:    req,
		resp:   &mockTracingResponseWriter{header: make(http.Header), status: http.StatusOK},
		params: make(map[string]string),
		values: make(map[string]interface{}),
	}
}

func (m *mockTracingContext) Request() *http.Request {
	return m.req
}

func (m *mockTracingContext) Response() router.ResponseWriter {
	return m.resp
}

func (m *mockTracingContext) SetResponse(w router.ResponseWriter) {
	if response, ok := w.(*mockTracingResponseWriter); ok {
		m.resp = response
	}
}

func (m *mockTracingContext) Param(name string) string {
	return m.params[name]
}

func (m *mockTracingContext) Query(name string) string {
	return m.req.URL.Query().Get(name)
}

func (m *mockTracingContext) Bind(v interface{}) error {
	return nil
}

func (m *mockTracingContext) JSON(code int, v interface{}) error {
	m.resp.status = code
	return nil
}

func (m *mockTracingContext) String(code int, s string) error {
	m.resp.status = code
	return nil
}

func (m *mockTracingContext) Get(key string) interface{} {
	return m.values[key]
}

func (m *mockTracingContext) Set(key string, value interface{}) {
	m.values[key] = value
}

func (m *mockTracingContext) SetRequest(req *http.Request) {
	m.req = req
}

type mockTracingResponseWriter struct {
	header  http.Header
	status  int
	written bool
}

func (m *mockTracingResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockTracingResponseWriter) Write(b []byte) (int, error) {
	m.written = true
	return len(b), nil
}

func (m *mockTracingResponseWriter) WriteHeader(statusCode int) {
	m.status = statusCode
	m.written = true
}

func (m *mockTracingResponseWriter) Status() int {
	return m.status
}

func (m *mockTracingResponseWriter) Written() bool {
	return m.written
}

func setupTestTracerProvider(t *testing.T) *tracetest.SpanRecorder {
	t.Helper()

	spanRecorder := tracetest.NewSpanRecorder()
	provider := sdktrace.NewTracerProvider(
		sdktrace.WithSpanProcessor(spanRecorder),
	)
	otel.SetTracerProvider(provider)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	return spanRecorder
}

func TestTracing_CreatesSpan(t *testing.T) {
	recorder := setupTestTracerProvider(t)

	req := httptest.NewRequest("GET", "/users", nil)
	ctx := newMockTracingContext(req)

	middleware := Tracing(TracingConfig{
		TracerName: "test-tracer",
	})

	handler := middleware(func(c router.Context) error {
		return nil
	})

	err := handler(ctx)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name() != "HTTP GET /users" {
		t.Errorf("expected span name 'HTTP GET /users', got %q", span.Name())
	}
}

func TestTracing_AddsHTTPAttributes(t *testing.T) {
	recorder := setupTestTracerProvider(t)

	req := httptest.NewRequest("POST", "/api/orders?filter=active", nil)
	req.Header.Set("User-Agent", "test-client/1.0")
	req.RemoteAddr = "192.168.1.1:12345"

	ctx := newMockTracingContext(req)

	middleware := Tracing(TracingConfig{})

	handler := middleware(func(c router.Context) error {
		return nil
	})

	err := handler(ctx)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	attrs := span.Attributes()

	expectedAttrs := map[string]interface{}{
		"http.method":      "POST",
		"http.target":      "/api/orders",
		"http.user_agent":  "test-client/1.0",
		"http.remote_addr": "192.168.1.1:12345",
	}

	for key, expectedValue := range expectedAttrs {
		found := false
		for _, attr := range attrs {
			if string(attr.Key) == key {
				found = true
				if attr.Value.AsInterface() != expectedValue {
					t.Errorf("expected attribute %s=%v, got %v", key, expectedValue, attr.Value.AsInterface())
				}
				break
			}
		}
		if !found {
			t.Errorf("expected attribute %s not found", key)
		}
	}
}

func TestTracing_ExcludedPathPrefixes(t *testing.T) {
	recorder := setupTestTracerProvider(t)

	req := httptest.NewRequest("GET", "/health/live", nil)
	ctx := newMockTracingContext(req)

	middleware := Tracing(TracingConfig{
		ExcludedPathPrefixes: []string{"/health"},
	})

	handler := middleware(func(c router.Context) error { return nil })
	if err := handler(ctx); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 0 {
		t.Fatalf("expected 0 spans for excluded path, got %d", len(spans))
	}
}

func TestTracing_PathPolicyMinimal(t *testing.T) {
	recorder := setupTestTracerProvider(t)

	req := httptest.NewRequest("GET", "/api/public/info", nil)
	req.Header.Set("User-Agent", "test-agent")
	req.RemoteAddr = "10.1.2.3:9999"
	ctx := newMockTracingContext(req)

	middleware := Tracing(TracingConfig{
		PathPolicies: []PathPolicy{
			{Prefix: "/api/public", Mode: ModeMinimal},
		},
	})

	handler := middleware(func(c router.Context) error { return nil })
	if err := handler(ctx); err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	attrs := spans[0].Attributes()
	if !hasAttr(attrs, "http.method") || !hasAttr(attrs, "http.target") {
		t.Fatalf("expected minimal attributes http.method and http.target")
	}
	if hasAttr(attrs, "http.user_agent") || hasAttr(attrs, "http.remote_addr") || hasAttr(attrs, "http.url") {
		t.Fatalf("did not expect full attributes in minimal mode")
	}
}

func TestTracing_AddsRequestID(t *testing.T) {
	recorder := setupTestTracerProvider(t)

	req := httptest.NewRequest("GET", "/users", nil)
	ctx := context.WithValue(req.Context(), requestid.RequestIDKey, "test-request-id-123")
	req = req.WithContext(ctx)

	mockCtx := newMockTracingContext(req)

	middleware := Tracing(TracingConfig{})

	handler := middleware(func(c router.Context) error {
		return nil
	})

	err := handler(mockCtx)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	attrs := span.Attributes()

	found := false
	for _, attr := range attrs {
		if string(attr.Key) == "request.id" {
			found = true
			if attr.Value.AsString() != "test-request-id-123" {
				t.Errorf("expected request.id=test-request-id-123, got %v", attr.Value.AsString())
			}
			break
		}
	}

	if !found {
		t.Error("expected request.id attribute not found")
	}
}

func TestTracing_RecordsStatusCode(t *testing.T) {
	recorder := setupTestTracerProvider(t)

	tests := []struct {
		name           string
		statusCode     int
		expectedStatus codes.Code
	}{
		{"success 200", http.StatusOK, codes.Ok},
		{"success 201", http.StatusCreated, codes.Ok},
		{"client error 400", http.StatusBadRequest, codes.Ok},
		{"client error 404", http.StatusNotFound, codes.Ok},
		{"server error 500", http.StatusInternalServerError, codes.Error},
		{"server error 503", http.StatusServiceUnavailable, codes.Error},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			recorder.Reset()

			req := httptest.NewRequest("GET", "/test", nil)
			ctx := newMockTracingContext(req)

			middleware := Tracing(TracingConfig{})

			handler := middleware(func(c router.Context) error {
				c.Response().WriteHeader(tt.statusCode)
				return nil
			})

			err := handler(ctx)
			if err != nil {
				t.Fatalf("handler returned error: %v", err)
			}

			spans := recorder.Ended()
			if len(spans) != 1 {
				t.Fatalf("expected 1 span, got %d", len(spans))
			}

			span := spans[0]

			// Check status code attribute
			attrs := span.Attributes()
			found := false
			for _, attr := range attrs {
				if string(attr.Key) == "http.status_code" {
					found = true
					if attr.Value.AsInt64() != int64(tt.statusCode) {
						t.Errorf("expected status_code=%d, got %v", tt.statusCode, attr.Value.AsInt64())
					}
					break
				}
			}
			if !found {
				t.Error("expected http.status_code attribute not found")
			}

			// Check span status
			if span.Status().Code != tt.expectedStatus {
				t.Errorf("expected span status %v, got %v", tt.expectedStatus, span.Status().Code)
			}
		})
	}
}

func TestTracing_RecordsError(t *testing.T) {
	recorder := setupTestTracerProvider(t)

	req := httptest.NewRequest("GET", "/error", nil)
	ctx := newMockTracingContext(req)

	middleware := Tracing(TracingConfig{})

	testErr := errors.New("test error")
	handler := middleware(func(c router.Context) error {
		return testErr
	})

	err := handler(ctx)
	if err != testErr {
		t.Fatalf("expected error to be propagated")
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]

	// Check that error was recorded
	events := span.Events()
	if len(events) != 1 {
		t.Fatalf("expected 1 event (error), got %d", len(events))
	}

	if events[0].Name != "exception" {
		t.Errorf("expected event name 'exception', got %q", events[0].Name)
	}

	// Check span status
	if span.Status().Code != codes.Error {
		t.Errorf("expected span status Error, got %v", span.Status().Code)
	}
}

func TestTracing_CustomSpanNameFormatter(t *testing.T) {
	recorder := setupTestTracerProvider(t)

	req := httptest.NewRequest("GET", "/users/123", nil)
	ctx := newMockTracingContext(req)
	ctx.params["id"] = "123"

	middleware := Tracing(TracingConfig{
		SpanNameFormatter: func(c router.Context) string {
			return "Custom: " + c.Request().Method + " " + c.Param("id")
		},
	})

	handler := middleware(func(c router.Context) error {
		return nil
	})

	err := handler(ctx)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	spans := recorder.Ended()
	if len(spans) != 1 {
		t.Fatalf("expected 1 span, got %d", len(spans))
	}

	span := spans[0]
	if span.Name() != "Custom: GET 123" {
		t.Errorf("expected span name 'Custom: GET 123', got %q", span.Name())
	}
}

func TestTracing_PropagatesContext(t *testing.T) {
	recorder := setupTestTracerProvider(t)

	// Create a parent span
	parentCtx, parentSpan := otel.Tracer("test").Start(context.Background(), "parent")

	// Inject trace context into headers
	req := httptest.NewRequest("GET", "/test", nil)
	propagator := otel.GetTextMapPropagator()
	propagator.Inject(parentCtx, propagation.HeaderCarrier(req.Header))

	ctx := newMockTracingContext(req)

	middleware := Tracing(TracingConfig{})

	handler := middleware(func(c router.Context) error {
		return nil
	})

	err := handler(ctx)
	if err != nil {
		t.Fatalf("handler returned error: %v", err)
	}

	// End parent span before checking
	parentSpan.End()

	spans := recorder.Ended()
	if len(spans) != 2 { // parent + child
		t.Fatalf("expected 2 spans, got %d", len(spans))
	}

	// Find the child span (HTTP request span)
	var childSpan sdktrace.ReadOnlySpan
	for _, span := range spans {
		if span.Name() == "HTTP GET /test" {
			childSpan = span
			break
		}
	}

	if childSpan == nil {
		t.Fatal("child span not found")
	}

	// Verify parent-child relationship
	if childSpan.Parent().TraceID() != parentSpan.SpanContext().TraceID() {
		t.Error("child span does not have correct parent trace ID")
	}
}
