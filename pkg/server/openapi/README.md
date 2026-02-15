# OpenAPI and Swagger UI

This package provides handlers for serving OpenAPI specifications and Swagger UI.

## Features

- Serve OpenAPI specification files (YAML and JSON)
- Optional Swagger UI integration
- Disabled by default for security
- Only available on management server

## Usage

### Serving OpenAPI Specification

```go
import (
    "github.com/nimburion/nimburion/pkg/server/openapi"
    "github.com/nimburion/nimburion/pkg/server/router"
)

// Create OpenAPI handler
openapiHandler := openapi.NewHandler("./api/openapi.yaml")

// Register routes on management server
openapiHandler.RegisterRoutes(managementRouter)
```

The specification will be available at:
- `/api/openapi/openapi.yaml`
- `/api/openapi/openapi.json`

### Enabling Swagger UI

Swagger UI is disabled by default. To enable it:

1. Set environment variable: `APP_SWAGGER_ENABLED=true`
2. Or in config file:
```yaml
swagger:
  enabled: true
  spec_path: "/api/openapi/openapi.yaml"
```

3. Register Swagger UI routes:
```go
// Create Swagger handler
swaggerHandler := openapi.NewSwaggerHandler(config.Swagger.Enabled, config.Swagger.SpecPath)

// Register routes on management server ONLY
swaggerHandler.RegisterRoutes(managementRouter)
```

Swagger UI will be available at:
- `/swagger`
- `/swagger/`

## Security Considerations

- Swagger UI is disabled by default (`APP_SWAGGER_ENABLED=false`)
- Swagger UI should ONLY be served on the management server, never on the public API server
- Consider restricting management server access in production environments
- Use authentication/authorization for management endpoints when exposed beyond localhost

## Configuration

| Environment Variable | Config Key | Default | Description |
|---------------------|------------|---------|-------------|
| `APP_SWAGGER_ENABLED` | `swagger.enabled` | `false` | Enable/disable Swagger UI |
| `APP_SWAGGER_SPEC_PATH` | `swagger.spec_path` | `/api/openapi/openapi.yaml` | URL path to OpenAPI spec |

## Example

See `examples/todo-service` for a complete example of OpenAPI and Swagger UI integration.
