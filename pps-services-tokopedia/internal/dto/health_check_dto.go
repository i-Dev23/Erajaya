package dto

type HealthCheckRequestDto struct {
	Timestamp string `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"`
}
type HealthCheckResponseDto struct {
	ResponseCode string `json:"response_code" validate:"required"`                          // Response code as specified (see 10. Response Codes)
	Message      string `json:"message" validate:"required"`                                // Additional informational or error message
	Timestamp    string `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
}

// DeepHealthCheckResponseDto represents the deep health check response structure
type DeepHealthCheckResponseDto struct {
	ResponseCode string            `json:"response_code" validate:"required"`                          // Response code as specified (see 10. Response Codes)
	Message      string            `json:"message" validate:"required"`                                // Additional informational or error message
	Timestamp    string            `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
	Services     map[string]string `json:"services"`                                                   // Service status map
}

// HealthCheckV2ResponseDto represents the response structure for v2 health check
type HealthCheckV2ResponseDto struct {
	ResponseCode string `json:"response_code" validate:"required"`                          // Response code as specified (see 10. Response Codes)
	Message      string `json:"message" validate:"required"`                                // Additional informational or error message
	Timestamp    string `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Status       string `json:"status"`
	Message      string `json:"message,omitempty"`
	ResponseTime string `json:"response_time,omitempty"`
}
