package repository

import (
	"context"
	"fmt"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
)

// PostgresPaymentRepository implements domain.PostgresPaymentRepository using PostgresService.
type PostgresPaymentRepository struct {
	postgresService service.PostgresService
	logger          service.Logger
}

// NewPostgresPaymentRepository creates a new PostgresPaymentRepository with the given PostgresService.
func NewPostgresPaymentRepository(postgresService service.PostgresService, logger service.Logger) domain.PostgresPaymentRepository {
	return &PostgresPaymentRepository{
		postgresService: postgresService,
		logger:          logger,
	}
}

// CheckRefIDExists checks if a ref_id already exists in the payment_requests table
func (r *PostgresPaymentRepository) CheckRefIDExists(ctx context.Context, refID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM "payment".payment_requests
			WHERE ref_id = $1
			LIMIT 1
		)
	`

	r.logger.Info("Checking if payment ref_id exists",
		"ref_id", refID)

	rows, err := r.postgresService.Query(ctx, query, refID)
	if err != nil {
		r.logger.Error("Failed to check payment ref_id existence", "error", err, "ref_id", refID)
		return false, fmt.Errorf("failed to check payment ref_id existence: %w", err)
	}
	defer rows.Close()

	var exists bool
	if rows.Next() {
		err = rows.Scan(&exists)
		if err != nil {
			r.logger.Error("Failed to scan payment ref_id check result", "error", err)
			return false, fmt.Errorf("failed to scan payment ref_id check result: %w", err)
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating payment ref_id check results", "error", err)
		return false, fmt.Errorf("error iterating payment ref_id check results: %w", err)
	}

	if exists {
		r.logger.Warn("Duplicate payment ref_id found", "ref_id", refID)
	} else {
		r.logger.Info("Payment ref_id is unique", "ref_id", refID)
	}

	return exists, nil
}

// CheckPartnerInquiryIDExists checks if a partner_inquiry_id already exists in the payment_requests table
// This ensures one inquiry can only be paid once
func (r *PostgresPaymentRepository) CheckPartnerInquiryIDExists(ctx context.Context, partnerInquiryID string) (bool, error) {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM "payment".payment_requests
			WHERE partner_inquiry_id = $1
			LIMIT 1
		)
	`

	r.logger.Info("Checking if partner_inquiry_id already used in payment",
		"partner_inquiry_id", partnerInquiryID)

	rows, err := r.postgresService.Query(ctx, query, partnerInquiryID)
	if err != nil {
		r.logger.Error("Failed to check partner_inquiry_id existence", "error", err, "partner_inquiry_id", partnerInquiryID)
		return false, fmt.Errorf("failed to check partner_inquiry_id existence: %w", err)
	}
	defer rows.Close()

	var exists bool
	if rows.Next() {
		err = rows.Scan(&exists)
		if err != nil {
			r.logger.Error("Failed to scan partner_inquiry_id check result", "error", err)
			return false, fmt.Errorf("failed to scan partner_inquiry_id check result: %w", err)
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating partner_inquiry_id check results", "error", err)
		return false, fmt.Errorf("error iterating partner_inquiry_id check results: %w", err)
	}

	if exists {
		r.logger.Warn("Inquiry already paid - partner_inquiry_id found in payment_requests", "partner_inquiry_id", partnerInquiryID)
	} else {
		r.logger.Info("Inquiry not yet paid - partner_inquiry_id is unique", "partner_inquiry_id", partnerInquiryID)
	}

	return exists, nil
}

// ValidatePartnerInquiryID validates that the partner_inquiry_id exists in inquiry_responses
// with response_code = '00' (cross-schema query to inquiry schema)
func (r *PostgresPaymentRepository) ValidatePartnerInquiryID(ctx context.Context, partnerInquiryID string) error {
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM "inquiry".inquiry_responses 
			WHERE pps_inquiry_id = $1 
			  AND response_code = '00'
			LIMIT 1
		)
	`

	r.logger.Info("Validating partner inquiry id for payment",
		"partner_inquiry_id", partnerInquiryID,
		"required_response_code", "00")

	rows, err := r.postgresService.Query(ctx, query, partnerInquiryID)
	if err != nil {
		r.logger.Error("Failed to query inquiry responses for payment validation", "error", err)
		return fmt.Errorf("failed to validate partner inquiry id: %w", err)
	}
	defer rows.Close()

	var exists bool
	if rows.Next() {
		err = rows.Scan(&exists)
		if err != nil {
			r.logger.Error("Failed to scan payment validation result", "error", err)
			return fmt.Errorf("failed to scan payment validation result: %w", err)
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating payment validation results", "error", err)
		return fmt.Errorf("error iterating payment validation results: %w", err)
	}

	if !exists {
		r.logger.Warn("Partner inquiry id validation failed - no matching record found or response_code not '00'",
			"partner_inquiry_id", partnerInquiryID,
			"required_response_code", "00")
		return domain.NewValidationError("partner inquiry id not found or inquiry not successful")
	}

	r.logger.Info("Partner inquiry id validation successful")
	return nil
}

// InsertPaymentBillDetail executes the payment_bill_detail_oninsert stored procedure
func (r *PostgresPaymentRepository) InsertPaymentBillDetail(ctx context.Context, req *domain.PaymentBillDetailInsertRequest) (*domain.PaymentBillDetailInsertResponse, error) {
	query := `SELECT payment_bill_detail_id, error, message FROM "payment".payment_bill_detail_oninsert($1, $2, $3, $4, $5)`

	r.logger.Info("Executing payment_bill_detail_oninsert procedure",
		"partner_ref_id", req.PartnerRefID,
		"name", req.Name,
		"value", req.Value,
		"is_pii", req.IsPII,
		"is_show", req.IsShow)

	rows, err := r.postgresService.Query(ctx, query,
		req.PartnerRefID,
		req.Name,
		req.Value,
		req.IsPII,
		req.IsShow)

	if err != nil {
		r.logger.Error("Failed to execute payment_bill_detail_oninsert procedure",
			"error", err,
			"partner_ref_id", req.PartnerRefID)
		return nil, fmt.Errorf("failed to execute payment_bill_detail_oninsert procedure: %w", err)
	}
	defer rows.Close()

	var paymentBillDetailID int64
	var errorCode int
	var message string

	if rows.Next() {
		// Use nullable pointer to handle NULL values from stored procedure
		var nullableID *int64
		err = rows.Scan(&nullableID, &errorCode, &message)
		if err != nil {
			r.logger.Error("Failed to scan payment_bill_detail_oninsert result",
				"error", err,
				"partner_ref_id", req.PartnerRefID)
			return nil, fmt.Errorf("failed to scan payment_bill_detail_oninsert result: %w", err)
		}

		if nullableID != nil {
			paymentBillDetailID = *nullableID
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating payment_bill_detail_oninsert results",
			"error", err,
			"partner_ref_id", req.PartnerRefID)
		return nil, fmt.Errorf("error iterating payment_bill_detail_oninsert results: %w", err)
	}

	// Check if there was an error from the stored procedure
	if errorCode != 0 {
		r.logger.Error("payment_bill_detail_oninsert procedure returned error",
			"payment_bill_detail_id", paymentBillDetailID,
			"error_code", errorCode,
			"message", message,
			"partner_ref_id", req.PartnerRefID,
			"name", req.Name)
		return &domain.PaymentBillDetailInsertResponse{
			PaymentBillDetailID: paymentBillDetailID,
			Error:               errorCode,
			Message:             message,
		}, nil // Return response with error info, don't fail the whole request
	}

	r.logger.Info("payment_bill_detail_oninsert procedure executed successfully",
		"payment_bill_detail_id", paymentBillDetailID,
		"error_code", errorCode,
		"message", message)

	return &domain.PaymentBillDetailInsertResponse{
		PaymentBillDetailID: paymentBillDetailID,
		Error:               errorCode,
		Message:             message,
	}, nil
}

// InsertPaymentRequest executes the payment_request_oninsert stored procedure
func (r *PostgresPaymentRepository) InsertPaymentRequest(ctx context.Context, req *domain.PaymentRequestInsertRequest) (*domain.PaymentRequestInsertResponse, error) {
	query := `SELECT payment_request_id, error, message FROM "payment".payment_request_oninsert($1, $2, $3, $4, $5, $6, $7, $8)`

	r.logger.Info("Executing payment_request_oninsert procedure",
		"ref_id", req.RefID,
		"partner_inquiry_id", req.PartnerInquiryID,
		"client_number", req.ClientNumber,
		"category", req.Category,
		"rsid", req.Rsid,
		"product_code", req.ProductCode,
		"total_amount", req.TotalAmount,
		"timestamp", req.Timestamp)

	// Convert empty strings to NULL for database compatibility
	var refID, partnerInquiryID, clientNumber, category, rsid, productCode, timestamp interface{}

	if req.RefID != "" {
		refID = req.RefID
	}
	if req.PartnerInquiryID != "" {
		partnerInquiryID = req.PartnerInquiryID
	}
	if req.ClientNumber != "" {
		clientNumber = req.ClientNumber
	}
	if req.Category != "" {
		category = req.Category
	}
	if req.Rsid != "" {
		rsid = req.Rsid
	}
	if req.ProductCode != "" {
		productCode = req.ProductCode
	}
	if req.Timestamp != "" {
		timestamp = req.Timestamp
	}

	var paymentRequestID int64
	var errorCode int
	var message string

	rows, err := r.postgresService.Query(ctx, query,
		refID,
		partnerInquiryID,
		clientNumber,
		category,
		rsid,
		productCode,
		req.TotalAmount,
		timestamp)

	if err != nil {
		r.logger.Error("Failed to execute payment_request_oninsert procedure",
			"error", err,
			"ref_id", req.RefID)
		return nil, fmt.Errorf("failed to execute payment_request_oninsert procedure: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		// Use nullable pointer to handle NULL values from stored procedure
		var nullableID *int64
		err = rows.Scan(&nullableID, &errorCode, &message)
		if err != nil {
			r.logger.Error("Failed to scan payment_request_oninsert result",
				"error", err,
				"ref_id", req.RefID)
			return nil, fmt.Errorf("failed to scan payment_request_oninsert result: %w", err)
		}

		if nullableID != nil {
			paymentRequestID = *nullableID
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating payment_request_oninsert results",
			"error", err,
			"ref_id", req.RefID)
		return nil, fmt.Errorf("error iterating payment_request_oninsert results: %w", err)
	}

	// Check if there was an error from the stored procedure
	if errorCode != 0 {
		r.logger.Error("payment_request_oninsert procedure returned error",
			"payment_request_id", paymentRequestID,
			"error_code", errorCode,
			"message", message,
			"ref_id", req.RefID)
		return &domain.PaymentRequestInsertResponse{
			PaymentRequestID: paymentRequestID,
			Error:            errorCode,
			Message:          message,
		}, nil // Return response with error info, don't fail the whole request
	}

	r.logger.Info("payment_request_oninsert procedure executed successfully",
		"payment_request_id", paymentRequestID,
		"error_code", errorCode,
		"message", message)

	return &domain.PaymentRequestInsertResponse{
		PaymentRequestID: paymentRequestID,
		Error:            errorCode,
		Message:          message,
	}, nil
}

// InsertPaymentResponse executes the payment_response_oninsert stored procedure
func (r *PostgresPaymentRepository) InsertPaymentResponse(ctx context.Context, req *domain.PaymentResponseInsertRequest) (*domain.PaymentResponseInsertResponse, error) {
	query := `SELECT payment_response_id, error, message FROM "payment".payment_response_oninsert($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`

	r.logger.Info("Executing payment_response_oninsert procedure",
		"payment_request_id", req.PaymentRequestID,
		"partner_ref_id", req.PartnerRefID,
		"client_number", req.ClientNumber,
		"product_code", req.ProductCode,
		"response_code", req.ResponseCode,
		"message", req.Message,
		"admin_fee", req.AdminFee,
		"total_amount", req.TotalAmount,
		"timestamp", req.Timestamp,
		"bill_count", req.BillCount)

	// Handle nullable payment_request_id
	var paymentRequestID interface{}
	if req.PaymentRequestID > 0 {
		paymentRequestID = req.PaymentRequestID
	} else {
		paymentRequestID = nil
	}

	rows, err := r.postgresService.Query(ctx, query,
		paymentRequestID,
		req.PartnerRefID,
		req.ClientNumber,
		req.ProductCode,
		req.ResponseCode,
		req.Message,
		req.AdminFee,
		req.TotalAmount,
		req.Timestamp,
		req.BillCount)

	if err != nil {
		r.logger.Error("Failed to execute payment_response_oninsert procedure",
			"error", err,
			"payment_request_id", req.PaymentRequestID)
		return nil, fmt.Errorf("failed to execute payment_response_oninsert procedure: %w", err)
	}
	defer rows.Close()

	var paymentResponseID int64
	var errorCode int
	var message string

	if rows.Next() {
		// Use nullable pointer to handle NULL values from stored procedure
		var nullableID *int64
		err = rows.Scan(&nullableID, &errorCode, &message)
		if err != nil {
			r.logger.Error("Failed to scan payment_response_oninsert result",
				"error", err,
				"payment_request_id", req.PaymentRequestID)
			return nil, fmt.Errorf("failed to scan payment_response_oninsert result: %w", err)
		}

		if nullableID != nil {
			paymentResponseID = *nullableID
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating payment_response_oninsert results",
			"error", err,
			"payment_request_id", req.PaymentRequestID)
		return nil, fmt.Errorf("error iterating payment_response_oninsert results: %w", err)
	}

	// Check if there was an error from the stored procedure
	if errorCode != 0 {
		r.logger.Error("payment_response_oninsert procedure returned error",
			"payment_response_id", paymentResponseID,
			"error_code", errorCode,
			"message", message,
			"partner_ref_id", req.PartnerRefID)
		return &domain.PaymentResponseInsertResponse{
			PaymentResponseID: paymentResponseID,
			Error:             errorCode,
			Message:           message,
		}, nil // Return response with error info, don't fail the whole request
	}

	r.logger.Info("payment_response_oninsert procedure executed successfully",
		"payment_response_id", paymentResponseID,
		"error_code", errorCode,
		"message", message)

	return &domain.PaymentResponseInsertResponse{
		PaymentResponseID: paymentResponseID,
		Error:             errorCode,
		Message:           message,
	}, nil
}

// GetPaymentStatusByRefID retrieves payment status information by ref_id
func (r *PostgresPaymentRepository) GetPaymentStatusByRefID(ctx context.Context, refID string) (*domain.PaymentStatusResult, error) {
	// Query to get payment request and response joined data
	query := `
        SELECT 
            pr.ref_id,
            COALESCE(pres.partner_ref_id, '') as partner_ref_id,
            pr.client_number,
            pr.product_code,
            COALESCE(pres.response_code, '12') as response_code,
            COALESCE(pres.message, 'Transaction not found') as message,
            COALESCE(pres.admin_fee, 0) as admin_fee,
            pr.total_amount,
            COALESCE(pres.ts, pr.ts)::text as timestamp,
            COALESCE(pres.bill_count, 0) as bill_count
        FROM payment.payment_requests pr
        LEFT JOIN payment.payment_responses pres ON pres.payment_request_id = pr.id
        WHERE pr.ref_id = $1
        ORDER BY pres.created_at DESC
        LIMIT 1
    `

	r.logger.Info("Getting payment status by ref_id", "ref_id", refID)

	rows, err := r.postgresService.Query(ctx, query, refID)
	if err != nil {
		r.logger.Error("Failed to query payment status", "error", err, "ref_id", refID)
		return nil, fmt.Errorf("failed to query payment status: %w", err)
	}
	defer rows.Close()

	var result domain.PaymentStatusResult

	if rows.Next() {
		err = rows.Scan(
			&result.RefID,
			&result.PartnerRefID,
			&result.ClientNumber,
			&result.ProductCode,
			&result.ResponseCode,
			&result.Message,
			&result.AdminFee,
			&result.TotalAmount,
			&result.Timestamp,
			&result.BillCount,
		)
		if err != nil {
			r.logger.Error("Failed to scan payment status result", "error", err)
			return nil, fmt.Errorf("failed to scan payment status result: %w", err)
		}
	} else {
		// No record found
		r.logger.Warn("Payment not found", "ref_id", refID)
		return nil, fmt.Errorf("payment not found for ref_id: %s", refID)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating payment status results", "error", err)
		return nil, fmt.Errorf("error iterating payment status results: %w", err)
	}

	// Get bill details if bill_count > 0
	if result.BillCount > 0 && result.PartnerRefID != "" {
		billDetails, err := r.getBillDetailsByPartnerRefID(ctx, result.PartnerRefID)
		if err != nil {
			r.logger.Warn("Failed to get bill details, continuing without them",
				"error", err,
				"partner_ref_id", result.PartnerRefID)
			// Don't fail the whole request if bill details can't be retrieved
			result.BillDetails = []domain.BillDetailDomain{}
		} else {
			result.BillDetails = billDetails
		}
	} else {
		result.BillDetails = []domain.BillDetailDomain{}
	}

	r.logger.Info("Payment status retrieved successfully",
		"ref_id", result.RefID,
		"partner_ref_id", result.PartnerRefID,
		"response_code", result.ResponseCode,
		"bill_count", result.BillCount)

	return &result, nil
}

// getBillDetailsByPartnerRefID retrieves bill details for a given partner_ref_id
func (r *PostgresPaymentRepository) getBillDetailsByPartnerRefID(ctx context.Context, partnerRefID string) ([]domain.BillDetailDomain, error) {
	query := `
		SELECT name, value, is_pii, is_show
		FROM payment.payment_bill_details
		WHERE partner_ref_id = $1
		ORDER BY id ASC
	`

	r.logger.Info("Getting bill details by partner_ref_id", "partner_ref_id", partnerRefID)

	rows, err := r.postgresService.Query(ctx, query, partnerRefID)
	if err != nil {
		r.logger.Error("Failed to query bill details", "error", err, "partner_ref_id", partnerRefID)
		return nil, fmt.Errorf("failed to query bill details: %w", err)
	}
	defer rows.Close()

	var billDetails []domain.BillDetailDomain

	for rows.Next() {
		var detail domain.BillDetailDomain
		err = rows.Scan(
			&detail.Name,
			&detail.Value,
			&detail.IsPII,
			&detail.IsShow,
		)
		if err != nil {
			r.logger.Error("Failed to scan bill detail", "error", err)
			return nil, fmt.Errorf("failed to scan bill detail: %w", err)
		}
		billDetails = append(billDetails, detail)
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating bill details", "error", err)
		return nil, fmt.Errorf("error iterating bill details: %w", err)
	}

	r.logger.Info("Bill details retrieved successfully",
		"partner_ref_id", partnerRefID,
		"count", len(billDetails))

	return billDetails, nil
}
