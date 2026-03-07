package object

import (
	corefeature "github.com/nimburion/nimburion/pkg/core/feature"
	objectconfig "github.com/nimburion/nimburion/pkg/persistence/object/config"
)

type configFeature struct{}

func (configFeature) Name() string { return "object-storage-config" }

func (configFeature) Contributions() corefeature.Contributions {
	return corefeature.Contributions{
		ConfigExtensions: []corefeature.ConfigExtension{
			{Name: "object_storage", Extension: &objectconfig.Extension{}},
		},
	}
}

// NewConfigFeature contributes the object_storage config section owned by the persistence object family.
func NewConfigFeature() corefeature.Feature { return configFeature{} }
