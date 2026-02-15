package contract

import (
	"net/http"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
	nethttpadapter "github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestPerformRequest_Helper(t *testing.T) {
	r := nethttpadapter.NewRouter()
	r.POST("/echo", func(c router.Context) error {
		return c.String(http.StatusOK, c.Request().Header.Get("Content-Type"))
	})

	res := performRequest(r, http.MethodPost, "/echo", nil, "application/json")
	if res.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", res.Code)
	}
	if got := res.Body.String(); got != "application/json" {
		t.Fatalf("expected content-type echo, got %q", got)
	}
}
