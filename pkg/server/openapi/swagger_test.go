package openapi

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSwaggerHandler_ServeSwaggerUI_Disabled(t *testing.T) {
	// Test that Swagger is disabled by default
	handler := NewSwaggerHandler(false, "/api/openapi/openapi.yaml")

	req := httptest.NewRequest(http.MethodGet, "/swagger", nil)
	w := httptest.NewRecorder()

	ctx := &mockContext{
		request:  req,
		response: &mockResponseWriter{ResponseRecorder: w},
	}

	err := handler.ServeSwaggerUI(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404 when disabled, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "disabled") {
		t.Error("expected error message about Swagger being disabled")
	}
}

func TestSwaggerHandler_ServeSwaggerUI_Enabled(t *testing.T) {
	// Test that Swagger is enabled when configured
	handler := NewSwaggerHandler(true, "/api/openapi/openapi.yaml")

	req := httptest.NewRequest(http.MethodGet, "/swagger", nil)
	w := httptest.NewRecorder()

	ctx := &mockContext{
		request:  req,
		response: &mockResponseWriter{ResponseRecorder: w},
	}

	err := handler.ServeSwaggerUI(ctx)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 when enabled, got %d", w.Code)
	}

	body := w.Body.String()
	
	// Verify HTML content
	if !strings.Contains(body, "<!DOCTYPE html>") {
		t.Error("expected HTML doctype")
	}
	
	if !strings.Contains(body, "swagger-ui") {
		t.Error("expected swagger-ui div")
	}
	
	// The template should have rendered the spec URL
	if !strings.Contains(body, "url:") && !strings.Contains(body, "/api/openapi/openapi.yaml") {
		t.Errorf("expected spec URL in HTML, got: %s", body)
	}

	// Verify content type
	contentType := w.Header().Get("Content-Type")
	if !strings.Contains(contentType, "text/html") {
		t.Errorf("expected HTML content type, got %s", contentType)
	}
}

func TestSwaggerHandler_RegisterRoutes_Enabled(t *testing.T) {
	// Test that routes are registered when enabled
	handler := NewSwaggerHandler(true, "/api/openapi/openapi.yaml")
	mockRouter := &mockRouter{}

	handler.RegisterRoutes(mockRouter)

	expectedRoutes := []string{
		"/swagger",
		"/swagger/",
	}

	if len(mockRouter.routes) != len(expectedRoutes) {
		t.Errorf("expected %d routes when enabled, got %d", len(expectedRoutes), len(mockRouter.routes))
	}

	for _, expectedRoute := range expectedRoutes {
		found := false
		for _, route := range mockRouter.routes {
			if route == expectedRoute {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected route %s not found", expectedRoute)
		}
	}
}

func TestSwaggerHandler_RegisterRoutes_Disabled(t *testing.T) {
	// Test that routes are NOT registered when disabled
	handler := NewSwaggerHandler(false, "/api/openapi/openapi.yaml")
	mockRouter := &mockRouter{}

	handler.RegisterRoutes(mockRouter)

	if len(mockRouter.routes) != 0 {
		t.Errorf("expected 0 routes when disabled, got %d", len(mockRouter.routes))
	}
}

func TestSwaggerHandler_OnlyOnManagementPort(t *testing.T) {
	// This is a documentation test to ensure developers understand
	// that Swagger should only be registered on management server
	
	// Simulate management server setup
	managementHandler := NewSwaggerHandler(true, "/api/openapi/openapi.yaml")
	managementRouter := &mockRouter{}
	managementHandler.RegisterRoutes(managementRouter)
	
	if len(managementRouter.routes) == 0 {
		t.Error("Swagger routes should be registered on management server")
	}
	
	// Simulate public API server setup (should NOT have Swagger)
	// In practice, developers should not register Swagger on public API
	publicHandler := NewSwaggerHandler(false, "/api/openapi/openapi.yaml")
	publicRouter := &mockRouter{}
	publicHandler.RegisterRoutes(publicRouter)
	
	if len(publicRouter.routes) != 0 {
		t.Error("Swagger routes should NOT be registered on public API server")
	}
}
