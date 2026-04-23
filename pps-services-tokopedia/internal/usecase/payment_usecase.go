package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"pps-services-tokopedia/internal/config"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"strings"
	"time"
)

// paymentUsecaseImpl implements the domain.PaymentUsecase interface
type paymentUsecaseImpl struct {
	config              *config.Config
	logger              service.Logger
	redisClient         service.RedisClient
	productRepo         domain.ProductRepository
	preorderRepo        domain.PreorderRepository
	cutOffRepo          domain.CutOffRepository
	postgresInquiryRepo domain.PostgresInquiryRepository
	postgresPaymentRepo domain.PostgresPaymentRepository // Add payment repository
	rabbitMQService     service.RabbitMQService
	errorMappingRepo    domain.ErrorMessageMappingRepository
}

// NewPaymentUsecase creates a new PaymentUsecase
func NewPaymentUsecase(
	config *config.Config,
	logger service.Logger,
	redisClient service.RedisClient,
	productRepo domain.ProductRepository,
	preorderRepo domain.PreorderRepository,
	cutOffRepo domain.CutOffRepository,
	postgresInquiryRepo domain.PostgresInquiryRepository,
	postgresPaymentRepo domain.PostgresPaymentRepository,
	errorMappingRepo domain.ErrorMessageMappingRepository,
	rabbitMQService service.RabbitMQService,
) domain.PaymentUsecase {
	return &paymentUsecaseImpl{
		config:              config,
		logger:              logger,
		redisClient:         redisClient,
		productRepo:         productRepo,
		preorderRepo:        preorderRepo,
		cutOffRepo:          cutOffRepo,
		postgresInquiryRepo: postgresInquiryRepo,
		postgresPaymentRepo: postgresPaymentRepo,
		errorMappingRepo:    errorMappingRepo,
		rabbitMQService:     rabbitMQService,
	}
}

// Payment implements the domain.PaymentUsecase interface
func (u *paymentUsecaseImpl) Payment(ctx context.Context, req *domain.PaymentRequestDomain) (*domain.PaymentResponseDomain, error) {
	ctx, cancel := withUsecaseTimeout(ctx)
	defer cancel()

	u.logger.Info("Starting payment process",
		"refID", req.RefID,
		"partnerInquiryID", req.PartnerInquiryID,
		"clientNumber", req.ClientNumber,
		"productCode", req.ProductCode,
		"totalAmount", req.TotalAmount)

	// 1. Validate cut off
	// 1a. Validate Redis-based cut off windows
	if response := u.validateCutOffRedis(ctx, req); response != nil {
		return response, nil
	}
	// 1b. Validate repository-based cut off
	if response := u.validateCutOff(ctx, req); response != nil {
		return response, nil
	}

	// 2. Validate mandatory parameters (before any insert)
	if response := u.validateMandatoryParamsBeforeInsert(ctx, req); response != nil {
		// Don't insert for audit trail if mandatory params are missing
		// because it will violate NOT NULL constraints in database
		return response, nil
	}

	// 3. Validate duplicate ref_id (before insert to prevent duplicates)
	if response := u.validateDuplicateRefIDBeforeInsert(ctx, req); response != nil {
		// Insert for audit trail even on duplicate detection
		paymentRequestID, err := u.insertPaymentRequest(ctx, req)
		if err != nil {
			// Database error - return server error code 62
			u.logger.Error("Failed to insert payment request on duplicate ref_id", "error", err)
			return u.buildErrorResponse(req, "62"), err
		}
		if insertErr := u.insertErrorResponse(ctx, paymentRequestID, response, nil); insertErr != nil {
			u.logger.Error("Failed to insert error response on duplicate ref_id", "error", insertErr)
			return u.buildErrorResponse(req, "62"), insertErr
		}
		return response, nil
	}

	// 5. Validate partner inquiry ID exists (cross-schema validation)
	if response := u.validatePartnerInquiryIDBeforeInsert(ctx, req); response != nil {
		// Insert for audit trail even on inquiry validation failure
		paymentRequestID, err := u.insertPaymentRequest(ctx, req)
		if err != nil {
			// Database error - return server error code 62
			u.logger.Error("Failed to insert payment request on inquiry validation", "error", err)
			return u.buildErrorResponse(req, "62"), err
		}
		if insertErr := u.insertErrorResponse(ctx, paymentRequestID, response, nil); insertErr != nil {
			u.logger.Error("Failed to insert error response on inquiry validation", "error", insertErr)
			return u.buildErrorResponse(req, "62"), insertErr
		}
		return response, nil
	}

	// 5.5. Validate payment data matches inquiry (client_number, product_code, amount)
	if response := u.validatePaymentDataMatchesInquiry(ctx, req); response != nil {
		// Insert for audit trail even on data mismatch
		paymentRequestID, err := u.insertPaymentRequest(ctx, req)
		if err != nil {
			// Database error - return server error code 62
			u.logger.Error("Failed to insert payment request on data mismatch", "error", err)
			return u.buildErrorResponse(req, "62"), err
		}
		if insertErr := u.insertErrorResponse(ctx, paymentRequestID, response, nil); insertErr != nil {
			u.logger.Error("Failed to insert error response on data mismatch", "error", insertErr)
			return u.buildErrorResponse(req, "62"), insertErr
		}
		return response, nil
	}

	// 6. Validate inquiry not already paid (one inquiry can only be paid once)
	if response := u.validateInquiryNotPaidBeforeInsert(ctx, req); response != nil {
		// Insert for audit trail even on already paid detection
		paymentRequestID, err := u.insertPaymentRequest(ctx, req)
		if err != nil {
			// Database error - return server error code 62
			u.logger.Error("Failed to insert payment request on already paid", "error", err)
			return u.buildErrorResponse(req, "62"), err
		}
		if insertErr := u.insertErrorResponse(ctx, paymentRequestID, response, nil); insertErr != nil {
			u.logger.Error("Failed to insert error response on already paid", "error", insertErr)
			return u.buildErrorResponse(req, "62"), insertErr
		}
		return response, nil
	}

	// 7. Insert payment request (only after all validations pass)
	paymentRequestID, err := u.insertPaymentRequest(ctx, req)
	if err != nil {
		// Database error - return server error code 62
		u.logger.Error("Failed to insert payment request", "error", err)
		return u.buildErrorResponse(req, "62"), err
	}

	// 8. Validate inquiry ID exists before processing payment (old logic kept)
	err = u.postgresInquiryRepo.ValidateInquiryId(ctx, req.PartnerInquiryID, req.ProductCode, req.ClientNumber)
	if err != nil {
		u.logger.Error("Invalid inquiry ID for payment",
			"error", err,
			"partnerInquiryID", req.PartnerInquiryID,
			"productCode", req.ProductCode,
			"clientNumber", req.ClientNumber)

		// Check if it's a validation error
		var validationErr *domain.ValidationError
		if errors.As(err, &validationErr) {
			response := u.buildErrorResponseWithCodeMessage(req, "12", "Transaction not found")
			if insertErr := u.insertErrorResponse(ctx, paymentRequestID, response, err); insertErr != nil {
				u.logger.Error("Failed to insert error response on inquiry validation", "error", insertErr)
				return u.buildErrorResponse(req, "62"), insertErr
			}
			return response, nil
		}
		return u.buildErrorResponse(req, "62"), err
	}

	// 4. Validate amount matches cached product price
	if response := u.validateAmountMatchesPrice(ctx, req); response != nil {
		if insertErr := u.insertErrorResponse(ctx, paymentRequestID, response, nil); insertErr != nil {
			u.logger.Error("Failed to insert error response on amount validation", "error", insertErr)
			return u.buildErrorResponse(req, "62"), insertErr
		}
		return response, nil
	}

	u.logger.Info("Inquiry ID validation successful", "partnerInquiryID", req.PartnerInquiryID)

	// Generate PPS payment ID
	ppsPaymentID := utils.GenerateTransactionID()

	// Call preorder repository
	// Get client ID from env, default to empty string if not set
	clientID := u.config.TPClientID
	signature := utils.GenerateSignature(req.ClientNumber, req.ProductCode, req.RefID, u.config.TPClientSecret)
	domainReq := &domain.PreorderRequestDomain{
		User:      clientID,
		MDN:       req.ClientNumber,
		Product:   req.ProductCode,
		NoTrx:     req.RefID,
		Signature: signature,
		Addr:      req.ClientIP, // IP address from X-Real-IP header
	}

	// insert preorder to database
	preorderResp, err := u.preorderRepo.Preorder(ctx, domainReq)
	if err != nil {
		u.logger.Error("Failed to create preorder", "error", err)
		// Map Oracle error to response code (DB first, fallback to static map)
		code, message := resolveOracleMapping(ctx, err.Error(), u.errorMappingRepo, u.logger)
		response := u.buildErrorResponseWithCodeMessage(req, code, message)
		if insertErr := u.insertErrorResponse(ctx, paymentRequestID, response, err); insertErr != nil {
			u.logger.Error("Failed to insert error response on preorder failure", "error", insertErr)
			return u.buildErrorResponse(req, "62"), insertErr
		}
		return response, nil
	}

	// Check if Oracle procedure returned error
	if preorderResp.OuterRCode == 1 {
		u.logger.Error("Failed to create preorder - Oracle error",
			"oracle_error", preorderResp.OuterRMsg,
			"outer_r_code", preorderResp.OuterRCode)

		// Map Oracle error message to Tokopedia response code (DB first, fallback to static map)
		code, message := resolveOracleMapping(ctx, preorderResp.OuterRMsg, u.errorMappingRepo, u.logger)

		u.logger.Info("Oracle error mapped",
			"oracle_message", preorderResp.OuterRMsg,
			"mapped_code", code,
			"mapped_message", message)

		response := u.buildErrorResponseWithCodeMessage(req, code, message)
		if insertErr := u.insertErrorResponse(ctx, paymentRequestID, response, fmt.Errorf("oracle error: %s (code: %d)", preorderResp.OuterRMsg, preorderResp.OuterRCode)); insertErr != nil {
			u.logger.Error("Failed to insert error response on Oracle preorder error", "error", insertErr)
			return u.buildErrorResponse(req, "62"), insertErr
		}
		return response, nil
	}

	// update preorder status to database
	preorderStatusResp, err := u.preorderRepo.UpdatePreorderStatus(ctx, preorderResp.ServerId, domain.FLAG_SUCCESS_PUBLISH, preorderResp.OuterRMsg)
	if err != nil {
		u.logger.Error("Failed to update preorder status", "error", err)
		// Map Oracle error to response code (DB first, fallback to static map)
		code, message := resolveOracleMapping(ctx, err.Error(), u.errorMappingRepo, u.logger)
		response := u.buildErrorResponseWithCodeMessage(req, code, message)
		if insertErr := u.insertErrorResponse(ctx, paymentRequestID, response, err); insertErr != nil {
			u.logger.Error("Failed to insert error response on update preorder status failure", "error", insertErr)
			return u.buildErrorResponse(req, "62"), insertErr
		}
		return response, nil
	}

	// Check if Oracle procedure returned error for update status
	if preorderStatusResp.OuterRCode == 1 {
		u.logger.Error("Failed to update preorder status - Oracle error",
			"oracle_error", preorderStatusResp.OuterRMsg,
			"outer_r_code", preorderStatusResp.OuterRCode)

		// Map Oracle error message to Tokopedia response code (DB first, fallback to static map)
		code, message := resolveOracleMapping(ctx, preorderStatusResp.OuterRMsg, u.errorMappingRepo, u.logger)

		u.logger.Info("Oracle error mapped for update status",
			"oracle_message", preorderStatusResp.OuterRMsg,
			"mapped_code", code,
			"mapped_message", message)

		response := u.buildErrorResponseWithCodeMessage(req, code, message)
		if insertErr := u.insertErrorResponse(ctx, paymentRequestID, response, fmt.Errorf("oracle error: %s (code: %d)", preorderStatusResp.OuterRMsg, preorderStatusResp.OuterRCode)); insertErr != nil {
			u.logger.Error("Failed to insert error response on Oracle update status error", "error", insertErr)
			return u.buildErrorResponse(req, "62"), insertErr
		}
		return response, nil
	}

	u.logger.Info("Preorder status updated successfully", "refID", preorderResp.ServerId, "status", domain.FLAG_SUCCESS_PUBLISH, "message", preorderResp.OuterRMsg)

	// Publish payment to RabbitMQ
	u.logger.Info("Publishing payment to RabbitMQ",
		"exchange", "payment",
		"routingKey", "payment",
		"ppsPaymentID", ppsPaymentID,
		"refID", req.RefID,
		"clientNumber", req.ClientNumber)

	// Compose payment message
	//dataPublish := constanta.PRE_ORDER_TYPE + "||" + request.Addr + "||" + request.MDN + "||" + request.NoTrx + "||" + request.Produk + "||" + request.Signature + "||" + ServerIDTrx + "||" + request.User
	dataPublish := domain.PRE_ORDER_TYPE + "||" + domainReq.Addr + "||" + domainReq.MDN + "||" + domainReq.NoTrx + "||" + domainReq.Product + "||" + signature + "||" + preorderResp.ServerId + "||" + domainReq.User

	// Publish payment to RabbitMQ - CRITICAL: payment will fail if MQ publish fails
	err = u.rabbitMQService.Publish(ctx, "payment", "payment", []byte(dataPublish), nil)
	if err != nil {
		u.logger.Error("Failed to publish payment to RabbitMQ (critical, payment set failed)",
			"error", err,
			"ppsPaymentID", ppsPaymentID,
			"refID", req.RefID,
			"clientNumber", req.ClientNumber,
			"message", "Payment failed due to MQ publish error, response_code 62")

		// Set payment as failed with response_code 62
		failResponse := u.buildErrorResponse(req, "62")
		if insertErr := u.insertErrorResponse(ctx, paymentRequestID, failResponse, err); insertErr != nil {
			u.logger.Error("Failed to insert error response on MQ publish failure", "error", insertErr)
		}
		return failResponse, err
	}

	u.logger.Info("Payment successfully published to RabbitMQ",
		"ppsPaymentID", ppsPaymentID,
		"refID", req.RefID,
		"clientNumber", req.ClientNumber)

	// TODO: Implement payment logic
	// 1. Validate payment request
	// 2. Check inquiry exists
	// 3. Process payment with external service (e.g., Ultima, bank, etc.)
	// 4. Update payment status in external system
	// 5. Return payment response

	// Build pending response (placeholder)
	rc, _ := utils.GetResponseCode("01")
	response := &domain.PaymentResponseDomain{
		RefID:        req.RefID,
		PartnerRefID: ppsPaymentID,
		ClientNumber: req.ClientNumber,
		ProductCode:  req.ProductCode,
		ResponseCode: rc.Code,
		Message:      rc.Message,
		AdminFee:     0, // Set based on your business logic
		TotalAmount:  req.TotalAmount,
		Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
		BillCount:    1,
		BillDetails:  []domain.InquiryBillDetail{},
	}

	u.logger.Info("Payment completed successfully",
		"refID", req.RefID,
		"partnerRefID", ppsPaymentID,
		"clientNumber", req.ClientNumber,
		"productCode", req.ProductCode,
		"totalAmount", req.TotalAmount)

	// 7. Insert payment response to database
	if err := u.insertSuccessResponse(ctx, paymentRequestID, response); err != nil {
		// Database error - return server error code 62
		u.logger.Error("Failed to insert success response", "error", err)
		errorResponse := u.buildErrorResponse(req, "62")
		return errorResponse, err
	}

	// 8. Copy bill details from inquiry to payment if status is pending (01)
	if response.ResponseCode == "01" {
		billDetails, err := u.copyBillDetailsFromInquiry(ctx, req.PartnerInquiryID, response.PartnerRefID)
		if err != nil {
			// Database error - return server error code 62
			u.logger.Error("Failed to copy bill details from inquiry", "error", err)
			errorResponse := u.buildErrorResponse(req, "62")
			return errorResponse, err
		}

		if len(billDetails) > 0 {
			response.BillDetails = billDetails
			response.BillCount = 1 // harcoded to 1 because of bill details PLN postpaid only
			u.logger.Info("Bill details attached to payment response",
				"partner_inquiry_id", req.PartnerInquiryID,
				"partner_ref_id", response.PartnerRefID,
				"bill_count", len(billDetails))
		}
	}

	return response, nil
}

// insertPaymentRequest persists payment request to database.
// Returns payment_request_id for linking to response, and error if insert fails.
func (u *paymentUsecaseImpl) insertPaymentRequest(ctx context.Context, req *domain.PaymentRequestDomain) (int64, error) {
	if u.postgresPaymentRepo == nil {
		return 0, nil
	}

	result, err := u.postgresPaymentRepo.InsertPaymentRequest(ctx, &domain.PaymentRequestInsertRequest{
		RefID:            req.RefID,
		PartnerInquiryID: req.PartnerInquiryID,
		ClientNumber:     req.ClientNumber,
		Category:         req.Category,
		Rsid:             req.Rsid,
		ProductCode:      req.ProductCode,
		TotalAmount:      req.TotalAmount,
		Timestamp:        req.Timestamp,
	})
	if err != nil {
		u.logger.Error("Failed to insert payment request",
			"error", err,
			"ref_id", req.RefID)
		return 0, fmt.Errorf("failed to insert payment request: %w", err)
	}

	return result.PaymentRequestID, nil
}

// insertSuccessResponse persists successful payment response to database.
// Returns error if insert fails.
func (u *paymentUsecaseImpl) insertSuccessResponse(ctx context.Context, paymentRequestID int64, response *domain.PaymentResponseDomain) error {
	if u.postgresPaymentRepo == nil || paymentRequestID == 0 {
		return nil
	}

	result, err := u.postgresPaymentRepo.InsertPaymentResponse(ctx, &domain.PaymentResponseInsertRequest{
		PaymentRequestID: paymentRequestID,
		PartnerRefID:     response.PartnerRefID,
		ClientNumber:     response.ClientNumber,
		ProductCode:      response.ProductCode,
		ResponseCode:     response.ResponseCode,
		Message:          response.Message,
		AdminFee:         response.AdminFee,
		TotalAmount:      response.TotalAmount,
		Timestamp:        response.Timestamp,
		BillCount:        response.BillCount,
	})
	if err != nil {
		u.logger.Error("Failed to insert payment response",
			"error", err,
			"payment_request_id", paymentRequestID)
		return fmt.Errorf("failed to insert payment response: %w", err)
	}

	u.logger.Info("Payment response inserted successfully",
		"payment_response_id", result.PaymentResponseID,
		"payment_request_id", paymentRequestID)

	// Insert bill details if any
	for _, billDetail := range response.BillDetails {
		name := strings.TrimSpace(billDetail.Name)
		if strings.EqualFold(name, "Nomor") {
			name = "Nomor Meter"
		}

		_, err := u.postgresPaymentRepo.InsertPaymentBillDetail(ctx, &domain.PaymentBillDetailInsertRequest{
			PartnerRefID: response.PartnerRefID,
			Name:         name,
			Value:        billDetail.Value,
			IsPII:        billDetail.IsPII,
			IsShow:       billDetail.IsShow,
		})
		if err != nil {
			u.logger.Error("Failed to insert payment bill detail",
				"error", err,
				"partner_ref_id", response.PartnerRefID,
				"name", billDetail.Name)
			return fmt.Errorf("failed to insert payment bill detail: %w", err)
		}
	}
	return nil
}

// insertErrorResponse persists error payment response to database.
// Returns error if insert fails.
// originalError is the error that caused the error response (can be nil for validation errors)
func (u *paymentUsecaseImpl) insertErrorResponse(ctx context.Context, paymentRequestID int64, response *domain.PaymentResponseDomain, originalError error) error {
	if u.postgresPaymentRepo == nil || paymentRequestID == 0 {
		return nil
	}

	result, err := u.postgresPaymentRepo.InsertPaymentResponse(ctx, &domain.PaymentResponseInsertRequest{
		PaymentRequestID: paymentRequestID,
		PartnerRefID:     response.PartnerRefID,
		ClientNumber:     response.ClientNumber,
		ProductCode:      response.ProductCode,
		ResponseCode:     response.ResponseCode,
		Message:          response.Message,
		AdminFee:         response.AdminFee,
		TotalAmount:      response.TotalAmount,
		Timestamp:        response.Timestamp,
		BillCount:        response.BillCount,
	})
	if err != nil {
		u.logger.Error("Failed to insert error payment response",
			"error", err,
			"payment_request_id", paymentRequestID)
		return fmt.Errorf("failed to insert error payment response: %w", err)
	}

	// Log with original error details if available
	if originalError != nil {
		u.logger.Info("Error payment response inserted successfully",
			"payment_response_id", result.PaymentResponseID,
			"payment_request_id", paymentRequestID,
			"response_code", response.ResponseCode,
			"original_error", originalError.Error())
	} else {
		u.logger.Info("Error payment response inserted successfully",
			"payment_response_id", result.PaymentResponseID,
			"payment_request_id", paymentRequestID,
			"response_code", response.ResponseCode)
	}
	return nil
}

// validateMandatoryParamsBeforeInsert validates required request parameters before any insert.
// Returns error response if validation fails, nil otherwise.
func (u *paymentUsecaseImpl) validateMandatoryParamsBeforeInsert(
	ctx context.Context,
	req *domain.PaymentRequestDomain,
) *domain.PaymentResponseDomain {
	type paymentMandatoryParams struct {
		RefID            string  `validate:"required"`
		PartnerInquiryID string  `validate:"required"`
		ClientNumber     string  `validate:"required"`
		Category         string  `validate:"required"`
		Rsid             string  `validate:"required"`
		ProductCode      string  `validate:"required"`
		Timestamp        string  `validate:"required"`
		TotalAmount      float64 `validate:"gt=0"`
	}

	validationErr := utils.ValidateStruct(paymentMandatoryParams{
		RefID:            req.RefID,
		PartnerInquiryID: req.PartnerInquiryID,
		ClientNumber:     req.ClientNumber,
		Category:         req.Category,
		Rsid:             req.Rsid,
		ProductCode:      req.ProductCode,
		Timestamp:        req.Timestamp,
		TotalAmount:      req.TotalAmount,
	})
	if validationErr == nil {
		return nil
	}

	fieldMap := map[string]string{
		"RefID":            "ref_id",
		"PartnerInquiryID": "partner_inquiry_id",
		"ClientNumber":     "client_number",
		"Category":         "category",
		"Rsid":             "rsid",
		"ProductCode":      "product_code",
		"Timestamp":        "timestamp",
		"TotalAmount":      "total_amount",
	}

	var missingParams []string
	for _, field := range utils.ValidationErrorFields(validationErr) {
		if name, ok := fieldMap[field]; ok {
			missingParams = append(missingParams, name)
		}
	}

	// Log missing parameters
	u.logger.Warn("Missing mandatory parameters in payment request",
		"missing_params", missingParams,
		"refID", req.RefID,
		"partnerInquiryID", req.PartnerInquiryID,
		"clientNumber", req.ClientNumber,
		"productCode", req.ProductCode)

	// Build error response with code 42 (Invalid parameter)
	return u.buildErrorResponse(req, "42")
}

// validateDuplicateRefIDBeforeInsert checks if ref_id already exists before any insert.
// Returns error response if duplicate, nil otherwise.
func (u *paymentUsecaseImpl) validateDuplicateRefIDBeforeInsert(
	ctx context.Context,
	req *domain.PaymentRequestDomain,
) *domain.PaymentResponseDomain {
	if u.postgresPaymentRepo == nil || req.RefID == "" {
		return nil
	}

	exists, err := u.postgresPaymentRepo.CheckRefIDExists(ctx, req.RefID)
	if err != nil {
		u.logger.Error("Failed to check payment ref_id existence",
			"error", err,
			"ref_id", req.RefID)
		return nil // Continue processing even if check fails
	}

	if !exists {
		return nil // Not duplicate, continue processing
	}

	// Duplicate detected
	u.logger.Warn("Duplicate payment ref_id detected",
		"ref_id", req.RefID,
		"partnerInquiryID", req.PartnerInquiryID,
		"clientNumber", req.ClientNumber,
		"productCode", req.ProductCode)

	return u.buildErrorResponse(req, "13")
}

// validateAmountMatchesPrice validates product status and amount matches price.
// Returns error response code 44 on amount mismatch, status-based error code on status != "00", or nil on success.
//
// Logic:
// 1. Try cache with key "product_with_status:{productCode}"
// 2. If cached → validate status (code from status if != "00")
// 3. If status OK → validate amount == price (code 44 if mismatch)
// 4. If cache miss → call getProductWithStatus() (Oracle fallback)
// 5. Apply same status & amount validation
func (u *paymentUsecaseImpl) validateAmountMatchesPrice(
	ctx context.Context,
	req *domain.PaymentRequestDomain,
) *domain.PaymentResponseDomain {
	// Step 1: Try to get product with status from cache
	cacheKey := fmt.Sprintf("product_with_status:%s", req.ProductCode)
	var productResp *domain.ProductPriceResponseDomain
	var fromCache bool

	if u.redisClient != nil {
		cmd := u.redisClient.Get(ctx, cacheKey)
		if cmd != nil && cmd.Err() == nil {
			cachedValue := cmd.Val()

			// Unmarshal cached JSON
			var cached struct {
				KodeVoucher string  `json:"kodevoucher"`
				Price       float64 `json:"price"`
				Status      string  `json:"status"`
			}
			if err := json.Unmarshal([]byte(cachedValue), &cached); err == nil {
				u.logger.Info("Product with status retrieved from cache",
					"productCode", req.ProductCode,
					"status", cached.Status,
					"price", cached.Price)

				productResp = &domain.ProductPriceResponseDomain{
					OuterRCode:  "0",
					KodeVoucher: cached.KodeVoucher,
					Price:       cached.Price,
					Status:      cached.Status,
				}
				fromCache = true
			} else {
				u.logger.Warn("Failed to unmarshal product_with_status from cache, will fallback to Oracle",
					"error", err,
					"productCode", req.ProductCode)
			}
		} else {
			u.logger.Info("Product with status not found in cache, will fallback to Oracle",
				"productCode", req.ProductCode)
		}
	}

	// Step 2: Fallback to Oracle if cache miss
	if productResp == nil && u.productRepo != nil {
		u.logger.Info("Fetching product with status from Oracle as fallback",
			"productCode", req.ProductCode)

		oracleResp, err := u.productRepo.GetProductByUserAndProductCode(ctx, u.config.TPClientID, req.ProductCode)
		if err != nil {
			u.logger.Error("Failed to get product with status from Oracle",
				"error", err,
				"productCode", req.ProductCode)
			return u.buildErrorResponse(req, "62")
		}

		productResp = oracleResp
		fromCache = false

		// Async cache warming (non-blocking)
		if u.redisClient != nil && productResp.Price > 0 {
			go func() {
				bgCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				defer cancel()

				cachePayload := map[string]interface{}{
					"kodevoucher": productResp.KodeVoucher,
					"price":       productResp.Price,
					"status":      productResp.Status,
				}
				jsonData, err := json.Marshal(cachePayload)
				if err != nil {
					u.logger.Error("Failed to marshal product_with_status for cache",
						"error", err,
						"productCode", req.ProductCode)
					return
				}

				if err := u.redisClient.Set(bgCtx, cacheKey, jsonData, 24*time.Hour).Err(); err != nil {
					u.logger.Error("Failed to save product_with_status to cache asynchronously",
						"error", err,
						"productCode", req.ProductCode)
				} else {
					u.logger.Info("Product with status saved to cache asynchronously",
						"productCode", req.ProductCode)
				}
			}()
		}
	}

	// If both cache and Oracle failed
	if productResp == nil {
		u.logger.Error("Failed to get product with status from both cache and Oracle",
			"productCode", req.ProductCode)
		return u.buildErrorResponse(req, "62")
	}

	// Step 3: Validate status field
	statusCode := strings.TrimSpace(productResp.Status)
	if statusCode != "" && statusCode != "00" {
		u.logger.Warn("Product status is not 00",
			"productCode", req.ProductCode,
			"status", statusCode,
			"fromCache", fromCache)

		// Return error response with status-based code
		response := u.buildErrorResponse(req, statusCode)
		return response
	}

	// Step 4: Validate amount matches price
	if req.TotalAmount != productResp.Price {
		u.logger.Warn("Total amount does not match price",
			"requestAmount", req.TotalAmount,
			"expectedPrice", productResp.Price,
			"productCode", req.ProductCode,
			"fromCache", fromCache)
		return u.buildErrorResponse(req, "44")
	}

	// Step 5: Validate price is valid
	if productResp.Price <= 0 {
		u.logger.Warn("Product price is not valid",
			"price", productResp.Price,
			"productCode", req.ProductCode,
			"fromCache", fromCache)
		return u.buildErrorResponse(req, "62")
	}

	u.logger.Info("Amount and status validation successful",
		"amount", req.TotalAmount,
		"price", productResp.Price,
		"status", statusCode,
		"productCode", req.ProductCode,
		"fromCache", fromCache)

	return nil
}

// validatePartnerInquiryIDBeforeInsert validates partner inquiry ID before insert.
// Returns error response if invalid, nil otherwise.
func (u *paymentUsecaseImpl) validatePartnerInquiryIDBeforeInsert(
	ctx context.Context,
	req *domain.PaymentRequestDomain,
) *domain.PaymentResponseDomain {
	if u.postgresPaymentRepo == nil || req.PartnerInquiryID == "" {
		return nil
	}

	err := u.postgresPaymentRepo.ValidatePartnerInquiryID(ctx, req.PartnerInquiryID)
	if err != nil {
		u.logger.Warn("Invalid partner inquiry ID for payment",
			"error", err,
			"partnerInquiryID", req.PartnerInquiryID)

		// Check if it's a validation error
		var validationErr *domain.ValidationError
		if errors.As(err, &validationErr) {
			return u.buildErrorResponseWithCodeMessage(req, "12", "Transaction not found")
		}
	}

	return nil
}

// buildErrorResponse creates a standardized error response using response code mapping
func (u *paymentUsecaseImpl) buildErrorResponse(
	req *domain.PaymentRequestDomain,
	responseCode string,
) *domain.PaymentResponseDomain {
	rc, _ := utils.GetResponseCode(responseCode)

	message := rc.Message
	if responseCode == "13" {
		message = strings.Replace(message, "${ref_id}", req.RefID, -1)
	}

	return &domain.PaymentResponseDomain{
		RefID:        req.RefID,
		PartnerRefID: "",
		ClientNumber: req.ClientNumber,
		ProductCode:  req.ProductCode,
		ResponseCode: rc.Code,
		Message:      message,
		AdminFee:     0,
		TotalAmount:  req.TotalAmount,
		Timestamp:    req.Timestamp,
		BillCount:    0,
		BillDetails:  []domain.InquiryBillDetail{},
	}
}

// buildErrorResponseWithCodeMessage creates error response with custom code and message (for Oracle error mapping)
func (u *paymentUsecaseImpl) buildErrorResponseWithCodeMessage(
	req *domain.PaymentRequestDomain,
	code string,
	message string,
) *domain.PaymentResponseDomain {
	// Override message with mapped response code message if available
	if rc, ok := utils.GetResponseCode(code); ok {
		message = rc.Message
		code = rc.Code
	}

	return &domain.PaymentResponseDomain{
		RefID:        req.RefID,
		PartnerRefID: "",
		ClientNumber: req.ClientNumber,
		ProductCode:  req.ProductCode,
		ResponseCode: code,
		Message:      message,
		AdminFee:     0,
		TotalAmount:  req.TotalAmount,
		Timestamp:    req.Timestamp,
		BillCount:    0,
		BillDetails:  []domain.InquiryBillDetail{},
	}
}

// validatePaymentDataMatchesInquiry validates that payment request data matches inquiry data.
// Checks client_number, product_code, and amount.
// Returns error response if any mismatch found, nil otherwise.
func (u *paymentUsecaseImpl) validatePaymentDataMatchesInquiry(
	ctx context.Context,
	req *domain.PaymentRequestDomain,
) *domain.PaymentResponseDomain {
	if u.postgresInquiryRepo == nil || req.PartnerInquiryID == "" {
		return nil
	}

	// Fetch inquiry data
	inquiryData, err := u.postgresInquiryRepo.GetInquiryByPartnerInquiryID(ctx, req.PartnerInquiryID)
	if err != nil {
		u.logger.Error("Failed to fetch inquiry data for payment validation",
			"error", err,
			"partner_inquiry_id", req.PartnerInquiryID)
		// Don't fail on fetch error - let other validations handle missing inquiry
		return nil
	}

	if inquiryData == nil {
		// Inquiry not found - will be caught by validatePartnerInquiryIDBeforeInsert
		return nil
	}

	u.logger.Debug("Comparing payment request with inquiry data",
		"partner_inquiry_id", req.PartnerInquiryID,
		"req_client_number", req.ClientNumber,
		"inq_client_number", inquiryData.ClientNumber,
		"req_product_code", req.ProductCode,
		"inq_product_code", inquiryData.ProductCode,
		"req_total_amount", req.TotalAmount,
		"inq_total_amount", inquiryData.TotalAmount)

	// 1. Validate client_number matches
	if req.ClientNumber != inquiryData.ClientNumber {
		u.logger.Warn("Payment client_number mismatch with inquiry",
			"partner_inquiry_id", req.PartnerInquiryID,
			"payment_client_number", req.ClientNumber,
			"inquiry_client_number", inquiryData.ClientNumber)
		return u.buildErrorResponseWithCodeMessage(req, "20", "Client number mismatch")
	}

	// 2. Validate product_code matches
	if req.ProductCode != inquiryData.ProductCode {
		u.logger.Warn("Payment product_code mismatch with inquiry",
			"partner_inquiry_id", req.PartnerInquiryID,
			"payment_product_code", req.ProductCode,
			"inquiry_product_code", inquiryData.ProductCode)
		return u.buildErrorResponseWithCodeMessage(req, "52", "Product code mismatch")
	}

	// 3. Validate amount matches
	if req.TotalAmount != inquiryData.TotalAmount {
		u.logger.Warn("Payment amount mismatch with inquiry",
			"partner_inquiry_id", req.PartnerInquiryID,
			"payment_amount", req.TotalAmount,
			"inquiry_amount", inquiryData.TotalAmount)
		return u.buildErrorResponseWithCodeMessage(req, "44", "Invalid transaction amount")
	}

	return nil
}

// validateInquiryNotPaidBeforeInsert checks if partner_inquiry_id already used in payment.
// Returns error response if already paid, nil otherwise.
func (u *paymentUsecaseImpl) validateInquiryNotPaidBeforeInsert(
	ctx context.Context,
	req *domain.PaymentRequestDomain,
) *domain.PaymentResponseDomain {
	if u.postgresPaymentRepo == nil || req.PartnerInquiryID == "" {
		return nil
	}

	exists, err := u.postgresPaymentRepo.CheckPartnerInquiryIDExists(ctx, req.PartnerInquiryID)
	if err != nil {
		u.logger.Error("Failed to check partner_inquiry_id existence in payments",
			"error", err,
			"partner_inquiry_id", req.PartnerInquiryID)
		return nil // Continue processing even if check fails
	}

	if !exists {
		return nil // Not paid yet, continue processing
	}

	// Inquiry already paid
	u.logger.Warn("Inquiry already paid - partner_inquiry_id found in payment_requests",
		"partner_inquiry_id", req.PartnerInquiryID,
		"ref_id", req.RefID,
		"client_number", req.ClientNumber,
		"product_code", req.ProductCode)

	return u.buildErrorResponse(req, "12")
}

// copyBillDetailsFromInquiry copies bill details from inquiry to payment and returns the copied slice for response.
func (u *paymentUsecaseImpl) copyBillDetailsFromInquiry(ctx context.Context, partnerInquiryID string, partnerRefID string) ([]domain.InquiryBillDetail, error) {
	if u.postgresInquiryRepo == nil || u.postgresPaymentRepo == nil {
		return nil, nil
	}

	// Get bill details from inquiry
	billDetails, err := u.postgresInquiryRepo.GetBillDetailsByInquiryID(ctx, partnerInquiryID)
	if err != nil {
		u.logger.Error("Failed to get bill details from inquiry",
			"error", err,
			"partner_inquiry_id", partnerInquiryID)
		return nil, fmt.Errorf("failed to get bill details from inquiry: %w", err)
	}

	if len(billDetails) == 0 {
		u.logger.Info("No bill details to copy from inquiry",
			"partner_inquiry_id", partnerInquiryID)
		return nil, nil
	}

	// Copy each bill detail to payment
	u.logger.Info("Copying bill details from inquiry to payment",
		"partner_inquiry_id", partnerInquiryID,
		"partner_ref_id", partnerRefID,
		"count", len(billDetails))

	var copiedBillDetails []domain.InquiryBillDetail

	for _, detail := range billDetails {
		// Skip copying price information; will be provided by payment response instead
		trimmedName := strings.TrimSpace(detail.Name)
		if strings.EqualFold(trimmedName, "harga") {
			u.logger.Debug("Skipping price detail when copying to payment",
				"partner_inquiry_id", partnerInquiryID,
				"partner_ref_id", partnerRefID,
				"name", detail.Name)
			continue
		}

		name := trimmedName
		if strings.EqualFold(trimmedName, "nomor") {
			name = "Nomor Meter"
		}

		_, err := u.postgresPaymentRepo.InsertPaymentBillDetail(ctx, &domain.PaymentBillDetailInsertRequest{
			PartnerRefID: partnerRefID,
			Name:         name,
			Value:        detail.Value,
			IsPII:        detail.IsPII,
			IsShow:       detail.IsShow,
		})
		if err != nil {
			u.logger.Error("Failed to copy bill detail to payment",
				"error", err,
				"partner_ref_id", partnerRefID,
				"name", detail.Name)
			return nil, fmt.Errorf("failed to copy bill detail to payment: %w", err)
		}

		copiedBillDetails = append(copiedBillDetails, domain.InquiryBillDetail{
			Name:   name,
			Value:  detail.Value,
			IsPII:  detail.IsPII,
			IsShow: detail.IsShow,
		})
	}

	u.logger.Info("Bill details copied successfully from inquiry to payment",
		"partner_inquiry_id", partnerInquiryID,
		"partner_ref_id", partnerRefID,
		"copied_count", len(billDetails),
		"copied_kept", len(copiedBillDetails))
	return copiedBillDetails, nil
}

// validateCutOff validates cut-off status before processing payment
func (u *paymentUsecaseImpl) validateCutOff(ctx context.Context, req *domain.PaymentRequestDomain) *domain.PaymentResponseDomain {
	active, err := validateCutOffRepo(ctx, u.cutOffRepo, u.logger, func(code string) bool {
		return code == "1"
	}, "")
	if err != nil {
		return u.buildErrorResponse(req, "62")
	}
	if active {
		return u.buildErrorResponse(req, "61")
	}
	return nil
}

// validateCutOffRedis checks maintenance windows defined in Redis keys for payment API.
func (u *paymentUsecaseImpl) validateCutOffRedis(ctx context.Context, req *domain.PaymentRequestDomain) *domain.PaymentResponseDomain {
	if validateCutOffRedisWithFallback(ctx, u.redisClient, u.productRepo, u.logger, " (payment)") {
		return u.buildErrorResponse(req, "61")
	}

	return nil
}
