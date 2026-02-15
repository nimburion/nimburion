// Package middleware provides HTTP middleware components for the framework.
package recovery

import (
	"net/http"
	"runtime/debug"

	"github.com/nimburion/nimburion/pkg/middleware/requestid"
	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/server/router"
)

// Recovery creates middleware that recovers from panics in HTTP handlers.
// It catches panics with defer/recover, logs the panic with stack trace,
// and returns HTTP 500 with an error response.
//
// Requirements: 3.3, 3.7
func Recovery(log logger.Logger) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			defer func() {
				if r := recover(); r != nil {
					// Extract request ID from context for correlation
					requestID := requestid.GetRequestID(c.Request().Context())
					
					// Get stack trace
					stackTrace := string(debug.Stack())
					
					// Log the panic with full context
					log.Error("panic recovered",
						"request_id", requestID,
						"panic", r,
						"stack", stackTrace,
					)
					
					// Return HTTP 500 with error response
					// Only write response if not already written
					if !c.Response().Written() {
						errorResponse := map[string]interface{}{
							"error":      "internal_server_error",
							"message":    "an unexpected error occurred",
							"request_id": requestID,
						}
						
						// Attempt to send JSON response
						if err := c.JSON(http.StatusInternalServerError, errorResponse); err != nil {
							// If JSON encoding fails, fall back to plain text
							log.Error("failed to send error response",
								"request_id", requestID,
								"error", err,
							)
						}
					}
				}
			}()
			
			return next(c)
		}
	}
}
