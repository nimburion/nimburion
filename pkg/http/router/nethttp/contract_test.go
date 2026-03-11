package nethttp

import (
	"testing"

	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/http/router/contract"
)

func TestNetHTTPRouterContract(t *testing.T) {
	contract.TestRouterContract(t, func() router.Router {
		return NewRouter()
	})
}
