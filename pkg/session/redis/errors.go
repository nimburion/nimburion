package redis

import (
	"errors"
	"fmt"
)

// ConstructorError wraps public constructor failures while preserving the underlying cause.
type ConstructorError struct {
	Name string
	Err  error
}

func (e *ConstructorError) Error() string {
	if e == nil {
		return "<nil>"
	}
	return fmt.Sprintf("%s failed: %v", e.Name, e.Err)
}

func (e *ConstructorError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}

func wrapConstructorError(name string, err error) error {
	if err == nil {
		return nil
	}
	var constructorErr *ConstructorError
	if errors.As(err, &constructorErr) {
		return err
	}
	return &ConstructorError{Name: name, Err: err}
}
