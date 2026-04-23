package unipin

import (
	"fmt"
	"strconv"
	"strings"
)

// APIError represents Unipin error payload for non-success statuses.
// Example:
//
//	{
//	  "error": {"message": "Internal error", "error_code": 999},
//	  "status": 0
//	}
type APIError struct {
	Message   string `json:"message"`
	ErrorCode int    `json:"error_code"`
	Code      int    `json:"code"`
}

// ResolveErrorCode returns the UniPin error code if present.
// UniPin sometimes returns `error_code`, sometimes `code`.
func ResolveErrorCode(apiErr *APIError) int {
	if apiErr == nil {
		return 0
	}
	if apiErr.ErrorCode != 0 {
		return apiErr.ErrorCode
	}
	return apiErr.Code
}

// ResolveStatusCode returns the status code string for downstream usage.
// For status=0, prefer error.error_code (or error.code) when available.
func ResolveStatusCode(status int, apiErr *APIError) string {
	if status == 0 {
		if code := ResolveErrorCode(apiErr); code != 0 {
			return strconv.Itoa(code)
		}
	}
	return strconv.Itoa(status)
}

// ResolveReason returns the most useful error message available.
// Some Unipin endpoints return status!=1 with reason, others return error.message.
func ResolveReason(reason string, apiErr *APIError) string {
	reason = strings.TrimSpace(reason)
	if reason != "" {
		return reason
	}
	if apiErr == nil {
		return ""
	}

	msg := strings.TrimSpace(apiErr.Message)
	if msg == "" {
		return ""
	}
	code := apiErr.ErrorCode
	if code == 0 {
		code = apiErr.Code
	}
	if code != 0 {
		return fmt.Sprintf("%s (error_code=%d)", msg, code)
	}
	return msg
}
