package domain

import (
	"context"
	"time"
)

// HTTPLogRequest represents the input parameters for HTTP request logging
type HTTPLogRequest struct {
	RequestID     string            `json:"request_id"`
	Method        string            `json:"method"`
	Path          string            `json:"path"`
	QueryParams   string            `json:"query_params"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	ClientIP      string            `json:"client_ip"`
	UserAgent     string            `json:"user_agent"`
	Timestamp     time.Time         `json:"timestamp"`
	ContentType   string            `json:"content_type"`
	ContentLength int64             `json:"content_length"`
}

// HTTPLogResponse represents the input parameters for HTTP response logging
type HTTPLogResponse struct {
	RequestID     string            `json:"request_id"`
	StatusCode    int               `json:"status_code"`
	Headers       map[string]string `json:"headers"`
	Body          string            `json:"body"`
	ResponseTime  int64             `json:"response_time_ms"`
	Timestamp     time.Time         `json:"timestamp"`
	ContentType   string            `json:"content_type"`
	ContentLength int64             `json:"content_length"`
	Error         string            `json:"error,omitempty"`
}

// HTTPLogInsertRequest represents the input parameters for http_log_oninsert procedure
type HTTPLogInsertRequest struct {
	RequestID       string    `json:"request_id"`
	Method          string    `json:"method"`
	Path            string    `json:"path"`
	QueryParams     string    `json:"query_params"`
	RequestHeaders  string    `json:"request_headers"`
	RequestBody     string    `json:"request_body"`
	StatusCode      int       `json:"status_code"`
	ResponseHeaders string    `json:"response_headers"`
	ResponseBody    string    `json:"response_body"`
	ClientIP        string    `json:"client_ip"`
	UserAgent       string    `json:"user_agent"`
	ResponseTimeMs  int64     `json:"response_time_ms"`
	RequestTime     time.Time `json:"request_time"`
	ResponseTime    time.Time `json:"response_time"`
	Error           string    `json:"error,omitempty"`
}

// CallbackLogInsertRequest represents the input parameters for callback.http_callback_log_oninsert procedure
type CallbackLogInsertRequest struct {
	RequestID       string    `json:"request_id"`
	Method          string    `json:"method"`
	Path            string    `json:"path"`
	QueryParams     string    `json:"query_params"`
	RequestHeaders  string    `json:"request_headers"`
	RequestBody     string    `json:"request_body"`
	StatusCode      int       `json:"status_code"`
	ResponseHeaders string    `json:"response_headers"`
	ResponseBody    string    `json:"response_body"`
	ClientIP        string    `json:"client_ip"`
	UserAgent       string    `json:"user_agent"`
	ResponseTimeMs  int64     `json:"response_time_ms"`
	RequestTime     time.Time `json:"request_time"`
	ResponseTime    time.Time `json:"response_time"`
	Error           string    `json:"error,omitempty"`
	CallbackType    string    `json:"callback_type"`
	PartnerRefID    string    `json:"partner_ref_id"`
	ClientNumber    string    `json:"client_number"`
	ProductCode     string    `json:"product_code"`
	ResponseCode    string    `json:"response_code"`
	TotalAmount     *float64  `json:"total_amount"`
}

// HTTPLogInsertResponse represents the output parameters for http_log_oninsert procedure
type HTTPLogInsertResponse struct {
	HTTPLogID int64  `json:"http_log_id"`
	Error     int    `json:"error"`
	Message   string `json:"message"`
}

// HTTPLoggingRepository defines the interface for HTTP logging operations
type HTTPLoggingRepository interface {
	InsertHTTPLog(ctx context.Context, req *HTTPLogInsertRequest) (*HTTPLogInsertResponse, error)
}

// CallbackLoggingRepository defines the interface for callback logging operations
type CallbackLoggingRepository interface {
	InsertCallbackLog(ctx context.Context, req *CallbackLogInsertRequest) (*HTTPLogInsertResponse, error)
}
