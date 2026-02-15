package router_test

import (
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
	ginadapter "github.com/nimburion/nimburion/pkg/server/router/gin"
	gorillaadapter "github.com/nimburion/nimburion/pkg/server/router/gorilla"
	nethttpadapter "github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func TestRouterImplementations_ConformToInterface(t *testing.T) {
	var _ router.Router = nethttpadapter.NewRouter()
	var _ router.Router = ginadapter.NewRouter()
	var _ router.Router = gorillaadapter.NewRouter()
}
