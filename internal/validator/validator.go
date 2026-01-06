package validator

import (
	"fmt"
	"strings"

	"github.com/go-playground/validator/v10"
)

// Validator wraps go-playground validator
type Validator struct {
	validate *validator.Validate
}

// New creates a new validator
func New() *Validator {
	v := validator.New()

	// Register custom validators if needed
	// v.RegisterValidation("customtag", customValidationFunc)

	return &Validator{
		validate: v,
	}
}

// Validate validates a struct
func (v *Validator) Validate(s interface{}) error {
	if err := v.validate.Struct(s); err != nil {
		return v.formatValidationError(err)
	}
	return nil
}

// formatValidationError formats validation errors into a readable message
func (v *Validator) formatValidationError(err error) error {
	if validationErrors, ok := err.(validator.ValidationErrors); ok {
		var messages []string
		for _, e := range validationErrors {
			messages = append(messages, v.formatFieldError(e))
		}
		return fmt.Errorf("validation failed: %s", strings.Join(messages, "; "))
	}
	return err
}

// formatFieldError formats a single field error
func (v *Validator) formatFieldError(e validator.FieldError) string {
	field := e.Field()
	tag := e.Tag()

	switch tag {
	case "required":
		return fmt.Sprintf("%s is required", field)
	case "email":
		return fmt.Sprintf("%s must be a valid email", field)
	case "min":
		return fmt.Sprintf("%s must be at least %s characters", field, e.Param())
	case "max":
		return fmt.Sprintf("%s must be at most %s characters", field, e.Param())
	case "len":
		return fmt.Sprintf("%s must be exactly %s characters", field, e.Param())
	default:
		return fmt.Sprintf("%s failed validation for %s", field, tag)
	}
}

// ValidateStruct is a helper function for quick validation
func ValidateStruct(s interface{}) error {
	v := New()
	return v.Validate(s)
}
