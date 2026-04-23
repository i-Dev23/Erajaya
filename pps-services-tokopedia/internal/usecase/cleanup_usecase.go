package usecase

import (
	"context"
	"fmt"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
)

// CleanupUsecase handles cleanup operations
type CleanupUsecase interface {
	CleanupOldHTTPLogs(ctx context.Context, daysToKeep int) (int, error)
	CleanupOldCallbackLogs(ctx context.Context, daysToKeep int) (int, error)
	CleanupOldInquiryLogs(ctx context.Context, daysToKeep int) (int, error)
	CleanupOldPaymentLogs(ctx context.Context, daysToKeep int) (int, error)
}

type cleanupUsecaseImpl struct {
	cleanupRepo domain.CleanupRepository
	logger      service.Logger
}

// NewCleanupUsecase creates a new instance of CleanupUsecase
func NewCleanupUsecase(
	cleanupRepo domain.CleanupRepository,
	logger service.Logger,
) CleanupUsecase {
	return &cleanupUsecaseImpl{
		cleanupRepo: cleanupRepo,
		logger:      logger,
	}
}

// CleanupOldHTTPLogs executes the database cleanup function to remove old HTTP logs
func (u *cleanupUsecaseImpl) CleanupOldHTTPLogs(ctx context.Context, daysToKeep int) (int, error) {
	u.logger.Info("Starting HTTP logs cleanup", "daysToKeep", daysToKeep)

	deletedCount, err := u.cleanupRepo.CleanupOldHTTPLogs(ctx, daysToKeep)
	if err != nil {
		u.logger.Error("Failed to cleanup old HTTP logs", "error", err)
		return 0, fmt.Errorf("failed to cleanup old HTTP logs: %w", err)
	}

	u.logger.Info("HTTP logs cleanup completed successfully", "deletedCount", deletedCount)
	return deletedCount, nil
}

// CleanupOldCallbackLogs executes the database cleanup function to remove old Callback logs
func (u *cleanupUsecaseImpl) CleanupOldCallbackLogs(ctx context.Context, daysToKeep int) (int, error) {
	u.logger.Info("Starting Callback logs cleanup", "daysToKeep", daysToKeep)

	deletedCount, err := u.cleanupRepo.CleanupOldCallbackLogs(ctx, daysToKeep)
	if err != nil {
		u.logger.Error("Failed to cleanup old Callback logs", "error", err)
		return 0, fmt.Errorf("failed to cleanup old Callback logs: %w", err)
	}

	u.logger.Info("Callback logs cleanup completed successfully", "deletedCount", deletedCount)
	return deletedCount, nil
}

// CleanupOldInquiryLogs executes the database cleanup function to remove old Inquiry logs
func (u *cleanupUsecaseImpl) CleanupOldInquiryLogs(ctx context.Context, daysToKeep int) (int, error) {
	u.logger.Info("Starting Inquiry logs cleanup", "daysToKeep", daysToKeep)

	deletedCount, err := u.cleanupRepo.CleanupOldInquiryLogs(ctx, daysToKeep)
	if err != nil {
		u.logger.Error("Failed to cleanup old Inquiry logs", "error", err)
		return 0, fmt.Errorf("failed to cleanup old Inquiry logs: %w", err)
	}

	u.logger.Info("Inquiry logs cleanup completed successfully", "deletedCount", deletedCount)
	return deletedCount, nil
}

// CleanupOldPaymentLogs executes the database cleanup function to remove old Payment logs
func (u *cleanupUsecaseImpl) CleanupOldPaymentLogs(ctx context.Context, daysToKeep int) (int, error) {
	u.logger.Info("Starting Payment logs cleanup", "daysToKeep", daysToKeep)

	deletedCount, err := u.cleanupRepo.CleanupOldPaymentLogs(ctx, daysToKeep)
	if err != nil {
		u.logger.Error("Failed to cleanup old Payment logs", "error", err)
		return 0, fmt.Errorf("failed to cleanup old Payment logs: %w", err)
	}

	u.logger.Info("Payment logs cleanup completed successfully", "deletedCount", deletedCount)
	return deletedCount, nil
}
