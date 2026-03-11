package descriptor

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

func TestGenerateHTTPOnlyDescriptor(t *testing.T) {
	root := newRoot("app")
	runCmd := &cobra.Command{Use: "run", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	runCmd.Annotations = map[string]string{"policies.run": "run"}
	root.AddCommand(runCmd)
	configCmd := &cobra.Command{Use: "config"}
	configCmd.AddCommand(&cobra.Command{Use: "show", RunE: func(_ *cobra.Command, _ []string) error { return nil }})
	configCmd.AddCommand(&cobra.Command{Use: "validate", RunE: func(_ *cobra.Command, _ []string) error { return nil }})
	root.AddCommand(configCmd)

	desc, err := Generate(root, Options{
		Application: Application{Name: "http-app", Kind: ApplicationKindService},
		EnvPrefix:   "APP",
		Management: &Management{
			Supported: true,
			Transport: "http",
			Endpoints: []Endpoint{{Kind: EndpointKindReadiness, Target: "/readyz"}},
		},
		Transports: []Transport{{Family: TransportFamilyHTTP}},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if desc.Runtime.DefaultCommand != "run" {
		t.Fatalf("expected run as default command, got %q", desc.Runtime.DefaultCommand)
	}
	if len(desc.Transports) != 1 || desc.Transports[0].Family != TransportFamilyHTTP {
		t.Fatalf("expected HTTP transport descriptor")
	}
	if !desc.Config.Render.Supported || !desc.Config.Validate.Supported {
		t.Fatal("expected config render and validate support")
	}
}

func TestGenerateGRPCOnlyDescriptor(t *testing.T) {
	root := newRoot("app")
	runCmd := &cobra.Command{Use: "run", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	runCmd.Annotations = map[string]string{"policies.run": "run"}
	root.AddCommand(runCmd)

	desc, err := Generate(root, Options{
		Application: Application{Name: "grpc-app", Kind: ApplicationKindService},
		Transports: []Transport{{
			Family:        TransportFamilyGRPC,
			HealthService: &Capability{Supported: true},
			Reflection:    &Capability{Supported: true},
		}},
		Management: &Management{
			Supported: true,
			Transport: "grpc",
			Endpoints: []Endpoint{{Kind: EndpointKindHealth, Target: "grpc.health.v1.Health/Check"}},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got := desc.Management.Transport; got != "grpc" {
		t.Fatalf("expected grpc management transport, got %q", got)
	}
	if !hasFeature(desc.Features, "grpc") || !hasFeature(desc.Features, "grpc_health") || !hasFeature(desc.Features, "grpc_reflection") {
		t.Fatalf("expected grpc feature flags in descriptor: %#v", desc.Features)
	}
}

func TestGenerateMixedAppDescriptor(t *testing.T) {
	root := newRoot("app")
	runCmd := &cobra.Command{Use: "run", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	runCmd.Annotations = map[string]string{"policies.run": "run"}
	root.AddCommand(runCmd)
	migrateCmd := &cobra.Command{Use: "migrate", RunE: func(_ *cobra.Command, _ []string) error { return nil }}
	migrateCmd.Annotations = map[string]string{"policies.run": "migration"}
	root.AddCommand(migrateCmd)

	desc, err := Generate(root, Options{
		Application: Application{Name: "mixed-app", Kind: ApplicationKindGateway},
		Transports: []Transport{
			{Family: TransportFamilyHTTP},
			{Family: TransportFamilyGRPC},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(desc.Transports) != 2 {
		t.Fatalf("expected two transports, got %d", len(desc.Transports))
	}
	if !desc.Migrations.Supported || strings.Join(desc.Migrations.Command, " ") != "migrate" {
		t.Fatalf("expected inferred migrate command, got %#v", desc.Migrations)
	}
}

func TestMarshalJSONAndYAML(t *testing.T) {
	desc := Descriptor{DescriptorVersion: VersionV1, Application: Application{Name: "app", Kind: ApplicationKindService}}
	if _, err := Marshal(desc, "json"); err != nil {
		t.Fatalf("json marshal failed: %v", err)
	}
	if _, err := Marshal(desc, "yaml"); err != nil {
		t.Fatalf("yaml marshal failed: %v", err)
	}
}

func newRoot(name string) *cobra.Command {
	root := &cobra.Command{Use: name}
	root.PersistentFlags().String("config-file", "", "")
	root.PersistentFlags().String("secret-file", "", "")
	return root
}

func hasFeature(features []Feature, name string) bool {
	for _, feature := range features {
		if feature.Name == name {
			return true
		}
	}
	return false
}
