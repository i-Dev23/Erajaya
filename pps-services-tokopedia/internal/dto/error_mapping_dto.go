package dto

// ErrorMappingRequestDto represents the request for error message mapping
type ErrorMappingRequestDto struct {
	ErrorMessage string `json:"error_message" validate:"required"` // Error message dari Ultima/Oracle
	SystemType   string `json:"system_type" validate:"required"`   // 'ultima' atau 'oracle'
}

// ErrorMappingResponseDto represents the response for error message mapping
type ErrorMappingResponseDto struct {
	ResponseCode string `json:"response_code"` // Response code (misal "63", "99")
	Message      string `json:"message"`       // Mapped message (misal "Biller maintenance")
}
