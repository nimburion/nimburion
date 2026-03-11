package server

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	serverconfig "github.com/nimburion/nimburion/pkg/http/server/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "http-server-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "http-server", Extension: &serverconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the http and management config sections owned by the HTTP server family.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
