package schema

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	schemaconfig "github.com/nimburion/nimburion/pkg/eventbus/schema/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "eventbus-schema-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "validation", Extension: &schemaconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the validation.kafka config section owned by the eventbus schema family.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
