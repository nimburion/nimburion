package persistence

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	persistenceconfig "github.com/nimburion/nimburion/pkg/persistence/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "persistence-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "database", Extension: &persistenceconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the database config section owned by the persistence family.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
