package usecase

import (
	"context"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
)

// PostgresUsecase defines the interface for PostgreSQL business logic
type PostgresUsecase interface {
	InsertBillDetail(ctx context.Context, req *domain.BillDetailInsertRequest) (*domain.BillDetailInsertResponse, error)
	InsertInquiryRequest(ctx context.Context, req *domain.InquiryRequestInsertRequest) (*domain.InquiryRequestInsertResponse, error)
	InsertInquiryResponse(ctx context.Context, req *domain.InquiryResponseInsertRequest) (*domain.InquiryResponseInsertResponse, error)
}

// postgresUsecase implements PostgresUsecase interface
type postgresUsecase struct {
	postgresRepo domain.PostgresInquiryRepository
	logger       service.Logger
}

// NewPostgresUsecase creates a new PostgresUsecase with the given dependencies
func NewPostgresUsecase(postgresRepo domain.PostgresInquiryRepository, logger service.Logger) PostgresUsecase {
	return &postgresUsecase{
		postgresRepo: postgresRepo,
		logger:       logger,
	}
}

// InsertBillDetail handles the business logic for inserting bill detail
func (u *postgresUsecase) InsertBillDetail(ctx context.Context, req *domain.BillDetailInsertRequest) (*domain.BillDetailInsertResponse, error) {
	u.logger.Info("Starting InsertBillDetail usecase",
		"pps_inquiry_id", req.PpsInquiryID,
		"name", req.Name)

	// Validate input parameters
	if req.PpsInquiryID == "" {
		u.logger.Error("PpsInquiryID is required")
		return nil, domain.NewValidationError("pps_inquiry_id is required")
	}
	if req.Name == "" {
		u.logger.Error("Name is required")
		return nil, domain.NewValidationError("name is required")
	}

	// Call repository to execute stored procedure
	result, err := u.postgresRepo.InsertBillDetail(ctx, req)
	if err != nil {
		u.logger.Error("Failed to insert bill detail",
			"error", err,
			"pps_inquiry_id", req.PpsInquiryID)
		return nil, err
	}

	u.logger.Info("Bill detail inserted successfully",
		"bill_detail_id", result.BillDetailID,
		"error_code", result.Error,
		"message", result.Message)

	return result, nil
}

// InsertInquiryRequest handles the business logic for inserting inquiry request
func (u *postgresUsecase) InsertInquiryRequest(ctx context.Context, req *domain.InquiryRequestInsertRequest) (*domain.InquiryRequestInsertResponse, error) {
	u.logger.Info("Starting InsertInquiryRequest usecase",
		"ref_id", req.RefID,
		"client_number", req.ClientNumber)

	// Validate input parameters
	/*if req.Category == "" {
		u.logger.Error("Category is required")
		return nil, domain.NewValidationError("category is required")
	}
	if req.Rsid == "" {
		u.logger.Error("Rsid is required")
		return nil, domain.NewValidationError("rsid is required")
	}
	if req.ProductCode == "" {
		u.logger.Error("ProductCode is required")
		return nil, domain.NewValidationError("product_code is required")
	}
	if req.Timestamp == "" {
		u.logger.Error("Timestamp is required")
		return nil, domain.NewValidationError("timestamp is required")
	}
	*/
	// Call repository to execute stored procedure
	result, err := u.postgresRepo.InsertInquiryRequest(ctx, req)
	if err != nil {
		u.logger.Error("Failed to insert inquiry request",
			"error", err,
			"ref_id", req.RefID)
		return nil, err
	}

	u.logger.Info("Inquiry request inserted successfully",
		"inquiry_request_id", result.InquiryRequestID,
		"error_code", result.Error,
		"message", result.Message)

	return result, nil
}

// InsertInquiryResponse handles the business logic for inserting inquiry response
func (u *postgresUsecase) InsertInquiryResponse(ctx context.Context, req *domain.InquiryResponseInsertRequest) (*domain.InquiryResponseInsertResponse, error) {
	u.logger.Info("Starting InsertInquiryResponse usecase",
		"inquiry_request_id", req.InquiryRequestID,
		"pps_inquiry_id", req.PpsInquiryID)

	// Validate input parameters
	// Note: InquiryRequestID can be 0 for best-effort logging without correlation
	if req.PpsInquiryID == "" {
		u.logger.Error("PpsInquiryID is required")
		return nil, domain.NewValidationError("pps_inquiry_id is required")
	}
	if req.ClientNumber == "" {
		u.logger.Error("ClientNumber is required")
		return nil, domain.NewValidationError("client_number is required")
	}
	if req.ProductCode == "" {
		u.logger.Error("ProductCode is required")
		return nil, domain.NewValidationError("product_code is required")
	}
	if req.ResponseCode == "" {
		u.logger.Error("ResponseCode is required")
		return nil, domain.NewValidationError("response_code is required")
	}
	if req.Message == "" {
		u.logger.Error("Message is required")
		return nil, domain.NewValidationError("message is required")
	}
	if req.Timestamp == "" {
		u.logger.Error("Timestamp is required")
		return nil, domain.NewValidationError("timestamp is required")
	}

	// Call repository to execute stored procedure
	result, err := u.postgresRepo.InsertInquiryResponse(ctx, req)
	if err != nil {
		u.logger.Error("Failed to insert inquiry response",
			"error", err,
			"inquiry_request_id", req.InquiryRequestID)
		return nil, err
	}

	u.logger.Info("Inquiry response inserted successfully",
		"inquiry_response_id", result.InquiryResponseID,
		"error_code", result.Error,
		"message", result.Message)

	return result, nil
}
