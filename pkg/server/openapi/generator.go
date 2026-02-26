package openapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"sync"

	"github.com/nimburion/nimburion/pkg/server/router"
	"gopkg.in/yaml.v3"
)

// Route describes one registered endpoint.
type Route struct {
	Method      string
	Path        string
	Annotations EndpointAnnotations
}

// EndpointAnnotations customizes generated OpenAPI operation metadata.
type EndpointAnnotations struct {
	Summary     string
	Description string
	Tags        []string
	OperationID string
}

// Spec is a minimal OpenAPI v3 document generated from registered routes.
type Spec struct {
	OpenAPI string               `json:"openapi" yaml:"openapi"`
	Info    SpecInfo             `json:"info" yaml:"info"`
	Paths   map[string]*PathItem `json:"paths" yaml:"paths"`
}

// SpecInfo contains API metadata.
type SpecInfo struct {
	Title   string `json:"title" yaml:"title"`
	Version string `json:"version" yaml:"version"`
}

// PathItem groups operations by HTTP method.
type PathItem struct {
	Get    *Operation `json:"get,omitempty" yaml:"get,omitempty"`
	Post   *Operation `json:"post,omitempty" yaml:"post,omitempty"`
	Put    *Operation `json:"put,omitempty" yaml:"put,omitempty"`
	Delete *Operation `json:"delete,omitempty" yaml:"delete,omitempty"`
	Patch  *Operation `json:"patch,omitempty" yaml:"patch,omitempty"`
}

// Operation contains minimal operation metadata.
type Operation struct {
	OperationID string               `json:"operationId,omitempty" yaml:"operationId,omitempty"`
	Summary     string               `json:"summary,omitempty" yaml:"summary,omitempty"`
	Description string               `json:"description,omitempty" yaml:"description,omitempty"`
	Tags        []string             `json:"tags,omitempty" yaml:"tags,omitempty"`
	Parameters  []Parameter          `json:"parameters,omitempty" yaml:"parameters,omitempty"`
	Responses   map[string]*Response `json:"responses" yaml:"responses"`
}

// Parameter describes one operation parameter.
type Parameter struct {
	Name     string      `json:"name" yaml:"name"`
	In       string      `json:"in" yaml:"in"`
	Required bool        `json:"required" yaml:"required"`
	Schema   ParamSchema `json:"schema" yaml:"schema"`
}

// ParamSchema is the JSON schema for parameters.
type ParamSchema struct {
	Type string `json:"type" yaml:"type"`
}

// Response describes an operation response.
type Response struct {
	Description string `json:"description" yaml:"description"`
}

var endpointAnnotationsRegistry sync.Map

// Annotate registers OpenAPI annotations for a specific handler.
// Pass the returned handler to route registration.
func Annotate(handler router.HandlerFunc, annotations EndpointAnnotations) router.HandlerFunc {
	if handler == nil {
		return nil
	}
	endpointAnnotationsRegistry.Store(handlerAnnotationsKey(handler), normalizeAnnotations(annotations))
	return handler
}

// CollectRoutes executes the provided route registration callback and returns all
// registered routes in declaration order.
func CollectRoutes(register func(router.Router)) []Route {
	if register == nil {
		return []Route{}
	}
	c := newRouteCollector()
	register(c)
	return c.Routes()
}

// BuildSpec builds an OpenAPI document from collected routes.
func BuildSpec(title, version string, routes []Route) *Spec {
	spec := &Spec{
		OpenAPI: "3.0.3",
		Info: SpecInfo{
			Title:   strings.TrimSpace(title),
			Version: strings.TrimSpace(version),
		},
		Paths: make(map[string]*PathItem),
	}
	if spec.Info.Title == "" {
		spec.Info.Title = "API"
	}
	if spec.Info.Version == "" {
		spec.Info.Version = "0.0.0"
	}

	normalized := normalizeRoutes(routes)
	for _, route := range normalized {
		path := normalizeOpenAPIPath(route.Path)
		if path == "" {
			continue
		}
		item := spec.Paths[path]
		if item == nil {
			item = &PathItem{}
			spec.Paths[path] = item
		}

		op := &Operation{
			OperationID: buildOperationID(route.Method, path),
			Summary:     fmt.Sprintf("%s %s", route.Method, path),
			Description: "",
			Tags:        buildTags(path),
			Parameters:  buildPathParameters(path),
			Responses: map[string]*Response{
				defaultResponseCode(route.Method): {Description: defaultResponseDescription(route.Method)},
			},
		}
		applyAnnotations(op, route.Annotations)

		switch route.Method {
		case http.MethodGet:
			item.Get = op
		case http.MethodPost:
			item.Post = op
		case http.MethodPut:
			item.Put = op
		case http.MethodDelete:
			item.Delete = op
		case http.MethodPatch:
			item.Patch = op
		}
	}

	return spec
}

// WriteSpec writes the OpenAPI document as YAML or JSON based on file extension.
func WriteSpec(path string, spec *Spec) error {
	if spec == nil {
		return fmt.Errorf("openapi spec is nil")
	}
	outputPath := strings.TrimSpace(path)
	if outputPath == "" {
		return fmt.Errorf("output path is required")
	}

	if dir := filepath.Dir(outputPath); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return fmt.Errorf("create output directory: %w", err)
		}
	}

	var (
		data []byte
		err  error
	)
	switch strings.ToLower(filepath.Ext(outputPath)) {
	case ".json":
		data, err = json.MarshalIndent(spec, "", "  ")
	default:
		data, err = yaml.Marshal(spec)
	}
	if err != nil {
		return fmt.Errorf("marshal openapi spec: %w", err)
	}
	if err := os.WriteFile(outputPath, data, 0o644); err != nil {
		return fmt.Errorf("write openapi spec: %w", err)
	}
	return nil
}

type routeCollector struct {
	routes *[]Route
	prefix string
}

func newRouteCollector() *routeCollector {
	routes := make([]Route, 0)
	return &routeCollector{routes: &routes}
}

// GET registers a handler for HTTP GET requests at the specified path.
func (r *routeCollector) GET(path string, handler router.HandlerFunc, _ ...router.MiddlewareFunc) {
	r.add(http.MethodGet, path, handler)
}

// POST registers a handler for HTTP POST requests at the specified path.
func (r *routeCollector) POST(path string, handler router.HandlerFunc, _ ...router.MiddlewareFunc) {
	r.add(http.MethodPost, path, handler)
}

// PUT registers a handler for HTTP PUT requests at the specified path.
func (r *routeCollector) PUT(path string, handler router.HandlerFunc, _ ...router.MiddlewareFunc) {
	r.add(http.MethodPut, path, handler)
}

// DELETE registers a handler for HTTP DELETE requests at the specified path.
func (r *routeCollector) DELETE(path string, handler router.HandlerFunc, _ ...router.MiddlewareFunc) {
	r.add(http.MethodDelete, path, handler)
}

// PATCH registers a handler for HTTP PATCH requests at the specified path.
func (r *routeCollector) PATCH(path string, handler router.HandlerFunc, _ ...router.MiddlewareFunc) {
	r.add(http.MethodPatch, path, handler)
}

// Group creates a new router group with the given prefix and optional middleware.
func (r *routeCollector) Group(prefix string, _ ...router.MiddlewareFunc) router.Router {
	return &routeCollector{
		routes: r.routes,
		prefix: joinPaths(r.prefix, prefix),
	}
}

// Use adds middleware to the router that will be applied to all subsequent routes.
func (r *routeCollector) Use(_ ...router.MiddlewareFunc) {}

// ServeHTTP implements the http.Handler interface, dispatching requests to registered handlers.
func (r *routeCollector) ServeHTTP(_ http.ResponseWriter, _ *http.Request) {}

// Routes returns all registered routes.
func (r *routeCollector) Routes() []Route {
	if r == nil || r.routes == nil {
		return []Route{}
	}
	cloned := make([]Route, len(*r.routes))
	copy(cloned, *r.routes)
	return cloned
}

func (r *routeCollector) add(method, path string, handler router.HandlerFunc) {
	fullPath := joinPaths(r.prefix, path)
	annotations, _ := annotationsForHandler(handler)
	*r.routes = append(*r.routes, Route{
		Method:      strings.ToUpper(strings.TrimSpace(method)),
		Path:        fullPath,
		Annotations: annotations,
	})
}

func joinPaths(prefix, path string) string {
	prefix = strings.TrimSpace(prefix)
	path = strings.TrimSpace(path)
	switch {
	case prefix == "" && path == "":
		return "/"
	case prefix == "":
		if strings.HasPrefix(path, "/") {
			return path
		}
		return "/" + path
	case path == "":
		if strings.HasPrefix(prefix, "/") {
			return prefix
		}
		return "/" + prefix
	}
	joined := strings.TrimSuffix(prefix, "/") + "/" + strings.TrimPrefix(path, "/")
	if strings.HasPrefix(joined, "/") {
		return joined
	}
	return "/" + joined
}

func normalizeRoutes(routes []Route) []Route {
	if len(routes) == 0 {
		return []Route{}
	}
	supportedMethods := map[string]struct{}{
		http.MethodGet:    {},
		http.MethodPost:   {},
		http.MethodPut:    {},
		http.MethodDelete: {},
		http.MethodPatch:  {},
	}
	seen := make(map[string]int, len(routes))
	filtered := make([]Route, 0, len(routes))
	for _, route := range routes {
		method := strings.ToUpper(strings.TrimSpace(route.Method))
		if _, ok := supportedMethods[method]; !ok {
			continue
		}
		path := strings.TrimSpace(route.Path)
		if path == "" {
			continue
		}
		key := method + " " + path
		route.Method = method
		route.Path = path
		route.Annotations = normalizeAnnotations(route.Annotations)
		if existingIndex, exists := seen[key]; exists {
			if !hasAnnotations(filtered[existingIndex].Annotations) && hasAnnotations(route.Annotations) {
				filtered[existingIndex].Annotations = route.Annotations
			}
			continue
		}
		seen[key] = len(filtered)
		filtered = append(filtered, route)
	}

	sort.SliceStable(filtered, func(i, j int) bool {
		if filtered[i].Path == filtered[j].Path {
			return filtered[i].Method < filtered[j].Method
		}
		return filtered[i].Path < filtered[j].Path
	})
	return filtered
}

func normalizeOpenAPIPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return ""
	}
	if !strings.HasPrefix(trimmed, "/") {
		trimmed = "/" + trimmed
	}
	parts := strings.Split(trimmed, "/")
	for i, part := range parts {
		if strings.HasPrefix(part, ":") && len(part) > 1 {
			parts[i] = "{" + strings.TrimSpace(part[1:]) + "}"
		}
	}
	return strings.Join(parts, "/")
}

func buildPathParameters(path string) []Parameter {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	parameters := make([]Parameter, 0, len(parts))
	for _, part := range parts {
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") && len(part) > 2 {
			name := strings.TrimSpace(strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}"))
			if name == "" {
				continue
			}
			parameters = append(parameters, Parameter{
				Name:     name,
				In:       "path",
				Required: true,
				Schema:   ParamSchema{Type: "string"},
			})
		}
	}
	return parameters
}

func buildTags(path string) []string {
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, part := range parts {
		if part == "" || strings.HasPrefix(part, "{") {
			continue
		}
		return []string{part}
	}
	return []string{"default"}
}

func buildOperationID(method, path string) string {
	base := strings.ToLower(strings.TrimSpace(method))
	parts := strings.Split(strings.Trim(path, "/"), "/")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		if strings.HasPrefix(part, "{") && strings.HasSuffix(part, "}") {
			part = "by_" + strings.TrimSuffix(strings.TrimPrefix(part, "{"), "}")
		}
		part = strings.ReplaceAll(part, "-", "_")
		part = strings.ReplaceAll(part, ".", "_")
		base += toPascalCase(part)
	}
	if base == strings.ToLower(strings.TrimSpace(method)) {
		base += "Root"
	}
	return base
}

func toPascalCase(value string) string {
	parts := strings.FieldsFunc(value, func(r rune) bool {
		return r == '_' || r == '-' || r == ' ' || r == '.'
	})
	var out strings.Builder
	for _, part := range parts {
		if part == "" {
			continue
		}
		lower := strings.ToLower(part)
		out.WriteString(strings.ToUpper(lower[:1]))
		if len(lower) > 1 {
			out.WriteString(lower[1:])
		}
	}
	return out.String()
}

func applyAnnotations(operation *Operation, annotations EndpointAnnotations) {
	if operation == nil {
		return
	}
	annotations = normalizeAnnotations(annotations)
	if annotations.Summary != "" {
		operation.Summary = annotations.Summary
	}
	if annotations.Description != "" {
		operation.Description = annotations.Description
	}
	if len(annotations.Tags) > 0 {
		operation.Tags = append([]string{}, annotations.Tags...)
	}
	if annotations.OperationID != "" {
		operation.OperationID = annotations.OperationID
	}
}

func normalizeAnnotations(annotations EndpointAnnotations) EndpointAnnotations {
	annotations.Summary = strings.TrimSpace(annotations.Summary)
	annotations.Description = strings.TrimSpace(annotations.Description)
	annotations.OperationID = strings.TrimSpace(annotations.OperationID)
	if len(annotations.Tags) == 0 {
		return annotations
	}
	seen := make(map[string]struct{}, len(annotations.Tags))
	tags := make([]string, 0, len(annotations.Tags))
	for _, tag := range annotations.Tags {
		trimmed := strings.TrimSpace(tag)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		tags = append(tags, trimmed)
	}
	annotations.Tags = tags
	return annotations
}

func hasAnnotations(annotations EndpointAnnotations) bool {
	return annotations.Summary != "" ||
		annotations.Description != "" ||
		annotations.OperationID != "" ||
		len(annotations.Tags) > 0
}

func handlerAnnotationsKey(handler router.HandlerFunc) uintptr {
	if handler == nil {
		return 0
	}
	return reflect.ValueOf(handler).Pointer()
}

func annotationsForHandler(handler router.HandlerFunc) (EndpointAnnotations, bool) {
	key := handlerAnnotationsKey(handler)
	if key == 0 {
		return EndpointAnnotations{}, false
	}
	value, ok := endpointAnnotationsRegistry.Load(key)
	if !ok {
		return EndpointAnnotations{}, false
	}
	annotations, ok := value.(EndpointAnnotations)
	if !ok {
		return EndpointAnnotations{}, false
	}
	return normalizeAnnotations(annotations), true
}

func defaultResponseCode(method string) string {
	if strings.EqualFold(method, http.MethodDelete) {
		return "204"
	}
	return "200"
}

func defaultResponseDescription(method string) string {
	if strings.EqualFold(method, http.MethodDelete) {
		return "No Content"
	}
	return "OK"
}
