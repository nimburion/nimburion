package requestsize

import (
	"errors"
	"fmt"
	"net/http"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// Middleware enforces a maximum request body size in bytes.
// A non-positive maxBytes disables the middleware.
func Middleware(maxBytes int64) router.MiddlewareFunc {
	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			if maxBytes <= 0 {
				return next(c)
			}

			req := c.Request()
			if req == nil || req.Body == nil {
				return next(c)
			}

			// Fail fast when Content-Length is declared and exceeds the limit.
			if req.ContentLength > maxBytes {
				return payloadTooLarge(c, maxBytes)
			}

			req.Body = http.MaxBytesReader(c.Response(), req.Body, maxBytes)
			c.SetRequest(req)

			err := next(c)
			if err == nil {
				return nil
			}

			var maxBytesErr *http.MaxBytesError
			if errors.As(err, &maxBytesErr) && !c.Response().Written() {
				return payloadTooLarge(c, maxBytes)
			}

			return err
		}
	}
}

func payloadTooLarge(c router.Context, maxBytes int64) error {
	return c.JSON(http.StatusRequestEntityTooLarge, map[string]interface{}{
		"error":    "request_too_large",
		"message":  fmt.Sprintf("request body exceeds maximum allowed size of %d bytes", maxBytes),
		"max_size": maxBytes,
	})
}
