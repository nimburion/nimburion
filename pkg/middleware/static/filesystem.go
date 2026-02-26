package static

import (
	"embed"
	"io/fs"
	"net/http"
	"path"
)

const indexFileName = "index.html"

// LocalFile returns a ServeFileSystem rooted at the provided directory.
// Setting allowIndexes to true allows directories with index.html to be served.
func LocalFile(root string, allowIndexes bool) ServeFileSystem {
	return &localFileSystem{
		FileSystem:   http.Dir(root),
		allowIndexes: allowIndexes,
	}
}

// EmbedFolder wraps an embed.FS subtree so it can be served with the static
// middleware.
func EmbedFolder(source embed.FS, target string) (ServeFileSystem, error) {
	sub, err := fs.Sub(source, target)
	if err != nil {
		return nil, err
	}
	return &embedFileSystem{FileSystem: http.FS(sub)}, nil
}

type localFileSystem struct {
	http.FileSystem
	allowIndexes bool
}

// Exists TODO: add description
func (l *localFileSystem) Exists(prefix, requestPath string) bool {
	relative, ok := sanitizeRequestPath(prefix, requestPath)
	if !ok {
		return false
	}

	f, err := l.Open(relative)
	if err != nil {
		return false
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return false
	}

	if info.IsDir() {
		if !l.allowIndexes {
			return false
		}
		indexPath := path.Join(relative, indexFileName)
		index, err := l.Open(indexPath)
		if err != nil {
			return false
		}
		index.Close()
	}

	return true
}

type embedFileSystem struct {
	http.FileSystem
}

// Exists TODO: add description
func (e *embedFileSystem) Exists(prefix, requestPath string) bool {
	relative, ok := sanitizeRequestPath(prefix, requestPath)
	if !ok {
		return false
	}

	f, err := e.Open(relative)
	if err != nil {
		return false
	}
	f.Close()
	return true
}
