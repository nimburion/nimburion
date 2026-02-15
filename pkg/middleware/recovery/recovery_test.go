package recovery

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/middleware/requestid"
	"github.com/nimburion/nimburion/pkg/middleware/testutil"
	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestRecovery_CatchesPanic(t *testing.T) {
	// Given: A router with recovery middleware and a handler that panics
	log := &testutil.MockLogger{}
	r := nethttp.NewRouter()
	r.Use(requestid.RequestID(), Recovery(log))

	r.GET("/panic", func(c router.Context) error {
		panic("something went wrong")
	})

	// When: A request is made to the panicking handler
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Then: The response should be HTTP 500
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	// And: The response should contain error details
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["error"] != "internal_server_error" {
		t.Errorf("expected error 'internal_server_error', got %v", response["error"])
	}

	if response["message"] != "an unexpected error occurred" {
		t.Errorf("expected message 'an unexpected error occurred', got %v", response["message"])
	}

	if response["request_id"] == nil || response["request_id"] == "" {
		t.Error("expected request_id in response")
	}

	// And: The panic should be logged
	if len(log.Logs) == 0 {
		t.Fatal("expected error to be logged")
	}

	panicLogged := false
	for _, entry := range log.Logs {
		if entry.Msg == "panic recovered" && entry.Level == "error" {
			panicLogged = true
			
			// Verify panic value is logged
			if entry.Fields["panic"] != "something went wrong" {
				t.Errorf("expected panic value 'something went wrong', got %v", entry.Fields["panic"])
			}
			
			// Verify stack trace is logged
			stack, ok := entry.Fields["stack"].(string)
			if !ok {
				t.Error("expected stack field to be string")
			} else if !strings.Contains(stack, "panic") {
				t.Error("expected stack trace to contain 'panic'")
			}
		}
	}

	if !panicLogged {
		t.Error("expected 'panic recovered' to be logged")
	}
}

func TestRecovery_DoesNotInterferWithNormalRequests(t *testing.T) {
	// Given: A router with recovery middleware and a normal handler
	log := &testutil.MockLogger{}
	r := nethttp.NewRouter()
	r.Use(Recovery(log))

	r.GET("/normal", func(c router.Context) error {
		return c.JSON(http.StatusOK, map[string]string{"status": "ok"})
	})

	// When: A request is made to the normal handler
	req := httptest.NewRequest(http.MethodGet, "/normal", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Then: The response should be HTTP 200
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// And: The response should contain the expected data
	var response map[string]string
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("expected status 'ok', got %v", response["status"])
	}

	// And: No errors should be logged
	errorCount := 0
	for _, entry := range log.Logs {
		if entry.Level == "error" {
			errorCount++
		}
	}
	if errorCount > 0 {
		t.Errorf("expected no errors to be logged, got %d", errorCount)
	}
}

func TestRecovery_IncludesRequestIDInLog(t *testing.T) {
	// Given: A router with RequestID and Recovery middleware
	log := &testutil.MockLogger{}
	r := nethttp.NewRouter()
	r.Use(requestid.RequestID(), Recovery(log))

	r.GET("/panic", func(c router.Context) error {
		panic("test panic")
	})

	// When: A request is made with a specific request ID
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	req.Header.Set("X-Request-ID", "test-request-id-123")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Then: The request ID should be in the log
	if len(log.Logs) == 0 {
		t.Fatal("expected error to be logged")
	}

	foundRequestID := false
	for _, entry := range log.Logs {
		if entry.Msg == "panic recovered" && entry.Level == "error" {
			if entry.Fields["request_id"] == "test-request-id-123" {
				foundRequestID = true
				break
			}
		}
	}

	if !foundRequestID {
		t.Error("expected request_id 'test-request-id-123' to be logged")
	}

	// And: The request ID should be in the response
	var response map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&response); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if response["request_id"] != "test-request-id-123" {
		t.Errorf("expected request_id 'test-request-id-123' in response, got %v", response["request_id"])
	}
}

func TestRecovery_HandlesStringPanic(t *testing.T) {
	// Given: A router with recovery middleware
	log := &testutil.MockLogger{}
	r := nethttp.NewRouter()
	r.Use(Recovery(log))

	r.GET("/panic", func(c router.Context) error {
		panic("string panic")
	})

	// When: A request triggers a string panic
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Then: The panic should be recovered and logged
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	errorCount := 0
	for _, entry := range log.Logs {
		if entry.Level == "error" && entry.Msg == "panic recovered" {
			errorCount++
		}
	}
	if errorCount == 0 {
		t.Fatal("expected error to be logged")
	}
}

func TestRecovery_HandlesErrorPanic(t *testing.T) {
	// Given: A router with recovery middleware
	log := &testutil.MockLogger{}
	r := nethttp.NewRouter()
	r.Use(Recovery(log))

	r.GET("/panic", func(c router.Context) error {
		panic(http.ErrAbortHandler)
	})

	// When: A request triggers an error panic
	req := httptest.NewRequest(http.MethodGet, "/panic", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Then: The panic should be recovered and logged
	if w.Code != http.StatusInternalServerError {
		t.Errorf("expected status %d, got %d", http.StatusInternalServerError, w.Code)
	}

	errorCount := 0
	for _, entry := range log.Logs {
		if entry.Level == "error" && entry.Msg == "panic recovered" {
			errorCount++
		}
	}
	if errorCount == 0 {
		t.Fatal("expected error to be logged")
	}
}

func TestRecovery_DoesNotWriteIfResponseAlreadyWritten(t *testing.T) {
	// Given: A router with recovery middleware and a handler that writes response before panicking
	log := &testutil.MockLogger{}
	r := nethttp.NewRouter()
	r.Use(Recovery(log))

	r.GET("/panic-after-write", func(c router.Context) error {
		// Write response first
		c.JSON(http.StatusOK, map[string]string{"status": "ok"})
		// Then panic
		panic("panic after write")
	})

	// When: A request is made
	req := httptest.NewRequest(http.MethodGet, "/panic-after-write", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	// Then: The original response should be preserved (HTTP 200)
	if w.Code != http.StatusOK {
		t.Errorf("expected status %d, got %d", http.StatusOK, w.Code)
	}

	// And: The panic should still be logged
	errorCount := 0
	for _, entry := range log.Logs {
		if entry.Level == "error" && entry.Msg == "panic recovered" {
			errorCount++
		}
	}
	if errorCount == 0 {
		t.Fatal("expected error to be logged")
	}
}
