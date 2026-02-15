// Package factory creates router implementations from configuration.
package factory

import (
	"fmt"
	"sort"
	"strings"

	"github.com/nimburion/nimburion/pkg/server/router"
	ginadapter "github.com/nimburion/nimburion/pkg/server/router/gin"
	gorillaadapter "github.com/nimburion/nimburion/pkg/server/router/gorilla"
	nethttpadapter "github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

var supported = map[string]func() router.Router{
	"nethttp": func() router.Router { return nethttpadapter.NewRouter() },
	"gin":     func() router.Router { return ginadapter.NewRouter() },
	"gorilla": func() router.Router { return gorillaadapter.NewRouter() },
}

// NewRouter creates a router from type.
func NewRouter(routerType string) (router.Router, error) {
	rt := strings.TrimSpace(strings.ToLower(routerType))
	if rt == "" {
		rt = "nethttp"
	}
	if create, ok := supported[rt]; ok {
		return create(), nil
	}

	return nil, fmt.Errorf("unsupported router type %q (supported: %s)", routerType, strings.Join(SupportedTypes(), ", "))
}

// SupportedTypes returns the supported router types.
func SupportedTypes() []string {
	types := make([]string, 0, len(supported))
	for t := range supported {
		types = append(types, t)
	}
	sort.Strings(types)
	return types
}
