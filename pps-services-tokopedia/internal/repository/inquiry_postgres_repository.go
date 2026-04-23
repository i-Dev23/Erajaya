package repository

import (
	"context"
	"fmt"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
	"time"
)

// PostgresInquiryRepository implements domain.PostgresInquiryRepository using PostgresService.
type PostgresInquiryRepository struct {
	postgresService service.PostgresService
	logger          service.Logger
}

// NewPostgresInquiryRepository creates a new PostgresInquiryRepository with the given PostgresService.
func NewPostgresInquiryRepository(postgresService service.PostgresService, logger service.Logger) domain.PostgresInquiryRepository {
	return &PostgresInquiryRepository{
		postgresService: postgresService,
		logger:          logger,
	}
}

func (r *PostgresInquiryRepository) CheckRefIDExists(ctx context.Context, refID string) (bool, error) {
	// Check if ref_id already exists in inquiry_requests table
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM "inquiry".inquiry_requests
			WHERE ref_id = $1
			LIMIT 1
		)
	`

	r.logger.Info("Checking if ref_id exists",
		"ref_id", refID)

	rows, err := r.postgresService.Query(ctx, query, refID)
	if err != nil {
		r.logger.Error("Failed to check ref_id existence", "error", err, "ref_id", refID)
		return false, fmt.Errorf("failed to check ref_id existence: %w", err)
	}
	defer rows.Close()

	var exists bool
	if rows.Next() {
		err = rows.Scan(&exists)
		if err != nil {
			r.logger.Error("Failed to scan ref_id check result", "error", err)
			return false, fmt.Errorf("failed to scan ref_id check result: %w", err)
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating ref_id check results", "error", err)
		return false, fmt.Errorf("error iterating ref_id check results: %w", err)
	}

	if exists {
		r.logger.Warn("Duplicate ref_id found", "ref_id", refID)
	} else {
		r.logger.Info("ref_id is unique", "ref_id", refID)
	}

	return exists, nil
}

func (r *PostgresInquiryRepository) ValidateInquiryId(ctx context.Context, inquiryRequestId string, productCode string, clientNumber string) error {
	// Use EXISTS instead of COUNT(*) for better performance with large tables (200M+ rows)
	// EXISTS stops at first match, COUNT(*) scans all matching rows
	query := `
		SELECT EXISTS (
			SELECT 1
			FROM "inquiry".inquiry_responses 
			WHERE pps_inquiry_id = $1 
			  AND product_code = $2 
			  AND client_number = $3
			LIMIT 1
		)
	`

	r.logger.Info("Validating inquiry id",
		"pps_inquiry_id", inquiryRequestId,
		"product_code", productCode,
		"client_number", clientNumber)

	rows, err := r.postgresService.Query(ctx, query, inquiryRequestId, productCode, clientNumber)
	if err != nil {
		r.logger.Error("Failed to query inquiry responses for validation", "error", err)
		return fmt.Errorf("failed to validate inquiry id: %w", err)
	}
	defer rows.Close()

	var exists bool
	if rows.Next() {
		err = rows.Scan(&exists)
		if err != nil {
			r.logger.Error("Failed to scan validation result", "error", err)
			return fmt.Errorf("failed to scan validation result: %w", err)
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating validation results", "error", err)
		return fmt.Errorf("error iterating validation results: %w", err)
	}

	if !exists {
		r.logger.Warn("Inquiry id validation failed - no matching record found",
			"pps_inquiry_id", inquiryRequestId,
			"product_code", productCode,
			"client_number", clientNumber)
		return domain.NewValidationError("inquiry id not found or invalid parameters")
	}

	r.logger.Info("Inquiry id validation successful")
	return nil
}

// InsertBillDetail executes the bill_detail_oninsert stored procedure
func (r *PostgresInquiryRepository) InsertBillDetail(ctx context.Context, req *domain.BillDetailInsertRequest) (*domain.BillDetailInsertResponse, error) {
	query := `SELECT bill_detail_id, error, message FROM "inquiry".bill_detail_oninsert($1, $2, $3, $4, $5)`

	r.logger.Info("Executing bill_detail_oninsert procedure",
		"pps_inquiry_id", req.PpsInquiryID,
		"name", req.Name,
		"value", req.Value,
		"is_pii", req.IsPII,
		"is_show", req.IsShow)

	rows, err := r.postgresService.Query(ctx, query,
		req.PpsInquiryID,
		req.Name,
		req.Value,
		req.IsPII,
		req.IsShow)

	if err != nil {
		r.logger.Error("Failed to execute bill_detail_oninsert procedure",
			"error", err,
			"pps_inquiry_id", req.PpsInquiryID)
		return nil, fmt.Errorf("failed to execute bill_detail_oninsert procedure: %w", err)
	}
	defer rows.Close()

	var billDetailID int64
	var errorCode int
	var message string

	if rows.Next() {
		// Use nullable pointer to handle NULL values from stored procedure
		var nullableID *int64
		err = rows.Scan(&nullableID, &errorCode, &message)
		if err != nil {
			r.logger.Error("Failed to scan bill_detail_oninsert result",
				"error", err,
				"pps_inquiry_id", req.PpsInquiryID)
			return nil, fmt.Errorf("failed to scan bill_detail_oninsert result: %w", err)
		}

		if nullableID != nil {
			billDetailID = *nullableID
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating bill_detail_oninsert results",
			"error", err,
			"pps_inquiry_id", req.PpsInquiryID)
		return nil, fmt.Errorf("error iterating bill_detail_oninsert results: %w", err)
	}

	// Check if there was an error from the stored procedure
	if errorCode != 0 {
		r.logger.Error("bill_detail_oninsert procedure returned error",
			"bill_detail_id", billDetailID,
			"error_code", errorCode,
			"message", message,
			"pps_inquiry_id", req.PpsInquiryID,
			"name", req.Name)
		return &domain.BillDetailInsertResponse{
			BillDetailID: billDetailID,
			Error:        errorCode,
			Message:      message,
		}, nil // Return response with error info, don't fail the whole request
	}

	r.logger.Info("bill_detail_oninsert procedure executed successfully",
		"bill_detail_id", billDetailID,
		"error_code", errorCode,
		"message", message)

	return &domain.BillDetailInsertResponse{
		BillDetailID: billDetailID,
		Error:        errorCode,
		Message:      message,
	}, nil
}

// InsertInquiryRequest executes the inquiry_request_oninsert stored procedure
func (r *PostgresInquiryRepository) InsertInquiryRequest(ctx context.Context, req *domain.InquiryRequestInsertRequest) (*domain.InquiryRequestInsertResponse, error) {

	// Use actual values from request, or set defaults if empty
	if req.RefID == "" {
		// Generate unique ref_id if not provided (include nanoseconds for uniqueness)
		now := time.Now()
		req.RefID = fmt.Sprintf("REF-%s-%d", now.Format("20060102-150405"), now.Nanosecond()/1000)
	}
	if req.Timestamp == "" {
		req.Timestamp = time.Now().Format("2006-01-02 15:04:05")
	}

	query := `SELECT inquiry_request_id, error, message FROM "inquiry".inquiry_request_oninsert($1, $2, $3, $4, $5, $6)`

	r.logger.Info("Executing inquiry_request_oninsert procedure",
		"ref_id", req.RefID,
		"client_number", req.ClientNumber,
		"category", req.Category,
		"rsid", req.Rsid,
		"product_code", req.ProductCode,
		"timestamp", req.Timestamp)

	// Convert empty strings to NULL for database compatibility
	var refID, clientNumber, category, rsid, productCode, timestamp interface{}

	if req.RefID != "" {
		refID = req.RefID
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

	var inquiryRequestID int64
	var errorCode int
	var message string

	rows, err := r.postgresService.Query(ctx, query,
		refID,
		clientNumber,
		category,
		rsid,
		productCode,
		timestamp)

	if err != nil {
		r.logger.Error("Failed to execute inquiry_request_oninsert procedure",
			"error", err,
			"ref_id", req.RefID)
		return nil, fmt.Errorf("failed to execute inquiry_request_oninsert procedure: %w", err)
	}
	defer rows.Close()

	if rows.Next() {
		// Use sql.NullInt64 to handle NULL values from stored procedure
		var nullableID *int64
		err = rows.Scan(&nullableID, &errorCode, &message)
		if err != nil {
			r.logger.Error("Failed to scan inquiry_request_oninsert result",
				"error", err,
				"ref_id", req.RefID)
			return nil, fmt.Errorf("failed to scan inquiry_request_oninsert result: %w", err)
		}

		if nullableID != nil {
			inquiryRequestID = *nullableID
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating inquiry_request_oninsert results",
			"error", err,
			"ref_id", req.RefID)
		return nil, fmt.Errorf("error iterating inquiry_request_oninsert results: %w", err)
	}

	// Check if there was an error from the stored procedure
	if errorCode != 0 {
		r.logger.Error("inquiry_request_oninsert procedure returned error",
			"inquiry_request_id", inquiryRequestID,
			"error_code", errorCode,
			"message", message,
			"ref_id", req.RefID)
		return &domain.InquiryRequestInsertResponse{
			InquiryRequestID: inquiryRequestID,
			Error:            errorCode,
			Message:          message,
		}, nil // Return response with error info, don't fail the whole request
	}

	r.logger.Info("inquiry_request_oninsert procedure executed successfully",
		"inquiry_request_id", inquiryRequestID,
		"error_code", errorCode,
		"message", message)

	return &domain.InquiryRequestInsertResponse{
		InquiryRequestID: inquiryRequestID,
		Error:            errorCode,
		Message:          message,
	}, nil
}

// InsertInquiryResponse executes the inquiry_response_oninsert stored procedure
func (r *PostgresInquiryRepository) InsertInquiryResponse(ctx context.Context, req *domain.InquiryResponseInsertRequest) (*domain.InquiryResponseInsertResponse, error) {
	query := `SELECT inquiry_response_id, error, message FROM "inquiry".inquiry_response_oninsert($1, $2, $3, $4, $5, $6, $7, $8, $9)`

	r.logger.Info("Executing inquiry_response_oninsert procedure",
		"inquiry_request_id", req.InquiryRequestID,
		"pps_inquiry_id", req.PpsInquiryID,
		"client_number", req.ClientNumber,
		"product_code", req.ProductCode,
		"response_code", req.ResponseCode,
		"message", req.Message,
		"total_amount", req.TotalAmount,
		"timestamp", req.Timestamp,
		"bill_count", req.BillCount)

	// Handle nullable inquiry_request_id
	var inquiryRequestID interface{}
	if req.InquiryRequestID > 0 {
		inquiryRequestID = req.InquiryRequestID
	} else {
		inquiryRequestID = nil
	}

	rows, err := r.postgresService.Query(ctx, query,
		inquiryRequestID,
		req.PpsInquiryID,
		req.ClientNumber,
		req.ProductCode,
		req.ResponseCode,
		req.Message,
		req.TotalAmount,
		req.Timestamp,
		req.BillCount)

	if err != nil {
		r.logger.Error("Failed to execute inquiry_response_oninsert procedure",
			"error", err,
			"inquiry_request_id", req.InquiryRequestID)
		return nil, fmt.Errorf("failed to execute inquiry_response_oninsert procedure: %w", err)
	}
	defer rows.Close()

	var inquiryResponseID int64
	var errorCode int
	var message string

	if rows.Next() {
		// Use nullable pointer to handle NULL values from stored procedure
		var nullableID *int64
		err = rows.Scan(&nullableID, &errorCode, &message)
		if err != nil {
			r.logger.Error("Failed to scan inquiry_response_oninsert result",
				"error", err,
				"inquiry_request_id", req.InquiryRequestID)
			return nil, fmt.Errorf("failed to scan inquiry_response_oninsert result: %w", err)
		}

		if nullableID != nil {
			inquiryResponseID = *nullableID
		}
	}

	if err = rows.Err(); err != nil {
		r.logger.Error("Error iterating inquiry_response_oninsert results",
			"error", err,
			"inquiry_request_id", req.InquiryRequestID)
		return nil, fmt.Errorf("error iterating inquiry_response_oninsert results: %w", err)
	}

	// Check if there was an error from the stored procedure
	if errorCode != 0 {
		r.logger.Error("inquiry_response_oninsert procedure returned error",
			"inquiry_response_id", inquiryResponseID,
			"error_code", errorCode,
			"message", message,
			"pps_inquiry_id", req.PpsInquiryID)
		return &domain.InquiryResponseInsertResponse{
			InquiryResponseID: inquiryResponseID,
			Error:             errorCode,
			Message:           message,
		}, nil // Return response with error info, don't fail the whole request
	}

	r.logger.Info("inquiry_response_oninsert procedure executed successfully",
		"inquiry_response_id", inquiryResponseID,
		"error_code", errorCode,
		"message", message)

	return &domain.InquiryResponseInsertResponse{
		InquiryResponseID: inquiryResponseID,
		Error:             errorCode,
		Message:           message,
	}, nil
}

// GetBillDetailsByInquiryID retrieves all bill details for a given inquiry ID
func (r *PostgresInquiryRepository) GetBillDetailsByInquiryID(ctx context.Context, ppsInquiryID string) ([]domain.InquiryBillDetail, error) {
	query := `
		SELECT name, value, is_pii, is_show
		FROM "inquiry".bill_details
		WHERE pps_inquiry_id = $1
		ORDER BY id ASC
	`

	r.logger.Info("Fetching bill details for inquiry",
		"pps_inquiry_id", ppsInquiryID)

	rows, err := r.postgresService.Query(ctx, query, ppsInquiryID)
	if err != nil {
		r.logger.Error("Failed to fetch bill details", "error", err, "pps_inquiry_id", ppsInquiryID)
		return nil, fmt.Errorf("failed to fetch bill details: %w", err)
	}
	defer rows.Close()

	var billDetails []domain.InquiryBillDetail
	for rows.Next() {
		var detail domain.InquiryBillDetail
		err = rows.Scan(&detail.Name, &detail.Value, &detail.IsPII, &detail.IsShow)
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

	r.logger.Info("Bill details fetched successfully",
		"pps_inquiry_id", ppsInquiryID,
		"count", len(billDetails))

	return billDetails, nil
}

// GetInquiryByPartnerInquiryID fetches inquiry data by pps_inquiry_id for validation
func (r *PostgresInquiryRepository) GetInquiryByPartnerInquiryID(ctx context.Context, partnerInquiryID string) (*domain.InquiryData, error) {
	query := `
		SELECT
			resp.inquiry_request_id,
			resp.client_number,
			resp.product_code,
			resp.total_amount,
			resp.response_code
		FROM "inquiry".inquiry_responses resp
		WHERE resp.pps_inquiry_id = $1
		ORDER BY resp.id DESC
		LIMIT 1
	`

	r.logger.Debug("Fetching inquiry data by pps_inquiry_id",
		"pps_inquiry_id", partnerInquiryID)

	rows, err := r.postgresService.Query(ctx, query, partnerInquiryID)
	if err != nil {
		r.logger.Error("Failed to fetch inquiry data",
			"error", err,
			"pps_inquiry_id", partnerInquiryID)
		return nil, fmt.Errorf("failed to fetch inquiry data: %w", err)
	}
	defer rows.Close()

	var inquiryData domain.InquiryData
	if rows.Next() {
		err = rows.Scan(
			&inquiryData.PartnerInquiryID,
			&inquiryData.ClientNumber,
			&inquiryData.ProductCode,
			&inquiryData.TotalAmount,
			&inquiryData.ResponseCode,
		)
		if err != nil {
			r.logger.Error("Failed to scan inquiry data",
				"error", err,
				"partner_inquiry_id", partnerInquiryID)
			return nil, fmt.Errorf("failed to scan inquiry data: %w", err)
		}
		return &inquiryData, nil
	}

	r.logger.Debug("Inquiry not found by partner_inquiry_id",
		"partner_inquiry_id", partnerInquiryID)

	return nil, nil
}
