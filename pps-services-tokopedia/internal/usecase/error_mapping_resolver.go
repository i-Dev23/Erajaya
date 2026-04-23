package usecase

import (
	"context"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
)

// resolveErrorMapping tries DB mapping first, returns error code 62 (Server error) on DB error, or code 99 (Other error) if mapping not found.
func resolveErrorMapping(
	ctx context.Context,
	systemType string,
	errorMessage string,
	repo domain.ErrorMessageMappingRepository,
	logger service.Logger,
) (string, string) {
	if repo != nil {
		mapping, err := repo.GetMapping(ctx, systemType, errorMessage)
		if err != nil {
			// DB error: return code 62 (Server error)
			logger.Warn("Failed to resolve error mapping from DB, return code 62 (Server error)",
				"system_type", systemType,
				"error", err)
			if rc, ok := utils.GetResponseCode("62"); ok {
				return rc.Code, rc.Message
			}
			return "62", "Server error"
		} else if mapping != nil && mapping.Found {
			if rc, ok := utils.GetResponseCode(mapping.ResponseCode); ok {
				return rc.Code, rc.Message
			}
			logger.Warn("Response code from DB mapping not found in static response codes, return default code 99",
				"system_type", systemType,
				"db_response_code", mapping.ResponseCode)
		}
	}

	// Error message not found in DB mapping, return code 99 (Other error)
	if rc, ok := utils.GetResponseCode("99"); ok {
		return rc.Code, rc.Message
	}
	return "99", "Other error"
}

// resolveUltimaMapping resolves Ultima error message to response code/message.
func resolveUltimaMapping(ctx context.Context, errorMessage string, repo domain.ErrorMessageMappingRepository, logger service.Logger) (string, string) {
	return resolveErrorMapping(ctx, "ultima", errorMessage, repo, logger)
}

// resolveOracleMapping resolves Oracle error message to response code/message.
func resolveOracleMapping(ctx context.Context, errorMessage string, repo domain.ErrorMessageMappingRepository, logger service.Logger) (string, string) {
	return resolveErrorMapping(ctx, "oracle", errorMessage, repo, logger)
}
