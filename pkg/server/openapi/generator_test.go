package openapi

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nimburion/nimburion/pkg/server/router"
)

func TestCollectRoutes_WithGroups(t *testing.T) {
	routes := CollectRoutes(func(r router.Router) {
		r.GET("/health", nil)
		api := r.Group("/api/v1")
		api.GET("/users/:id", nil)
		api.POST("/users", nil)
	})

	if len(routes) != 3 {
		t.Fatalf("expected 3 routes, got %d", len(routes))
	}
	want := map[string]struct{}{
		"GET /health":           {},
		"GET /api/v1/users/:id": {},
		"POST /api/v1/users":    {},
	}
	for _, route := range routes {
		key := route.Method + " " + route.Path
		if _, ok := want[key]; !ok {
			t.Fatalf("unexpected route %s", key)
		}
	}
}

func TestBuildSpec_ConvertsPathParamsAndGeneratesOperations(t *testing.T) {
	spec := BuildSpec("Test API", "1.0.0", []Route{
		{Method: "GET", Path: "/users/:id"},
		{Method: "DELETE", Path: "/users/:id"},
	})

	item := spec.Paths["/users/{id}"]
	if item == nil {
		t.Fatal("expected /users/{id} path")
	}
	if item.Get == nil {
		t.Fatal("expected GET operation")
	}
	if item.Delete == nil {
		t.Fatal("expected DELETE operation")
	}

	if len(item.Get.Parameters) != 1 || item.Get.Parameters[0].Name != "id" {
		t.Fatalf("expected one path parameter id, got %+v", item.Get.Parameters)
	}
	if _, ok := item.Get.Responses["200"]; !ok {
		t.Fatal("expected 200 response for GET")
	}
	if _, ok := item.Delete.Responses["204"]; !ok {
		t.Fatal("expected 204 response for DELETE")
	}
}

func TestBuildSpec_AppliesEndpointAnnotations(t *testing.T) {
	handler := Annotate(func(c router.Context) error { return nil }, EndpointAnnotations{
		Summary:     "Get user by ID",
		Description: "Returns user details for the provided identifier.",
		Tags:        []string{"users", "read"},
		OperationID: "getUserByID",
	})

	routes := CollectRoutes(func(r router.Router) {
		r.GET("/users/:id", handler)
	})
	spec := BuildSpec("Test API", "1.0.0", routes)

	op := spec.Paths["/users/{id}"].Get
	if op == nil {
		t.Fatal("expected GET operation")
	}
	if op.Summary != "Get user by ID" {
		t.Fatalf("unexpected summary: %q", op.Summary)
	}
	if op.Description != "Returns user details for the provided identifier." {
		t.Fatalf("unexpected description: %q", op.Description)
	}
	if op.OperationID != "getUserByID" {
		t.Fatalf("unexpected operation id: %q", op.OperationID)
	}
	if len(op.Tags) != 2 || op.Tags[0] != "users" || op.Tags[1] != "read" {
		t.Fatalf("unexpected tags: %#v", op.Tags)
	}
}

func TestWriteSpec_JSONAndYAML(t *testing.T) {
	spec := BuildSpec("Test API", "1.0.0", []Route{
		{Method: "GET", Path: "/health"},
	})

	tmp := t.TempDir()
	jsonPath := filepath.Join(tmp, "openapi.json")
	yamlPath := filepath.Join(tmp, "openapi.yaml")

	if err := WriteSpec(jsonPath, spec); err != nil {
		t.Fatalf("write json spec: %v", err)
	}
	if err := WriteSpec(yamlPath, spec); err != nil {
		t.Fatalf("write yaml spec: %v", err)
	}

	jsonData, err := os.ReadFile(jsonPath)
	if err != nil {
		t.Fatalf("read json spec: %v", err)
	}
	var parsed map[string]interface{}
	if err := json.Unmarshal(jsonData, &parsed); err != nil {
		t.Fatalf("json must be valid: %v", err)
	}
	if parsed["openapi"] != "3.0.3" {
		t.Fatalf("unexpected openapi version: %v", parsed["openapi"])
	}

	yamlData, err := os.ReadFile(yamlPath)
	if err != nil {
		t.Fatalf("read yaml spec: %v", err)
	}
	if !strings.Contains(string(yamlData), "openapi: 3.0.3") {
		t.Fatalf("expected yaml content, got: %s", string(yamlData))
	}
}
