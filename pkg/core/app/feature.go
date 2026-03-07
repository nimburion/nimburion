package app

import (
	appconfig "github.com/nimburion/nimburion/pkg/core/app/config"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
)

type configFeature struct{}

func (configFeature) Name() string { return "core-app-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "app", Extension: &appconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the app config section owned by core app.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
