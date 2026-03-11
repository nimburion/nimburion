package openapi

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	openapiconfig "github.com/nimburion/nimburion/pkg/http/openapi/config"
)

type swaggerConfigFeature struct{}

func (swaggerConfigFeature) Name() string { return "http-openapi-config" }

func (swaggerConfigFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "swagger", Extension: &openapiconfig.Extension{}},
		},
	}
}

// NewConfigFeature returns the feature that registers the OpenAPI config extension.
func NewConfigFeature() corefeature.Feature { return swaggerConfigFeature{} }
