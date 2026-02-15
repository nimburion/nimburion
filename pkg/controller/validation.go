package controller

import (
	"errors"
	"fmt"
	"reflect"
	"strings"
)

// Validator is an interface that DTOs can implement to provide custom validation logic
// DTOs implementing this interface will have their Validate method called by ValidateDTO
type Validator interface {
	Validate() error
}

// ValidateDTO validates a DTO by checking if it implements the Validator interface
// If the DTO implements Validator, it calls the Validate method
// If the DTO does not implement Validator, it performs basic nil checks
// Returns a validation error if validation fails, nil otherwise
func ValidateDTO(dto interface{}) error {
	if dto == nil {
		return NewValidationErrorWithCode("validation.dto_nil", "dto cannot be nil", nil, nil)
	}

	// Check for nil pointer before type assertion
	v := reflect.ValueOf(dto)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return NewValidationErrorWithCode("validation.dto_nil", "dto cannot be nil", nil, nil)
	}

	// Check if DTO implements Validator interface
	if validator, ok := dto.(Validator); ok {
		if err := validator.Validate(); err != nil {
			// Wrap validation errors in AppError if not already wrapped
			var appErr *AppError
			if errors.As(err, &appErr) {
				return err
			}
			return NewValidationErrorWithCode("validation.failed", err.Error(), map[string]interface{}{"cause": err.Error()}, nil)
		}
		return nil
	}

	// For DTOs that don't implement Validator, perform basic validation
	// Check for nil pointers in struct fields
	return validateStruct(dto)
}

// validateStruct performs basic validation on struct fields
// This is a fallback for DTOs that don't implement the Validator interface
func validateStruct(dto interface{}) error {
	v := reflect.ValueOf(dto)

	// Dereference pointer if needed
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return NewValidationErrorWithCode("validation.dto_nil", "dto cannot be nil", nil, nil)
		}
		v = v.Elem()
	}

	// Only validate structs
	if v.Kind() != reflect.Struct {
		return nil
	}

	// Check for required fields (basic validation)
	// This is a minimal implementation - DTOs should implement Validator for complex validation
	t := v.Type()
	var validationErrors []string

	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)

		// Skip unexported fields
		if !fieldType.IsExported() {
			continue
		}

		// Check for nil pointers in required fields
		// A field is considered required if it has a "required" tag
		if tag := fieldType.Tag.Get("validate"); tag != "" {
			if strings.Contains(tag, "required") {
				if isZeroValue(field) {
					validationErrors = append(validationErrors,
						fmt.Sprintf("field '%s' is required", fieldType.Name))
				}
			}
		}
	}

	if len(validationErrors) > 0 {
		return NewValidationErrorWithCode("validation.failed", "validation failed", map[string]interface{}{
			"errors": validationErrors,
		}, map[string]interface{}{
			"errors": validationErrors,
		})
	}

	return nil
}

// isZeroValue checks if a reflect.Value is the zero value for its type
func isZeroValue(v reflect.Value) bool {
	switch v.Kind() {
	case reflect.String:
		return v.String() == ""
	case reflect.Bool:
		return !v.Bool()
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return v.Int() == 0
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return v.Uint() == 0
	case reflect.Float32, reflect.Float64:
		return v.Float() == 0
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return v.IsNil()
	case reflect.Struct:
		return v.IsZero()
	default:
		return false
	}
}
