package usecase

import (
	"context"
	"errors"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"time"
)

type checkStatusUsecaseImpl struct {
	logger              service.Logger
	postgresPaymentRepo domain.PostgresPaymentRepository
}

func NewCheckStatusUsecase(
	logger service.Logger,
	postgresPaymentRepo domain.PostgresPaymentRepository,
) domain.CheckStatusUsecase {
	return &checkStatusUsecaseImpl{
		logger:              logger,
		postgresPaymentRepo: postgresPaymentRepo,
	}
}

func (u *checkStatusUsecaseImpl) CheckStatus(ctx context.Context, req *domain.CheckStatusRequestDomain) (*domain.CheckStatusResponseDomain, error) {
	u.logger.Info("CheckStatus request received",
		"ref_id", req.RefID,
		"timestamp", req.Timestamp,
		"category", req.Category)

	// Validate mandatory parameters
	type checkStatusMandatoryParams struct {
		RefID     string `validate:"required"`
		Timestamp string `validate:"required"`
		Category  string `validate:"required"`
	}

	validationErr := utils.ValidateStruct(checkStatusMandatoryParams{
		RefID:     req.RefID,
		Timestamp: req.Timestamp,
		Category:  req.Category,
	})
	if validationErr != nil {
		fieldMap := map[string]string{
			"RefID":     "ref_id",
			"Timestamp": "timestamp",
			"Category":  "category",
		}
		var missingParams []string
		for _, field := range utils.ValidationErrorFields(validationErr) {
			if name, ok := fieldMap[field]; ok {
				missingParams = append(missingParams, name)
			}
		}

		u.logger.Warn("Missing mandatory parameters in check status request",
			"missing_params", missingParams,
			"ref_id", req.RefID,
			"timestamp", req.Timestamp,
			"category", req.Category)
		rc, _ := utils.GetResponseCode("42")
		return &domain.CheckStatusResponseDomain{
			RefID:        req.RefID,
			ResponseCode: rc.Code,
			Message:      rc.Message,
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	// Get payment status from database by ref_id
	paymentStatus, err := u.postgresPaymentRepo.GetPaymentStatusByRefID(ctx, req.RefID)
	if err != nil {
		u.logger.Error("Failed to get payment status",
			"error", err,
			"ref_id", req.RefID)

		// Check if it's a "not found" error
		if errors.Is(err, errors.New("payment not found")) || err.Error() == "payment not found for ref_id: "+req.RefID {
			rc, _ := utils.GetResponseCode("12")
			return &domain.CheckStatusResponseDomain{
				RefID:        req.RefID,
				ResponseCode: rc.Code,
				Message:      rc.Message,
				Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
			}, nil
		}

		// For other errors, return server error
		rc, _ := utils.GetResponseCode("62")
		return &domain.CheckStatusResponseDomain{
			RefID:        req.RefID,
			ResponseCode: rc.Code,
			Message:      rc.Message,
			Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		}, nil
	}

	// Build response from payment status
	response := &domain.CheckStatusResponseDomain{
		RefID:        paymentStatus.RefID,
		PartnerRefID: paymentStatus.PartnerRefID,
		ClientNumber: paymentStatus.ClientNumber,
		ProductCode:  paymentStatus.ProductCode,
		ResponseCode: paymentStatus.ResponseCode,
		Message:      paymentStatus.Message,
		AdminFee:     paymentStatus.AdminFee,
		TotalAmount:  paymentStatus.TotalAmount,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"), // Current timestamp for check status response
		BillCount:    paymentStatus.BillCount,
		BillDetails:  paymentStatus.BillDetails,
	}

	u.logger.Info("CheckStatus completed successfully",
		"ref_id", response.RefID,
		"partner_ref_id", response.PartnerRefID,
		"response_code", response.ResponseCode,
		"bill_count", response.BillCount)

	return response, nil
}
