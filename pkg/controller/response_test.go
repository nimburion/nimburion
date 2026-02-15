package controller

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// mockResponseWriter implements router.ResponseWriter for testing
type mockResponseWriter struct {
	statusCode int
	written    bool
	header     http.Header
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{
		header:     make(http.Header),
		statusCode: http.StatusOK,
	}
}

func (m *mockResponseWriter) Header() http.Header {
	return m.header
}

func (m *mockResponseWriter) Write([]byte) (int, error) {
	m.written = true
	return 0, nil
}

func (m *mockResponseWriter) WriteHeader(statusCode int) {
	m.statusCode = statusCode
	m.written = true
}

func (m *mockResponseWriter) Status() int {
	return m.statusCode
}

func (m *mockResponseWriter) Written() bool {
	return m.written
}

// mockContext implements router.Context for testing
type mockContext struct {
	request      *http.Request
	response     *mockResponseWriter
	responseCode int
	responseBody interface{}
}

func (m *mockContext) Request() *http.Request {
	return m.request
}

func (m *mockContext) SetRequest(r *http.Request) {
	m.request = r
}

func (m *mockContext) Response() router.ResponseWriter {
	return m.response
}

func (m *mockContext) SetResponse(w router.ResponseWriter) {
	if response, ok := w.(*mockResponseWriter); ok {
		m.response = response
	}
}

func (m *mockContext) Param(name string) string {
	return ""
}

func (m *mockContext) Query(name string) string {
	return ""
}

func (m *mockContext) Bind(v interface{}) error {
	return nil
}

func (m *mockContext) JSON(code int, v interface{}) error {
	m.responseCode = code
	m.responseBody = v
	return nil
}

func (m *mockContext) String(code int, s string) error {
	m.responseCode = code
	m.responseBody = s
	return nil
}

func (m *mockContext) Get(key string) interface{} {
	return nil
}

func (m *mockContext) Set(key string, value interface{}) {
}

func TestSuccess(t *testing.T) {
	tests := []struct {
		name      string
		data      interface{}
		requestID string
		wantCode  int
	}{
		{
			name:      "success with string data",
			data:      "test data",
			requestID: "req-123",
			wantCode:  http.StatusOK,
		},
		{
			name: "success with struct data",
			data: map[string]string{
				"id":   "1",
				"name": "test",
			},
			requestID: "req-456",
			wantCode:  http.StatusOK,
		},
		{
			name:      "success without request ID",
			data:      "test",
			requestID: "",
			wantCode:  http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), "request_id", tt.requestID)
				req = req.WithContext(ctx)
			}

			mockCtx := &mockContext{
				request:  req,
				response: newMockResponseWriter(),
			}
			err := Success(mockCtx, tt.data)

			if err != nil {
				t.Errorf("Success() error = %v, want nil", err)
			}

			if mockCtx.responseCode != tt.wantCode {
				t.Errorf("Success() status code = %d, want %d", mockCtx.responseCode, tt.wantCode)
			}

			response, ok := mockCtx.responseBody.(SuccessResponse)
			if !ok {
				t.Fatalf("Success() response body is not SuccessResponse")
			}

			// Compare data using JSON marshaling for complex types
			dataJSON, _ := json.Marshal(response.Data)
			wantDataJSON, _ := json.Marshal(tt.data)
			if string(dataJSON) != string(wantDataJSON) {
				t.Errorf("Success() data = %v, want %v", response.Data, tt.data)
			}

			if response.RequestID != tt.requestID {
				t.Errorf("Success() request_id = %s, want %s", response.RequestID, tt.requestID)
			}
		})
	}
}

func TestCreated(t *testing.T) {
	tests := []struct {
		name      string
		data      interface{}
		requestID string
		wantCode  int
	}{
		{
			name: "created with new resource",
			data: map[string]interface{}{
				"id":   "123",
				"name": "new resource",
			},
			requestID: "req-789",
			wantCode:  http.StatusCreated,
		},
		{
			name:      "created without request ID",
			data:      "created",
			requestID: "",
			wantCode:  http.StatusCreated,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/test", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), "request_id", tt.requestID)
				req = req.WithContext(ctx)
			}

			mockCtx := &mockContext{
				request:  req,
				response: newMockResponseWriter(),
			}
			err := Created(mockCtx, tt.data)

			if err != nil {
				t.Errorf("Created() error = %v, want nil", err)
			}

			if mockCtx.responseCode != tt.wantCode {
				t.Errorf("Created() status code = %d, want %d", mockCtx.responseCode, tt.wantCode)
			}

			response, ok := mockCtx.responseBody.(SuccessResponse)
			if !ok {
				t.Fatalf("Created() response body is not SuccessResponse")
			}

			// Compare data using JSON marshaling for complex types
			dataJSON, _ := json.Marshal(response.Data)
			wantDataJSON, _ := json.Marshal(tt.data)
			if string(dataJSON) != string(wantDataJSON) {
				t.Errorf("Created() data = %v, want %v", response.Data, tt.data)
			}

			if response.RequestID != tt.requestID {
				t.Errorf("Created() request_id = %s, want %s", response.RequestID, tt.requestID)
			}
		})
	}
}

func TestNoContent(t *testing.T) {
	tests := []struct {
		name     string
		wantCode int
	}{
		{
			name:     "no content response",
			wantCode: http.StatusNoContent,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodDelete, "/test", nil)
			mockCtx := &mockContext{
				request:  req,
				response: newMockResponseWriter(),
			}

			err := NoContent(mockCtx)

			if err != nil {
				t.Errorf("NoContent() error = %v, want nil", err)
			}

			if mockCtx.responseCode != tt.wantCode {
				t.Errorf("NoContent() status code = %d, want %d", mockCtx.responseCode, tt.wantCode)
			}

			if mockCtx.responseBody != nil {
				t.Errorf("NoContent() response body = %v, want nil", mockCtx.responseBody)
			}
		})
	}
}

func TestError(t *testing.T) {
	tests := []struct {
		name        string
		err         error
		requestID   string
		wantCode    int
		wantError   string
		wantMessage string
		wantDetails map[string]interface{}
	}{
		{
			name:        "validation error",
			err:         NewValidationError("invalid input", map[string]interface{}{"field": "email"}),
			requestID:   "req-001",
			wantCode:    http.StatusBadRequest,
			wantError:   "validation_error",
			wantMessage: "invalid input",
			wantDetails: map[string]interface{}{"field": "email"},
		},
		{
			name:        "not found error",
			err:         NewNotFoundError("resource not found"),
			requestID:   "req-002",
			wantCode:    http.StatusNotFound,
			wantError:   "not_found",
			wantMessage: "resource not found",
		},
		{
			name:        "conflict error",
			err:         NewConflictError("resource already exists", map[string]interface{}{"id": "123"}),
			requestID:   "req-003",
			wantCode:    http.StatusConflict,
			wantError:   "conflict",
			wantMessage: "resource already exists",
			wantDetails: map[string]interface{}{"id": "123"},
		},
		{
			name:        "unauthorized error",
			err:         NewUnauthorizedError("invalid credentials"),
			requestID:   "req-004",
			wantCode:    http.StatusUnauthorized,
			wantError:   "unauthorized",
			wantMessage: "invalid credentials",
		},
		{
			name:        "forbidden error",
			err:         NewForbiddenError("insufficient permissions"),
			requestID:   "req-005",
			wantCode:    http.StatusForbidden,
			wantError:   "forbidden",
			wantMessage: "insufficient permissions",
		},
		{
			name:        "internal error",
			err:         NewInternalError("database connection failed", errors.New("connection timeout")),
			requestID:   "req-006",
			wantCode:    http.StatusInternalServerError,
			wantError:   "internal_server_error",
			wantMessage: "database connection failed",
		},
		{
			name:        "unknown error",
			err:         errors.New("unknown error"),
			requestID:   "req-007",
			wantCode:    http.StatusInternalServerError,
			wantError:   "internal_server_error",
			wantMessage: "an unexpected error occurred",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestID != "" {
				ctx := context.WithValue(req.Context(), "request_id", tt.requestID)
				req = req.WithContext(ctx)
			}

			mockCtx := &mockContext{
				request:  req,
				response: newMockResponseWriter(),
			}
			err := Error(mockCtx, tt.err)

			if err != nil {
				t.Errorf("Error() error = %v, want nil", err)
			}

			if mockCtx.responseCode != tt.wantCode {
				t.Errorf("Error() status code = %d, want %d", mockCtx.responseCode, tt.wantCode)
			}

			response, ok := mockCtx.responseBody.(ErrorResponse)
			if !ok {
				t.Fatalf("Error() response body is not ErrorResponse")
			}

			if response.Error != tt.wantError {
				t.Errorf("Error() error = %s, want %s", response.Error, tt.wantError)
			}

			if response.Message != tt.wantMessage {
				t.Errorf("Error() message = %s, want %s", response.Message, tt.wantMessage)
			}

			if response.RequestID != tt.requestID {
				t.Errorf("Error() request_id = %s, want %s", response.RequestID, tt.requestID)
			}

			if tt.wantDetails != nil {
				detailsJSON, _ := json.Marshal(response.Details)
				wantDetailsJSON, _ := json.Marshal(tt.wantDetails)
				if string(detailsJSON) != string(wantDetailsJSON) {
					t.Errorf("Error() details = %v, want %v", response.Details, tt.wantDetails)
				}
			}
		})
	}
}
