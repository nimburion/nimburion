package contract

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// TestRouterContract runs the shared router conformance suite.
func TestRouterContract(t *testing.T, createRouter func() router.Router) {
	t.Helper()

	t.Run("http_methods", func(t *testing.T) {
		tests := []struct {
			method string
			path   string
			add    func(r router.Router, h router.HandlerFunc)
		}{
			{method: http.MethodGet, path: "/m/get", add: func(r router.Router, h router.HandlerFunc) { r.GET("/m/get", h) }},
			{method: http.MethodPost, path: "/m/post", add: func(r router.Router, h router.HandlerFunc) { r.POST("/m/post", h) }},
			{method: http.MethodPut, path: "/m/put", add: func(r router.Router, h router.HandlerFunc) { r.PUT("/m/put", h) }},
			{method: http.MethodDelete, path: "/m/delete", add: func(r router.Router, h router.HandlerFunc) { r.DELETE("/m/delete", h) }},
			{method: http.MethodPatch, path: "/m/patch", add: func(r router.Router, h router.HandlerFunc) { r.PATCH("/m/patch", h) }},
		}

		for _, tt := range tests {
			t.Run(tt.method, func(t *testing.T) {
				r := createRouter()
				tt.add(r, func(c router.Context) error {
					return c.String(http.StatusOK, tt.method)
				})

				res := performRequest(r, tt.method, tt.path, nil, "")
				if res.Code != http.StatusOK {
					t.Fatalf("expected 200, got %d", res.Code)
				}
				if res.Body.String() != tt.method {
					t.Fatalf("expected body %q, got %q", tt.method, res.Body.String())
				}
			})
		}

		r := createRouter()
		res := performRequest(r, http.MethodGet, "/not-registered", nil, "")
		if res.Code != http.StatusNotFound {
			t.Fatalf("expected 404 for unregistered route, got %d", res.Code)
		}
	})

	t.Run("groups", func(t *testing.T) {
		r := createRouter()
		api := r.Group("/api")
		api.GET("/users", func(c router.Context) error { return c.String(http.StatusOK, "ok") })

		res := performRequest(r, http.MethodGet, "/api/users", nil, "")
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}

		v1 := api.Group("/v1")
		v1.GET("/posts", func(c router.Context) error { return c.String(http.StatusOK, "nested") })

		res = performRequest(r, http.MethodGet, "/api/v1/posts", nil, "")
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}

		secured := r.Group("/secured", func(next router.HandlerFunc) router.HandlerFunc {
			return func(c router.Context) error {
				c.Set("group_mw", "on")
				return next(c)
			}
		})
		secured.GET("/hello", func(c router.Context) error {
			return c.String(http.StatusOK, c.Get("group_mw").(string))
		})

		res = performRequest(r, http.MethodGet, "/secured/hello", nil, "")
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}
		if res.Body.String() != "on" {
			t.Fatalf("expected middleware value, got %q", res.Body.String())
		}
	})

	t.Run("middleware", func(t *testing.T) {
		r := createRouter()
		order := make([]string, 0, 3)

		r.Use(func(next router.HandlerFunc) router.HandlerFunc {
			return func(c router.Context) error {
				order = append(order, "global")
				return next(c)
			}
		})

		r.GET("/m", func(c router.Context) error {
			order = append(order, "handler")
			return c.String(http.StatusOK, "ok")
		}, func(next router.HandlerFunc) router.HandlerFunc {
			return func(c router.Context) error {
				order = append(order, "route")
				return next(c)
			}
		})

		res := performRequest(r, http.MethodGet, "/m", nil, "")
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}

		expected := []string{"global", "route", "handler"}
		if len(order) != len(expected) {
			t.Fatalf("unexpected order length: %v", order)
		}
		for i := range expected {
			if order[i] != expected[i] {
				t.Fatalf("unexpected middleware order: %v", order)
			}
		}

		r = createRouter()
		handlerCalled := false
		r.GET("/stop", func(c router.Context) error {
			handlerCalled = true
			return c.String(http.StatusOK, "never")
		}, func(next router.HandlerFunc) router.HandlerFunc {
			return func(c router.Context) error {
				return errors.New("stop")
			}
		})

		res = performRequest(r, http.MethodGet, "/stop", nil, "")
		if handlerCalled {
			t.Fatal("handler should not be called when middleware returns error")
		}
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", res.Code)
		}
	})

	t.Run("path_params", func(t *testing.T) {
		r := createRouter()
		r.GET("/user/:id", func(c router.Context) error {
			return c.String(http.StatusOK, c.Param("id"))
		})
		r.GET("/users/:userId/posts/:postId", func(c router.Context) error {
			v := c.Param("userId") + ":" + c.Param("postId")
			return c.String(http.StatusOK, v)
		})
		r.GET("/no/:id", func(c router.Context) error {
			if c.Param("missing") != "" {
				t.Fatal("missing parameter must return empty string")
			}
			return c.String(http.StatusOK, "ok")
		})

		if res := performRequest(r, http.MethodGet, "/user/42", nil, ""); res.Body.String() != "42" {
			t.Fatalf("expected param 42, got %q", res.Body.String())
		}
		if res := performRequest(r, http.MethodGet, "/users/u1/posts/p9", nil, ""); res.Body.String() != "u1:p9" {
			t.Fatalf("unexpected multiple params result: %q", res.Body.String())
		}
		if res := performRequest(r, http.MethodGet, "/no/1", nil, ""); res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}
	})

	t.Run("query_params", func(t *testing.T) {
		r := createRouter()
		r.GET("/q", func(c router.Context) error { return c.String(http.StatusOK, c.Query("q")) })

		if res := performRequest(r, http.MethodGet, "/q?q=one", nil, ""); res.Body.String() != "one" {
			t.Fatalf("expected one, got %q", res.Body.String())
		}
		if res := performRequest(r, http.MethodGet, "/q?q=first&q=second", nil, ""); res.Body.String() != "first" {
			t.Fatalf("expected first, got %q", res.Body.String())
		}
		if res := performRequest(r, http.MethodGet, "/q", nil, ""); res.Body.String() != "" {
			t.Fatalf("expected empty query value, got %q", res.Body.String())
		}
	})

	t.Run("bind", func(t *testing.T) {
		type in struct {
			Name string `json:"name"`
		}

		r := createRouter()
		r.POST("/bind", func(c router.Context) error {
			var payload in
			if err := c.Bind(&payload); err != nil {
				return c.String(http.StatusBadRequest, "bind-error")
			}
			return c.String(http.StatusOK, payload.Name)
		})

		body, _ := json.Marshal(in{Name: "alice"})
		if res := performRequest(r, http.MethodPost, "/bind", bytes.NewReader(body), "application/json"); res.Body.String() != "alice" {
			t.Fatalf("expected alice, got %q", res.Body.String())
		}

		if res := performRequest(r, http.MethodPost, "/bind", strings.NewReader("{"), "application/json"); res.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 invalid json, got %d", res.Code)
		}

		if res := performRequest(r, http.MethodPost, "/bind", nil, "application/json"); res.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 empty body, got %d", res.Code)
		}

		if res := performRequest(r, http.MethodPost, "/bind", strings.NewReader("name=x"), "text/plain"); res.Code != http.StatusBadRequest {
			t.Fatalf("expected 400 unsupported content-type, got %d", res.Code)
		}
	})

	t.Run("responses", func(t *testing.T) {
		r := createRouter()
		r.GET("/json", func(c router.Context) error {
			return c.JSON(http.StatusCreated, map[string]string{"x": "y"})
		})
		r.GET("/string", func(c router.Context) error {
			return c.String(http.StatusAccepted, "hello")
		})

		res := performRequest(r, http.MethodGet, "/json", nil, "")
		if res.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", res.Code)
		}
		if !strings.Contains(res.Header().Get("Content-Type"), "application/json") {
			t.Fatalf("expected json content-type, got %q", res.Header().Get("Content-Type"))
		}

		res = performRequest(r, http.MethodGet, "/string", nil, "")
		if res.Code != http.StatusAccepted {
			t.Fatalf("expected 202, got %d", res.Code)
		}
		if !strings.Contains(res.Header().Get("Content-Type"), "text/plain") {
			t.Fatalf("expected text/plain content-type, got %q", res.Header().Get("Content-Type"))
		}
		if res.Body.String() != "hello" {
			t.Fatalf("expected hello, got %q", res.Body.String())
		}
	})

	t.Run("context_storage", func(t *testing.T) {
		r := createRouter()
		r.GET("/ctx", func(c router.Context) error {
			if c.Get("missing") != nil {
				t.Fatal("expected nil for missing key")
			}
			c.Set("k1", "v1")
			c.Set("k2", 7)
			if c.Get("k1") != "v1" {
				t.Fatal("expected k1=v1")
			}
			if c.Get("k2") != 7 {
				t.Fatal("expected k2=7")
			}
			return c.String(http.StatusOK, "ok")
		})

		res := performRequest(r, http.MethodGet, "/ctx", nil, "")
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}

		r = createRouter()
		r.Use(func(next router.HandlerFunc) router.HandlerFunc {
			return func(c router.Context) error {
				c.Set("from_mw", "yes")
				return next(c)
			}
		})
		r.GET("/ctx2", func(c router.Context) error {
			if c.Get("from_mw") != "yes" {
				t.Fatal("expected value set by middleware")
			}
			return c.String(http.StatusOK, "ok")
		})
		res = performRequest(r, http.MethodGet, "/ctx2", nil, "")
		if res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}
	})

	t.Run("error_handling", func(t *testing.T) {
		r := createRouter()
		r.GET("/err1", func(c router.Context) error { return errors.New("boom") })
		res := performRequest(r, http.MethodGet, "/err1", nil, "")
		if res.Code != http.StatusInternalServerError {
			t.Fatalf("expected 500, got %d", res.Code)
		}

		r = createRouter()
		r.GET("/err2", func(c router.Context) error {
			if err := c.String(http.StatusBadRequest, "bad"); err != nil {
				return err
			}
			return errors.New("ignored")
		})
		res = performRequest(r, http.MethodGet, "/err2", nil, "")
		if res.Code != http.StatusBadRequest {
			t.Fatalf("expected 400, got %d", res.Code)
		}
		if res.Body.String() != "bad" {
			t.Fatalf("expected bad, got %q", res.Body.String())
		}
	})

	t.Run("response_writer", func(t *testing.T) {
		r := createRouter()
		r.GET("/rw1", func(c router.Context) error {
			rw := c.Response()
			if rw.Written() {
				t.Fatal("Written must be false before writes")
			}
			_, err := rw.Write([]byte("ok"))
			if err != nil {
				return err
			}
			if !rw.Written() {
				t.Fatal("Written must be true after write")
			}
			if rw.Status() != http.StatusOK {
				t.Fatalf("expected status 200, got %d", rw.Status())
			}
			return nil
		})

		if res := performRequest(r, http.MethodGet, "/rw1", nil, ""); res.Code != http.StatusOK {
			t.Fatalf("expected 200, got %d", res.Code)
		}

		r = createRouter()
		r.GET("/rw2", func(c router.Context) error {
			rw := c.Response()
			rw.WriteHeader(http.StatusCreated)
			if rw.Status() != http.StatusCreated {
				t.Fatalf("expected status 201, got %d", rw.Status())
			}
			if !rw.Written() {
				t.Fatal("Written must be true after WriteHeader")
			}
			return nil
		})
		if res := performRequest(r, http.MethodGet, "/rw2", nil, ""); res.Code != http.StatusCreated {
			t.Fatalf("expected 201, got %d", res.Code)
		}
	})
}

func performRequest(r router.Router, method, path string, body io.Reader, contentType string) *httptest.ResponseRecorder {
	var testBody io.Reader = http.NoBody
	if body != nil {
		testBody = body
	}
	req := httptest.NewRequest(method, path, testBody)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}
