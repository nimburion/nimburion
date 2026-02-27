package tracing

import (
	"context"
	"fmt"
	"strings"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/trace"

	"github.com/nimburion/nimburion/pkg/middleware/requestid"
	"github.com/nimburion/nimburion/pkg/server/router"
)

// Config holds configuration for the tracing middleware.
type Config struct {
	// TracerName identifies the tracer (e.g., "http-server")
	TracerName string

	// SpanNameFormatter formats the span name from the request
	// If nil, defaults to "HTTP {method} {path}"
	SpanNameFormatter func(router.Context) string

	// ExcludedPathPrefixes disables tracing for matching path prefixes.
	ExcludedPathPrefixes []string

	// PathPolicies applies mode by best-matching path prefix.
	PathPolicies []PathPolicy
}

// Mode defines tracing verbosity for matching request paths.
type Mode string

// Tracing mode constants
const (
	// ModeOff disables tracing
	ModeOff Mode = "off"
	// ModeMinimal creates minimal span with basic attributes
	ModeMinimal Mode = "minimal"
	// ModeFull creates detailed span with all attributes
	ModeFull Mode = "full"
)

// PathPolicy configures tracing mode for a path prefix.
type PathPolicy struct {
	Prefix string
	Mode   Mode
}

// Tracing creates middleware that adds OpenTelemetry distributed tracing to HTTP requests.
// It creates a span for each request, propagates trace context from incoming headers,
// and includes request ID and HTTP attributes in the span.
//
// Requirements: 14.2, 14.3, 14.6
func Tracing(cfg Config) router.MiddlewareFunc {
	if cfg.TracerName == "" {
		cfg.TracerName = "http-server"
	}

	if cfg.SpanNameFormatter == nil {
		cfg.SpanNameFormatter = defaultSpanNameFormatter
	}
	cfg = normalize(cfg)

	tracer := otel.Tracer(cfg.TracerName)
	propagator := otel.GetTextMapPropagator()

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			req := c.Request()
			mode := cfg.modeForPath(req.URL.Path)
			if mode == ModeOff {
				return next(c)
			}

			// Extract trace context from incoming headers
			ctx := propagator.Extract(req.Context(), propagation.HeaderCarrier(req.Header))

			// Create span for this request
			spanName := cfg.SpanNameFormatter(c)
			ctx, span := tracer.Start(ctx, spanName, trace.WithSpanKind(trace.SpanKindServer))
			defer span.End()

			// Add HTTP attributes to span
			if mode == ModeMinimal {
				span.SetAttributes(
					attribute.String("http.method", req.Method),
					attribute.String("http.target", req.URL.Path),
				)
			} else {
				span.SetAttributes(
					attribute.String("http.method", req.Method),
					attribute.String("http.url", req.URL.String()),
					attribute.String("http.scheme", req.URL.Scheme),
					attribute.String("http.host", req.Host),
					attribute.String("http.target", req.URL.Path),
					attribute.String("http.user_agent", req.UserAgent()),
					attribute.String("http.remote_addr", req.RemoteAddr),
				)
			}

			// Add request ID to span if available
			if requestID := requestid.GetRequestID(req.Context()); requestID != "" {
				span.SetAttributes(attribute.String("request.id", requestID))
			}

			// Update request context with span context
			c.SetRequest(req.WithContext(ctx))

			// Call next handler
			err := next(c)

			// Record error if present (do this first to ensure error status takes precedence)
			if err != nil {
				span.RecordError(err)
				span.SetStatus(codes.Error, err.Error())
			} else {
				// Only set status based on HTTP status code if no error occurred
				status := c.Response().Status()
				span.SetAttributes(attribute.Int("http.status_code", status))

				// Set span status based on HTTP status code
				if status >= 500 {
					span.SetStatus(codes.Error, fmt.Sprintf("HTTP %d", status))
				} else {
					span.SetStatus(codes.Ok, "")
				}
			}

			return err
		}
	}
}

func normalize(cfg Config) Config {
	for index := range cfg.PathPolicies {
		cfg.PathPolicies[index].Mode = parseMode(cfg.PathPolicies[index].Mode)
	}
	return cfg
}

func (cfg Config) modeForPath(path string) Mode {
	for _, prefix := range cfg.ExcludedPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return ModeOff
		}
	}

	bestLen := -1
	bestMode := ModeFull
	for _, policy := range cfg.PathPolicies {
		if strings.TrimSpace(policy.Prefix) == "" {
			continue
		}
		if strings.HasPrefix(path, policy.Prefix) && len(policy.Prefix) > bestLen {
			bestLen = len(policy.Prefix)
			bestMode = policy.Mode
		}
	}
	return bestMode
}

func parseMode(mode Mode) Mode {
	switch strings.ToLower(strings.TrimSpace(string(mode))) {
	case string(ModeOff):
		return ModeOff
	case string(ModeMinimal):
		return ModeMinimal
	case string(ModeFull):
		return ModeFull
	default:
		return ModeFull
	}
}

// defaultSpanNameFormatter creates a span name from HTTP method and path.
func defaultSpanNameFormatter(c router.Context) string {
	return fmt.Sprintf("HTTP %s %s", c.Request().Method, c.Request().URL.Path)
}

// PropagateTraceContext injects trace context into outgoing HTTP request headers.
// This should be used when making HTTP calls to other services to propagate the trace.
//
// Requirements: 14.2
func PropagateTraceContext(ctx context.Context, headers map[string]string) {
	propagator := otel.GetTextMapPropagator()
	carrier := propagation.MapCarrier(headers)
	propagator.Inject(ctx, carrier)
}
