package static

import (
	"net/http"
	"path"
	"strings"

	"github.com/nimburion/nimburion/pkg/server/router"
)

// ServeFileSystem extends http.FileSystem with an Exists helper that
// understands the URL prefix that static middleware uses before delegating
// to the underlying filesystem.
type ServeFileSystem interface {
	http.FileSystem
	Exists(prefix, requestPath string) bool
}

// ServeRoot is a convenience wrapper around Serve that uses LocalFile with
// directory listing disabled (same as gin-contrib/static ServeRoot).
func ServeRoot(urlPrefix, root string) router.MiddlewareFunc {
	return Serve(urlPrefix, LocalFile(root, false))
}

// Serve returns middleware that serves files from the provided filesystem.
// If the requested path exists, the middleware writes the file and stops the
// chain; otherwise it calls the next handler.
func Serve(urlPrefix string, fs ServeFileSystem) router.MiddlewareFunc {
	prefix := normalizePrefix(urlPrefix)

	fileserver := http.FileServer(fs)
	if prefix != "" {
		fileserver = http.StripPrefix(prefix, fileserver)
	}

	return func(next router.HandlerFunc) router.HandlerFunc {
		return func(c router.Context) error {
			if fs.Exists(prefix, c.Request().URL.Path) {
				fileserver.ServeHTTP(c.Response(), c.Request())
				return nil
			}
			return next(c)
		}
	}
}

func normalizePrefix(prefix string) string {
	prefix = strings.TrimSpace(prefix)
	if prefix == "" {
		return ""
	}
	cleaned := path.Clean("/" + strings.Trim(prefix, "/"))
	if cleaned == "/" {
		return ""
	}
	return cleaned
}

func sanitizeRequestPath(prefix, requestPath string) (string, bool) {
	cleaned := path.Clean("/" + requestPath)
	if cleaned == "." {
		cleaned = "/"
	}

	normalizedPrefix := normalizePrefix(prefix)
	if normalizedPrefix == "" {
		return cleaned, true
	}

	if cleaned == normalizedPrefix {
		return "/", true
	}

	if strings.HasPrefix(cleaned, normalizedPrefix+"/") {
		relative := strings.TrimPrefix(cleaned, normalizedPrefix)
		if relative == "" {
			return "/", true
		}
		return relative, true
	}

	return "", false
}
