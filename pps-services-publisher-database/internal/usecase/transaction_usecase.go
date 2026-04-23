package usecase

import (
	"context"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/google/uuid"
	"github.com/rs/zerolog"

	"pps-services-publisher-database/internal/gateway/messaging"
	"pps-services-publisher-database/internal/model"
	"pps-services-publisher-database/internal/pkg/logger"
	"pps-services-publisher-database/internal/repository"
)

type TransactionUseCase struct {
	repo      *repository.TransactionRepository
	publisher *messaging.Publisher
	validate  *validator.Validate
	log       zerolog.Logger
}

func NewTransactionUseCase(
	repo *repository.TransactionRepository,
	publisher *messaging.Publisher,
	validate *validator.Validate,
	log zerolog.Logger,
) *TransactionUseCase {
	return &TransactionUseCase{
		repo:      repo,
		publisher: publisher,
		validate:  validate,
		log:       log,
	}
}

func (u *TransactionUseCase) ProcessTopupDataCallback(ctx context.Context, headers map[string][]string,
	req *model.CallbackRequest[model.TopupDataPayload]) (*model.CallbackResponse, error) {
	traceID := uuid.New().String()
	ctxLog := logger.ContextLogger(u.log, traceID)
	ctxLog.Info().Msgf("Processing topup/data callback for transaction %s", req.Data.MsgID)

	if err := u.validate.Struct(req); err != nil {
		ctxLog.Warn().Msgf("Topup/data callback validation failed: %v", err)
		return nil, fmt.Errorf("validation error: %w", err)
	}

	event := req.ToCallbackEvent(headers)

	if err := u.publisher.Publish(event); err != nil {
		ctxLog.Error().Err(err).Msgf("Failed to publish topup/data callback event %s", req.Data.MsgID)
		return nil, fmt.Errorf("failed to publish topup/data callback: %w", err)
	}

	ctxLog.Info().Msgf("Topup/data callback for transaction %s processed successfully", req.Data.MsgID)

	return &model.CallbackResponse{
		MsgID:  req.Data.MsgID,
		Status: "published",
	}, nil
}

func (u *TransactionUseCase) PingRabbitMQ() error {
	return u.publisher.PingRabbitMQ()
}
