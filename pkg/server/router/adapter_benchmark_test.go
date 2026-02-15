package router_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
	ginadapter "github.com/nimburion/nimburion/pkg/server/router/gin"
	gorillaadapter "github.com/nimburion/nimburion/pkg/server/router/gorilla"
	nethttpadapter "github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func benchmarkRouter(b *testing.B, name string, create func() router.Router) {
	b.Helper()
	r := create()
	r.Use(func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			c.Set("mw", "1")
			return next(c)
		}
	})
	r.GET("/bench/:id", func(c router.Context) error {
		_ = c.Param("id")
		_ = c.Query("q")
		_ = c.Get("mw")
		return c.String(http.StatusOK, "ok")
	})

	req := httptest.NewRequest(http.MethodGet, "/bench/42?q=x", nil)

	b.Run(name, func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			if w.Code != http.StatusOK {
				b.Fatalf("unexpected status: %d", w.Code)
			}
		}
	})
}

func BenchmarkRouterAdapters(b *testing.B) {
	benchmarkRouter(b, "nethttp", func() router.Router { return nethttpadapter.NewRouter() })
	benchmarkRouter(b, "gin", func() router.Router { return ginadapter.NewRouter() })
	benchmarkRouter(b, "gorilla", func() router.Router { return gorillaadapter.NewRouter() })
}
