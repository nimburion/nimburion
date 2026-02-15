package gorilla

import (
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
	"github.com/nimburion/nimburion/pkg/server/router/contract"
)

func TestGorillaRouterContract(t *testing.T) {
	contract.TestRouterContract(t, func() router.Router {
		return NewRouter()
	})
}
