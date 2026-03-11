package gin

import (
	"testing"

	"github.com/nimburion/nimburion/pkg/http/router"
	"github.com/nimburion/nimburion/pkg/http/router/contract"
)

func TestRouterContract(t *testing.T) {
	contract.TestRouterContract(t, func() router.Router {
		return NewRouter()
	})
}
