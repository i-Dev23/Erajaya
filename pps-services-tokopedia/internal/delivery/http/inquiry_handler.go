package http

import (
	"context"
	"encoding/json"
	"net/http"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/dto"
	"pps-services-tokopedia/internal/service"
	"pps-services-tokopedia/internal/utils"
	"time"

	"github.com/gofiber/fiber/v2"
)

// Response code cache untuk menghindari lookup berulang
var (
	responseCodeCache = map[string]utils.ResponseCode{
		"42": utils.ResponseCodeMap["42"],
		"62": utils.ResponseCodeMap["62"],
	}
)

type InquiryHandler struct {
	inquiryUsecase domain.InquiryUsecase
	logger         service.Logger
}

func NewInquiryHandler(inquiryUsecase domain.InquiryUsecase, logger service.Logger) *InquiryHandler {
	return &InquiryHandler{
		inquiryUsecase: inquiryUsecase,
		logger:         logger,
	}
}

func (h *InquiryHandler) RegisterRoutes(router fiber.Router) {
	router.Post("/inquiry", h.Inquiry)
}

func (h *InquiryHandler) Inquiry(c *fiber.Ctx) error {
	startTime := time.Now()
	var reqDto dto.InquiryRequestDto

	//get context - optimize context handling
	ctx := c.UserContext()
	if ctx == nil {
		ctx = context.Background()
	}

	decryptedBody, ok := c.Locals("decryptedBody").([]byte)
	if !ok || decryptedBody == nil {
		return h.handleError(c, startTime, "42", "missing decrypted body", "", "", nil)
	}

	// Optimize: Single log call dengan minimal data
	h.logger.Info("Inquiry API Request Started",
		"endpoint", "/inquiry",
		"method", "POST",
		"timestamp", startTime.Format(timeFormat))

	//parse request body
	if err := json.Unmarshal(decryptedBody, &reqDto); err != nil {
		return h.handleError(c, startTime, "42", "failed to parse request body", reqDto.ProductCode, reqDto.ClientNumber, err)
	}

	//call inquiry usecase - optimize domain conversion
	domainReq := &domain.InquiryRequestDomain{
		RefID:        reqDto.RefID,
		ClientNumber: reqDto.ClientNumber,
		Category:     reqDto.Category,
		Rsid:         reqDto.Rsid,
		ProductCode:  reqDto.ProductCode,
		Timestamp:    reqDto.Timestamp,
	}

	resp, err := h.inquiryUsecase.Inquiry(ctx, domainReq)
	if err != nil {
		return h.handleError(c, startTime, "62", "Failed to inquiry", reqDto.ProductCode, reqDto.ClientNumber, err)
	}

	// Optimize response conversion
	duration := time.Since(startTime)
	response := h.convertDomainToDtoOptimized(resp)

	// Single log call untuk response
	h.logger.Info("Inquiry API Response",
		"endpoint", "/inquiry",
		"status_code", http.StatusOK,
		"duration_ms", duration.Milliseconds())

	return c.Status(http.StatusOK).JSON(response)
}

// handleError - centralized error handling untuk mengurangi code duplication
func (h *InquiryHandler) handleError(c *fiber.Ctx, startTime time.Time, code, message, productCode, clientNumber string, err error) error {
	duration := time.Since(startTime)

	response := newInquiryResponseOptimized(code, productCode, clientNumber)

	// Single log call untuk error
	if err != nil {
		h.logger.Error(message, "error", err, "duration_ms", duration.Milliseconds())
	} else {
		h.logger.Error(message, "duration_ms", duration.Milliseconds())
	}

	h.logger.Info("Inquiry API Response",
		"endpoint", "/inquiry",
		"status_code", http.StatusOK,
		"duration_ms", duration.Milliseconds())

	return c.Status(http.StatusOK).JSON(response)
}

// newInquiryResponseOptimized - optimized response creation
func newInquiryResponseOptimized(code string, productCode string, clientNumber string) dto.InquiryResponseDto {
	rc, ok := utils.GetResponseCode(code)
	if !ok {
		rc = utils.ResponseCodeMap["62"]
	}
	pc := productCode
	if pc == "" {
		pc = "-"
	}
	cn := clientNumber
	if cn == "" {
		cn = "-"
	}
	return dto.InquiryResponseDto{
		ProductCode:  pc,
		ClientNumber: cn,
		ResponseCode: rc.Code,
		Message:      rc.Message,
		Timestamp:    time.Now().Format(timeFormat),
	}
}

// convertDomainToDtoOptimized - optimized domain to DTO conversion
func (h *InquiryHandler) convertDomainToDtoOptimized(domain *domain.InquiryResponseDomain) dto.InquiryResponseDto {
	billDetails := make([]dto.BillDetailDto, 0, len(domain.BillDetails))

	// Convert dengan minimal allocations
	for _, detail := range domain.BillDetails {
		billDetails = append(billDetails, dto.BillDetailDto{
			Name:   detail.Name,
			Value:  detail.Value,
			IsPII:  &detail.IsPII,
			IsShow: &detail.IsShow,
		})
	}

	response := dto.InquiryResponseDto{
		PartnerInquiryID: domain.PartnerInquiryID,
		ClientNumber:     domain.ClientNumber,
		ProductCode:      domain.ProductCode,
		ResponseCode:     domain.ResponseCode,
		Message:          domain.Message,
		Timestamp:        time.Now().Format(timeFormat),
		TotalAmount:      domain.TotalAmount,
		IsOpenAmount:     domain.IsOpenAmount,
		AdminFee:         domain.AdminFee,
		BillDetails:      billDetails,
	}

	return response
}

// Legacy functions untuk backward compatibility
func newInquiryResponse(code string, productCode string, clientNumber string) dto.InquiryResponseDto {
	return newInquiryResponseOptimized(code, productCode, clientNumber)
}

func convertDomainToDto(domain *domain.InquiryResponseDomain) dto.InquiryResponseDto {
	handler := &InquiryHandler{}
	return handler.convertDomainToDtoOptimized(domain)
}
