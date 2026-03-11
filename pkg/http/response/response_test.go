package response

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	coreerrors "github.com/nimburion/nimburion/pkg/core/errors"
	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/jobs"
)

type mockResponseWriter struct {
	statusCode int
	written    bool
	header     http.Header
}

func newMockResponseWriter() *mockResponseWriter {
	return &mockResponseWriter{header: make(http.Header), statusCode: http.StatusOK}
}

func (m *mockResponseWriter) Header() http.Header        { return m.header }
func (m *mockResponseWriter) Write([]byte) (int, error)  { m.written = true; return 0, nil }
func (m *mockResponseWriter) WriteHeader(statusCode int) { m.statusCode = statusCode; m.written = true }
func (m *mockResponseWriter) Status() int                { return m.statusCode }
func (m *mockResponseWriter) Written() bool              { return m.written }

type mockContext struct {
	request      *http.Request
	response     *mockResponseWriter
	responseCode int
	responseBody interface{}
}

func (m *mockContext) Request() *http.Request          { return m.request }
func (m *mockContext) SetRequest(r *http.Request)      { m.request = r }
func (m *mockContext) Response() router.ResponseWriter { return m.response }
func (m *mockContext) SetResponse(w router.ResponseWriter) {
	if response, ok := w.(*mockResponseWriter); ok {
		m.response = response
	}
}
func (m *mockContext) Param(name string) string { return "" }
func (m *mockContext) Query(name string) string { return "" }
func (m *mockContext) Bind(v interface{}) error { return nil }
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
func (m *mockContext) Get(key string) interface{}        { return nil }
func (m *mockContext) Set(key string, value interface{}) {}

func TestSuccess(t *testing.T) {
	tests := []struct {
		name, requestID string
		data            interface{}
		wantCode        int
	}{
		{name: "success with string data", data: "test data", requestID: "req-123", wantCode: http.StatusOK},
		{name: "success with struct data", data: map[string]string{"id": "1", "name": "test"}, requestID: "req-456", wantCode: http.StatusOK},
		{name: "success without request ID", data: "test", wantCode: http.StatusOK},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/test", nil)
			if tt.requestID != "" {
				req = req.WithContext(context.WithValue(req.Context(), "request_id", tt.requestID))
			}
			mockCtx := &mockContext{request: req, response: newMockResponseWriter()}

			if err := Success(mockCtx, tt.data); err != nil {
				t.Fatalf("Success() error = %v", err)
			}
			if mockCtx.responseCode != tt.wantCode {
				t.Fatalf("Success() status code = %d, want %d", mockCtx.responseCode, tt.wantCode)
			}
			response, ok := mockCtx.responseBody.(SuccessBody)
			if !ok {
				t.Fatalf("Success() response body is not SuccessBody")
			}
			dataJSON, _ := json.Marshal(response.Data)
			wantDataJSON, _ := json.Marshal(tt.data)
			if string(dataJSON) != string(wantDataJSON) {
				t.Fatalf("Success() data = %v, want %v", response.Data, tt.data)
			}
			if response.RequestID != tt.requestID {
				t.Fatalf("Success() request_id = %s, want %s", response.RequestID, tt.requestID)
			}
		})
	}
}

func TestCreated(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/test", nil).WithContext(context.WithValue(context.Background(), "request_id", "req-789"))
	mockCtx := &mockContext{request: req, response: newMockResponseWriter()}
	data := map[string]interface{}{"id": "123", "name": "new resource"}

	if err := Created(mockCtx, data); err != nil {
		t.Fatalf("Created() error = %v", err)
	}
	if mockCtx.responseCode != http.StatusCreated {
		t.Fatalf("Created() status code = %d, want %d", mockCtx.responseCode, http.StatusCreated)
	}
	response, ok := mockCtx.responseBody.(SuccessBody)
	if !ok {
		t.Fatalf("Created() response body is not SuccessBody")
	}
	if response.RequestID != "req-789" {
		t.Fatalf("Created() request_id = %s, want req-789", response.RequestID)
	}
}

func TestMapError(t *testing.T) {
	tests := []struct {
		name, wantErrorCode, wantMessage, wantRequestID string
		err                                             error
		ctx                                             context.Context
		wantStatus                                      int
		wantHasDetails                                  bool
	}{
		{
			name: "core app error",
			err:  coreerrors.New("validation.email_required", coreerrors.Params{"field": "email"}, nil).WithHTTPStatus(http.StatusBadRequest),
			ctx:  context.WithValue(context.Background(), "request_id", "req-i18n"), wantStatus: http.StatusBadRequest, wantErrorCode: "validation_error", wantMessage: "validation.email_required", wantRequestID: "req-i18n",
		},
		{
			name: "validation error", err: NewValidationError("invalid input", map[string]interface{}{"field": "email"}),
			ctx: context.WithValue(context.Background(), "request_id", "req-123"), wantStatus: http.StatusBadRequest, wantErrorCode: "validation_error", wantMessage: "invalid input", wantRequestID: "req-123", wantHasDetails: true,
		},
		{
			name: "unknown error type", err: errors.New("some random error"),
			ctx: context.WithValue(context.Background(), "request_id", "req-jkl"), wantStatus: http.StatusInternalServerError, wantErrorCode: "internal_server_error", wantMessage: "an unexpected error occurred", wantRequestID: "req-jkl",
		},
		{
			name: "jobs family error", err: jobs.ErrConflict,
			ctx: context.WithValue(context.Background(), "request_id", "req-jobs"), wantStatus: http.StatusConflict, wantErrorCode: "conflict", wantMessage: jobs.ErrConflict.Error(), wantRequestID: "req-jobs",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, errResp := MapError(tt.ctx, tt.err)
			if status != tt.wantStatus || errResp.Error != tt.wantErrorCode || errResp.Message != tt.wantMessage || errResp.RequestID != tt.wantRequestID {
				t.Fatalf("MapError() = (%d,%+v), want status=%d error=%s msg=%s req=%s", status, errResp, tt.wantStatus, tt.wantErrorCode, tt.wantMessage, tt.wantRequestID)
			}
			if tt.wantHasDetails && errResp.Details == nil {
				t.Fatal("expected details but got nil")
			}
		})
	}
}

func TestErrorConstructors(t *testing.T) {
	if err := NewNotFoundError("missing"); err.HTTPStatus != http.StatusNotFound {
		t.Fatalf("NewNotFoundError status = %d", err.HTTPStatus)
	}
	if err := NewConflictError("conflict", map[string]interface{}{"id": "1"}); err.HTTPStatus != http.StatusConflict {
		t.Fatalf("NewConflictError status = %d", err.HTTPStatus)
	}
	if err := NewUnauthorizedError("nope"); err.HTTPStatus != http.StatusUnauthorized {
		t.Fatalf("NewUnauthorizedError status = %d", err.HTTPStatus)
	}
	if err := NewForbiddenError("stop"); err.HTTPStatus != http.StatusForbidden {
		t.Fatalf("NewForbiddenError status = %d", err.HTTPStatus)
	}
	if err := NewInternalError("boom", errors.New("cause")); err.HTTPStatus != http.StatusInternalServerError {
		t.Fatalf("NewInternalError status = %d", err.HTTPStatus)
	}
}
