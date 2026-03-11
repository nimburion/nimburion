package sse

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	sseconfig "github.com/nimburion/nimburion/pkg/http/sse/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "http-sse-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "sse", Extension: &sseconfig.Extension{}},
		},
	}
}

// NewConfigFeature returns the feature that registers the SSE config extension.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
