package router_test

import (
	"testing"

	"github.com/nimburion/nimburion/pkg/http/router"
	ginadapter "github.com/nimburion/nimburion/pkg/http/router/gin"
	gorillaadapter "github.com/nimburion/nimburion/pkg/http/router/gorilla"
	nethttpadapter "github.com/nimburion/nimburion/pkg/http/router/nethttp"
)

func TestRouterImplementations_ConformToInterface(_ *testing.T) {
	var _ router.Router = nethttpadapter.NewRouter()
	var _ router.Router = ginadapter.NewRouter()
	var _ router.Router = gorillaadapter.NewRouter()
}
