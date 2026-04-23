package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
)

// HTTPLoggingRepository implements domain.HTTPLoggingRepository using PostgresService.
type HTTPLoggingRepository struct {
	postgresService service.PostgresService
	logger          service.Logger
}

// NewHTTPLoggingRepository creates a new HTTPLoggingRepository with the given PostgresService.
func NewHTTPLoggingRepository(postgresService service.PostgresService, logger service.Logger) domain.HTTPLoggingRepository {
	return &HTTPLoggingRepository{
		postgresService: postgresService,
		logger:          logger,
	}
}

// InsertHTTPLog executes the http_log_oninsert stored procedure
func (r *HTTPLoggingRepository) InsertHTTPLog(ctx context.Context, req *domain.HTTPLogInsertRequest) (*domain.HTTPLogInsertResponse, error) {
	// Convert headers to JSON string
	requestHeadersJSON, err := json.Marshal(req.RequestHeaders)
	if err != nil {
		r.logger.Error("Failed to marshal request headers", "error", err)
		return nil, fmt.Errorf("failed to marshal request headers: %w", err)
	}

	responseHeadersJSON, err := json.Marshal(req.ResponseHeaders)
	if err != nil {
		r.logger.Error("Failed to marshal response headers", "error", err)
		return nil, fmt.Errorf("failed to marshal response headers: %w", err)
	}

	query := `SELECT * FROM log.http_log_oninsert($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15) AS t(http_log_id BIGINT, error_code INTEGER, message TEXT)`

	rows, err := r.postgresService.Query(ctx, query,
		req.RequestID,
		req.Method,
		req.Path,
		req.QueryParams,
		string(requestHeadersJSON),
		req.RequestBody,
		req.StatusCode,
		string(responseHeadersJSON),
		req.ResponseBody,
		req.ClientIP,
		req.UserAgent,
		req.ResponseTimeMs,
		req.RequestTime,
		req.ResponseTime,
		req.Error,
	)
	if err != nil {
		r.logger.Error("Failed to execute http_log_oninsert", "error", err)
		return nil, fmt.Errorf("failed to execute http_log_oninsert: %w", err)
	}
	defer rows.Close()

	var response domain.HTTPLogInsertResponse
	if rows.Next() {
		err = rows.Scan(&response.HTTPLogID, &response.Error, &response.Message)
		if err != nil {
			r.logger.Error("Failed to scan http_log_oninsert result", "error", err)
			return nil, fmt.Errorf("failed to scan http_log_oninsert result: %w", err)
		}
	}

	if response.Error != 0 {
		r.logger.Error("HTTP log insert failed", "error_code", response.Error, "message", response.Message)
		return &response, fmt.Errorf("HTTP log insert failed: %s", response.Message)
	}

	r.logger.Info("HTTP log inserted successfully", "http_log_id", response.HTTPLogID)
	return &response, nil
}
