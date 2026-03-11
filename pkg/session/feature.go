package session

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	sessionconfig "github.com/nimburion/nimburion/pkg/session/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "session-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "session", Extension: &sessionconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the session config section owned by the session family.
func NewConfigFeature() corefeature.Feature {
	return configFeature{}
}
