package compression

import (
	"bufio"
	"bytes"
	"compress/gzip"
	"fmt"
	"io"
	"mime"
	"net"
	"net/http"
	"path"
	"regexp"
	"strconv"
	"strings"

	"github.com/andybalholm/brotli"
	"github.com/nimburion/nimburion/pkg/server/router"
)

const (
	encodingBrotli = "br"
	encodingGzip   = "gzip"
)

// Config controls response compression behavior.
type Config struct {
	Enabled                  bool
	EnableGzip               bool
	EnableBrotli             bool
	GzipLevel                int
	BrotliLevel              int
	MinSize                  int
	CompressibleContentTypes []string
	ExcludedPathPrefixes     []string
	ExcludedPathRegex        []string
	ExcludedExtensions       []string
	ExcludedMIMETypes        []string
	ExcludeServerPush        bool
}

// DefaultConfig returns a sane default for HTTP response compression.
func DefaultConfig() Config {
	return Config{
		Enabled:      true,
		EnableGzip:   true,
		EnableBrotli: true,
		GzipLevel:    gzip.DefaultCompression,
		BrotliLevel:  4,
		MinSize:      0,
		CompressibleContentTypes: []string{
			"text/",
			"application/json",
			"application/javascript",
			"application/xml",
			"image/svg+xml",
		},
		ExcludedPathRegex:  []string{},
		ExcludedExtensions: []string{},
		ExcludedMIMETypes:  []string{},
		ExcludeServerPush:  false,
	}
}

// Middleware compresses HTTP responses using Brotli and Gzip based on Accept-Encoding negotiation.
func Middleware(cfg Config) router.MiddlewareFunc {
	cfg = normalizeConfig(cfg)
	excludeRegex := compileRegexes(cfg.ExcludedPathRegex)

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			if !cfg.Enabled || c.Request() == nil {
				return next(c)
			}
			if c.Request().Method == http.MethodHead {
				return next(c)
			}
			if isExcludedPath(c.Request().URL.Path, cfg.ExcludedPathPrefixes, excludeRegex) {
				return next(c)
			}
			if cfg.ExcludeServerPush && supportsServerPush(c.Response()) {
				return next(c)
			}
			if hasExcludedExtension(c.Request().URL.Path, cfg.ExcludedExtensions) {
				return next(c)
			}
			if hasExcludedMIMEFromExtension(c.Request().URL.Path, cfg.ExcludedMIMETypes) {
				return next(c)
			}

			encoding := negotiateEncoding(c.Request().Header.Get("Accept-Encoding"), cfg)
			if encoding == "" {
				return next(c)
			}

			appendVary(c.Response().Header(), "Accept-Encoding")

			wrapped := newCompressResponseWriter(c.Response(), encoding, cfg)
			c.SetResponse(wrapped)
			defer wrapped.Close()

			return next(c)
		}
	}
}

func normalizeConfig(cfg Config) Config {
	def := DefaultConfig()
	if !cfg.Enabled &&
		!cfg.EnableGzip &&
		!cfg.EnableBrotli &&
		cfg.GzipLevel == 0 &&
		cfg.BrotliLevel == 0 &&
		cfg.MinSize == 0 &&
		len(cfg.CompressibleContentTypes) == 0 &&
		len(cfg.ExcludedPathPrefixes) == 0 &&
		len(cfg.ExcludedPathRegex) == 0 &&
		len(cfg.ExcludedExtensions) == 0 &&
		len(cfg.ExcludedMIMETypes) == 0 &&
		!cfg.ExcludeServerPush {
		return def
	}
	if cfg.GzipLevel == 0 {
		cfg.GzipLevel = def.GzipLevel
	}
	if cfg.BrotliLevel <= 0 {
		cfg.BrotliLevel = def.BrotliLevel
	}
	if cfg.MinSize < 0 {
		cfg.MinSize = 0
	}
	if len(cfg.CompressibleContentTypes) == 0 {
		cfg.CompressibleContentTypes = def.CompressibleContentTypes
	}
	return cfg
}

func compileRegexes(patterns []string) []*regexp.Regexp {
	compiled := make([]*regexp.Regexp, 0, len(patterns))
	for _, pattern := range patterns {
		pattern = strings.TrimSpace(pattern)
		if pattern == "" {
			continue
		}
		re, err := regexp.Compile(pattern)
		if err != nil {
			continue
		}
		compiled = append(compiled, re)
	}
	return compiled
}

func isExcludedPath(path string, prefixes []string, regex []*regexp.Regexp) bool {
	for _, prefix := range prefixes {
		if strings.TrimSpace(prefix) != "" && strings.HasPrefix(path, prefix) {
			return true
		}
	}
	for _, re := range regex {
		if re.MatchString(path) {
			return true
		}
	}
	return false
}

func normalizeExt(ext string) string {
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return ""
	}
	if !strings.HasPrefix(ext, ".") {
		return "." + ext
	}
	return ext
}

func hasExcludedExtension(requestPath string, excluded []string) bool {
	ext := normalizeExt(path.Ext(requestPath))
	if ext == "" {
		return false
	}
	for _, candidate := range excluded {
		if ext == normalizeExt(candidate) {
			return true
		}
	}
	return false
}

func hasExcludedMIMEFromExtension(requestPath string, excludedMIME []string) bool {
	ext := normalizeExt(path.Ext(requestPath))
	if ext == "" {
		return false
	}
	detected := strings.ToLower(strings.TrimSpace(mime.TypeByExtension(ext)))
	if detected == "" {
		return false
	}
	return matchesMIME(detected, excludedMIME)
}

func supportsServerPush(w router.ResponseWriter) bool {
	_, ok := w.(http.Pusher)
	return ok
}

func negotiateEncoding(acceptEncoding string, cfg Config) string {
	if acceptEncoding == "" {
		return ""
	}

	qBr, hasBr := qualityForEncoding(acceptEncoding, encodingBrotli)
	qGzip, hasGzip := qualityForEncoding(acceptEncoding, encodingGzip)
	qAny, hasAny := qualityForEncoding(acceptEncoding, "*")

	if cfg.EnableBrotli && !hasBr && hasAny {
		qBr = qAny
		hasBr = true
	}
	if cfg.EnableGzip && !hasGzip && hasAny {
		qGzip = qAny
		hasGzip = true
	}

	best := ""
	bestQ := float64(0)

	if cfg.EnableBrotli && hasBr && qBr > 0 {
		best = encodingBrotli
		bestQ = qBr
	}
	if cfg.EnableGzip && hasGzip && qGzip > 0 {
		if qGzip > bestQ {
			best = encodingGzip
		}
	}

	return best
}

func qualityForEncoding(acceptEncoding, encoding string) (float64, bool) {
	parts := strings.Split(acceptEncoding, ",")
	for _, part := range parts {
		token := strings.TrimSpace(part)
		if token == "" {
			continue
		}

		sections := strings.Split(token, ";")
		name := strings.ToLower(strings.TrimSpace(sections[0]))
		if name != strings.ToLower(encoding) {
			continue
		}

		q := 1.0
		if len(sections) > 1 {
			for _, section := range sections[1:] {
				kv := strings.SplitN(strings.TrimSpace(section), "=", 2)
				if len(kv) != 2 || strings.ToLower(kv[0]) != "q" {
					continue
				}
				if parsed, err := strconv.ParseFloat(kv[1], 64); err == nil {
					q = parsed
				}
			}
		}
		return q, true
	}
	return 0, false
}

type compressResponseWriter struct {
	base             router.ResponseWriter
	encoding         string
	cfg              Config
	statusCode       int
	headerWritten    bool
	decided          bool
	compress         bool
	gzipWriter       *gzip.Writer
	brotliWriter     *brotli.Writer
	compressedWriter io.Writer
	buffer           bytes.Buffer
}

func newCompressResponseWriter(base router.ResponseWriter, encoding string, cfg Config) *compressResponseWriter {
	return &compressResponseWriter{
		base:     base,
		encoding: encoding,
		cfg:      cfg,
	}
}

// Header returns the HTTP response headers that can be modified before writing the response.
func (w *compressResponseWriter) Header() http.Header {
	return w.base.Header()
}

// WriteHeader sends an HTTP response header with the provided status code.
func (w *compressResponseWriter) WriteHeader(code int) {
	if w.headerWritten {
		return
	}
	w.statusCode = code
	w.headerWritten = true
	if noBodyStatus(code) {
		w.decided = true
		w.compress = false
		w.base.WriteHeader(code)
	}
}

// Write writes data to the response body. Implements io.Writer interface.
func (w *compressResponseWriter) Write(p []byte) (int, error) {
	if !w.headerWritten {
		w.WriteHeader(http.StatusOK)
	}

	if w.decided {
		if w.compress {
			n, err := w.compressedWriter.Write(p)
			if err != nil {
				return n, err
			}
			return len(p), nil
		}
		_, err := w.base.Write(p)
		if err != nil {
			return 0, err
		}
		return len(p), nil
	}

	_, _ = w.buffer.Write(p)
	if w.buffer.Len() < w.cfg.MinSize {
		return len(p), nil
	}

	if err := w.decideAndFlushBuffer(); err != nil {
		return 0, err
	}
	return len(p), nil
}

func (w *compressResponseWriter) decideAndFlushBuffer() error {
	if w.decided {
		return nil
	}

	w.decided = true
	if noBodyStatus(w.statusCode) {
		w.compress = false
		if !w.base.Written() {
			w.base.WriteHeader(w.statusOrOK())
		}
		return nil
	}
	if existing := strings.TrimSpace(w.Header().Get("Content-Encoding")); existing != "" {
		w.compress = false
		return w.flushPlainBuffer()
	}
	if w.buffer.Len() < w.cfg.MinSize {
		w.compress = false
		return w.flushPlainBuffer()
	}
	contentType := strings.ToLower(strings.TrimSpace(w.Header().Get("Content-Type")))
	if matchesMIME(contentType, w.cfg.ExcludedMIMETypes) {
		w.compress = false
		return w.flushPlainBuffer()
	}
	if !isCompressibleContentType(contentType, w.cfg.CompressibleContentTypes) {
		w.compress = false
		return w.flushPlainBuffer()
	}

	w.compress = true
	w.Header().Del("Content-Length")
	w.Header().Set("Content-Encoding", w.encoding)
	appendVary(w.Header(), "Accept-Encoding")
	if !w.base.Written() {
		w.base.WriteHeader(w.statusOrOK())
	}

	switch w.encoding {
	case encodingBrotli:
		w.brotliWriter = brotli.NewWriterLevel(w.base, w.cfg.BrotliLevel)
		w.compressedWriter = w.brotliWriter
	case encodingGzip:
		gzWriter, err := gzip.NewWriterLevel(w.base, w.cfg.GzipLevel)
		if err != nil {
			return fmt.Errorf("create gzip writer: %w", err)
		}
		w.gzipWriter = gzWriter
		w.compressedWriter = gzWriter
	default:
		w.compress = false
		return w.flushPlainBuffer()
	}

	if w.buffer.Len() == 0 {
		return nil
	}
	if _, err := w.compressedWriter.Write(w.buffer.Bytes()); err != nil {
		return err
	}
	w.buffer.Reset()
	return nil
}

func (w *compressResponseWriter) flushPlainBuffer() error {
	if !w.base.Written() {
		w.base.WriteHeader(w.statusOrOK())
	}
	if w.buffer.Len() == 0 {
		return nil
	}
	if _, err := w.base.Write(w.buffer.Bytes()); err != nil {
		return err
	}
	w.buffer.Reset()
	return nil
}

func matchesMIME(contentType string, excluded []string) bool {
	if contentType == "" || len(excluded) == 0 {
		return false
	}

	baseType := strings.ToLower(strings.TrimSpace(strings.Split(contentType, ";")[0]))
	for _, rule := range excluded {
		rule = strings.ToLower(strings.TrimSpace(rule))
		if rule == "" {
			continue
		}
		if strings.HasSuffix(rule, "/*") {
			prefix := strings.TrimSuffix(rule, "*")
			if strings.HasPrefix(baseType, prefix) {
				return true
			}
			continue
		}
		if baseType == rule {
			return true
		}
	}
	return false
}

// Close releases all resources held by this instance. Should be called when the instance is no longer needed.
func (w *compressResponseWriter) Close() error {
	if !w.decided {
		if err := w.decideAndFlushBuffer(); err != nil {
			return err
		}
	}
	if !w.compress {
		if w.headerWritten && !w.base.Written() {
			w.base.WriteHeader(w.statusOrOK())
		}
		return nil
	}

	if w.gzipWriter != nil {
		return w.gzipWriter.Close()
	}
	if w.brotliWriter != nil {
		return w.brotliWriter.Close()
	}
	return nil
}

func (w *compressResponseWriter) statusOrOK() int {
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

// Status returns the HTTP status code that was written, or 0 if not yet written.
func (w *compressResponseWriter) Status() int {
	if w.base.Written() {
		return w.base.Status()
	}
	if w.statusCode == 0 {
		return http.StatusOK
	}
	return w.statusCode
}

// Written returns true if the response headers and body have been written.
func (w *compressResponseWriter) Written() bool {
	return w.base.Written() || w.headerWritten
}

// Flush sends any buffered data to the client immediately.
func (w *compressResponseWriter) Flush() {
	if flusher, ok := w.compressedWriter.(interface{ Flush() error }); ok {
		_ = flusher.Flush()
	}
	if f, ok := w.base.(http.Flusher); ok {
		f.Flush()
	}
}

// Hijack takes over the underlying connection for custom protocols like WebSocket.
func (w *compressResponseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hijacker, ok := w.base.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hijacker.Hijack()
}

func noBodyStatus(statusCode int) bool {
	return statusCode == http.StatusNoContent || statusCode == http.StatusNotModified || (statusCode >= 100 && statusCode < 200)
}

func isCompressibleContentType(contentType string, allow []string) bool {
	ct := strings.ToLower(strings.TrimSpace(contentType))
	if ct == "" {
		// For handlers that do not set Content-Type explicitly, compress by default.
		return true
	}
	for _, prefix := range allow {
		if strings.HasPrefix(ct, strings.ToLower(strings.TrimSpace(prefix))) {
			return true
		}
	}
	return false
}

func appendVary(header http.Header, value string) {
	current := header.Get("Vary")
	if current == "" {
		header.Set("Vary", value)
		return
	}
	for _, part := range strings.Split(current, ",") {
		if strings.EqualFold(strings.TrimSpace(part), value) {
			return
		}
	}
	header.Set("Vary", current+", "+value)
}
