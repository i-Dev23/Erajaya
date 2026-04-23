package config

import (
	"github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// NewValidator creates a new validator instance.
func NewValidator(config *viper.Viper) *validator.Validate {
	return validator.New()
}
