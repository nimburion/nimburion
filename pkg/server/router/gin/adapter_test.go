package gin

import (
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/contract"
)

func TestGinRouterContract(t *testing.T) {
	contract.TestRouterContract(t, func() router.Router {
		return NewRouter()
	})
}
