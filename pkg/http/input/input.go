// Package input provides HTTP-adjacent input validation helpers.
package input

import (
	"errors"
	"fmt"
	"reflect"
	"strings"

	httpresponse "github.com/nimburion/nimburion/pkg/http/response"
)

// Validator is implemented by DTOs that provide custom validation logic.
type Validator interface {
	Validate() error
}

// ValidateDTO validates a DTO using either its Validator implementation or basic required-tag checks.
func ValidateDTO(dto interface{}) error {
	if dto == nil {
		return httpresponse.NewValidationErrorWithCode("validation.dto_nil", "dto cannot be nil", nil, nil)
	}

	v := reflect.ValueOf(dto)
	if v.Kind() == reflect.Ptr && v.IsNil() {
		return httpresponse.NewValidationErrorWithCode("validation.dto_nil", "dto cannot be nil", nil, nil)
	}

	if validator, ok := dto.(Validator); ok {
		if err := validator.Validate(); err != nil {
			var appErr *httpresponse.AppError
			if errors.As(err, &appErr) {
				return err
			}
			return httpresponse.NewValidationErrorWithCode("validation.failed", err.Error(), map[string]interface{}{"cause": err.Error()}, nil)
		}
		return nil
	}

	return validateStruct(dto)
}

func validateStruct(dto interface{}) error {
	v := reflect.ValueOf(dto)
	if v.Kind() == reflect.Ptr {
		if v.IsNil() {
			return httpresponse.NewValidationErrorWithCode("validation.dto_nil", "dto cannot be nil", nil, nil)
		}
		v = v.Elem()
	}
	if v.Kind() != reflect.Struct {
		return nil
	}

	t := v.Type()
	var validationErrors []string
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		fieldType := t.Field(i)
		if !fieldType.IsExported() {
			continue
		}
		if tag := fieldType.Tag.Get("validate"); tag != "" && strings.Contains(tag, "required") && isZeroValue(field) {
			validationErrors = append(validationErrors, fmt.Sprintf("field '%s' is required", fieldType.Name))
		}
	}
	if len(validationErrors) > 0 {
		return httpresponse.NewValidationErrorWithCode("validation.failed", "validation failed", map[string]interface{}{"errors": validationErrors}, map[string]interface{}{"errors": validationErrors})
	}
	return nil
}

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
