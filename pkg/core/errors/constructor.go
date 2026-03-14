package errors

import (
	stderrors "errors"
	"fmt"
)

// ConstructorError wraps failures returned by public constructors while preserving the underlying cause.
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

// WrapConstructorError normalizes constructor failures into a shared exported type.
func WrapConstructorError(name string, err error) error {
	if err == nil {
		return nil
	}
	var constructorErr *ConstructorError
	if stderrors.As(err, &constructorErr) {
		return err
	}
	return &ConstructorError{Name: name, Err: err}
}
