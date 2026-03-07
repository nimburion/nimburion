package cors

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	corsconfig "github.com/nimburion/nimburion/pkg/http/cors/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "http-cors-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "cors", Extension: &corsconfig.Extension{}},
		},
	}
}

func NewConfigFeature() corefeature.Feature { return configFeature{} }
