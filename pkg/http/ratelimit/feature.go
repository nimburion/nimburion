package ratelimit

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	ratelimitconfig "github.com/nimburion/nimburion/pkg/http/ratelimit/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "http-rate-limit-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "rate_limit", Extension: &ratelimitconfig.Extension{}},
		},
	}
}

// NewConfigFeature returns the feature that registers the rate limit config extension.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
