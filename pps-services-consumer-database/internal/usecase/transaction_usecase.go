package usecase

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/go-playground/validator/v10"
	"github.com/rs/zerolog"

	"pps-services-consumer-database/internal/gateway/downstream"
	"pps-services-consumer-database/internal/model"
	"pps-services-consumer-database/internal/pkg/logger"
	"pps-services-consumer-database/internal/repository"
)

type TransactionUseCase struct {
	repo       *repository.TransactionRepository
	downstream *downstream.DownstreamClient
	validate   *validator.Validate
	log        zerolog.Logger
}

func NewTransactionUseCase(
	repo *repository.TransactionRepository,
	downstream *downstream.DownstreamClient,
	validate *validator.Validate,
	log zerolog.Logger,
) *TransactionUseCase {
	return &TransactionUseCase{
		repo:       repo,
		downstream: downstream,
		validate:   validate,
		log:        log,
	}
}

// HandleConsumedMessage routes the message based on the source field inside body.
func (u *TransactionUseCase) HandleConsumedMessage(ctx context.Context, event *model.TransactionEvent) error {
	ctxLog := logger.ContextLogger(u.log, event.Id)
	ctxLog.Info().Msgf("Handling consumed message %s", event.Id)

	var transactionPayload model.TransactionPayload
	if err := json.Unmarshal(event.Payload, &transactionPayload); err != nil {
		ctxLog.Error().Err(err).Msg("Failed to decode callback body")
		return fmt.Errorf("decode callback body: %w", err)
	}

	switch transactionPayload.Source {
	case model.SourceProvider:
		return u.handleProvider(ctx, event, &transactionPayload.Data, ctxLog)
	case model.SourcePreOrder:
		return u.handleOrder(ctx, event, &transactionPayload.Data, ctxLog)
	default:
		return fmt.Errorf("unknown source: %s", transactionPayload.Source)
	}
}

// handleProvider routes PROVIDER messages to the correct SP based on queue name.
func (u *TransactionUseCase) handleProvider(ctx context.Context, event *model.TransactionEvent, data *json.RawMessage, ctxLog zerolog.Logger) error {
	var spResult *model.SPResult
	var err error

	if vals, ok := event.Headers["X-Type-Transaction"]; ok && len(vals) > 0 {
		switch vals[0] {
		case "topup":
			fallthrough
		case "data":
			ctxLog.Debug().Msg("Routing to CallSetTransactionStatus (topup/data)")

			var payload model.TopupDataPayload
			if err := json.Unmarshal(*data, &payload); err != nil {
				return fmt.Errorf("Failed to decode data: %w", err)
			}

			spResult, err = u.repo.CallSetTransactionStatus(ctx, event, &payload)
		case "game":

		default:
			fmt.Println("Unknown transaction type:", vals[0])
		}
	}

	if err != nil {
		ctxLog.Error().Err(err).Msgf("Failed to execute SP for %s", event.Id)
		return fmt.Errorf("SP execution failed: %w", err)
	}

	if spResult.Error != 0 {
		ctxLog.Warn().Msgf("SP returned error for %s: %d - %s", event.Id, spResult.Error, spResult.Message)
		return fmt.Errorf("SP returned error: %w", err)
	}

	ctxLog.Info().Msgf("Message %s processed successfully (PROVIDER)", event.Id)
	return nil
}

// handleOrder processes ORDER messages through the 3-step flow.
func (u *TransactionUseCase) handleOrder(ctx context.Context, event *model.TransactionEvent, data *json.RawMessage, ctxLog zerolog.Logger) error {
	var payload model.OrderPayload
	if err := json.Unmarshal(*data, &payload); err != nil {
		return fmt.Errorf("Failed to decode data: %w", err)
	}

	ctxLog.Debug().Msg("Step 1: call updPreOrderConsume")
	preOrder, preOrderResult, err := u.repo.CallUpdPreOrderConsume(ctx, event, &payload)
	if err != nil {
		ctxLog.Warn().Msgf("updPreOrderConsume execution failed for %s", event.Id)
		return fmt.Errorf("updPreOrderConsume execution failed: %w", err)
	}
	if preOrderResult.Error != 0 {
		ctxLog.Warn().Msgf("updPreOrderConsume returned error for %s: %d - %s", event.Id, preOrderResult.Error, preOrderResult.Message)
		return fmt.Errorf("updPreOrderConsume returned error %d: %s", preOrderResult.Error, preOrderResult.Message)
	}

	ctxLog.Debug().Msg("Step 2: call request2JualRandomWithID")
	orderResult, err := u.repo.CallRequest2JualRandomWithID(ctx, event, &payload, preOrder)
	if err != nil {
		ctxLog.Warn().Msgf("request2JualRandomWithID execution failed for %s", event.Id)
		return fmt.Errorf("request2JualRandomWithID execution failed: %w", err)
	}
	if orderResult.Error != 0 {
		ctxLog.Warn().Msgf("request2JualRandomWithID returned error for %s: %d - %s", event.Id, preOrderResult.Error, preOrderResult.Message)
		return fmt.Errorf("request2JualRandomWithID returned error %d: %s", orderResult.Error, orderResult.Message)
	}

	// Step 3: Send to downstream REST API
	ctxLog.Debug().Msg("Step 3: sendOrderResult to downstream")
	downstreamReq := &model.DownstreamRequest{
		MsgID:         payload.MsgID,
		ClientNumber:  payload.ClientNumber,
		IMSI:          preOrder.IMSI,
		RemarkIMSI:    preOrder.RemarkIMSI,
		MID:           preOrder.MID,
		StoreID:       preOrder.StoreID,
		QueueName:     preOrder.QueueName,
		TypeVoucher:   preOrder.TypeVoucher,
		VoucherCode:   payload.VoucherCode,
		BID:           preOrder.BID,
		TypeOfStock:   preOrder.TypeOfStock,
		Provider:      preOrder.Provider,
		MQTransaction: "",
	}

	if err := u.downstream.SendOrderResult(ctx, downstreamReq); err != nil {
		return fmt.Errorf("downstream SendOrderResult failed: %w", err)
	}

	ctxLog.Info().Msgf("Message %s processed successfully (ORDER)", event.Id)
	return nil
}

func (u *TransactionUseCase) PingOracle(ctx context.Context) error {
	return u.repo.PingOracle(ctx)
}
