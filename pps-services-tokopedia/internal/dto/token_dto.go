package dto

type TokenRequestDto struct {
	ClientID     string `json:"client_id" validate:"required"`
	ClientSecret string `json:"client_secret" validate:"required"`
	Timestamp    string `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"`
}
type TokenResponseDto struct {
	ResponseCode string `json:"response_code" validate:"required"`                          // Response code as specified (see 10. Response Codes)
	Message      string `json:"message" validate:"required"`                                // Additional informational or error message
	Token        string `json:"token" validate:"required"`                                  // Valid token for transactional use
	Timestamp    string `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
}
