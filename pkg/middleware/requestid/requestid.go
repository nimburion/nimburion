package requestid

import (
	"context"

	"github.com/google/uuid"
	"github.com/nimburion/nimburion/pkg/middleware"
	"github.com/nimburion/nimburion/pkg/server/router"
)

// RequestIDHeader is the HTTP header name for request ID.
const RequestIDHeader = "X-Request-ID"

// RequestID creates middleware that generates or extracts request IDs.
// It generates a UUID for requests without X-Request-ID header,
// preserves existing X-Request-ID from header,
// and adds request ID to response headers and context.
//
// Requirements: 3.1, 15.1, 15.2, 15.3, 15.4
func RequestID() router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			// Extract request ID from header if present
			requestID := c.Request().Header.Get(RequestIDHeader)
			
			// Generate new UUID if no request ID provided
			if requestID == "" {
				requestID = generateRequestID()
			}
			
			// Store request ID in context for use by other middleware and handlers
			c.Set(string(middleware.RequestIDKey), requestID)
			
			// Add request ID to response headers
			c.Response().Header().Set(RequestIDHeader, requestID)
			
			// Add request ID to request context for propagation
			ctx := context.WithValue(c.Request().Context(), middleware.RequestIDKey, requestID)
			c.SetRequest(c.Request().WithContext(ctx))
			
			return next(c)
		}
	}
}

// generateRequestID generates a new UUID for request identification.
func generateRequestID() string {
	return uuid.New().String()
}

// GetRequestID extracts the request ID from a context.
// Returns empty string if no request ID is found.
func GetRequestID(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	
	if requestID, ok := ctx.Value(middleware.RequestIDKey).(string); ok {
		return requestID
	}
	
	return ""
}
