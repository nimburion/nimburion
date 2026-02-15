package controller

import (
	"errors"
	"reflect"
	"testing"
)

// Test DTOs

// ValidDTO implements the Validator interface with valid data
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

// InvalidDTO implements the Validator interface but returns an error
type InvalidDTO struct {
	Name string
}

func (d InvalidDTO) Validate() error {
	return errors.New("invalid data")
}

// NonValidatorDTO does not implement the Validator interface
type NonValidatorDTO struct {
	Name  string `validate:"required"`
	Email string
}

// EmptyDTO is an empty struct
type EmptyDTO struct{}

func TestValidateDTO(t *testing.T) {
	tests := []struct {
		name        string
		dto         interface{}
		expectError bool
		errorMsg    string
	}{
		{
			name:        "nil DTO returns error",
			dto:         nil,
			expectError: true,
			errorMsg:    "dto cannot be nil",
		},
		{
			name: "valid DTO with Validator interface passes",
			dto: ValidDTO{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   30,
			},
			expectError: false,
		},
		{
			name: "invalid DTO with Validator interface fails",
			dto: InvalidDTO{
				Name: "test",
			},
			expectError: true,
			errorMsg:    "invalid data",
		},
		{
			name: "DTO with missing required field fails validation",
			dto: ValidDTO{
				Name:  "",
				Email: "john@example.com",
				Age:   30,
			},
			expectError: true,
			errorMsg:    "name is required",
		},
		{
			name: "DTO with negative age fails validation",
			dto: ValidDTO{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   -5,
			},
			expectError: true,
			errorMsg:    "age must be non-negative",
		},
		{
			name: "non-validator DTO with all fields passes",
			dto: NonValidatorDTO{
				Name:  "John Doe",
				Email: "john@example.com",
			},
			expectError: false,
		},
		{
			name: "non-validator DTO with missing required field fails",
			dto: NonValidatorDTO{
				Name:  "",
				Email: "john@example.com",
			},
			expectError: true,
			errorMsg:    "validation failed",
		},
		{
			name: "pointer to valid DTO passes",
			dto: &ValidDTO{
				Name:  "John Doe",
				Email: "john@example.com",
				Age:   30,
			},
			expectError: false,
		},
		{
			name:        "nil pointer to DTO fails",
			dto:         (*ValidDTO)(nil),
			expectError: true,
			errorMsg:    "dto cannot be nil",
		},
		{
			name:        "empty struct passes",
			dto:         EmptyDTO{},
			expectError: false,
		},
		{
			name:        "non-struct type passes basic validation",
			dto:         "string value",
			expectError: false,
		},
		{
			name:        "int value passes basic validation",
			dto:         42,
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateDTO(tt.dto)

			if tt.expectError {
				if err == nil {
					t.Errorf("expected error but got nil")
					return
				}
				if tt.errorMsg != "" && err.Error() != tt.errorMsg {
					// Check if error message contains the expected message
					if !contains(err.Error(), tt.errorMsg) {
						t.Errorf("expected error message to contain %q, got %q", tt.errorMsg, err.Error())
					}
				}
			} else {
				if err != nil {
					t.Errorf("expected no error but got: %v", err)
				}
			}
		})
	}
}

func TestValidateDTO_ErrorWrapping(t *testing.T) {
	// Test that validation errors are wrapped in AppError
	dto := InvalidDTO{Name: "test"}
	err := ValidateDTO(dto)

	if err == nil {
		t.Fatal("expected error but got nil")
	}

	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Errorf("expected error to be wrapped in AppError, got %T", err)
	}

	if appErr.HTTPStatus != 400 {
		t.Errorf("expected HTTP status to be 400, got %v", appErr.HTTPStatus)
	}
}

func TestValidateDTO_AppErrorPreserved(t *testing.T) {
	// Test that if Validate() returns an AppError, it's preserved
	type AppErrorDTO struct{}

	dto := struct {
		Validator
	}{
		Validator: validatorFunc(func() error {
			return NewValidationError("custom validation error", map[string]interface{}{
				"field": "test",
			})
		}),
	}

	err := ValidateDTO(dto)

	if err == nil {
		t.Fatal("expected error but got nil")
	}

	var appErr *AppError
	if !errors.As(err, &appErr) {
		t.Errorf("expected error to be AppError, got %T", err)
	}

	if appErr.Details == nil {
		t.Error("expected details to be preserved")
	}
}

func TestValidateStruct_RequiredFields(t *testing.T) {
	tests := []struct {
		name        string
		dto         interface{}
		expectError bool
	}{
		{
			name: "struct with all required fields filled",
			dto: struct {
				Name  string `validate:"required"`
				Email string `validate:"required"`
			}{
				Name:  "John",
				Email: "john@example.com",
			},
			expectError: false,
		},
		{
			name: "struct with missing required field",
			dto: struct {
				Name  string `validate:"required"`
				Email string `validate:"required"`
			}{
				Name:  "John",
				Email: "",
			},
			expectError: true,
		},
		{
			name: "struct with no validation tags",
			dto: struct {
				Name  string
				Email string
			}{
				Name:  "",
				Email: "",
			},
			expectError: false,
		},
		{
			name: "struct with optional fields",
			dto: struct {
				Name     string `validate:"required"`
				Optional string
			}{
				Name:     "John",
				Optional: "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateStruct(tt.dto)

			if tt.expectError && err == nil {
				t.Error("expected error but got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("expected no error but got: %v", err)
			}
		})
	}
}

func TestIsZeroValue(t *testing.T) {
	tests := []struct {
		name     string
		value    interface{}
		expected bool
	}{
		{"empty string", "", true},
		{"non-empty string", "hello", false},
		{"zero int", 0, true},
		{"non-zero int", 42, false},
		{"false bool", false, true},
		{"true bool", true, false},
		{"nil pointer", (*int)(nil), true},
		{"non-nil pointer", new(int), false},
		{"nil slice", []int(nil), true},
		{"empty slice", []int{}, false},
		{"nil map", map[string]int(nil), true},
		{"empty map", map[string]int{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			v := reflect.ValueOf(tt.value)
			result := isZeroValue(v)
			if result != tt.expected {
				t.Errorf("isZeroValue(%v) = %v, expected %v", tt.value, result, tt.expected)
			}
		})
	}
}

// Helper types and functions

// validatorFunc is a function type that implements the Validator interface
type validatorFunc func() error

func (f validatorFunc) Validate() error {
	return f()
}

// contains checks if a string contains a substring
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		(len(s) > 0 && len(substr) > 0 && containsHelper(s, substr)))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
