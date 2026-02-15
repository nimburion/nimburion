# Router Package

The router package provides an abstraction layer for HTTP routing in the Go microservices framework. It defines interfaces that allow pluggable router implementations (net/http, gin-gonic, gorilla/mux).

## Design Principles

- **Interface-driven**: All routing functionality is defined through interfaces
- **Pluggable**: Support for multiple router implementations
- **Middleware support**: First-class support for middleware composition
- **Type-safe**: Strong typing for handlers and middleware

## Core Interfaces

### Router

The `Router` interface defines the contract for HTTP routing:

```go
type Router interface {
    // HTTP method handlers
    GET(path string, handler HandlerFunc, middleware ...MiddlewareFunc)
    POST(path string, handler HandlerFunc, middleware ...MiddlewareFunc)
    PUT(path string, handler HandlerFunc, middleware ...MiddlewareFunc)
    DELETE(path string, handler HandlerFunc, middleware ...MiddlewareFunc)
    PATCH(path string, handler HandlerFunc, middleware ...MiddlewareFunc)

    // Group creates a route group with common prefix and middleware
    Group(prefix string, middleware ...MiddlewareFunc) Router

    // Use applies middleware to all routes
    Use(middleware ...MiddlewareFunc)

    // ServeHTTP implements http.Handler
    ServeHTTP(w http.ResponseWriter, r *http.Request)
}
```

### Context

The `Context` interface provides access to request and response in a router-agnostic way:

```go
type Context interface {
    Request() *http.Request
    SetRequest(r *http.Request)
    Response() ResponseWriter
    Param(name string) string
    Query(name string) string
    Bind(v interface{}) error
    JSON(code int, v interface{}) error
    String(code int, s string) error
    Get(key string) interface{}
    Set(key string, value interface{})
}
```

### HandlerFunc and MiddlewareFunc

```go
type HandlerFunc func(Context) error
type MiddlewareFunc func(HandlerFunc) HandlerFunc
```

## Usage Example

```go
package main

import (
    "net/http"
    "github.com/nimburion/nimburion/pkg/server/router/nethttp"
)

func main() {
    // Create router
    r := nethttp.NewRouter()

    // Add global middleware
    r.Use(loggingMiddleware, authMiddleware)

    // Register routes
    r.GET("/users", listUsers)
    r.GET("/users/:id", getUser)
    r.POST("/users", createUser)

    // Create route groups
    api := r.Group("/api/v1")
    api.GET("/posts", listPosts)
    api.POST("/posts", createPost)

    // Start server
    http.ListenAndServe(":8080", r)
}

func listUsers(c router.Context) error {
    users := []string{"Alice", "Bob"}
    return c.JSON(http.StatusOK, users)
}

func getUser(c router.Context) error {
    id := c.Param("id")
    return c.JSON(http.StatusOK, map[string]string{"id": id})
}

func createUser(c router.Context) error {
    var user struct {
        Name string `json:"name"`
    }
    if err := c.Bind(&user); err != nil {
        return err
    }
    return c.JSON(http.StatusCreated, user)
}

func loggingMiddleware(next router.HandlerFunc) router.HandlerFunc {
    return func(c router.Context) error {
        // Log before
        err := next(c)
        // Log after
        return err
    }
}
```

## Available Implementations

### net/http Adapter

The `nethttp` package provides a net/http-based implementation:

```go
import "github.com/nimburion/nimburion/pkg/server/router/nethttp"

r := nethttp.NewRouter()
```

Features:
- Simple pattern matching for path parameters (`:param`)
- Query parameter extraction
- JSON request/response binding
- Middleware composition
- Route grouping with prefixes
- Thread-safe

### Gin Adapter

The `gin` package provides a gin-gonic based implementation:

```go
import "github.com/nimburion/nimburion/pkg/server/router/gin"

r := gin.NewRouter()
```

### Gorilla Adapter

The `gorilla` package provides a gorilla/mux based implementation:

```go
import "github.com/nimburion/nimburion/pkg/server/router/gorilla"

r := gorilla.NewRouter()
```

### Factory

Use the factory to select a router at runtime:

```go
import "github.com/nimburion/nimburion/pkg/server/router/factory"

r, err := factory.NewRouter("gin") // nethttp | gin | gorilla
```

## Selection Guide

- `nethttp`: minimum dependencies, predictable behavior.
- `gin`: higher throughput and rich ecosystem middleware.
- `gorilla`: mature matcher/subrouter model with low migration friction.

All adapters are validated through the shared router contract test suite.

## Dependency Compatibility

- `github.com/gin-gonic/gin` v1.11.x
- `github.com/gorilla/mux` v1.8.x

The default router remains `nethttp`; `gin` and `gorilla` are optional runtime selections through configuration.

## Testing

The router package includes comprehensive tests:

- **Unit tests**: Test basic routing, middleware, parameters, and error handling
- **Property-based tests**: Verify router interface compatibility across all implementations

Run tests:

```bash
go test ./pkg/server/router/...
```

## Requirements

Validates Requirement 2.1: The Framework SHALL support pluggable HTTP routers (net/http, gin-gonic, gorilla/mux)
