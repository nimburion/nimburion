package eventbus

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	eventbusconfig "github.com/nimburion/nimburion/pkg/eventbus/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "eventbus-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "eventbus", Extension: &eventbusconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the eventbus config section owned by the eventbus family.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
