package securityheaders

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	securityheadersconfig "github.com/nimburion/nimburion/pkg/http/securityheaders/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "http-security-headers-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "security_headers", Extension: &securityheadersconfig.Extension{}},
		},
	}
}

// NewConfigFeature returns the feature that registers the security headers config extension.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
