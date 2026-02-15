package openapi

import (
	"html/template"
	"net/http"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// SwaggerHandler serves Swagger UI
type SwaggerHandler struct {
	enabled  bool
	specPath string // URL path to the OpenAPI spec
}

// NewSwaggerHandler creates a new Swagger UI handler
func NewSwaggerHandler(enabled bool, specPath string) *SwaggerHandler {
	return &SwaggerHandler{
		enabled:  enabled,
		specPath: specPath,
	}
}

// ServeSwaggerUI serves the Swagger UI HTML page
func (h *SwaggerHandler) ServeSwaggerUI(c router.Context) error {
	if !h.enabled {
		return c.JSON(http.StatusNotFound, map[string]interface{}{
			"error": "Swagger UI is disabled",
		})
	}

	// Render Swagger UI HTML
	tmpl := template.Must(template.New("swagger").Parse(swaggerUITemplate))

	c.Response().Header().Set("Content-Type", "text/html; charset=utf-8")
	c.Response().WriteHeader(http.StatusOK)

	return tmpl.Execute(c.Response(), map[string]interface{}{
		"SpecURL": h.specPath,
	})
}

// RegisterRoutes registers Swagger UI routes on the given router
func (h *SwaggerHandler) RegisterRoutes(r router.Router) {
	if h.enabled {
		r.GET("/swagger", h.ServeSwaggerUI)
		r.GET("/swagger/", h.ServeSwaggerUI)
	}
}

// swaggerUITemplate is the HTML template for Swagger UI
// Uses Swagger UI from CDN for simplicity
const swaggerUITemplate = `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>API Documentation - Swagger UI</title>
    <link rel="stylesheet" type="text/css" href="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui.css">
    <style>
        html {
            box-sizing: border-box;
            overflow: -moz-scrollbars-vertical;
            overflow-y: scroll;
        }
        *, *:before, *:after {
            box-sizing: inherit;
        }
        body {
            margin: 0;
            padding: 0;
        }
    </style>
</head>
<body>
    <div id="swagger-ui"></div>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-bundle.js"></script>
    <script src="https://unpkg.com/swagger-ui-dist@5.10.0/swagger-ui-standalone-preset.js"></script>
    <script>
        window.onload = function() {
            window.ui = SwaggerUIBundle({
                url: "{{.SpecURL}}",
                dom_id: '#swagger-ui',
                deepLinking: true,
                presets: [
                    SwaggerUIBundle.presets.apis,
                    SwaggerUIStandalonePreset
                ],
                plugins: [
                    SwaggerUIBundle.plugins.DownloadUrl
                ],
                layout: "StandaloneLayout"
            });
        };
    </script>
</body>
</html>
`
