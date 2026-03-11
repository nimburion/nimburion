package csrf

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	csrfconfig "github.com/nimburion/nimburion/pkg/http/csrf/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "http-csrf-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "csrf", Extension: &csrfconfig.Extension{}},
		},
	}
}

// NewConfigFeature returns the feature that registers the CSRF config extension.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
