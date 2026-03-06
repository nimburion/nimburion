package input

import (
	"errors"
	"strings"
	"testing"

	httpresponse "github.com/nimburion/nimburion/pkg/http/response"
)

type ValidDTO struct {
	Name  string `validate:"required"`
	Email string `validate:"required"`
	Age   int
}

func (d ValidDTO) Validate() error {
	if d.Name == "" {
		return errors.New("name is required")
	}
	if d.Email == "" {
		return errors.New("email is required")
	}
	if d.Age < 0 {
		return errors.New("age must be non-negative")
	}
	return nil
}

type InvalidDTO struct{ Name string }

func (d InvalidDTO) Validate() error { return errors.New("invalid data") }

type NonValidatorDTO struct {
	Name  string `validate:"required"`
	Email string
}

type EmptyDTO struct{}

type validatorFunc func() error

func (f validatorFunc) Validate() error { return f() }

func TestValidateDTO(t *testing.T) {
	tests := []struct {
		name string
		dto  interface{}
		err  string
	}{
		{name: "nil dto", dto: nil, err: "dto cannot be nil"},
		{name: "valid validator", dto: ValidDTO{Name: "John Doe", Email: "john@example.com", Age: 30}},
		{name: "invalid validator", dto: InvalidDTO{Name: "test"}, err: "invalid data"},
		{name: "missing required field", dto: NonValidatorDTO{Name: "", Email: "john@example.com"}, err: "validation failed"},
		{name: "pointer valid", dto: &ValidDTO{Name: "John Doe", Email: "john@example.com", Age: 30}},
		{name: "nil pointer", dto: (*ValidDTO)(nil), err: "dto cannot be nil"},
		{name: "empty struct", dto: EmptyDTO{}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDTO(tt.dto)
			if tt.err == "" && err != nil {
				t.Fatalf("ValidateDTO() unexpected error = %v", err)
			}
			if tt.err != "" {
				if err == nil || !contains(err.Error(), tt.err) {
					t.Fatalf("ValidateDTO() error = %v, want contains %q", err, tt.err)
				}
			}
		})
	}
}

func TestValidateDTOErrorWrapping(t *testing.T) {
	err := ValidateDTO(InvalidDTO{Name: "test"})
	var appErr *httpresponse.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.HTTPStatus != 400 {
		t.Fatalf("expected HTTP status 400, got %v", appErr.HTTPStatus)
	}
}

func TestValidateDTOAppErrorPreserved(t *testing.T) {
	dto := struct{ Validator }{
		Validator: validatorFunc(func() error {
			return httpresponse.NewValidationError("custom validation error", map[string]interface{}{"field": "test"})
		}),
	}
	err := ValidateDTO(dto)
	var appErr *httpresponse.AppError
	if !errors.As(err, &appErr) {
		t.Fatalf("expected AppError, got %T", err)
	}
	if appErr.Details == nil {
		t.Fatal("expected details to be preserved")
	}
}

func contains(haystack, needle string) bool {
	return haystack == needle || strings.Contains(haystack, needle)
}
