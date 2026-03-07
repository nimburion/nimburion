package auth

import (
	authconfig "github.com/nimburion/nimburion/pkg/auth/config"
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
)

type configFeature struct{}

func (configFeature) Name() string { return "auth-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "auth", Extension: &authconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the auth config section owned by the auth family.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
