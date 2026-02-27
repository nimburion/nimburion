package security

import (
	"errors"
	"path/filepath"
	"strings"
)

var (
	ErrPathTraversal = errors.New("path traversal detected")
	ErrInvalidPath   = errors.New("invalid file path")
)

// ValidateFilePath checks if a file path is safe to use
func ValidateFilePath(path, baseDir string) error {
	if path == "" {
		return ErrInvalidPath
	}

	// Clean the path
	cleanPath := filepath.Clean(path)

	// Check for path traversal
	if strings.Contains(cleanPath, "..") {
		return ErrPathTraversal
	}

	// If baseDir is provided, ensure path is within it
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
