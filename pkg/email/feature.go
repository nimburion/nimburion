package email

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	emailconfig "github.com/nimburion/nimburion/pkg/email/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "email-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "email", Extension: &emailconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the email config section owned by the email family.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
