package schema

import (
	"testing"

	"github.com/google/jsonschema-go/jsonschema"
)

type nestedAppExtension struct {
	App struct {
		Foo string `mapstructure:"foo"`
	} `mapstructure:"app"`
}

type disableCoreSectionsExtension struct{}

func (disableCoreSectionsExtension) DisabledCoreConfigSections() []string {
	return []string{"email", "eventbus"}
}

type schemaCustomizerExtension struct {
	Portal struct {
		Mode string `mapstructure:"mode"`
	} `mapstructure:"portal"`
}

func (schemaCustomizerExtension) CustomizeSchema(schema *jsonschema.Schema) error {
	if schema.Properties == nil {
		schema.Properties = map[string]*jsonschema.Schema{}
	}
	portalSchema := schema.Properties["portal"]
	if portalSchema == nil {
		portalSchema = &jsonschema.Schema{}
		schema.Properties["portal"] = portalSchema
	}
	if portalSchema.Properties == nil {
		portalSchema.Properties = map[string]*jsonschema.Schema{}
	}
	modeSchema := portalSchema.Properties["mode"]
	if modeSchema == nil {
		modeSchema = &jsonschema.Schema{}
		portalSchema.Properties["mode"] = modeSchema
	}
	modeSchema.Enum = []any{"read-only", "managed"}
	return nil
}

func TestBuildSchema_UsesEventBusRootKey(t *testing.T) {
	schema, err := BuildSchema()
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	if _, ok := schema.Properties["eventbus"]; !ok {
		t.Fatal("expected eventbus root key in generated schema")
	}
	if _, ok := schema.Properties["event_bus"]; ok {
		t.Fatal("did not expect event_bus root key in generated schema")
	}
}

func TestBuildSchema_MergesNestedExtensionIntoCoreSection(t *testing.T) {
	schema, err := BuildSchema(nestedAppExtension{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	appSchema := schema.Properties["app"]
	if appSchema == nil {
		t.Fatal("expected app section in schema")
	}
	if _, ok := appSchema.Properties["name"]; !ok {
		t.Fatal("expected core app.name property to be preserved")
	}
	if _, ok := appSchema.Properties["foo"]; !ok {
		t.Fatal("expected extension app.foo property to be merged")
	}
}

func TestBuildSchema_DisablesCoreSectionsViaExtension(t *testing.T) {
	schema, err := BuildSchema(disableCoreSectionsExtension{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	if _, ok := schema.Properties["email"]; ok {
		t.Fatal("expected email section to be disabled")
	}
	if _, ok := schema.Properties["eventbus"]; ok {
		t.Fatal("expected eventbus section to be disabled")
	}
}

func TestBuildSchema_AppliesSchemaCustomizer(t *testing.T) {
	schema, err := BuildSchema(schemaCustomizerExtension{})
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}

	portalSchema := schema.Properties["portal"]
	if portalSchema == nil {
		t.Fatal("expected portal section in schema")
	}
	modeSchema := portalSchema.Properties["mode"]
	if modeSchema == nil {
		t.Fatal("expected portal.mode property in schema")
	}
	if len(modeSchema.Enum) != 2 {
		t.Fatalf("expected enum injected by customizer, got %d values", len(modeSchema.Enum))
	}
}

func TestBuildSchema_NoExtensions(t *testing.T) {
	schema, err := BuildSchema()
	if err != nil {
		t.Fatalf("build schema: %v", err)
	}
	if schema == nil {
		t.Fatal("expected schema")
	}
	if len(schema.Properties) == 0 {
		t.Fatal("expected properties in schema")
	}
}
