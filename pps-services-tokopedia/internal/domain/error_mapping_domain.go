package domain

import "context"

// ErrorMessageMapping represents a mapping result from database
// found indicates whether mapping exists.
type ErrorMessageMapping struct {
	ResponseCode string
	Description  string
	Found        bool
}

// ErrorMessageMappingRepository defines how to resolve error messages to response codes.
type ErrorMessageMappingRepository interface {
	GetMapping(ctx context.Context, systemType string, errorMessage string) (*ErrorMessageMapping, error)
}
