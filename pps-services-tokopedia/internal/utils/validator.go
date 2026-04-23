package utils

import "github.com/go-playground/validator/v10"

var validate = validator.New()

// ValidateStruct validates a struct using go-playground/validator tags.
func ValidateStruct(v interface{}) error {
	return validate.Struct(v)
}

// ValidationErrorFields returns the struct field names that failed validation.
func ValidationErrorFields(err error) []string {
	ve, ok := err.(validator.ValidationErrors)
	if !ok {
		return nil
	}

	fields := make([]string, 0, len(ve))
	for _, fe := range ve {
		fields = append(fields, fe.Field())
	}
	return fields
}
