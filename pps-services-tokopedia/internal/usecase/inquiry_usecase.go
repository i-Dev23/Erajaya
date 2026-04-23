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
	"strconv"
	"strings"
	"time"
)

// inquiryUsecaseImpl implements the domain.InquiryUsecase interface.
type inquiryUsecaseImpl struct {
	config           *config.Config
	logger           service.Logger
	redisClient      service.RedisClient
	productRepo      domain.ProductRepository // Add ProductRepository dependency
	ultimaService    domain.UltimaService
	postgresRepo     domain.PostgresInquiryRepository
	cutOffRepo       domain.CutOffRepository
	errorMappingRepo domain.ErrorMessageMappingRepository
}

// NewInquiryUsecase creates a new InquiryUsecase.
func NewInquiryUsecase(config *config.Config, logger service.Logger, redisClient service.RedisClient, productRepo domain.ProductRepository, ultimaService domain.UltimaService, postgresRepo domain.PostgresInquiryRepository, cutOffRepo domain.CutOffRepository, errorMappingRepo domain.ErrorMessageMappingRepository) domain.InquiryUsecase {
	return &inquiryUsecaseImpl{
		config:           config,
		logger:           logger,
		redisClient:      redisClient,
		productRepo:      productRepo,
		ultimaService:    ultimaService,
		postgresRepo:     postgresRepo,
		cutOffRepo:       cutOffRepo,
		errorMappingRepo: errorMappingRepo,
	}
}

// Inquiry implements the domain.InquiryUsecase interface.
func (u *inquiryUsecaseImpl) Inquiry(ctx context.Context, req *domain.InquiryRequestDomain) (*domain.InquiryResponseDomain, error) {
	ctx, cancel := withUsecaseTimeout(ctx)
	defer cancel()

	u.logger.Info("Starting inquiry process",
		"productCode", req.ProductCode,
		"clientNumber", req.ClientNumber,
		"category", req.Category,
		"refID", req.RefID)

	// 0. Validate cut off (maintenance window)
	// 0a. Validate Redis-based cut off windows
	if response := u.validateCutOffRedis(ctx, req); response != nil {
		return response, nil
	}
	// 0b. Validate repository-based cut off
	if response := u.validateCutOff(ctx, req); response != nil {
		return response, nil
	}

	// 1. Validate mandatory parameters (before any insert)
	if response := u.validateMandatoryParamsBeforeInsert(ctx, req); response != nil {
		// Don't insert for audit trail if mandatory params are missing
		// because it will violate NOT NULL constraints in database
		return response, nil
	}

	// 2. Validate duplicate ref_id (before insert to prevent duplicates)
	if response := u.validateDuplicateRefIDBeforeInsert(ctx, req); response != nil {
		// Insert for audit trail even on duplicate detection
		inquiryRequestID, err := u.insertInquiryRequest(ctx, req)
		if err != nil {
			// Database error - return server error code 62
			u.logger.Error("Failed to insert inquiry request on duplicate ref_id", "error", err)
			return u.buildErrorResponse(req, "62"), err
		}
		u.insertErrorResponse(ctx, inquiryRequestID, response)
		return response, nil
	}

	// 3. Insert inquiry request (only after validation passes)
	inquiryRequestID, err := u.insertInquiryRequest(ctx, req)
	if err != nil {
		// Database error - return server error code 62
		u.logger.Error("Failed to insert inquiry request", "error", err)
		return u.buildErrorResponse(req, "62"), err
	}

	// 4. Validate product price
	productPrice, response, err := u.validateProductPrice(ctx, req, inquiryRequestID)
	if response != nil {
		return response, err
	}

	// 5. Validate PLN inquiry and data
	plnData, response, err := u.validatePLNInquiry(ctx, req, inquiryRequestID)
	if response != nil {
		return response, err
	}

	// 6. Build and persist success response
	return u.buildSuccessResponse(ctx, req, inquiryRequestID, productPrice, plnData)
}

// insertInquiryRequest persists inquiry request to database.
// Returns inquiry_request_id for linking to response, and error if insert fails.
func (u *inquiryUsecaseImpl) insertInquiryRequest(ctx context.Context, req *domain.InquiryRequestDomain) (int64, error) {
	if u.postgresRepo == nil {
		return 0, nil
	}

	result, err := u.postgresRepo.InsertInquiryRequest(ctx, &domain.InquiryRequestInsertRequest{
		RefID:        req.RefID,
		ClientNumber: req.ClientNumber,
		Category:     req.Category,
		Rsid:         req.Rsid,
		ProductCode:  req.ProductCode,
		Timestamp:    req.Timestamp,
	})
	if err != nil {
		u.logger.Error("Failed to insert inquiry request",
			"error", err,
			"ref_id", req.RefID)
		return 0, fmt.Errorf("failed to insert inquiry request: %w", err)
	}

	return result.InquiryRequestID, nil
}

// validateMandatoryParamsBeforeInsert validates required request parameters before any insert.
// Returns error response if validation fails, nil otherwise.
func (u *inquiryUsecaseImpl) validateMandatoryParamsBeforeInsert(
	ctx context.Context,
	req *domain.InquiryRequestDomain,
) *domain.InquiryResponseDomain {
	type inquiryMandatoryParams struct {
		RefID        string `validate:"required"`
		ClientNumber string `validate:"required"`
		Category     string `validate:"required"`
		Rsid         string `validate:"required"`
		ProductCode  string `validate:"required"`
		Timestamp    string `validate:"required"`
	}

	validationErr := utils.ValidateStruct(inquiryMandatoryParams{
		RefID:        req.RefID,
		ClientNumber: req.ClientNumber,
		Category:     req.Category,
		Rsid:         req.Rsid,
		ProductCode:  req.ProductCode,
		Timestamp:    req.Timestamp,
	})
	if validationErr == nil {
		return nil
	}

	fieldMap := map[string]string{
		"RefID":        "ref_id",
		"ClientNumber": "client_number",
		"Category":     "category",
		"Rsid":         "rsid",
		"ProductCode":  "product_code",
		"Timestamp":    "timestamp",
	}

	var missingParams []string
	for _, field := range utils.ValidationErrorFields(validationErr) {
		if name, ok := fieldMap[field]; ok {
			missingParams = append(missingParams, name)
		}
	}

	// Log missing parameters
	u.logger.Warn("Missing mandatory parameters",
		"missing_params", missingParams,
		"refID", req.RefID,
		"clientNumber", req.ClientNumber,
		"productCode", req.ProductCode,
		"category", req.Category)

	// Build error response with code 42 (Invalid parameter)
	return u.buildErrorResponse(req, "42")
}

// validateDuplicateRefIDBeforeInsert checks if ref_id already exists before any insert.
// Returns error response if duplicate, nil otherwise.
func (u *inquiryUsecaseImpl) validateDuplicateRefIDBeforeInsert(
	ctx context.Context,
	req *domain.InquiryRequestDomain,
) *domain.InquiryResponseDomain {
	if u.postgresRepo == nil || req.RefID == "" {
		return nil
	}

	exists, err := u.postgresRepo.CheckRefIDExists(ctx, req.RefID)
	if err != nil {
		u.logger.Error("Failed to check ref_id existence",
			"error", err,
			"ref_id", req.RefID)
		return nil // Continue processing even if check fails
	}

	if !exists {
		return nil // Not duplicate, continue processing
	}

	// Duplicate detected
	u.logger.Warn("Duplicate ref_id detected",
		"ref_id", req.RefID,
		"clientNumber", req.ClientNumber,
		"productCode", req.ProductCode)

	return u.buildErrorResponse(req, "13") // Duplicate transaction
}

// validateProductPrice validates product price from cache or oracle.
// Returns (price, errorResponse, error).
// If errorResponse is not nil, caller should return immediately.
func (u *inquiryUsecaseImpl) validateProductPrice(
	ctx context.Context,
	req *domain.InquiryRequestDomain,
	inquiryRequestID int64,
) (float64, *domain.InquiryResponseDomain, error) {

	productPriceResp, fromCache, err := u.getProductWithStatus(ctx, req.ProductCode)
	if err != nil {
		u.logger.Error("Failed to get product with status",
			"error", err,
			"productCode", req.ProductCode)

		response := u.buildErrorResponse(req, "62")
		u.insertErrorResponse(ctx, inquiryRequestID, response)
		return 0, response, err
	}

	statusCode := strings.TrimSpace(productPriceResp.Status)
	if statusCode != "" && statusCode != "00" {
		response := u.buildErrorResponse(req, statusCode)
		u.insertErrorResponse(ctx, inquiryRequestID, response)
		return 0, response, nil
	}

	if productPriceResp.OuterRCode != "" && productPriceResp.OuterRCode != "0" {
		errorMsg := productPriceResp.OuterRMsg
		if errorMsg == "" {
			errorMsg = "Oracle error code: " + productPriceResp.OuterRCode
		}

		responseCode, responseMessage := resolveOracleMapping(ctx, errorMsg, u.errorMappingRepo, u.logger)
		u.logger.Error("Oracle returned error when getting product price",
			"oracleErrorMsg", errorMsg,
			"outerRCode", productPriceResp.OuterRCode,
			"mappedResponseCode", responseCode,
			"mappedResponseMessage", responseMessage,
			"productCode", req.ProductCode,
			"fromCache", fromCache)

		response := u.buildErrorResponse(req, responseCode)
		response.Message = responseMessage
		u.insertErrorResponse(ctx, inquiryRequestID, response)

		return 0, response, nil
	}

	if productPriceResp.Price == 0 {
		u.logger.Warn("Product price not found",
			"clientNumber", req.ClientNumber,
			"productCode", req.ProductCode,
			"fromCache", fromCache)

		response := u.buildErrorResponse(req, "14")
		u.insertErrorResponse(ctx, inquiryRequestID, response)

		return 0, response, nil
	}

	return productPriceResp.Price, nil, nil
}

// validatePLNInquiry validates PLN inquiry data from Ultima or cache.
// Returns (plnData, errorResponse, error).
// If errorResponse is not nil, caller should return immediately.
func (u *inquiryUsecaseImpl) validatePLNInquiry(
	ctx context.Context,
	req *domain.InquiryRequestDomain,
	inquiryRequestID int64,
) (*domain.PLNTransactionInquiry, *domain.InquiryResponseDomain, error) {

	// Validate client_number is not empty (required for PLN inquiry)
	if req.ClientNumber == "" {
		u.logger.Error("Client number is empty, cannot proceed with PLN inquiry",
			"refID", req.RefID,
			"productCode", req.ProductCode)

		response := u.buildErrorResponse(req, "42") // Invalid parameter
		u.insertErrorResponse(ctx, inquiryRequestID, response)

		return nil, response, nil
	}

	// Get PLN inquiry from cache or Ultima
	plnData, err := u.getPLNInquiry(ctx, req.ClientNumber)
	if err != nil {
		u.logger.Error("Failed to get pln inquiry",
			"error", err,
			"clientNumber", req.ClientNumber)

		// Check if error is UltimaMappingError with mapped response code
		var responseCode string
		var responseMessage string
		var ultimaErr *domain.UltimaMappingError
		if errors.As(err, &ultimaErr) {
			responseCode = ultimaErr.GetResponseCode()
			responseMessage = ultimaErr.GetResponseMessage()
			u.logger.Info("Using mapped response code and message from Ultima error",
				"responseCode", responseCode,
				"responseMessage", responseMessage,
				"clientNumber", req.ClientNumber)
		} else {
			responseCode = "62" // Server error (Ultima service unavailable/error)
			rc, _ := utils.GetResponseCode(responseCode)
			responseMessage = rc.Message
		}

		response := u.buildErrorResponseWithMessage(req, responseCode, responseMessage)
		u.insertErrorResponse(ctx, inquiryRequestID, response)

		return nil, response, nil
	}

	// PLN detail data completeness is already validated in checkIdPlnUltima
	// If data is incomplete, checkIdPlnUltima will return UltimaMappingError
	// with appropriate mapped response code

	// Validation passed
	return plnData, nil, nil
}

// buildSuccessResponse builds and persists successful inquiry response.
func (u *inquiryUsecaseImpl) buildSuccessResponse(
	ctx context.Context,
	req *domain.InquiryRequestDomain,
	inquiryRequestID int64,
	productPrice float64,
	plnData *domain.PLNTransactionInquiry,
) (*domain.InquiryResponseDomain, error) {

	// Generate PPS inquiry ID
	ppsInquiryID := utils.GeneratePPSRequestID()
	rc, _ := utils.GetResponseCode("00") // Success
	response := &domain.InquiryResponseDomain{
		PartnerInquiryID: ppsInquiryID,
		ClientNumber:     req.ClientNumber,
		ProductCode:      req.ProductCode,
		ResponseCode:     rc.Code,
		Message:          rc.Message,
		TotalAmount:      productPrice,
		Timestamp:        req.Timestamp,
		BillCount:        1,
		IsOpenAmount:     false,
		BillDetails: []domain.InquiryBillDetail{
			{
				Name:   "Nama",
				Value:  plnData.Name,
				IsPII:  true,
				IsShow: true,
			},
			{
				Name:   "Nomor",
				Value:  plnData.MeterNumber,
				IsPII:  false,
				IsShow: true,
			},
			{
				Name:   "ID Pelanggan",
				Value:  plnData.IDPelanggan,
				IsPII:  false,
				IsShow: true,
			},
			{
				Name:   "Tarif/Daya",
				Value:  plnData.TarifDaya,
				IsPII:  false,
				IsShow: true,
			},
			{
				Name:   "Harga",
				Value:  utils.FormatRupiah(productPrice),
				IsPII:  false,
				IsShow: true,
			},
		},
		AdditionalDetails: []domain.InquiryAdditionalDetail{},
	}

	// Persist response
	if u.postgresRepo != nil {
		_, err := u.postgresRepo.InsertInquiryResponse(ctx, &domain.InquiryResponseInsertRequest{
			InquiryRequestID: inquiryRequestID,
			PpsInquiryID:     ppsInquiryID,
			ClientNumber:     response.ClientNumber,
			ProductCode:      response.ProductCode,
			ResponseCode:     response.ResponseCode,
			Message:          response.Message,
			TotalAmount:      response.TotalAmount,
			Timestamp:        response.Timestamp,
			BillCount:        float64(response.BillCount),
		})
		if err != nil {
			u.logger.Error("Failed to insert inquiry response",
				"error", err,
				"ppsInquiryID", ppsInquiryID)
			// Return error code 62 for database error
			errorResponse := u.buildErrorResponse(req, "62")
			return errorResponse, fmt.Errorf("failed to insert inquiry response: %w", err)
		}

		// Insert bill details
		for _, d := range response.BillDetails {
			if _, derr := u.postgresRepo.InsertBillDetail(ctx, &domain.BillDetailInsertRequest{
				PpsInquiryID: ppsInquiryID,
				Name:         d.Name,
				Value:        d.Value,
				IsPII:        d.IsPII,
				IsShow:       d.IsShow,
			}); derr != nil {
				u.logger.Error("Failed to insert bill detail",
					"error", derr,
					"name", d.Name)
				// Return error code 62 for database error
				errorResponse := u.buildErrorResponse(req, "62")
				return errorResponse, fmt.Errorf("failed to insert bill detail: %w", derr)
			}
		}
	}

	u.logger.Info("Inquiry completed successfully",
		"clientNumber", req.ClientNumber,
		"productCode", req.ProductCode,
		"totalAmount", productPrice,
		"ppsInquiryID", ppsInquiryID)

	return response, nil
}

// buildErrorResponse builds error response domain object.
func (u *inquiryUsecaseImpl) buildErrorResponse(
	req *domain.InquiryRequestDomain,
	responseCode string,
) *domain.InquiryResponseDomain {
	rc, _ := utils.GetResponseCode(responseCode)

	message := rc.Message
	if responseCode == "13" {
		message = strings.Replace(message, "${ref_id}", req.RefID, -1)
	}

	return &domain.InquiryResponseDomain{
		ClientNumber: req.ClientNumber,
		ProductCode:  req.ProductCode,
		ResponseCode: rc.Code,
		Message:      message,
		Timestamp:    req.Timestamp,
	}
}

// buildErrorResponseWithMessage builds error response domain object with custom message.
func (u *inquiryUsecaseImpl) buildErrorResponseWithMessage(
	req *domain.InquiryRequestDomain,
	responseCode string,
	responseMessage string,
) *domain.InquiryResponseDomain {
	rc, _ := utils.GetResponseCode(responseCode)

	return &domain.InquiryResponseDomain{
		ClientNumber: req.ClientNumber,
		ProductCode:  req.ProductCode,
		ResponseCode: rc.Code,
		Message:      responseMessage,
		Timestamp:    req.Timestamp,
	}
}

// validateCutOff validates cut-off status before processing inquiry
func (u *inquiryUsecaseImpl) validateCutOff(ctx context.Context, req *domain.InquiryRequestDomain) *domain.InquiryResponseDomain {
	active, err := validateCutOffRepo(ctx, u.cutOffRepo, u.logger, func(code string) bool {
		return strings.TrimSpace(code) == domain.CutOffFlagActive
	}, " for inquiry")
	if err != nil {
		return u.buildErrorResponse(req, "62")
	}
	if active {
		return u.buildErrorResponse(req, "61")
	}
	return nil
}

// insertErrorResponse persists error response to database.
func (u *inquiryUsecaseImpl) insertErrorResponse(
	ctx context.Context,
	inquiryRequestID int64,
	response *domain.InquiryResponseDomain,
) {
	if u.postgresRepo == nil {
		return
	}

	_, err := u.postgresRepo.InsertInquiryResponse(ctx, &domain.InquiryResponseInsertRequest{
		InquiryRequestID: inquiryRequestID,
		ClientNumber:     response.ClientNumber,
		ProductCode:      response.ProductCode,
		ResponseCode:     response.ResponseCode,
		Message:          response.Message,
		TotalAmount:      0,
		Timestamp:        response.Timestamp,
		BillCount:        0,
	})

	if err != nil {
		u.logger.Error("Failed to insert error inquiry response",
			"error", err,
			"inquiry_request_id", inquiryRequestID,
			"response_code", response.ResponseCode)
	}
}

// validateCutOffRedis checks maintenance windows defined in Redis keys.
// Keys:
// - CUT_OFF_TIME_START (format HH:MM)
// - CUT_OFF_DURATION_SECOND (seconds)
// - CUT_OFF_TIME_START_TOKOPEDIA (format HH:MM)
// - CUT_OFF_DURATION_SECOND_TOKOPEDIA (seconds)
// Logic:
// - If both start keys empty -> fallback to Oracle
// - If only one start has value -> check only that window
// - If both have values -> if now is within either window -> cut off
func (u *inquiryUsecaseImpl) validateCutOffRedis(ctx context.Context, req *domain.InquiryRequestDomain) *domain.InquiryResponseDomain {
	if validateCutOffRedisWithFallback(ctx, u.redisClient, u.productRepo, u.logger, "") {
		return u.buildErrorResponse(req, "61")
	}

	return nil
}

func (u *inquiryUsecaseImpl) getPLNInquiry(ctx context.Context, idPel string) (*domain.PLNTransactionInquiry, error) {

	//check id pln on cache redis
	cmdRedis := u.redisClient.Get(ctx, idPel)
	if cmdRedis.Err() != nil {
		u.logger.Info("PLN inquiry cache miss or Redis unavailable",
			"error", cmdRedis.Err(),
			"idPel", idPel)
	}
	PLNTransactionInquiry := cmdRedis.Val()

	//if pln inquiry is not in cache, get it from ultima
	if PLNTransactionInquiry == "" {

		//get id pln from ultima
		parsedData, err := u.checkIdPlnUltima(ctx, idPel, idPel)
		if err != nil {
			// Check if error is UltimaMappingError - preserve it without wrapping
			var ultimaErr *domain.UltimaMappingError
			if errors.As(err, &ultimaErr) {
				return nil, err // Return the original UltimaMappingError
			}
			return nil, fmt.Errorf("failed to get id pln from ultima: %w", err)
		}

		// Validate that all required fields are present before saving to cache
		if parsedData.Name == "" || parsedData.MeterNumber == "" || parsedData.IDPelanggan == "" || parsedData.TarifDaya == "" {
			u.logger.Warn("PLN inquiry data is incomplete, skipping cache save",
				"idPel", idPel,
				"name", parsedData.Name,
				"meterNumber", parsedData.MeterNumber,
				"idPelanggan", parsedData.IDPelanggan,
				"tarifDaya", parsedData.TarifDaya)
			// Return the incomplete data without saving to cache
			return parsedData, nil
		}

		//save pln inquiry to cache
		parsedDataJSON, err := json.Marshal(parsedData)
		if err != nil {
			u.logger.Error("Failed to marshal parsed data",
				"error", err,
				"idPel", idPel)
			return nil, fmt.Errorf("failed to marshal parsed data: %w", err)
		}

		// Only save to Redis if value is not empty
		if len(parsedDataJSON) > 0 {
			cmdRedis := u.redisClient.Set(ctx, idPel, parsedDataJSON, 24*time.Hour)
			if cmdRedis.Err() != nil {
				u.logger.Error("Failed to save id pln to cache",
					"error", cmdRedis.Err(),
					"idPel", idPel)
			} else {
				u.logger.Info("PLN inquiry data saved to cache",
					"idPel", idPel)
			}
		} else {
			u.logger.Warn("Skipped saving empty PLN inquiry data to cache",
				"idPel", idPel)
		}
		return parsedData, nil
	} else {
		//if pln inquiry is in cache, unmarshal it
		parsedData := domain.PLNTransactionInquiry{}
		err := json.Unmarshal([]byte(PLNTransactionInquiry), &parsedData)
		if err != nil {
			u.logger.Error("Failed to unmarshal parsed data",
				"error", err,
				"idPel", idPel)
			return nil, fmt.Errorf("failed to unmarshal parsed data: %w", err)
		}
		return &parsedData, nil
	}
}

func (u *inquiryUsecaseImpl) checkIdPlnUltima(ctx context.Context, idPel string, idTrx string) (*domain.PLNTransactionInquiry, error) {
	// Validate idPel is not empty before calling Ultima
	if idPel == "" {
		u.logger.Error("IdPel is empty, cannot check id pln ultima",
			"idPel", idPel,
			"idTrx", idTrx)
		return nil, fmt.Errorf("idPel is empty")
	}

	ultimaResp, parsedData, err := u.ultimaService.CheckIdPlnUltima(ctx, &domain.UltimaCheckIdPlnRequestDomain{
		IdPel: idPel,
		IdTrx: idTrx,
	})

	// Check for error first
	if err != nil {
		u.logger.Error("Failed to check id pln ultima",
			"error", err.Error(),
			"ultimaResp", ultimaResp,
			"clientNumber", idPel,
			"productCode", idTrx)
		return nil, fmt.Errorf("failed to check id pln ultima: %w", err)
	}

	// Check if ultimaResp is nil
	if ultimaResp == nil {
		u.logger.Error("Ultima response is nil",
			"clientNumber", idPel,
			"productCode", idTrx)
		return nil, fmt.Errorf("ultima response is nil")
	}

	// Check status code and error message from Ultima
	if ultimaResp.HttpStatusCode != 200 {
		// Check for error message in Ultima response
		errorMsg := ultimaResp.HttpResponseBody.Msg
		if errorMsg != "" {
			// Map Ultima error message to Tokopedia response code (DB first, fallback to static map)
			responseCode, responseMessage := resolveUltimaMapping(ctx, errorMsg, u.errorMappingRepo, u.logger)
			u.logger.Error("Ultima returned error",
				"httpStatusCode", ultimaResp.HttpStatusCode,
				"ultimaErrorMsg", errorMsg,
				"mappedResponseCode", responseCode,
				"mappedResponseMessage", responseMessage,
				"clientNumber", idPel,
				"productCode", idTrx)
			// Return UltimaMappingError with mapped response code
			return nil, &domain.UltimaMappingError{
				Message:         fmt.Sprintf("ultima error: %s", errorMsg),
				ResponseCode:    responseCode,
				ResponseMessage: responseMessage,
			}
		}

		u.logger.Error("Ultima returned non-200 status code",
			"httpStatusCode", ultimaResp.HttpStatusCode,
			"ultimaResp", ultimaResp,
			"clientNumber", idPel,
			"productCode", idTrx)
		return nil, fmt.Errorf("ultima returned status code %d", ultimaResp.HttpStatusCode)
	}

	// Check if parsedData is nil
	if parsedData == nil {
		u.logger.Error("Parsed data is nil",
			"clientNumber", idPel,
			"productCode", idTrx,
			"ultimaResp", ultimaResp)
		return nil, fmt.Errorf("parsed data is nil")
	}

	// Check if PLN detail data is complete
	if parsedData.Name == "" || parsedData.MeterNumber == "" || parsedData.IDPelanggan == "" || parsedData.TarifDaya == "" {
		u.logger.Warn("PLN detail data is incomplete from Ultima",
			"clientNumber", idPel,
			"name", parsedData.Name,
			"meterNumber", parsedData.MeterNumber,
			"idPelanggan", parsedData.IDPelanggan,
			"tarifDaya", parsedData.TarifDaya)

		// Check if there's error message in Ultima response that can be triggered
		errorMsg := ultimaResp.HttpResponseBody.Msg
		if errorMsg != "" {
			// Try to map Ultima error message to Tokopedia response code (DB first, fallback to static map)
			responseCode, responseMessage := resolveUltimaMapping(ctx, errorMsg, u.errorMappingRepo, u.logger)
			u.logger.Error("Ultima returned incomplete data, trying to map error message",
				"ultimaErrorMsg", errorMsg,
				"mappedResponseCode", responseCode,
				"mappedResponseMessage", responseMessage,
				"clientNumber", idPel,
				"productCode", idTrx)
			// Return UltimaMappingError with mapped response code
			return nil, &domain.UltimaMappingError{
				Message:         fmt.Sprintf("ultima error: %s", errorMsg),
				ResponseCode:    responseCode,
				ResponseMessage: responseMessage,
			}
		}

		// If no error message from Ultima, return default error
		return nil, fmt.Errorf("pln detail data is incomplete from ultima")
	}

	// Check if there's error message in Ultima response (even with 200 status code)
	errorMsg := ultimaResp.HttpResponseBody.Msg
	if errorMsg != "" {
		// Check if message contains error patterns (e.g., "TRX GAGAL")
		if strings.Contains(errorMsg, "TRX GAGAL") {
			// Map Ultima error message to Tokopedia response code (DB first, fallback to static map)
			responseCode, responseMessage := resolveUltimaMapping(ctx, errorMsg, u.errorMappingRepo, u.logger)
			u.logger.Error("Ultima returned error in message",
				"ultimaErrorMsg", errorMsg,
				"mappedResponseCode", responseCode,
				"mappedResponseMessage", responseMessage,
				"clientNumber", idPel,
				"productCode", idTrx)
			// Return UltimaMappingError with mapped response code
			return nil, &domain.UltimaMappingError{
				Message:         fmt.Sprintf("ultima error: %s", errorMsg),
				ResponseCode:    responseCode,
				ResponseMessage: responseMessage,
			}
		}
	}

	u.logger.Info("CheckIdPlnUltima completed successfully",
		"idPel", idPel,
		"idTrx", idTrx,
		"ultimaResp", ultimaResp)
	return parsedData, nil
}

type productWithStatusCache struct {
	KodeVoucher string  `json:"kodevoucher"`
	Price       float64 `json:"price"`
	Status      string  `json:"status"`
}

func (u *inquiryUsecaseImpl) getProductWithStatus(ctx context.Context, productCode string) (*domain.ProductPriceResponseDomain, bool, error) {
	cacheKey := fmt.Sprintf("%s%s", utils.RedisKeyProductWithStatusPrefix, productCode)

	cmd := u.redisClient.Get(ctx, cacheKey)
	if cmd != nil && cmd.Err() == nil {
		cachedValue := cmd.Val()
		var cached productWithStatusCache
		if err := json.Unmarshal([]byte(cachedValue), &cached); err != nil {
			u.logger.Error("Failed to unmarshal product_with_status cache",
				"error", err,
				"cacheKey", cacheKey,
				"value", cachedValue)
		} else {
			return &domain.ProductPriceResponseDomain{
				OuterRCode:  "0",
				OuterRMsg:   "",
				KodeVoucher: cached.KodeVoucher,
				Price:       cached.Price,
				Status:      cached.Status,
			}, true, nil
		}
	} else if cmd != nil && cmd.Err() != nil {
		u.logger.Info("Product-with-status cache miss or Redis unavailable",
			"error", cmd.Err(),
			"cacheKey", cacheKey)
	}

	productResp, err := u.productRepo.GetProductByUserAndProductCode(ctx, u.config.TPClientID, productCode)
	if err != nil {
		return nil, false, err
	}

	go u.saveProductWithStatusToRedisAsync(cacheKey, productResp)

	return productResp, false, nil
}

func (u *inquiryUsecaseImpl) saveProductWithStatusToRedisAsync(cacheKey string, product *domain.ProductPriceResponseDomain) {
	if product == nil {
		return
	}

	payload := productWithStatusCache{
		KodeVoucher: product.KodeVoucher,
		Price:       product.Price,
		Status:      product.Status,
	}

	data, err := json.Marshal(payload)
	if err != nil {
		u.logger.Error("Failed to marshal product_with_status payload",
			"error", err,
			"cacheKey", cacheKey)
		return
	}

	bgCtx := context.Background()
	ctx, cancel := context.WithTimeout(bgCtx, 5*time.Second)
	defer cancel()

	if err := u.redisClient.Set(ctx, cacheKey, data, 24*time.Hour).Err(); err != nil {
		u.logger.Error("Failed to save product_with_status to Redis asynchronously",
			"error", err,
			"cacheKey", cacheKey)
		return
	}

	u.logger.Info("Saved product_with_status to Redis asynchronously",
		"cacheKey", cacheKey)
}

func (u *inquiryUsecaseImpl) getProductPrice(ctx context.Context, productCode string) (*domain.ProductPriceResponseDomain, error) {

	//check product price from cache redis
	cmdRedis := u.redisClient.Get(ctx, productCode)
	if cmdRedis.Err() != nil {
		u.logger.Error("Failed to get product price from cache",
			"error", cmdRedis.Err(),
			"productCode", productCode)

		//get product price from oracle
		productPrice, err := u.productRepo.GetPriceByUserAndProductCode(ctx, u.config.TPClientID, productCode)
		if err != nil {
			u.logger.Error("Failed to get product price from oracle",
				"error", err,
				"productCode", productCode)

			//return error if get product price from oracle failed
			return nil, err
		}

		//save product price to cache redis asynchronously if product price is not 0 AND outerrcode is 0
		if productPrice.Price != 0 && productPrice.OuterRCode == "0" {
			priceValue := fmt.Sprintf("%.2f", productPrice.Price)
			// Only save to Redis if value is not empty
			if priceValue != "" && priceValue != "0.00" {
				// Save to Redis asynchronously using goroutine to prevent blocking API
				go u.saveProductPriceToRedisAsync(productCode, priceValue)
			} else {
				u.logger.Warn("Skipped saving empty or zero product price to cache",
					"productCode", productCode,
					"priceValue", priceValue)
			}
		} else {
			u.logger.Info("Product price not saved to cache",
				"productCode", productCode,
				"outerrcode", productPrice.OuterRCode)
		}

		//return product price response (includes OuterRCode and OuterRMsg for error mapping)
		return productPrice, nil
	}

	price, err := strconv.ParseFloat(cmdRedis.Val(), 64)
	// error parsing cached price
	if err != nil {
		u.logger.Error("Failed to parse cached price",
			"error", err,
			"cachedValue", cmdRedis.Val())
		return nil, err
	}

	// Return from cache as ProductPriceResponseDomain
	return &domain.ProductPriceResponseDomain{
		OuterRCode:  "0",
		OuterRMsg:   "",
		KodeVoucher: productCode,
		Price:       price,
	}, nil
}

// saveProductPriceToRedisAsync saves product price to Redis asynchronously
// to prevent blocking the API when Redis is down or slow
func (u *inquiryUsecaseImpl) saveProductPriceToRedisAsync(productCode string, priceValue string) {
	// Use background context instead of request context to prevent cancellation
	bgCtx := context.Background()

	// Add timeout to prevent goroutine from hanging indefinitely
	ctx, cancel := context.WithTimeout(bgCtx, 5*time.Second)
	defer cancel()

	cmdRedis := u.redisClient.Set(ctx, productCode, priceValue, time.Duration(24)*time.Hour)
	if cmdRedis.Err() != nil {
		u.logger.Error("Failed to save product price to cache asynchronously",
			"error", cmdRedis.Err(),
			"productCode", productCode,
			"price", priceValue)
	} else {
		u.logger.Info("Product price saved to cache asynchronously",
			"productCode", productCode,
			"price", priceValue)
	}
}
