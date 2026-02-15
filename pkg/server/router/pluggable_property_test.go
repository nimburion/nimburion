package router_test

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	"github.com/nimburion/nimburion/pkg/server/router"
	ginadapter "github.com/nimburion/nimburion/pkg/server/router/gin"
	gorillaadapter "github.com/nimburion/nimburion/pkg/server/router/gorilla"
	nethttpadapter "github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

type adapterFactory struct {
	name   string
	create func() router.Router
}

var adapters = []adapterFactory{
	{name: "nethttp", create: func() router.Router { return nethttpadapter.NewRouter() }},
	{name: "gin", create: func() router.Router { return ginadapter.NewRouter() }},
	{name: "gorilla", create: func() router.Router { return gorillaadapter.NewRouter() }},
}

type responseSnapshot struct {
	status int
	body   string
}

func perform(r router.Router, method, path, contentType string, body *bytes.Reader) responseSnapshot {
	var reqBody io.Reader = http.NoBody
	if body != nil {
		reqBody = body
	}
	req := httptest.NewRequest(method, path, reqBody)
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return responseSnapshot{status: w.Code, body: w.Body.String()}
}

// Property 3: Routing Behavior Equivalence
// Validates: Requirements 5.8
func TestProperty_RoutingBehaviorEquivalence(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 50
	props := gopter.NewProperties(params)

	genMethod := gen.OneConstOf(http.MethodGet, http.MethodPost, http.MethodPut, http.MethodDelete, http.MethodPatch)
	genSeg := gen.Identifier().Map(func(v string) string {
		if len(v) == 0 {
			return "x"
		}
		if len(v) > 8 {
			return v[:8]
		}
		return strings.ToLower(v)
	})

	props.Property("all adapters route method/path equivalently", prop.ForAll(func(method, seg string) bool {
		path := "/eq/" + seg
		results := make(map[string]responseSnapshot, len(adapters))
		for _, a := range adapters {
			r := a.create()
			h := func(c router.Context) error { return c.String(http.StatusCreated, method+":"+seg) }
			switch method {
			case http.MethodGet:
				r.GET(path, h)
			case http.MethodPost:
				r.POST(path, h)
			case http.MethodPut:
				r.PUT(path, h)
			case http.MethodDelete:
				r.DELETE(path, h)
			case http.MethodPatch:
				r.PATCH(path, h)
			}
			results[a.name] = perform(r, method, path, "", nil)
		}

		baseline := results[adapters[0].name]
		for _, a := range adapters[1:] {
			if results[a.name] != baseline {
				return false
			}
		}
		return true
	}, genMethod, genSeg))

	props.TestingRun(t)
}

// Property 2: Middleware Chain Equivalence
// Validates: Requirements 4.3, 4.4, 4.5, 4.7
func TestProperty_MiddlewareChainEquivalence(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 40
	props := gopter.NewProperties(params)

	props.Property("all adapters preserve middleware execution order", prop.ForAll(func() bool {
		for _, a := range adapters {
			r := a.create()
			order := make([]string, 0, 5)

			r.Use(func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					order = append(order, "g1")
					return next(c)
				}
			})

			grp := r.Group("/m", func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					order = append(order, "grp")
					return next(c)
				}
			})

			grp.GET("/x", func(c router.Context) error {
				order = append(order, "h")
				return c.String(http.StatusOK, "ok")
			}, func(next router.HandlerFunc) router.HandlerFunc {
				return func(c router.Context) error {
					order = append(order, "r1")
					return next(c)
				}
			})

			res := perform(r, http.MethodGet, "/m/x", "", nil)
			if res.status != http.StatusOK {
				return false
			}
			expected := []string{"g1", "grp", "r1", "h"}
			if len(order) != len(expected) {
				return false
			}
			for i := range expected {
				if order[i] != expected[i] {
					return false
				}
			}
		}
		return true
	}))

	props.TestingRun(t)
}

// Property 1: Context Method Equivalence
// Validates: Requirements 6.6, 7.4, 8.6, 9.5, 10.4, 11.6, 12.6
func TestProperty_ContextMethodEquivalence(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 40
	props := gopter.NewProperties(params)

	genSeg := gen.Identifier().Map(func(v string) string {
		if len(v) == 0 {
			return "u1"
		}
		if len(v) > 6 {
			return v[:6]
		}
		return strings.ToLower(v)
	})

	type reqPayload struct {
		Name string `json:"name"`
	}

	props.Property("all adapters extract param/query and bind equivalently", prop.ForAll(func(id, q, name string) bool {
		if q == "" {
			q = "q"
		}
		payload := reqPayload{Name: name}
		body, _ := json.Marshal(payload)
		for _, a := range adapters {
			r := a.create()
			r.POST("/users/:id", func(c router.Context) error {
				var in reqPayload
				if err := c.Bind(&in); err != nil {
					return c.String(http.StatusBadRequest, "bind-error")
				}
				c.Set("k", "v")
				if c.Get("k") != "v" {
					return c.String(http.StatusInternalServerError, "ctx-store")
				}
				return c.String(http.StatusOK, fmt.Sprintf("%s|%s|%s", c.Param("id"), c.Query("q"), in.Name))
			})

			res := perform(r, http.MethodPost, "/users/"+id+"?q="+q, "application/json", bytes.NewReader(body))
			want := fmt.Sprintf("%s|%s|%s", id, q, payload.Name)
			if res.status != http.StatusOK || res.body != want {
				return false
			}
		}
		return true
	}, genSeg, genSeg, genSeg))

	props.Property("all adapters track response status/written equivalently", prop.ForAll(func() bool {
		for _, a := range adapters {
			r := a.create()
			r.GET("/rw", func(c router.Context) error {
				rw := c.Response()
				if rw.Written() {
					return fmt.Errorf("written before write")
				}
				rw.WriteHeader(http.StatusAccepted)
				if rw.Status() != http.StatusAccepted || !rw.Written() {
					return fmt.Errorf("unexpected rw state")
				}
				return nil
			})
			res := perform(r, http.MethodGet, "/rw", "", nil)
			if res.status != http.StatusAccepted {
				return false
			}
		}
		return true
	}))

	props.TestingRun(t)
}

// Property 4: Error Handling Equivalence
// Validates: Requirements 13.5
func TestProperty_ErrorHandlingEquivalence(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 20
	props := gopter.NewProperties(params)

	props.Property("all adapters preserve error handling semantics", prop.ForAll(func() bool {
		for _, a := range adapters {
			// Error before write -> 500
			r1 := a.create()
			r1.GET("/e1", func(c router.Context) error { return fmt.Errorf("boom") })
			if got := perform(r1, http.MethodGet, "/e1", "", nil); got.status != http.StatusInternalServerError {
				return false
			}

			// Error after write -> preserve first response
			r2 := a.create()
			r2.GET("/e2", func(c router.Context) error {
				if err := c.String(http.StatusBadRequest, "bad"); err != nil {
					return err
				}
				return fmt.Errorf("ignored")
			})
			if got := perform(r2, http.MethodGet, "/e2", "", nil); got.status != http.StatusBadRequest || got.body != "bad" {
				return false
			}
		}
		return true
	}))

	props.TestingRun(t)
}

// Property 5: Edge Case Handling Equivalence
// Validates: Requirements 26.6
func TestProperty_EdgeCaseHandlingEquivalence(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 20
	props := gopter.NewProperties(params)

	props.Property("all adapters handle malformed/empty/unsupported input equivalently", prop.ForAll(func() bool {
		type in struct {
			N string `json:"n"`
		}
		for _, a := range adapters {
			r := a.create()
			r.POST("/bind", func(c router.Context) error {
				var v in
				if err := c.Bind(&v); err != nil {
					return c.String(http.StatusBadRequest, "bind-error")
				}
				return c.String(http.StatusOK, v.N)
			})
			r.GET("/param/:id", func(c router.Context) error {
				return c.String(http.StatusOK, c.Param("missing"))
			})
			r.GET("/multi", func(c router.Context) error {
				_ = c.String(http.StatusCreated, "a")
				_ = c.String(http.StatusAccepted, "b")
				return nil
			})

			if got := perform(r, http.MethodPost, "/bind", "application/json", bytes.NewReader([]byte("{"))); got.status != http.StatusBadRequest {
				return false
			}
			if got := perform(r, http.MethodPost, "/bind", "application/json", nil); got.status != http.StatusBadRequest {
				return false
			}
			if got := perform(r, http.MethodPost, "/bind", "text/plain", bytes.NewReader([]byte("x=1"))); got.status != http.StatusBadRequest {
				return false
			}
			if got := perform(r, http.MethodGet, "/param/1", "", nil); got.status != http.StatusOK || got.body != "" {
				return false
			}
			if got := perform(r, http.MethodGet, "/multi", "", nil); got.status != http.StatusCreated || got.body != "ab" {
				return false
			}
		}
		return true
	}))

	props.TestingRun(t)
}

// Property 6: Thread Safety
// Validates: Requirements 24.1-24.5
func TestProperty_ThreadSafety(t *testing.T) {
	params := gopter.DefaultTestParameters()
	params.MinSuccessfulTests = 10
	props := gopter.NewProperties(params)

	genN := gen.IntRange(10, 80)

	props.Property("all adapters handle concurrent requests safely", prop.ForAll(func(n int) bool {
		for _, a := range adapters {
			r := a.create()
			var served atomic.Int64
			r.GET("/ping", func(c router.Context) error {
				served.Add(1)
				return c.String(http.StatusOK, "ok")
			})

			var wg sync.WaitGroup
			wg.Add(n)
			ok := atomic.Bool{}
			ok.Store(true)
			for i := 0; i < n; i++ {
				go func() {
					defer wg.Done()
					if got := perform(r, http.MethodGet, "/ping", "", nil); got.status != http.StatusOK {
						ok.Store(false)
					}
				}()
			}
			wg.Wait()
			if !ok.Load() || int(served.Load()) != n {
				return false
			}
		}
		return true
	}, genN))

	props.TestingRun(t)
}
