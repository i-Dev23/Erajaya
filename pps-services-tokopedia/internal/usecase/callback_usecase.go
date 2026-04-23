package usecase

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/middleware"
	"pps-services-tokopedia/internal/service"
)

// callbackUsecase implements domain.CallbackUsecase
type callbackUsecase struct {
	rabbit                    service.RabbitMQService
	logger                    service.Logger
	cryptoService             service.CryptoService
	digitalSignatureService   service.DigitalSignatureService
	callbackLoggingRepository domain.CallbackLoggingRepository
	postgresPaymentRepo       domain.PostgresPaymentRepository
	httpClient                *http.Client
}

// NewCallbackUsecase constructs a callback usecase with dependencies injected.
func NewCallbackUsecase(
	rabbit service.RabbitMQService,
	logger service.Logger,
	cryptoService service.CryptoService,
	digitalSignatureService service.DigitalSignatureService,
	callbackLoggingRepository domain.CallbackLoggingRepository,
	postgresPaymentRepo domain.PostgresPaymentRepository,
) domain.CallbackUsecase {
	return &callbackUsecase{
		rabbit:                    rabbit,
		logger:                    logger,
		cryptoService:             cryptoService,
		digitalSignatureService:   digitalSignatureService,
		callbackLoggingRepository: callbackLoggingRepository,
		postgresPaymentRepo:       postgresPaymentRepo,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// CallbackToTokopedia sends callback to Tokopedia with signing, encryption, and logging
func (u *callbackUsecase) CallbackToTokopedia(ctx context.Context, req *domain.CallbackRequestDomain) error {
	startTime := time.Now()

	u.logger.Info("CallbackToTokopedia started", "ref_id", req.RefID, "partner_ref_id", req.PartnerRefID)

	// 1. Convert request to JSON for signing
	// Build custom JSON based on response_code to omit fields when not success
	var requestBody []byte
	var err error

	if req.ResponseCode == "00" || req.ResponseCode == "01" {
		// Success: include all business fields except bill_count
		filteredRequest := map[string]interface{}{
			"ref_id":         req.RefID,
			"partner_ref_id": req.PartnerRefID,
			"client_number":  req.ClientNumber,
			"product_code":   req.ProductCode,
			"response_code":  req.ResponseCode,
			"message":        req.Message,
			"admin_fee":      req.AdminFee,
			"total_amount":   req.TotalAmount,
			"timestamp":      req.Timestamp,
			"bill_details":   req.BillDetails,
		}
		requestBody, err = json.Marshal(filteredRequest)
	} else {
		// Error: exclude partner_ref_id, total_amount, bill_count, admin_fee, bill_details
		filteredRequest := map[string]interface{}{
			"ref_id":        req.RefID,
			"client_number": req.ClientNumber,
			"product_code":  req.ProductCode,
			"response_code": req.ResponseCode,
			"message":       req.Message,
			"timestamp":     req.Timestamp,
		}
		requestBody, err = json.Marshal(filteredRequest)
	}

	if err != nil {
		u.logger.Error("Failed to marshal callback request", "error", err, "ref_id", req.RefID)
		return fmt.Errorf("failed to marshal callback request: %w", err)
	}

	// 2. Sign the request
	signature, err := u.digitalSignatureService.SignPayload(ctx, string(requestBody))
	if err != nil {
		u.logger.Error("Failed to sign callback request", "error", err, "ref_id", req.RefID)
		return fmt.Errorf("failed to sign callback request: %w", err)
	}

	// 3. Encrypt the request
	encryptedPayload, encryptedKey, err := u.cryptoService.Encrypt(ctx, requestBody)
	if err != nil {
		u.logger.Error("Failed to encrypt callback request", "error", err, "ref_id", req.RefID)
		return fmt.Errorf("failed to encrypt callback request: %w", err)
	}

	// 4. Get environment variables
	callbackURL := os.Getenv("CALLBACK_URL")
	guid := os.Getenv("GUID")

	if callbackURL == "" {
		u.logger.Error("CALLBACK_URL environment variable not set", "ref_id", req.RefID)
		return fmt.Errorf("CALLBACK_URL environment variable not set")
	}

	if guid == "" {
		u.logger.Error("GUID environment variable not set", "ref_id", req.RefID)
		return fmt.Errorf("GUID environment variable not set")
	}

	// 5. Create HTTP request with encrypted payload directly as body
	httpReq, err := http.NewRequestWithContext(ctx, "POST", callbackURL, bytes.NewBuffer(encryptedPayload))
	if err != nil {
		u.logger.Error("Failed to create HTTP request", "error", err, "ref_id", req.RefID)
		return fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// 6. Set headers
	httpReq.Header.Set("Content-Type", "application/octet-stream")
	httpReq.Header.Set("Client-Guid", guid)
	httpReq.Header.Set("Key", encryptedKey)
	httpReq.Header.Set("Signature", signature)

	// 7. Make HTTP request
	u.logger.Info("Sending callback to Tokopedia", "url", callbackURL, "ref_id", req.RefID)

	resp, err := u.httpClient.Do(httpReq)
	if err != nil {
		u.logger.Error("Failed to send callback to Tokopedia", "error", err, "ref_id", req.RefID)
		// Save failed request to database (store plain JSON request)
		u.saveCallbackLog(ctx, req, httpReq, nil, err, startTime, string(requestBody), nil, nil)
		return fmt.Errorf("failed to send callback to Tokopedia: %w", err)
	}
	defer resp.Body.Close()

	// 8. Read response
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		u.logger.Error("Failed to read callback response", "error", err, "ref_id", req.RefID)
		u.saveCallbackLog(ctx, req, httpReq, resp, err, startTime, string(requestBody), nil, nil)
		return fmt.Errorf("failed to read callback response: %w", err)
	}

	// 9. Decrypt response from Tokopedia
	var decryptedResponseBody []byte
	responseKey := resp.Header.Get("Key")
	if responseKey != "" && len(responseBody) > 0 {
		decryptedResponseBody, err = u.cryptoService.Decrypt(ctx, responseBody, responseKey)
		if err != nil {
			u.logger.Error("Failed to decrypt callback response", "error", err, "ref_id", req.RefID)
			// Continue even if decryption fails - save encrypted response
			decryptedResponseBody = nil
		} else {
			u.logger.Info("Successfully decrypted callback response",
				"ref_id", req.RefID,
				"decrypted_response", string(decryptedResponseBody),
				"decrypted_length", len(decryptedResponseBody))
		}
	} else {
		u.logger.Warn("Response key or body is empty, skipping decryption",
			"ref_id", req.RefID,
			"has_key", responseKey != "",
			"has_body", len(responseBody) > 0)
	}

	// 10. Save request and response to database (store plain JSON request)
	u.saveCallbackLog(ctx, req, httpReq, resp, nil, startTime, string(requestBody), responseBody, decryptedResponseBody)

	// 11. Log response
	u.logger.Info("Callback sent successfully",
		"ref_id", req.RefID,
		"status_code", resp.StatusCode,
		"response_time_ms", time.Since(startTime).Milliseconds())

	return nil
}

// saveCallbackLog saves the callback request and response to the database
func (u *callbackUsecase) saveCallbackLog(ctx context.Context, req *domain.CallbackRequestDomain, httpReq *http.Request, httpResp *http.Response, reqErr error, startTime time.Time, requestBody string, responseBody []byte, decryptedResponseBody []byte) {

	// Prepare request headers as JSON string
	requestHeadersJSON, _ := json.Marshal(httpReq.Header)

	// Prepare response data
	var statusCode int = 0
	var responseHeadersJSON []byte
	var errorMessage string

	if httpResp != nil {
		statusCode = httpResp.StatusCode
		responseHeadersJSON, _ = json.Marshal(httpResp.Header)
	} else {
		// Ensure valid JSON for response headers when no response (e.g., DNS/timeout)
		responseHeadersJSON = []byte("{}")
	}

	if reqErr != nil {
		errorMessage = reqErr.Error()
	}

	// Prepare response body - use decrypted if available, otherwise use encrypted; empty on network errors
	var responseBodyToStore string
	if len(decryptedResponseBody) > 0 {
		responseBodyToStore = string(decryptedResponseBody)
		u.logger.Info("Storing decrypted response body", "ref_id", req.RefID, "body_length", len(decryptedResponseBody))
	} else if len(responseBody) > 0 {
		responseBodyToStore = string(responseBody)
		u.logger.Info("Storing encrypted response body", "ref_id", req.RefID, "body_length", len(responseBody))
	}

	// Parse decrypted response to extract business data if available
	var responseCode string
	var totalAmount *float64
	if len(decryptedResponseBody) > 0 {
		var responseData map[string]interface{}
		if err := json.Unmarshal(decryptedResponseBody, &responseData); err == nil {
			if rc, ok := responseData["response_code"].(string); ok {
				responseCode = rc
			}
			if ta, ok := responseData["total_amount"].(float64); ok {
				totalAmount = &ta
			}
		}
	}

	// Create callback log request
	logReq := &domain.CallbackLogInsertRequest{
		RequestID:       req.RefID,
		Method:          "POST",
		Path:            os.Getenv("CALLBACK_URL"),
		QueryParams:     "",
		RequestHeaders:  string(requestHeadersJSON),
		RequestBody:     requestBody,
		StatusCode:      statusCode,
		ResponseHeaders: string(responseHeadersJSON),
		ResponseBody:    responseBodyToStore,
		ClientIP:        "", // Not applicable for outbound requests
		UserAgent:       "", // Not applicable for outbound requests
		ResponseTimeMs:  time.Since(startTime).Milliseconds(),
		RequestTime:     startTime,
		ResponseTime:    time.Now(),
		Error:           errorMessage,
		CallbackType:    "payment_status",
		PartnerRefID:    req.PartnerRefID,
		ClientNumber:    req.ClientNumber,
		ProductCode:     req.ProductCode,
		ResponseCode:    responseCode,
		TotalAmount:     totalAmount,
	}

	// Save to database (best effort - don't fail the main operation)
	go func() {
		u.logger.Info("Saving callback log", "log_req", logReq)
		if _, err := u.callbackLoggingRepository.InsertCallbackLog(context.Background(), logReq); err != nil {
			u.logger.Error("Failed to save callback log", "error", err, "ref_id", req.RefID)
		}
	}()
}

// ListenCallbackQueue consumes messages from queue "callback" and processes them.
// Auto-reconnects with exponential backoff if RabbitMQ connection is lost.
func (u *callbackUsecase) ListenCallbackQueue(ctx context.Context) error {
	queueName := os.Getenv("RABBITMQ_CALLBACK_QUEUE_NAME")
	maxRetries := 10
	baseDelay := 2 * time.Second
	maxDelay := 2 * time.Minute

	for {
		select {
		case <-ctx.Done():
			u.logger.Info("Callback consumer stopped by context cancellation")
			return ctx.Err()
		default:
		}

		u.logger.Info("Starting callback consumer", "queue", queueName)
		deliveries, err := u.rabbit.Consume(ctx, queueName)
		if err != nil {
			u.logger.Error("Failed to start consume, will retry", "error", err, "queue", queueName)

			// Retry with exponential backoff
			for retry := 1; retry <= maxRetries; retry++ {
				delay := baseDelay * time.Duration(1<<uint(retry-1)) // 2s, 4s, 8s, 16s, 32s, 64s, 120s (max)
				if delay > maxDelay {
					delay = maxDelay
				}

				u.logger.Info("Retrying callback consumer connection", "retry", retry, "maxRetries", maxRetries, "delay", delay)

				select {
				case <-ctx.Done():
					return ctx.Err()
				case <-time.After(delay):
				}

				deliveries, err = u.rabbit.Consume(ctx, queueName)
				if err == nil {
					u.logger.Info("Callback consumer reconnected successfully", "retry", retry)
					break
				}
				u.logger.Error("Retry failed", "retry", retry, "error", err)
			}

			if err != nil {
				u.logger.Error("Failed to reconnect callback consumer after max retries, will restart loop", "maxRetries", maxRetries)
				time.Sleep(maxDelay) // Wait before restarting the whole loop
				continue
			}
		}

		u.logger.Info("Callback consumer connected successfully, listening for messages", "queue", queueName)

		// Process messages
		func() {
			for msg := range deliveries {
				u.logger.Info("Callback received", "message", string(msg.Body))
				var payload domain.CallbackRequestDomain
				if err := json.Unmarshal(msg.Body, &payload); err != nil {
					u.logger.Error("Failed to unmarshal callback payload", "error", err)
					msg.Nack(false, false) // Reject message without requeue
					continue
				}

				// Get payment status from database using ref_id (like check-status logic)
				u.logger.Info("Getting payment status from database for callback", "ref_id", payload.RefID)
				paymentStatus, err := u.postgresPaymentRepo.GetPaymentStatusByRefID(ctx, payload.RefID)
				if err != nil {
					u.logger.Error("Failed to get payment status for callback", "error", err, "ref_id", payload.RefID)
					// Don't reject message, use payload data as fallback
					u.logger.Warn("Using payload data as fallback for callback", "ref_id", payload.RefID)
				} else {
					// Build formatted callback request from database data
					payload = domain.CallbackRequestDomain{
						RefID:        paymentStatus.RefID,
						PartnerRefID: paymentStatus.PartnerRefID,
						ClientNumber: paymentStatus.ClientNumber,
						ProductCode:  paymentStatus.ProductCode,
						ResponseCode: paymentStatus.ResponseCode,
						Message:      paymentStatus.Message,
						AdminFee:     paymentStatus.AdminFee,
						TotalAmount:  paymentStatus.TotalAmount,
						Timestamp:    time.Now().Format("2006-01-02 15:04:05"),
						BillCount:    paymentStatus.BillCount,
						BillDetails:  paymentStatus.BillDetails,
					}
					u.logger.Info("Callback data formatted from database",
						"ref_id", payload.RefID,
						"partner_ref_id", payload.PartnerRefID,
						"response_code", payload.ResponseCode,
						"bill_count", payload.BillCount)
				}

				// Apply response formatter (same logic as check-status API)
				payloadMap := map[string]interface{}{
					"ref_id":         payload.RefID,
					"partner_ref_id": payload.PartnerRefID,
					"client_number":  payload.ClientNumber,
					"product_code":   payload.ProductCode,
					"response_code":  payload.ResponseCode,
					"message":        payload.Message,
					"admin_fee":      payload.AdminFee,
					"total_amount":   payload.TotalAmount,
					"timestamp":      payload.Timestamp,
					"bill_count":     payload.BillCount,
					"bill_details":   payload.BillDetails,
				}

				// Format response based on response_code (filter fields if not success)
				formattedMap := middleware.FormatCallbackResponse(payloadMap)
				u.logger.Info("Callback response formatted",
					"ref_id", payload.RefID,
					"response_code", payload.ResponseCode,
					"formatted_fields_count", len(formattedMap))

				// Rebuild payload from formatted map
				if refID, ok := formattedMap["ref_id"].(string); ok {
					payload.RefID = refID
				}
				if partnerRefID, ok := formattedMap["partner_ref_id"].(string); ok {
					payload.PartnerRefID = partnerRefID
				}
				if clientNumber, ok := formattedMap["client_number"].(string); ok {
					payload.ClientNumber = clientNumber
				}
				if productCode, ok := formattedMap["product_code"].(string); ok {
					payload.ProductCode = productCode
				}
				if responseCode, ok := formattedMap["response_code"].(string); ok {
					payload.ResponseCode = responseCode
				}
				if message, ok := formattedMap["message"].(string); ok {
					payload.Message = message
				}
				if timestamp, ok := formattedMap["timestamp"].(string); ok {
					payload.Timestamp = timestamp
				}

				// If response_code != "00" and != "01", clear optional fields (filtered by formatter)
				if payload.ResponseCode != "00" && payload.ResponseCode != "01" {
					payload.AdminFee = 0
					payload.TotalAmount = 0
					payload.BillCount = 0
					payload.BillDetails = nil
					u.logger.Info("Callback filtered for non-success response",
						"ref_id", payload.RefID,
						"response_code", payload.ResponseCode)
				}

				// Call callback api to tokopedia asynchronously
				go u.CallbackToTokopedia(ctx, &payload)

				msg.Ack(false)
			}
		}()

		// If we reach here, deliveries channel was closed (connection lost)
		u.logger.Warn("Callback consumer connection lost, deliveries channel closed. Reconnecting...")
		time.Sleep(baseDelay) // Brief pause before reconnecting
	}
}
