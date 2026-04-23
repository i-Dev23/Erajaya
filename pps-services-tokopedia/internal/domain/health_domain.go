package domain

import "context"

// model domain health check request business logic
type HealthCheckRequestDomain struct {
	Timestamp string // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
}

// model domain health check response business logic
type HealthCheckResponseDomain struct {
	ResponseCode string // Response code as specified (see 10. Response Codes)
	Message      string // Additional informational or error message
	Timestamp    string // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
}

// model domain deep health check response
type DeepHealthCheckResponseDomain struct {
	ResponseCode string            // Response code as specified (see 10. Response Codes)
	Message      string            // Additional informational or error message
	Timestamp    string            // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	Services     map[string]string // Service status map
}

// HealthCheckUsecase defines the interface for health check business logic
// * pointer because we don't want to pass by value, we want to pass by reference
type HealthCheckUsecase interface {
	HealthCheck(ctx context.Context, req *HealthCheckRequestDomain) (*HealthCheckResponseDomain, error)
	DeepHealthCheck(ctx context.Context, req *HealthCheckRequestDomain) (*DeepHealthCheckResponseDomain, error)
}
