package search

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	searchconfig "github.com/nimburion/nimburion/pkg/persistence/search/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "search-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "search", Extension: &searchconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the search config section owned by the persistence search family.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
