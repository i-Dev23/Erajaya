package repository

import (
	"context"
	"database/sql"
	"fmt"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
)

// ErrorMappingPostgresRepository resolves error message mappings using Postgres function inquiry.get_error_message_mapping.
type ErrorMappingPostgresRepository struct {
	postgresService service.PostgresService
	logger          service.Logger
}

// NewErrorMappingPostgresRepository constructs repository with Postgres service and logger.
func NewErrorMappingPostgresRepository(postgresService service.PostgresService, logger service.Logger) domain.ErrorMessageMappingRepository {
	return &ErrorMappingPostgresRepository{
		postgresService: postgresService,
		logger:          logger,
	}
}

// GetMapping resolves response code/description for a given system type and error message.
// Returns Found=false when no mapping exists. On DB errors, returns error for caller to decide fallback.
func (r *ErrorMappingPostgresRepository) GetMapping(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error) {
	if r.postgresService == nil {
		return &domain.ErrorMessageMapping{Found: false}, fmt.Errorf("postgres service is nil")
	}

	query := `SELECT response_code, description, found FROM "mapping".get_error_message_mapping($1, $2)`

	r.logger.Info("Resolving error message mapping from database",
		"system_type", systemType,
		"error_message", errorMessage)

	rows, err := r.postgresService.Query(ctx, query, systemType, errorMessage)
	if err != nil {
		r.logger.Error("Failed to query error message mapping", "error", err)
		return &domain.ErrorMessageMapping{Found: false}, fmt.Errorf("failed to query error message mapping: %w", err)
	}
	defer rows.Close()

	result := &domain.ErrorMessageMapping{Found: false}

	if rows.Next() {
		var description sql.NullString
		if err := rows.Scan(&result.ResponseCode, &description, &result.Found); err != nil {
			r.logger.Error("Failed to scan error message mapping", "error", err)
			return &domain.ErrorMessageMapping{Found: false}, fmt.Errorf("failed to scan error message mapping: %w", err)
		}

		// Handle NULL description
		if description.Valid {
			result.Description = description.String
		} else {
			result.Description = ""
		}
	}

	if err := rows.Err(); err != nil {
		r.logger.Error("Error iterating error message mapping rows", "error", err)
		return &domain.ErrorMessageMapping{Found: false}, fmt.Errorf("error iterating error message mapping rows: %w", err)
	}

	return result, nil
}
