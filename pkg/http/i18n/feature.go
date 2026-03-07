package i18n

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	i18nconfig "github.com/nimburion/nimburion/pkg/http/i18n/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "http-i18n-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "i18n", Extension: &i18nconfig.Extension{}},
		},
	}
}

func NewConfigFeature() corefeature.Feature { return configFeature{} }
