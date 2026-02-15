package nethttp

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
)

func TestNetHTTPRouter_BasicRouting(t *testing.T) {
	tests := []struct {
		name           string
		method         string
		path           string
		registerPath   string
		expectedStatus int
		expectedBody   string
	}{
		{
			name:           "GET route",
			method:         http.MethodGet,
			path:           "/users",
			registerPath:   "/users",
			expectedStatus: http.StatusOK,
			expectedBody:   "users list",
		},
		{
			name:           "POST route",
			method:         http.MethodPost,
			path:           "/users",
			registerPath:   "/users",
			expectedStatus: http.StatusCreated,
			expectedBody:   "user created",
		},
		{
			name:           "PUT route",
			method:         http.MethodPut,
			path:           "/users/1",
			registerPath:   "/users/:id",
			expectedStatus: http.StatusOK,
			expectedBody:   "user updated",
		},
		{
			name:           "DELETE route",
			method:         http.MethodDelete,
			path:           "/users/1",
			registerPath:   "/users/:id",
			expectedStatus: http.StatusNoContent,
			expectedBody:   "",
		},
		{
			name:           "PATCH route",
			method:         http.MethodPatch,
			path:           "/users/1",
			registerPath:   "/users/:id",
			expectedStatus: http.StatusOK,
			expectedBody:   "user patched",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := NewRouter()

			handler := func(c router.Context) error {
				switch tt.expectedStatus {
				case http.StatusOK:
					return c.String(http.StatusOK, tt.expectedBody)
				case http.StatusCreated:
					return c.String(http.StatusCreated, tt.expectedBody)
				case http.StatusNoContent:
					return c.String(http.StatusNoContent, tt.expectedBody)
				}
				return nil
			}

			switch tt.method {
			case http.MethodGet:
				r.GET(tt.registerPath, handler)
			case http.MethodPost:
				r.POST(tt.registerPath, handler)
			case http.MethodPut:
				r.PUT(tt.registerPath, handler)
			case http.MethodDelete:
				r.DELETE(tt.registerPath, handler)
			case http.MethodPatch:
				r.PATCH(tt.registerPath, handler)
			}

			req := httptest.NewRequest(tt.method, tt.path, nil)
			w := httptest.NewRecorder()

			r.ServeHTTP(w, req)

			if w.Code != tt.expectedStatus {
				t.Errorf("expected status %d, got %d", tt.expectedStatus, w.Code)
			}

			if tt.expectedBody != "" && w.Body.String() != tt.expectedBody {
				t.Errorf("expected body %q, got %q", tt.expectedBody, w.Body.String())
			}
		})
	}
}

func TestNetHTTPRouter_PathParameters(t *testing.T) {
	r := NewRouter()

	r.GET("/users/:id/posts/:postId", func(c router.Context) error {
		userID := c.Param("id")
		postID := c.Param("postId")
		return c.JSON(http.StatusOK, map[string]string{
			"userId": userID,
			"postId": postID,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/users/123/posts/456", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["userId"] != "123" {
		t.Errorf("expected userId 123, got %s", result["userId"])
	}

	if result["postId"] != "456" {
		t.Errorf("expected postId 456, got %s", result["postId"])
	}
}

func TestNetHTTPRouter_QueryParameters(t *testing.T) {
	r := NewRouter()

	r.GET("/search", func(c router.Context) error {
		query := c.Query("q")
		return c.JSON(http.StatusOK, map[string]string{
			"query": query,
		})
	})

	req := httptest.NewRequest(http.MethodGet, "/search?q=golang", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}

	var result map[string]string
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result["query"] != "golang" {
		t.Errorf("expected query 'golang', got %s", result["query"])
	}
}

func TestNetHTTPRouter_JSONBinding(t *testing.T) {
	r := NewRouter()

	type User struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	}

	r.POST("/users", func(c router.Context) error {
		var user User
		if err := c.Bind(&user); err != nil {
			return err
		}
		return c.JSON(http.StatusCreated, user)
	})

	userData := User{Name: "John", Email: "john@example.com"}
	body, _ := json.Marshal(userData)

	req := httptest.NewRequest(http.MethodPost, "/users", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Errorf("expected status 201, got %d", w.Code)
	}

	var result User
	if err := json.NewDecoder(w.Body).Decode(&result); err != nil {
		t.Fatalf("failed to decode response: %v", err)
	}

	if result.Name != userData.Name {
		t.Errorf("expected name %s, got %s", userData.Name, result.Name)
	}

	if result.Email != userData.Email {
		t.Errorf("expected email %s, got %s", userData.Email, result.Email)
	}
}

func TestNetHTTPRouter_Middleware(t *testing.T) {
	r := NewRouter()

	// Global middleware
	called := false
	r.Use(func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			called = true
			c.Set("middleware", "global")
			return next(c)
		}
	})

	r.GET("/test", func(c router.Context) error {
		value := c.Get("middleware")
		return c.String(http.StatusOK, value.(string))
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if !called {
		t.Error("middleware was not called")
	}

	if w.Body.String() != "global" {
		t.Errorf("expected body 'global', got %q", w.Body.String())
	}
}

func TestNetHTTPRouter_RouteSpecificMiddleware(t *testing.T) {
	r := NewRouter()

	middleware := func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			c.Set("route_middleware", "applied")
			return next(c)
		}
	}

	r.GET("/protected", func(c router.Context) error {
		value := c.Get("route_middleware")
		if value == nil {
			return c.String(http.StatusOK, "not applied")
		}
		return c.String(http.StatusOK, value.(string))
	}, middleware)

	r.GET("/unprotected", func(c router.Context) error {
		value := c.Get("route_middleware")
		if value == nil {
			return c.String(http.StatusOK, "not applied")
		}
		return c.String(http.StatusOK, value.(string))
	})

	// Test protected route
	req := httptest.NewRequest(http.MethodGet, "/protected", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "applied" {
		t.Errorf("expected 'applied' on protected route, got %q", w.Body.String())
	}

	// Test unprotected route
	req = httptest.NewRequest(http.MethodGet, "/unprotected", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Body.String() != "not applied" {
		t.Errorf("expected 'not applied' on unprotected route, got %q", w.Body.String())
	}
}

func TestNetHTTPRouter_Group(t *testing.T) {
	r := NewRouter()

	api := r.Group("/api")
	api.GET("/users", func(c router.Context) error {
		return c.String(http.StatusOK, "users")
	})

	v1 := api.Group("/v1")
	v1.GET("/posts", func(c router.Context) error {
		return c.String(http.StatusOK, "posts")
	})

	// Test /api/users
	req := httptest.NewRequest(http.MethodGet, "/api/users", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for /api/users, got %d", w.Code)
	}

	if w.Body.String() != "users" {
		t.Errorf("expected body 'users', got %q", w.Body.String())
	}

	// Test /api/v1/posts
	req = httptest.NewRequest(http.MethodGet, "/api/v1/posts", nil)
	w = httptest.NewRecorder()
	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200 for /api/v1/posts, got %d", w.Code)
	}

	if w.Body.String() != "posts" {
		t.Errorf("expected body 'posts', got %q", w.Body.String())
	}
}

func TestNetHTTPRouter_NotFound(t *testing.T) {
	r := NewRouter()

	r.GET("/users", func(c router.Context) error {
		return c.String(http.StatusOK, "users")
	})

	req := httptest.NewRequest(http.MethodGet, "/posts", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusNotFound {
		t.Errorf("expected status 404, got %d", w.Code)
	}
}

func TestNetHTTPRouter_ContextStore(t *testing.T) {
	r := NewRouter()

	r.GET("/test", func(c router.Context) error {
		c.Set("key1", "value1")
		c.Set("key2", 42)

		if c.Get("key1") != "value1" {
			t.Error("expected key1 to be 'value1'")
		}

		if c.Get("key2") != 42 {
			t.Error("expected key2 to be 42")
		}

		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/test", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("expected status 200, got %d", w.Code)
	}
}
