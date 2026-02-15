package openapi

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestHandler_ServeSpec(t *testing.T) {
	tests := []struct {
		name           string
		setupFile      func(t *testing.T) string
		expectedStatus int
		expectedType   string
		expectError    bool
	}{
		{
			name: "serve yaml spec successfully",
			setupFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				specPath := filepath.Join(tmpDir, "openapi.yaml")
				content := []byte("openapi: 3.0.0\ninfo:\n  title: Test API\n  version: 1.0.0\n")
				if err := os.WriteFile(specPath, content, 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				return specPath
			},
			expectedStatus: http.StatusOK,
			expectedType:   "application/x-yaml",
			expectError:    false,
		},
		{
			name: "serve json spec successfully",
			setupFile: func(t *testing.T) string {
				tmpDir := t.TempDir()
				specPath := filepath.Join(tmpDir, "openapi.json")
				content := []byte(`{"openapi":"3.0.0","info":{"title":"Test API","version":"1.0.0"}}`)
				if err := os.WriteFile(specPath, content, 0644); err != nil {
					t.Fatalf("failed to create test file: %v", err)
				}
				return specPath
			},
			expectedStatus: http.StatusOK,
			expectedType:   "application/json",
			expectError:    false,
		},
		{
			name: "return 404 when spec file not found",
			setupFile: func(t *testing.T) string {
				return "/nonexistent/path/openapi.yaml"
			},
			expectedStatus: http.StatusNotFound,
			expectError:    false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			specPath := tt.setupFile(t)
			handler := NewHandler(specPath)

			req := httptest.NewRequest(http.MethodGet, "/api/openapi/openapi.yaml", nil)
			w := httptest.NewRecorder()

			// Create a mock context
			ctx := &mockContext{
				request:  req,
				response: &mockResponseWriter{ResponseRecorder: w},
			}

			err := handler.ServeSpec(ctx)

			if tt.expectError && err == nil {
				t.Error("expected error but got none")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected error: %v", err)
			}

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedType != "" {
				contentType := w.Header().Get("Content-Type")
				if contentType != tt.expectedType {
					t.Errorf("expected content type %s, got %s", tt.expectedType, contentType)
				}
			}
		})
	}
}

func TestHandler_RegisterRoutes(t *testing.T) {
	tmpDir := t.TempDir()
	specPath := filepath.Join(tmpDir, "openapi.yaml")
	content := []byte("openapi: 3.0.0\n")
	if err := os.WriteFile(specPath, content, 0644); err != nil {
		t.Fatalf("failed to create test file: %v", err)
	}

	handler := NewHandler(specPath)
	mockRouter := &mockRouter{}

	handler.RegisterRoutes(mockRouter)

	// Verify routes were registered
	expectedRoutes := []string{
		"/api/openapi/openapi.yaml",
		"/api/openapi/openapi.json",
	}

	if len(mockRouter.routes) != len(expectedRoutes) {
		t.Errorf("expected %d routes, got %d", len(expectedRoutes), len(mockRouter.routes))
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
