// Package observability exposes framework features for observability components.
package observability

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	observabilityconfig "github.com/nimburion/nimburion/pkg/observability/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "observability-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "observability", Extension: &observabilityconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the observability config section owned by the observability family.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
