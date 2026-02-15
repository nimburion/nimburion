// Package openapi provides handlers for serving OpenAPI specifications and Swagger UI.
package openapi

import (
	"net/http"
	"os"
	"path/filepath"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// Handler serves OpenAPI specification files
type Handler struct {
	specPath string // Path to the OpenAPI spec file on disk
}

// NewHandler creates a new OpenAPI handler
func NewHandler(specPath string) *Handler {
	return &Handler{
		specPath: specPath,
	}
}

// ServeSpec serves the OpenAPI specification file
func (h *Handler) ServeSpec(c router.Context) error {
	// Read the spec file
	data, err := os.ReadFile(h.specPath)
	if err != nil {
		if os.IsNotExist(err) {
			return c.JSON(http.StatusNotFound, map[string]interface{}{
				"error": "OpenAPI specification not found",
			})
		}
		return c.JSON(http.StatusInternalServerError, map[string]interface{}{
			"error": "Failed to read OpenAPI specification",
		})
	}

	// Determine content type based on file extension
	ext := filepath.Ext(h.specPath)
	contentType := "application/x-yaml"
	if ext == ".json" {
		contentType = "application/json"
	}

	// Set headers
	c.Response().Header().Set("Content-Type", contentType)
	c.Response().Header().Set("Cache-Control", "public, max-age=300") // Cache for 5 minutes

	// Write the spec
	c.Response().WriteHeader(http.StatusOK)
	_, err = c.Response().Write(data)
	return err
}

// RegisterRoutes registers OpenAPI routes on the given router
func (h *Handler) RegisterRoutes(r router.Router) {
	r.GET("/api/openapi/openapi.yaml", h.ServeSpec)
	r.GET("/api/openapi/openapi.json", h.ServeSpec)
}
