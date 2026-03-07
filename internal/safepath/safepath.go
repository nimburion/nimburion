package safepath

import (
	"errors"
	"path/filepath"
	"strings"
)

var (
	// ErrPathTraversal reports an attempt to escape the allowed base path.
	ErrPathTraversal = errors.New("path traversal detected")
	// ErrInvalidPath reports an empty or malformed file path.
	ErrInvalidPath = errors.New("invalid file path")
)

// ValidateFilePath checks whether a file path is safe to use.
func ValidateFilePath(path, baseDir string) error {
	if path == "" {
		return ErrInvalidPath
	}

	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return ErrPathTraversal
	}

	if baseDir != "" {
		absBase, err := filepath.Abs(baseDir)
		if err != nil {
			return err
		}
		absPath, err := filepath.Abs(cleanPath)
		if err != nil {
			return err
		}
		if !strings.HasPrefix(absPath, absBase) {
			return ErrPathTraversal
		}
	}

	return nil
}
