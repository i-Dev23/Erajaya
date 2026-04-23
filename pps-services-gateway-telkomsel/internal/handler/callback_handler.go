package handler

import (
	"context"
	"encoding/json"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
	"pps-services-gateway-telkomsel/internal/infrastructure/mqpublisher"
	"pps-services-gateway-telkomsel/internal/util"

	"github.com/gofiber/fiber/v2"
)

// CallbackQuery merepresentasikan query parameter dari callback Telkomsel.
type CallbackQuery struct {
	TransactionID    string `query:"transaction_id"`
	OrganizationCode string `query:"organization_code"`
	ServiceID        string `query:"service_id"`
	Status           string `query:"status"`
	Message          string `query:"message"`
	SerialNumber     string `query:"serial_number"`
}

// CallbackResponse adalah format response JSON untuk callback endpoint.
type CallbackResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// CallbackHandler menangani HTTP callback dari Telkomsel.
type CallbackHandler struct {
	logger            contractsvc.Logger
	transactionLogger contractsvc.TransactionLogger
	mqPublisher       contractsvc.MQPublisher
	queueName         string
}

// NewCallbackHandler membuat instance baru CallbackHandler.
func NewCallbackHandler(
	logger contractsvc.Logger,
	transactionLogger contractsvc.TransactionLogger,
	mqPublisher contractsvc.MQPublisher,
	queueName string,
) *CallbackHandler {
	return &CallbackHandler{
		logger:            logger,
		transactionLogger: transactionLogger,
		mqPublisher:       mqPublisher,
		queueName:         queueName,
	}
}

// Handle memproses callback GET request dari Telkomsel.
// Route: GET /callback/ext
func (h *CallbackHandler) Handle(c *fiber.Ctx) error {
	callbackRequestedAt := time.Now()
	var query CallbackQuery
	if err := c.QueryParser(&query); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(CallbackResponse{
			Status: "ERROR", Message: "Invalid query parameters: " + err.Error(),
		})
	}

	// Validate mandatory parameters
	if strings.TrimSpace(query.TransactionID) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(CallbackResponse{
			Status: "ERROR", Message: "transaction_id is required",
		})
	}
	if strings.TrimSpace(query.OrganizationCode) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(CallbackResponse{
			Status: "ERROR", Message: "organization_code is required",
		})
	}
	orgCodeLen := len(strings.TrimSpace(query.OrganizationCode))
	if orgCodeLen < 6 || orgCodeLen > 13 {
		return c.Status(fiber.StatusBadRequest).JSON(CallbackResponse{
			Status: "ERROR", Message: fmt.Sprintf("organization_code length must be 6-13 characters, got %d", orgCodeLen),
		})
	}
	if strings.TrimSpace(query.ServiceID) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(CallbackResponse{
			Status: "ERROR", Message: "service_id is required",
		})
	}
	if len(strings.TrimSpace(query.ServiceID)) != 13 {
		return c.Status(fiber.StatusBadRequest).JSON(CallbackResponse{
			Status: "ERROR", Message: fmt.Sprintf("service_id length must be 13 characters, got %d", len(strings.TrimSpace(query.ServiceID))),
		})
	}
	status := strings.TrimSpace(query.Status)
	if status != "SUCCESS" && status != "FAILED" {
		return c.Status(fiber.StatusBadRequest).JSON(CallbackResponse{
			Status: "ERROR", Message: fmt.Sprintf("status must be SUCCESS or FAILED, got '%s'", status),
		})
	}

	// Normalize callback status for downstream consumers.
	callbackStatusCode := "1" // FAILED
	statusToBe := "C"         // Cancel/Failed
	if status == "SUCCESS" {
		callbackStatusCode = "0"
		statusToBe = "F"
	}
	if strings.TrimSpace(query.Message) == "" {
		return c.Status(fiber.StatusBadRequest).JSON(CallbackResponse{
			Status: "ERROR", Message: "message is required",
		})
	}

	// URL-decode message
	decodedMessage, err := url.QueryUnescape(query.Message)
	if err != nil {
		decodedMessage = query.Message
	}

	h.logger.Info("callback received",
		"transaction_id", query.TransactionID,
		"organization_code", query.OrganizationCode,
		"service_id", query.ServiceID,
		"status", status,
		"message", decodedMessage,
		"serial_number", query.SerialNumber,
	)

	// Lookup transaction to get msg_id, MQTransaction, QueueName
	msgID := query.TransactionID // fallback
	mqTransactionURL := ""
	publishQueueName := h.queueName
	amount := 0
	clientNumber := ""

	if h.transactionLogger != nil {
		dbCtx, dbCancel := context.WithTimeout(c.UserContext(), 5*time.Second)
		defer dbCancel()

		txRec, lookupErr := h.transactionLogger.GetTransactionByOurTrxID(dbCtx, query.TransactionID)
		if lookupErr != nil {
			h.logger.Warn("failed to lookup transaction for callback, using fallback",
				"transaction_id", query.TransactionID, "error", lookupErr)
		} else {
			msgID = txRec.MsgID
			amount = txRec.Amount
			clientNumber = txRec.MSISDN
			mqTransactionURL = txRec.MQTransaction
			if txRec.QueueName != "" {
				publishQueueName = txRec.QueueName
			}
		}

		rawPayload, _ := json.Marshal(map[string]string{
			"transaction_id":    query.TransactionID,
			"organization_code": query.OrganizationCode,
			"service_id":        query.ServiceID,
			"status":            status,
			"message":           decodedMessage,
			"serial_number":     query.SerialNumber,
		})

		insertCtx, insertCancel := context.WithTimeout(c.UserContext(), 5*time.Second)
		defer insertCancel()

		if insertErr := h.transactionLogger.InsertCallbackResponse(insertCtx, contractsvc.ResponseRecord{
			MsgID:      msgID,
			OurTrxID:   query.TransactionID,
			StatusCode: callbackStatusCode,
			StatusDesc: decodedMessage,
			RawPayload: rawPayload,
		}); insertErr != nil {
			h.logger.Error("failed to insert callback response",
				"transaction_id", query.TransactionID, "error", insertErr)
		}
	}

	// Publish to downstream RabbitMQ
	if h.mqPublisher != nil && mqTransactionURL != "" {
		msgIDInt := 0
		if n, err := strconv.Atoi(msgID); err == nil {
			msgIDInt = n
		}

		{
			dbCtx, cancel := context.WithTimeout(c.UserContext(), 5*time.Second)
			defer cancel()
			if err := h.transactionLogger.UpdateTransactionStatus(dbCtx, msgID, status); err != nil {
				h.logger.Error("failed to update transaction status", "msg_id", msgID, "status", status, "error", err)
			}
		}

		messageToCustomer := util.GenerateMessage(amount, clientNumber, callbackRequestedAt, statusToBe, query.SerialNumber, callbackStatusCode)
		msg := mqpublisher.NewProviderPublishMessage(mqpublisher.ProviderPublishData{
			MsgID:             msgIDInt,
			StatusToBe:        statusToBe,
			SerialNumber:      query.SerialNumber,
			ClientNumber:      query.ServiceID,
			ConversationID:    query.TransactionID,
			MessageToCustomer: messageToCustomer,
			QueueName:         publishQueueName,
		})
		body, _ := json.Marshal(msg)

		if pubErr := h.mqPublisher.Publish(c.UserContext(), mqTransactionURL, publishQueueName, body); pubErr != nil {
			h.logger.Error("failed to publish callback to downstream rabbitmq",
				"transaction_id", query.TransactionID, "queue_name", publishQueueName,
				"mq_transaction", sanitizeAMQPURL(mqTransactionURL), "error", pubErr)
		} else {
			h.logger.Info("published callback to downstream rabbitmq",
				"transaction_id", query.TransactionID, "queue_name", publishQueueName,
				"mq_transaction", sanitizeAMQPURL(mqTransactionURL))
		}
	} else if h.mqPublisher == nil {
		h.logger.Warn("mq publisher not initialized, skipping callback publish",
			"transaction_id", query.TransactionID)
	} else if mqTransactionURL == "" {
		h.logger.Warn("mq_transaction URL not available, skipping callback publish",
			"transaction_id", query.TransactionID)
	}

	return c.Status(fiber.StatusOK).JSON(CallbackResponse{
		Status: "OK", Message: "Callback received",
	})
}

func sanitizeAMQPURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil || u == nil || u.User == nil {
		return raw
	}
	username := u.User.Username()
	if _, hasPass := u.User.Password(); hasPass {
		u.User = url.UserPassword(username, "***")
	}
	return u.String()
}
