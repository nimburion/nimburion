package gin

import (
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/contract"
)

func TestRouterContract(t *testing.T) {
	contract.TestRouterContract(t, func() router.Router {
		return NewRouter()
	})
}
