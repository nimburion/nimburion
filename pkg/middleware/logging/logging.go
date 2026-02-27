// Package middleware provides HTTP middleware components for the framework.
package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/nimburion/nimburion/pkg/observability/logger"
	"github.com/nimburion/nimburion/pkg/server/router"
)

// Mode defines logging verbosity for matching request paths.
// Mode defines the logging verbosity level.
type Mode string

// Logging mode constants
const (
	// ModeOff disables request logging
	ModeOff Mode = "off"
	// ModeMinimal logs only essential request info
	ModeMinimal Mode = "minimal"
	// ModeFull logs complete request details
	ModeFull Mode = "full"
)

// Output defines where request logs are written.
type Output string

// Logging output constants
const (
	// OutputLogger writes to the configured logger
	OutputLogger Output = "logger"
	// OutputStdout writes to standard output
	OutputStdout Output = "stdout"
	// OutputStderr writes to standard error
	OutputStderr Output = "stderr"
)

// Log field name constants
const (
	// FieldRequestID is the request identifier field
	FieldRequestID = "request_id"
	// FieldMethod is the HTTP method field
	FieldMethod = "method"
	// FieldPath is the request path field
	FieldPath = "path"
	// FieldStatus is the HTTP status code field
	FieldStatus = "status"
	// FieldDurationMS is the request duration in milliseconds
	FieldDurationMS = "duration_ms"
	// FieldError is the error message field
	FieldError = "error"
	// FieldRemoteAddr is the client IP address
	FieldRemoteAddr = "remote_addr"
	// FieldRemotePort is the client port
	FieldRemotePort = "remote_port"
	// FieldRequestMethod is the HTTP request method
	FieldRequestMethod = "request_method"
	// FieldRequestURI is the full request URI
	FieldRequestURI = "request_uri"
	// FieldURI is the request URI
	FieldURI = "uri"
	// FieldArgs is the request arguments
	FieldArgs = "args"
	// FieldQueryString is the URL query string
	FieldQueryString = "query_string"
	FieldRequestTime   = "request_time"
	FieldTimeLocal     = "time_local"
	FieldHost          = "host"
	FieldServerProto   = "server_protocol"
	FieldScheme        = "scheme"
	FieldHTTPReferer   = "http_referer"
	FieldHTTPUserAgent = "http_user_agent"
	FieldXForwardedFor = "x_forwarded_for"
	FieldRemoteUser    = "remote_user"
	FieldRequestLength = "request_length"
)

var (
	defaultFields = []string{
		FieldRequestID,
		FieldMethod,
		FieldPath,
		FieldStatus,
		FieldDurationMS,
		FieldRemoteAddr,
		FieldError,
	}
	validFields = map[string]struct{}{
		FieldRequestID:     {},
		FieldMethod:        {},
		FieldPath:          {},
		FieldStatus:        {},
		FieldDurationMS:    {},
		FieldError:         {},
		FieldRemoteAddr:    {},
		FieldRemotePort:    {},
		FieldRequestMethod: {},
		FieldRequestURI:    {},
		FieldURI:           {},
		FieldArgs:          {},
		FieldQueryString:   {},
		FieldRequestTime:   {},
		FieldTimeLocal:     {},
		FieldHost:          {},
		FieldServerProto:   {},
		FieldScheme:        {},
		FieldHTTPReferer:   {},
		FieldHTTPUserAgent: {},
		FieldXForwardedFor: {},
		FieldRemoteUser:    {},
		FieldRequestLength: {},
	}
	fieldAliases = map[string]string{
		"referer":    FieldHTTPReferer,
		"user_agent": FieldHTTPUserAgent,
		"protocol":   FieldServerProto,
		"uri_path":   FieldURI,
		"query":      FieldQueryString,
	}
)

// Config configures request logging middleware behavior.
type Config struct {
	Enabled              bool
	LogStart             bool
	Output               Output
	Fields               []string
	ExcludedPathPrefixes []string
	PathPolicies         []PathPolicy
}

// PathPolicy configures a logging mode for a path prefix.
type PathPolicy struct {
	Prefix string
	Mode   Mode
}

// DefaultConfig returns default request logging behavior.
func DefaultConfig() Config {
	return Config{
		Enabled:              true,
		LogStart:             true,
		Output:               OutputLogger,
		Fields:               append([]string{}, defaultFields...),
		ExcludedPathPrefixes: []string{},
		PathPolicies:         []PathPolicy{},
	}
}

// Logging creates middleware with default configuration.
func Logging(log logger.Logger) router.MiddlewareFunc {
	return WithConfig(log, DefaultConfig())
}

// WithConfig creates middleware that logs HTTP requests and responses.
// WithConfig creates request logging middleware with custom configuration
func WithConfig(log logger.Logger, cfg Config) router.MiddlewareFunc {
	normalized := normalize(cfg)
	emitter := newEventEmitter(log, normalized.Output)

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			path := c.Request().URL.Path
			mode := normalized.modeForPath(path)
			if mode == ModeOff {
				return next(c)
			}

			start := time.Now()
			requestID := getRequestIDFromContext(c.Request().Context())
			req := c.Request()

			if normalized.LogStart && mode == ModeFull {
				emitter.Info("request started", normalized.buildFields(req, requestID, start, c.Response().Status(), 0, nil, true)...)
			}

			err := next(c)
			duration := time.Since(start)
			status := c.Response().Status()

			if err != nil {
				emitter.Error("request failed", normalized.buildFields(req, requestID, start, status, duration, err, false)...)
				return err
			}

			emitter.Info("request completed", normalized.buildFields(req, requestID, start, status, duration, nil, false)...)
			return nil
		}
	}
}

func normalize(cfg Config) Config {
	normalized := cfg
	if !normalized.Enabled {
		return normalized
	}

	for index := range normalized.PathPolicies {
		normalized.PathPolicies[index].Mode = parseMode(normalized.PathPolicies[index].Mode)
	}
	normalized.Output = parseOutput(normalized.Output)
	normalized.Fields = normalizeFields(normalized.Fields)
	return normalized
}

func (c Config) modeForPath(path string) Mode {
	if !c.Enabled {
		return ModeOff
	}

	for _, prefix := range c.ExcludedPathPrefixes {
		if strings.HasPrefix(path, prefix) {
			return ModeOff
		}
	}

	bestLen := -1
	bestMode := ModeFull
	for _, policy := range c.PathPolicies {
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

func parseOutput(output Output) Output {
	switch strings.ToLower(strings.TrimSpace(string(output))) {
	case string(OutputLogger):
		return OutputLogger
	case string(OutputStdout):
		return OutputStdout
	case string(OutputStderr):
		return OutputStderr
	default:
		return OutputLogger
	}
}

func normalizeFields(fields []string) []string {
	if len(fields) == 0 {
		return append([]string{}, defaultFields...)
	}

	normalized := make([]string, 0, len(fields))
	seen := make(map[string]struct{}, len(fields))
	for _, field := range fields {
		name := strings.ToLower(strings.TrimSpace(field))
		if alias, ok := fieldAliases[name]; ok {
			name = alias
		}
		if _, ok := validFields[name]; !ok {
			continue
		}
		if _, ok := seen[name]; ok {
			continue
		}
		seen[name] = struct{}{}
		normalized = append(normalized, name)
	}
	if len(normalized) == 0 {
		return append([]string{}, defaultFields...)
	}
	return normalized
}

func (c Config) buildFields(req *http.Request, requestID string, start time.Time, status int, duration time.Duration, err error, isStart bool) []any {
	args := make([]any, 0, len(c.Fields)*2)
	for _, field := range c.Fields {
		value, ok := resolveFieldValue(field, req, requestID, start, status, duration, err, isStart)
		if !ok {
			continue
		}
		args = append(args, field, value)
	}
	return args
}

func resolveFieldValue(field string, req *http.Request, requestID string, startedAt time.Time, status int, duration time.Duration, err error, isStart bool) (any, bool) {
	path := req.URL.Path
	query := req.URL.RawQuery
	switch field {
	case FieldRequestID:
		return requestID, true
	case FieldMethod:
		return req.Method, true
	case FieldPath, FieldURI:
		return path, true
	case FieldStatus:
		if isStart {
			return nil, false
		}
		return status, true
	case FieldDurationMS:
		if isStart {
			return nil, false
		}
		return duration.Milliseconds(), true
	case FieldError:
		if err == nil {
			return nil, false
		}
		return err, true
	case FieldRemoteAddr:
		return req.RemoteAddr, true
	case FieldRemotePort:
		_, port, splitErr := net.SplitHostPort(req.RemoteAddr)
		if splitErr != nil {
			return "", false
		}
		return port, true
	case FieldRequestMethod:
		return req.Method, true
	case FieldRequestURI:
		if query == "" {
			return path, true
		}
		return fmt.Sprintf("%s?%s", path, query), true
	case FieldArgs, FieldQueryString:
		return query, true
	case FieldRequestTime:
		if isStart {
			return nil, false
		}
		return float64(duration) / float64(time.Second), true
	case FieldTimeLocal:
		return startedAt.Format("02/Jan/2006:15:04:05 -0700"), true
	case FieldHost:
		return req.Host, true
	case FieldServerProto:
		return req.Proto, true
	case FieldScheme:
		if req.TLS != nil {
			return "https", true
		}
		if xfProto := strings.TrimSpace(req.Header.Get("X-Forwarded-Proto")); xfProto != "" {
			return xfProto, true
		}
		return "http", true
	case FieldHTTPReferer:
		return req.Referer(), true
	case FieldHTTPUserAgent:
		return req.UserAgent(), true
	case FieldXForwardedFor:
		return req.Header.Get("X-Forwarded-For"), true
	case FieldRemoteUser:
		if req.URL.User == nil {
			return "", true
		}
		return req.URL.User.Username(), true
	case FieldRequestLength:
		if req.ContentLength < 0 {
			return 0, true
		}
		return req.ContentLength, true
	default:
		return nil, false
	}
}

type eventEmitter struct {
	logger logger.Logger
	output Output
	writer io.Writer
	mu     sync.Mutex
}

func newEventEmitter(log logger.Logger, output Output) *eventEmitter {
	switch output {
	case OutputStdout:
		return &eventEmitter{logger: log, output: output, writer: os.Stdout}
	case OutputStderr:
		return &eventEmitter{logger: log, output: output, writer: os.Stderr}
	default:
		return &eventEmitter{logger: log, output: OutputLogger}
	}
}

// Info emits an info-level log event.
func (e *eventEmitter) Info(msg string, args ...any) {
	e.emit("info", msg, args...)
}

// Error emits an error-level log event.
func (e *eventEmitter) Error(msg string, args ...any) {
	e.emit("error", msg, args...)
}

func (e *eventEmitter) emit(level, msg string, args ...any) {
	if e.output == OutputLogger {
		if level == "error" {
			e.logger.Error(msg, args...)
			return
		}
		e.logger.Info(msg, args...)
		return
	}

	entry := map[string]any{
		"level":     level,
		"timestamp": time.Now().Format(time.RFC3339Nano),
		"message":   msg,
	}
	for key, value := range argsToMap(args) {
		entry[key] = value
	}

	payload, err := json.Marshal(entry)
	if err != nil {
		payload = []byte(fmt.Sprintf(`{"level":"error","message":"request logging marshal failed","error":%q}`, err.Error()))
	}

	e.mu.Lock()
	defer e.mu.Unlock()
	_, _ = e.writer.Write(append(payload, '\n'))
}

func argsToMap(args []any) map[string]any {
	fields := make(map[string]any)
	for index := 0; index < len(args)-1; index += 2 {
		key, ok := args[index].(string)
		if !ok {
			continue
		}
		fields[key] = args[index+1]
	}
	return fields
}

// getRequestIDFromContext extracts the request ID from the context.
// This function looks for the request ID under the "request_id" key.
func getRequestIDFromContext(ctx interface{}) string {
	if ctx == nil {
		return ""
	}

	if c, ok := ctx.(interface{ Value(interface{}) interface{} }); ok {
		if requestID, ok := c.Value("request_id").(string); ok {
			return requestID
		}
	}

	return ""
}
