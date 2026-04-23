package repository

import (
	"context"
	"fmt"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
)

// CallbackLoggingRepository implements callback logging using PostgresService.
type CallbackLoggingRepository struct {
	postgresService service.PostgresService
	logger          service.Logger
}

// NewCallbackLoggingRepository creates a new CallbackLoggingRepository with the given PostgresService.
func NewCallbackLoggingRepository(postgresService service.PostgresService, logger service.Logger) domain.CallbackLoggingRepository {
	return &CallbackLoggingRepository{
		postgresService: postgresService,
		logger:          logger,
	}
}

// InsertCallbackLog executes the callback.http_callback_log_oninsert stored procedure
func (r *CallbackLoggingRepository) InsertCallbackLog(ctx context.Context, req *domain.CallbackLogInsertRequest) (*domain.HTTPLogInsertResponse, error) {
	// Headers are already JSON strings in CallbackLogInsertRequest
	requestHeadersJSON := req.RequestHeaders
	responseHeadersJSON := req.ResponseHeaders

	// Try a simpler approach - direct insert instead of procedure
	query := `INSERT INTO callback.http_callback_logs (
        ref_id, method, path, query_params, request_headers, request_body,
        status_code, response_headers, response_body, client_ip, user_agent,
        response_time_ms, request_time, response_time, error_message,
        callback_type, partner_ref_id, client_number, product_code, response_code, total_amount
    ) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8::jsonb, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)
    RETURNING id`

	// Log parameters for debugging
	r.logger.Info("Calling callback.http_callback_log_oninsert",
		"ref_id", req.RequestID,
		"method", req.Method,
		"path", req.Path,
		"status_code", req.StatusCode,
		"callback_type", req.CallbackType,
		"partner_ref_id", req.PartnerRefID,
		"client_number", req.ClientNumber,
		"product_code", req.ProductCode,
		"response_code", req.ResponseCode,
		"total_amount", req.TotalAmount,
		"request_time", req.RequestTime,
		"response_time", req.ResponseTime,
		"request_headers", requestHeadersJSON,
		"response_headers", responseHeadersJSON)

	// client_ip should be NULL if empty to satisfy INET type
	var clientIP any
	if req.ClientIP == "" {
		clientIP = nil
	} else {
		clientIP = req.ClientIP
	}

	rows, err := r.postgresService.Query(ctx, query,
		req.RequestID,       // p_ref_id
		req.Method,          // p_method
		req.Path,            // p_path
		req.QueryParams,     // p_query_params
		requestHeadersJSON,  // p_request_headers
		req.RequestBody,     // p_request_body
		req.StatusCode,      // p_status_code
		responseHeadersJSON, // p_response_headers
		req.ResponseBody,    // p_response_body
		clientIP,            // p_client_ip
		req.UserAgent,       // p_user_agent
		req.ResponseTimeMs,  // p_response_time_ms
		req.RequestTime,     // p_request_time
		req.ResponseTime,    // p_response_time
		req.Error,           // p_error_message
		req.CallbackType,    // p_callback_type
		req.PartnerRefID,    // p_partner_ref_id
		req.ClientNumber,    // p_client_number
		req.ProductCode,     // p_product_code
		req.ResponseCode,    // p_response_code
		req.TotalAmount,     // p_total_amount
	)
	if err != nil {
		r.logger.Error("Failed to execute callback.http_callback_log_oninsert", "error", err, "query", query)
		return nil, fmt.Errorf("failed to execute callback.http_callback_log_oninsert: %w", err)
	}
	defer rows.Close()

	var response domain.HTTPLogInsertResponse
	if rows.Next() {
		err = rows.Scan(&response.HTTPLogID)
		if err != nil {
			r.logger.Error("Failed to scan callback insert result", "error", err)
			return nil, fmt.Errorf("failed to scan callback insert result: %w", err)
		}
		response.Error = 0
		response.Message = "OK"
	} else {
		// Fallback: run non-returning insert to ensure the log is persisted even if RETURNING is not supported in this context
		nonReturning := `INSERT INTO callback.http_callback_logs (
            ref_id, method, path, query_params, request_headers, request_body,
            status_code, response_headers, response_body, client_ip, user_agent,
            response_time_ms, request_time, response_time, error_message,
            callback_type, partner_ref_id, client_number, product_code, response_code, total_amount
        ) VALUES ($1, $2, $3, $4, $5::jsonb, $6, $7, $8::jsonb, $9, $10, $11, $12, $13, $14, $15, $16, $17, $18, $19, $20, $21)`

		if _, execErr := r.postgresService.Exec(ctx, nonReturning,
			req.RequestID,
			req.Method,
			req.Path,
			req.QueryParams,
			requestHeadersJSON,
			req.RequestBody,
			req.StatusCode,
			responseHeadersJSON,
			req.ResponseBody,
			clientIP,
			req.UserAgent,
			req.ResponseTimeMs,
			req.RequestTime,
			req.ResponseTime,
			req.Error,
			req.CallbackType,
			req.PartnerRefID,
			req.ClientNumber,
			req.ProductCode,
			req.ResponseCode,
			req.TotalAmount,
		); execErr != nil {
			r.logger.Error("Fallback insert failed for callback log", "error", execErr)
			return nil, fmt.Errorf("fallback insert failed for callback log: %w", execErr)
		}

		response.Error = 0
		response.Message = "OK"
		response.HTTPLogID = 0
	}

	// Log response details for debugging
	r.logger.Info("Callback log insert response",
		"response_code", response.Error,
		"message", response.Message,
		"http_callback_log_id", response.HTTPLogID)

	if response.HTTPLogID == 0 {
		r.logger.Error("Callback log insert returned ID 0 - possible database issue")
		return &response, fmt.Errorf("callback log insert returned ID 0 - possible database issue")
	}

	r.logger.Info("Callback log inserted successfully", "http_callback_log_id", response.HTTPLogID)
	return &response, nil
}
