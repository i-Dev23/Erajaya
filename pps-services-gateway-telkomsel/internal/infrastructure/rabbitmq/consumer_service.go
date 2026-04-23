package rabbitmq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"pps-services-gateway-telkomsel/internal/config"
	contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
	"pps-services-gateway-telkomsel/internal/infrastructure/mqpublisher"
	"pps-services-gateway-telkomsel/internal/util"
	"pps-services-gateway-telkomsel/pkg/telkomsel"

	amqp "github.com/rabbitmq/amqp091-go"
)

type consumePayload struct {
	Amount        int
	StockType     string
	ProductCode   string
	ProductID     string
	ProductType   string
	MID           string
	StoreID       string
	QueueName     string
	MSISDN        string
	MsgID         string
	CallbackURL   string
	TypeVoucher   string
	Command       string
	MQTransaction string // RabbitMQ URL untuk update status transaksi via MQ
}

func (p *consumePayload) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return err
	}

	p.Amount = parseInt(getAny(raw, "amount"))
	p.StockType = parseString(getAny(raw, "stock_type", "stockType"))
	p.ProductCode = parseString(getAny(raw, "product_code", "productCode"))
	p.ProductID = parseString(getAny(raw, "product_id", "productId", "productID"))
	p.ProductType = normalizeProductType(parseString(getAny(raw, "product_type", "productType")))
	p.MID = parseString(getAny(raw, "mid"))
	p.StoreID = parseString(getAny(raw, "store_id", "storeId", "storeID"))
	p.QueueName = parseString(getAny(raw, "queue_name", "queueName"))
	p.MSISDN = parseString(getAny(raw, "msisdn"))
	p.MsgID = parseString(getAny(raw, "msgid", "msgID"))
	p.CallbackURL = parseString(getAny(raw, "callback_url", "callbackUrl", "callbackURL"))
	if p.CallbackURL == "" {
		p.CallbackURL = telkomsel.GenerateCallbackURL()
	}
	p.TypeVoucher = parseString(getAny(raw, "typeVoucher", "type_voucher"))
	p.Command = parseString(getAny(raw, "command"))
	p.MQTransaction = parseString(getAny(raw, "MQTransaction", "mqTransaction", "mq_transaction"))

	// Fallback: jika msisdn kosong, gunakan clientNumber
	if p.MSISDN == "" {
		p.MSISDN = parseString(getAny(raw, "clientNumber", "client_number"))
	}

	// Parse command field untuk extract productCode, amount, productId, stockType
	// Format pulsa:      {kodeVoucher}*{nominal}*{stockType}        contoh: SP15*15000*BULK
	// Format data:       {kodeVoucher}*{nominal}*{BID}*{stockType}  contoh: PP10*5000*00061469*BULK_VALUE_VAS_AKUISISI
	if p.Command != "" {
		p.parseCommand()
	}

	// Derive productType dari typeVoucher jika productType kosong
	if p.ProductType == "" && p.TypeVoucher != "" {
		switch strings.ToLower(strings.TrimSpace(p.TypeVoucher)) {
		case "pulsa":
			p.ProductType = "pulsa"
		case "paket data", "data":
			p.ProductType = "data"
		}
	}

	// Backward-compat: normalize legacy value "paket data" to canonical "data".
	p.ProductType = normalizeProductType(p.ProductType)

	return nil
}

// parseCommand parses the command field to extract productCode, amount, productId, and stockType.
// Pulsa format:      {kodeVoucher}*{nominal}*{stockType}        contoh: SP15*15000*BULK
// Data format:       {kodeVoucher}*{nominal}*{BID}*{stockType}  contoh: PP10*5000*00061469*BULK_VALUE_VAS_AKUISISI
func (p *consumePayload) parseCommand() {
	parts := strings.Split(p.Command, "*")
	typeVoucher := strings.ToLower(strings.TrimSpace(p.TypeVoucher))

	switch typeVoucher {
	case "pulsa":
		// Format: {kodeVoucher}*{nominal}*{stockType}
		if len(parts) >= 3 {
			if p.ProductCode == "" {
				p.ProductCode = strings.TrimSpace(parts[0])
			}
			if p.Amount == 0 {
				p.Amount = parseInt(parts[1])
			}
			if p.StockType == "" {
				p.StockType = strings.TrimSpace(parts[2])
			}
		}
	case "paket data", "data":
		// Format: {kodeVoucher}*{nominal}*{BID}*{stockType}
		if len(parts) >= 4 {
			if p.ProductCode == "" {
				p.ProductCode = strings.TrimSpace(parts[0])
			}
			if p.Amount == 0 {
				p.Amount = parseInt(parts[1])
			}
			if p.ProductID == "" {
				p.ProductID = strings.TrimSpace(parts[2])
			}
			if p.StockType == "" {
				p.StockType = strings.TrimSpace(parts[3])
			}
		}
	}
}

func normalizeProductType(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	switch s {
	case "pulsa":
		return "pulsa"
	case "paket data", "data":
		return "data"
	default:
		return strings.TrimSpace(raw)
	}
}

func getAny(m map[string]any, keys ...string) any {
	if m == nil {
		return nil
	}
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

func parseString(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(t)
	case json.Number:
		return t.String()
	case float64:
		if t == float64(int64(t)) {
			return strconv.FormatInt(int64(t), 10)
		}
		return strconv.FormatFloat(t, 'f', -1, 64)
	default:
		return strings.TrimSpace(fmt.Sprintf("%v", v))
	}
}

func parseInt(v any) int {
	switch t := v.(type) {
	case nil:
		return 0
	case json.Number:
		if n, err := t.Int64(); err == nil {
			return int(n)
		}
		if f, err := t.Float64(); err == nil {
			return int(f)
		}
	case float64:
		return int(t)
	case string:
		if n, err := strconv.Atoi(strings.TrimSpace(t)); err == nil {
			return n
		}
	}
	return 0
}

const (
	initialReconnectDelay = 1 * time.Second
	maxReconnectDelay     = 30 * time.Second

	// StatusToBe constants for downstream MQ publish messages.
	StatusToBeFinish  = "F" // transaksi selesai sukses (pulsa)
	StatusToBeCancel  = "C" // transaksi gagal / dibatalkan
	StatusToBeProcess = "S" // transaksi masih diproses (data, menunggu callback)
)

// Compile-time interface compliance check.
var _ contractsvc.ConsumerService = (*ConsumerServiceImpl)(nil)

// ConsumerServiceImpl handles RabbitMQ message consumption.
type ConsumerServiceImpl struct {
	cfg               *config.Config
	logger            contractsvc.Logger
	mqPublisher       contractsvc.MQPublisher
	transactionLogger contractsvc.TransactionLogger
	retryConfig       *config.RetryConfig
}

// NewConsumerServiceImpl creates a new RabbitMQ consumer service instance.
func NewConsumerServiceImpl(cfg *config.Config, logger contractsvc.Logger) *ConsumerServiceImpl {
	return &ConsumerServiceImpl{cfg: cfg, logger: logger}
}

// SetMQPublisher menyuntikkan MQ publisher untuk forwarding status transaksi.
func (s *ConsumerServiceImpl) SetMQPublisher(pub contractsvc.MQPublisher) {
	s.mqPublisher = pub
}

// SetRetryConfig sets the retry configuration for check status retries.
func (s *ConsumerServiceImpl) SetRetryConfig(cfg *config.RetryConfig) {
	s.retryConfig = cfg
}

// SetTransactionLogger sets the transaction logger for database logging.
func (s *ConsumerServiceImpl) SetTransactionLogger(tl contractsvc.TransactionLogger) {
	s.transactionLogger = tl
}

// Start opens RabbitMQ connection and consumes messages until context is canceled.
func (s *ConsumerServiceImpl) Start(ctx context.Context) error {
	reconnectDelay := initialReconnectDelay

	for {
		if ctx.Err() != nil {
			s.logger.Info("rabbitmq consumer stopped", "reason", "context canceled")
			return nil
		}

		err := s.consumeSession(ctx)
		if err == nil {
			s.logger.Info("rabbitmq consumer stopped", "reason", "context canceled")
			return nil
		}

		s.logger.Warn("rabbitmq consume session ended, attempting recovery", "error", err, "nextReconnectIn", reconnectDelay.String())

		select {
		case <-ctx.Done():
			s.logger.Info("rabbitmq consumer stopped", "reason", "context canceled")
			return nil
		case <-time.After(reconnectDelay):
		}

		reconnectDelay *= 2
		if reconnectDelay > maxReconnectDelay {
			reconnectDelay = maxReconnectDelay
		}
	}
}

func (s *ConsumerServiceImpl) consumeSession(ctx context.Context) error {
	conn, err := amqp.Dial(s.cfg.RabbitMQURL)
	if err != nil {
		return fmt.Errorf("failed to connect rabbitmq: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open rabbitmq channel: %w", err)
	}
	defer ch.Close()

	if _, err = ch.QueueDeclare(
		s.cfg.QueueName,
		true,
		false,
		false,
		false,
		nil,
	); err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}

	if err = ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("failed to set qos: %w", err)
	}

	deliveries, err := ch.Consume(
		s.cfg.QueueName,
		s.cfg.ConsumerTag,
		false,
		false,
		false,
		false,
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}

	notifyClose := make(chan *amqp.Error, 1)
	ch.NotifyClose(notifyClose)

	s.logger.Info("rabbitmq consumer started", "queue", s.cfg.QueueName, "consumerTag", s.cfg.ConsumerTag)

	var waitGroup sync.WaitGroup
	waitGroup.Add(1)

	go func() {
		defer waitGroup.Done()
		for delivery := range deliveries {
			s.logger.Info(
				"message received",
				"queue", s.cfg.QueueName,
				"messageId", delivery.MessageId,
				"correlationId", delivery.CorrelationId,
				"routingKey", delivery.RoutingKey,
				"payload", string(delivery.Body),
			)

			var payload consumePayload
			if err := json.Unmarshal(delivery.Body, &payload); err != nil {
				s.logger.Error(
					"failed to parse message payload",
					"queue", s.cfg.QueueName,
					"messageId", delivery.MessageId,
					"error", err,
				)
			} else {
				queueName := s.cfg.QueueName
				if strings.TrimSpace(payload.QueueName) != "" {
					queueName = payload.QueueName
				}

				msgID := strings.TrimSpace(payload.MsgID)
				if msgID == "" {
					msgID = strings.TrimSpace(delivery.MessageId)
				}
				if msgID == "" {
					msgID = strings.TrimSpace(delivery.CorrelationId)
				}

				s.logger.Info(
					"payload parsed",
					"amount", payload.Amount,
					"stock_type", payload.StockType,
					"product_code", payload.ProductCode,
					"product_id", payload.ProductID,
					"product_type", payload.ProductType,
					"mid", payload.MID,
					"store_id", payload.StoreID,
					"queue_name", queueName,
					"msisdn", payload.MSISDN,
					"msgid", msgID,
				)

				productType := strings.ToLower(strings.TrimSpace(payload.ProductType))
				switch productType {
				case "pulsa":
					requestedAt := time.Now()
					ourTrxID, genErr := telkomsel.GenerateTransactionID(payload.MID, msgID, requestedAt)
					if genErr != nil {
						// Keep consumer resilient; the Telkomsel call will likely fail too if MID mapping is missing.
						s.logger.Warn("failed to generate telkomsel transaction_id; falling back to msgid", "mid", payload.MID, "msgid", msgID, "error", genErr)
						ourTrxID = msgID
					}

					// Log transaction as PROCESSING before calling Telkomsel API
					s.logInsertTransaction(ctx, contractsvc.TransactionRecord{
						MsgID:         msgID,
						OurTrxID:      ourTrxID,
						MSISDN:        payload.MSISDN,
						MID:           payload.MID,
						ProductType:   "pulsa",
						ProductID:     payload.ProductID,
						Amount:        payload.Amount,
						StockType:     payload.StockType,
						QueueName:     queueName,
						MQTransaction: payload.MQTransaction,
					})

					resp, callErr := telkomsel.InitiateRegularRechargeOnConsumeWithTransactionID(ctx, payload.MSISDN, payload.MID, queueName, msgID, ourTrxID, payload.Amount, payload.StockType)
					if callErr != nil {
						responseLatencyMs := time.Since(requestedAt).Milliseconds()
						s.logger.Error(
							"telkomsel initiate regular recharge failed",
							"queue", queueName,
							"msisdn", payload.MSISDN,
							"mid", payload.MID,
							"msgid", msgID,
							"error", callErr,
						)
						// Log error sync response
						esbStatusCode := ""
						if resp != nil {
							esbStatusCode = resp.Transaction.StatusCode
						}
						s.logInsertSyncResponse(ctx, contractsvc.ResponseRecord{
							MsgID:             msgID,
							OurTrxID:          ourTrxID,
							StatusCode:        "ERROR",
							StatusDesc:        callErr.Error(),
							RequestedAt:       requestedAt,
							ResponseLatencyMs: responseLatencyMs,
						})

						// Extract HTTP status code from error type
						httpStatusCode := 400 // default for BusinessError
						var techErr *telkomsel.TechnicalError
						if errors.As(callErr, &techErr) && techErr.StatusCode > 0 {
							httpStatusCode = techErr.StatusCode
						}
						var bizErr *telkomsel.BusinessError
						if errors.As(callErr, &bizErr) {
							esbStatusCode = bizErr.Code
						}
						// If Telkomsel/edge returns an HTML "Request Rejected" page, treat as definitive FAILED (no retry).
						rcPPS := 9
						var rejectedErr *telkomsel.RejectedError
						if errors.As(callErr, &rejectedErr) {
							rcPPS = 1
							if strings.TrimSpace(esbStatusCode) == "" {
								esbStatusCode = "REJECTED"
							}
							if rejectedErr.StatusCode > 0 {
								httpStatusCode = rejectedErr.StatusCode
							}
						} else if strings.TrimSpace(esbStatusCode) == "" {
							// If Telkomsel doesn't provide status_code, treat as FAILED and do not retry.
							rcPPS = 1
						} else {
							rcPPS = telkomsel.ResolveRCPPS(ctx, httpStatusCode, esbStatusCode)
						}
						switch rcPPS {
						case 0:
							s.logUpdateStatus(ctx, msgID, "SUCCESS")
							s.logCheckDuplicateSyncResponse(ctx, msgID)
							msgIDInt := 0
							if n, err := strconv.Atoi(msgID); err == nil {
								msgIDInt = n
							}
							statusToBe := StatusToBeFinish
							serialNumber := ""
							if resp != nil {
								serialNumber = resp.SerialNumber
							}
							messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, serialNumber, esbStatusCode)
							s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
								MsgID:             msgIDInt,
								StatusToBe:        statusToBe,
								SerialNumber:      serialNumber,
								ClientNumber:      payload.MSISDN,
								Nominal:           fmt.Sprintf("%d", payload.Amount),
								ConversationID:    msgID,
								MessageToCustomer: messageToCustomer,
								AdditionalMessage: callErr.Error(),
								QueueName:         queueName,
							})
						case 1:
							s.logUpdateStatus(ctx, msgID, "FAILED")
							s.logCheckDuplicateSyncResponse(ctx, msgID)
							msgIDInt := 0
							if n, err := strconv.Atoi(msgID); err == nil {
								msgIDInt = n
							}
							statusToBe := StatusToBeCancel
							serialNumber := ""
							messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, serialNumber, esbStatusCode)
							s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
								MsgID:             msgIDInt,
								StatusToBe:        statusToBe,
								SerialNumber:      serialNumber,
								ClientNumber:      payload.MSISDN,
								Nominal:           fmt.Sprintf("%d", payload.Amount),
								ConversationID:    msgID,
								MessageToCustomer: messageToCustomer,
								AdditionalMessage: callErr.Error(),
								QueueName:         queueName,
							})
						case 9:
							s.retryCheckStatus(ctx, payload, msgID, queueName, requestedAt, callErr)
						}
					} else if resp != nil {
						responseLatencyMs := time.Since(requestedAt).Milliseconds()
						s.logger.Info(
							"telkomsel initiate regular recharge response",
							"queue", queueName,
							"msisdn", payload.MSISDN,
							"mid", payload.MID,
							"transaction_id", resp.Transaction.TransactionID,
							"status_code", resp.Transaction.StatusCode,
							"status_desc", resp.Transaction.StatusDesc,
						)
						// Log sync response
						s.logInsertSyncResponse(ctx, contractsvc.ResponseRecord{
							MsgID:             msgID,
							OurTrxID:          resp.Transaction.TransactionID,
							TelkomselTrxID:    resp.Transaction.TransactionID,
							StatusCode:        resp.Transaction.StatusCode,
							StatusDesc:        resp.Transaction.StatusDesc,
							RequestedAt:       requestedAt,
							ResponseLatencyMs: responseLatencyMs,
						})
						// Resolve RC PPS using error mapping
						if strings.TrimSpace(resp.Transaction.StatusCode) == "" {
							s.logUpdateStatus(ctx, msgID, "FAILED")
							s.logCheckDuplicateSyncResponse(ctx, msgID)

							msgIDInt := 0
							if n, err := strconv.Atoi(msgID); err == nil {
								msgIDInt = n
							}
							statusToBe := StatusToBeCancel
							messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, resp.SerialNumber, "")
							s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
								MsgID:             msgIDInt,
								StatusToBe:        statusToBe,
								SerialNumber:      resp.SerialNumber,
								ClientNumber:      payload.MSISDN,
								Nominal:           fmt.Sprintf("%d", payload.Amount),
								ConversationID:    resp.Transaction.TransactionID,
								MessageToCustomer: messageToCustomer,
								AdditionalMessage: "missing telkomsel status_code",
								QueueName:         queueName,
							})
							return
						}
						rcPPS := telkomsel.ResolveRCPPS(ctx, resp.HTTPStatusCode, resp.Transaction.StatusCode)
						switch rcPPS {
						case 0:
							s.logUpdateStatus(ctx, msgID, "SUCCESS")
							s.logCheckDuplicateSyncResponse(ctx, msgID)
							msgIDInt := 0
							if n, err := strconv.Atoi(msgID); err == nil {
								msgIDInt = n
							}
							statusToBe := StatusToBeFinish
							messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, resp.SerialNumber, resp.Transaction.StatusCode)
							s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
								MsgID:             msgIDInt,
								StatusToBe:        statusToBe,
								SerialNumber:      resp.SerialNumber,
								ClientNumber:      payload.MSISDN,
								Nominal:           fmt.Sprintf("%d", payload.Amount),
								ConversationID:    resp.Transaction.TransactionID,
								MessageToCustomer: messageToCustomer,
								QueueName:         queueName,
							})
						case 1:
							s.logUpdateStatus(ctx, msgID, "FAILED")
							s.logCheckDuplicateSyncResponse(ctx, msgID)
							msgIDInt := 0
							if n, err := strconv.Atoi(msgID); err == nil {
								msgIDInt = n
							}
							statusToBe := StatusToBeCancel
							messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, resp.SerialNumber, resp.Transaction.StatusCode)
							s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
								MsgID:             msgIDInt,
								StatusToBe:        statusToBe,
								SerialNumber:      resp.SerialNumber,
								ClientNumber:      payload.MSISDN,
								Nominal:           fmt.Sprintf("%d", payload.Amount),
								ConversationID:    resp.Transaction.TransactionID,
								MessageToCustomer: messageToCustomer,
								QueueName:         queueName,
							})
						case 9:
							s.retryCheckStatus(ctx, payload, msgID, queueName, requestedAt, nil)
						}
					}
				case "data", "paket data":
					productID := strings.TrimSpace(payload.ProductID)

					// our_trx_id must match Telkomsel callback transaction_id (OrderDealer transaction_id).
					processingAt := time.Now()
					ourTrxID, genErr := telkomsel.GenerateTransactionID(payload.MID, msgID, processingAt)
					if genErr != nil {
						s.logger.Warn("failed to generate telkomsel transaction_id for order dealer; falling back to msgid", "mid", payload.MID, "msgid", msgID, "error", genErr)
						ourTrxID = msgID
					}

					// Log transaction as PROCESSING before calling Telkomsel API
					s.logInsertTransaction(ctx, contractsvc.TransactionRecord{
						MsgID:         msgID,
						OurTrxID:      ourTrxID,
						MSISDN:        payload.MSISDN,
						MID:           payload.MID,
						ProductType:   "data",
						ProductID:     productID,
						Amount:        payload.Amount,
						StockType:     payload.StockType,
						QueueName:     queueName,
						MQTransaction: payload.MQTransaction,
					})

					browseRequestedAt := time.Now()
					browseResp, callErr := telkomsel.BrowseOfferOnConsume(ctx, payload.MSISDN, payload.MID, queueName, msgID, productID)
					browseStatusCode := ""
					browseStatusDesc := ""
					if browseResp != nil {
						browseStatusCode = strings.TrimSpace(browseResp.Transaction.StatusCode)
						browseStatusDesc = strings.TrimSpace(browseResp.Transaction.StatusDesc)
					}
					browseSuccess := browseStatusCode == "00000" || browseStatusCode == "RV-0000"
					if callErr != nil || browseResp == nil || !browseSuccess {
						browseLatencyMs := time.Since(browseRequestedAt).Milliseconds()
						s.logger.Error(
							"telkomsel browse offer failed",
							"queue", queueName,
							"msisdn", payload.MSISDN,
							"mid", payload.MID,
							"store_id", payload.StoreID,
							"msgid", msgID,
							"product_id", productID,
							"error", callErr,
						)
						statusDesc := ""
						if callErr != nil {
							statusDesc = callErr.Error()
						} else if browseResp == nil {
							statusDesc = "nil response"
						} else {
							statusDesc = fmt.Sprintf("browse offer failed: status_code=%s status_desc=%s", browseStatusCode, browseStatusDesc)
						}

						// Log error sync response for BrowseOffer
						s.logInsertSyncResponse(ctx, contractsvc.ResponseRecord{
							MsgID:             msgID,
							OurTrxID:          ourTrxID,
							StatusCode:        "ERROR",
							StatusDesc:        statusDesc,
							RequestedAt:       browseRequestedAt,
							ResponseLatencyMs: browseLatencyMs,
						})
						s.logUpdateStatus(ctx, msgID, "FAILED")
						s.logCheckDuplicateSyncResponse(ctx, msgID)
						// Forward failure to downstream RabbitMQ
						msgIDInt := 0
						if n, err := strconv.Atoi(msgID); err == nil {
							msgIDInt = n
						}
						statusToBe := StatusToBeCancel
						serialNumber := ""
						messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, browseRequestedAt, statusToBe, serialNumber, browseStatusCode)
						s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
							MsgID:             msgIDInt,
							StatusToBe:        statusToBe,
							SerialNumber:      serialNumber,
							ClientNumber:      payload.MSISDN,
							Nominal:           fmt.Sprintf("%d", payload.Amount),
							ConversationID:    msgID,
							MessageToCustomer: messageToCustomer,
							AdditionalMessage: statusDesc,
							QueueName:         queueName,
						})
					} else {
						browseLatencyMs := time.Since(browseRequestedAt).Milliseconds()
						s.logger.Info(
							"telkomsel browse offer response",
							"queue", queueName,
							"msisdn", payload.MSISDN,
							"mid", payload.MID,
							"store_id", payload.StoreID,
							"msgid", msgID,
							"transaction_id", browseResp.Transaction.TransactionID,
							"status_code", browseStatusCode,
							"status_desc", browseStatusDesc,
							"product_id", productID,
						)
						// Log sync response for BrowseOffer
						s.logInsertSyncResponse(ctx, contractsvc.ResponseRecord{
							MsgID:             msgID,
							OurTrxID:          browseResp.Transaction.TransactionID,
							TelkomselTrxID:    browseResp.Transaction.TransactionID,
							StatusCode:        browseStatusCode,
							StatusDesc:        browseStatusDesc,
							RequestedAt:       browseRequestedAt,
							ResponseLatencyMs: browseLatencyMs,
						})

						// Proceed to order dealer after successful browse offer.
						orderRequestedAt := time.Now()
						orderResp, orderErr := telkomsel.OrderDealerOnConsumeWithTransactionID(
							ctx,
							payload.MSISDN,
							payload.MID,
							queueName,
							msgID,
							ourTrxID,
							productID,
							payload.StockType,
							payload.StoreID,
							payload.CallbackURL,
						)
						if orderErr != nil {
							orderLatencyMs := time.Since(orderRequestedAt).Milliseconds()
							s.logger.Error(
								"telkomsel order dealer failed",
								"queue", queueName,
								"msisdn", payload.MSISDN,
								"mid", payload.MID,
								"store_id", payload.StoreID,
								"msgid", msgID,
								"product_id", productID,
								"error", orderErr,
							)
							// Log error sync response for OrderDealer
							s.logInsertSyncResponse(ctx, contractsvc.ResponseRecord{
								MsgID:             msgID,
								OurTrxID:          ourTrxID,
								StatusCode:        "ERROR",
								StatusDesc:        orderErr.Error(),
								RequestedAt:       orderRequestedAt,
								ResponseLatencyMs: orderLatencyMs,
							})
							s.logUpdateStatus(ctx, msgID, "FAILED")
							// Forward failure to downstream RabbitMQ
							msgIDInt := 0
							if n, err := strconv.Atoi(msgID); err == nil {
								msgIDInt = n
							}
							statusToBe := StatusToBeCancel
							serialNumber := ""
							statusCode := ""
							if orderResp != nil {
								statusCode = orderResp.Transaction.StatusCode
							}
							messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, orderRequestedAt, statusToBe, serialNumber, statusCode)
							s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
								MsgID:             msgIDInt,
								StatusToBe:        statusToBe,
								SerialNumber:      serialNumber,
								ClientNumber:      payload.MSISDN,
								Nominal:           fmt.Sprintf("%d", payload.Amount),
								ConversationID:    msgID,
								MessageToCustomer: messageToCustomer,
								AdditionalMessage: orderErr.Error(),
								QueueName:         queueName,
							})
						} else if orderResp != nil {
							orderLatencyMs := time.Since(orderRequestedAt).Milliseconds()
							s.logger.Info(
								"telkomsel order dealer response",
								"queue", queueName,
								"msisdn", payload.MSISDN,
								"mid", payload.MID,
								"store_id", payload.StoreID,
								"msgid", msgID,
								"transaction_id", orderResp.Transaction.TransactionID,
								"status_code", orderResp.Transaction.StatusCode,
								"status_desc", orderResp.Transaction.StatusDesc,
								"product_id", productID,
							)
							// Log sync response for OrderDealer
							s.logInsertSyncResponse(ctx, contractsvc.ResponseRecord{
								MsgID:             msgID,
								OurTrxID:          orderResp.Transaction.TransactionID,
								TelkomselTrxID:    orderResp.Transaction.TransactionID,
								StatusCode:        orderResp.Transaction.StatusCode,
								StatusDesc:        orderResp.Transaction.StatusDesc,
								RequestedAt:       orderRequestedAt,
								ResponseLatencyMs: orderLatencyMs,
							})
							// Resolve RC PPS using error mapping
							if strings.TrimSpace(orderResp.Transaction.StatusCode) == "" {
								s.logUpdateStatus(ctx, msgID, "FAILED")
								s.logCheckDuplicateSyncResponse(ctx, msgID)
								msgIDInt := 0
								if n, err := strconv.Atoi(msgID); err == nil {
									msgIDInt = n
								}
								statusToBe := StatusToBeCancel
								serialNumber := ""
								messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, orderRequestedAt, statusToBe, serialNumber, "")
								s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
									MsgID:             msgIDInt,
									StatusToBe:        statusToBe,
									SerialNumber:      serialNumber,
									ClientNumber:      payload.MSISDN,
									Nominal:           fmt.Sprintf("%d", payload.Amount),
									ConversationID:    orderResp.Transaction.TransactionID,
									MessageToCustomer: messageToCustomer,
									AdditionalMessage: "missing telkomsel status_code",
									QueueName:         queueName,
								})
								return
							}
							rcPPS := telkomsel.ResolveRCPPS(ctx, orderResp.HTTPStatusCode, orderResp.Transaction.StatusCode)
							switch rcPPS {
							case 0:
								s.logUpdateStatus(ctx, msgID, "PROCESSING")
								s.logCheckDuplicateSyncResponse(ctx, msgID)
								msgIDInt := 0
								if n, err := strconv.Atoi(msgID); err == nil {
									msgIDInt = n
								}
								statusToBe := StatusToBeProcess
								serialNumber := ""
								messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, orderRequestedAt, statusToBe, serialNumber, orderResp.Transaction.StatusCode)
								s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
									MsgID:             msgIDInt,
									StatusToBe:        statusToBe,
									SerialNumber:      serialNumber,
									ClientNumber:      payload.MSISDN,
									Nominal:           fmt.Sprintf("%d", payload.Amount),
									ConversationID:    orderResp.Transaction.TransactionID,
									MessageToCustomer: messageToCustomer,
									QueueName:         queueName,
								})
								s.retryCheckStatusPaketData(ctx, payload, msgID, queueName, orderRequestedAt, orderResp.Transaction.TransactionID, "")
							case 1:
								s.logUpdateStatus(ctx, msgID, "FAILED")
								s.logCheckDuplicateSyncResponse(ctx, msgID)
								msgIDInt := 0
								if n, err := strconv.Atoi(msgID); err == nil {
									msgIDInt = n
								}
								statusToBe := StatusToBeCancel
								serialNumber := ""
								messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, orderRequestedAt, statusToBe, serialNumber, orderResp.Transaction.StatusCode)
								s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
									MsgID:             msgIDInt,
									StatusToBe:        statusToBe,
									SerialNumber:      serialNumber,
									ClientNumber:      payload.MSISDN,
									Nominal:           fmt.Sprintf("%d", payload.Amount),
									ConversationID:    orderResp.Transaction.TransactionID,
									MessageToCustomer: messageToCustomer,
									QueueName:         queueName,
								})
							case 9:
								s.retryCheckStatusPaketData(ctx, payload, msgID, queueName, orderRequestedAt, orderResp.Transaction.TransactionID, "")
							}
						}
					}
				default:
					s.logger.Warn(
						"unsupported product type",
						"queue", queueName,
						"msisdn", payload.MSISDN,
						"mid", payload.MID,
						"msgid", msgID,
						"product_type", payload.ProductType,
					)
				}
			}

			if ackErr := delivery.Ack(false); ackErr != nil {
				s.logger.Error("failed to ack message", "error", ackErr)
			}
		}
	}()

	select {
	case <-ctx.Done():
		_ = ch.Cancel(s.cfg.ConsumerTag, false)
		awaitDrain(&waitGroup, s.cfg.ReadTimeout, s.logger)
		return nil
	case closeErr := <-notifyClose:
		awaitDrain(&waitGroup, s.cfg.ReadTimeout, s.logger)
		if closeErr == nil {
			return fmt.Errorf("rabbitmq channel closed")
		}
		return fmt.Errorf("rabbitmq channel closed unexpectedly: %w", closeErr)
	}
}

// publishToDownstream mempublikasikan status transaksi ke RabbitMQ tujuan.
// Non-blocking: error di-log saja, tidak menghentikan alur utama.
func (s *ConsumerServiceImpl) publishToDownstream(ctx context.Context, mqTransactionURL, queueName string, data mqpublisher.ProviderPublishData) {
	if s.mqPublisher == nil {
		s.logger.Warn("mq publisher not initialized, skipping publish",
			"msg_id", data.MsgID, "queue_name", queueName)
		return
	}

	msg := mqpublisher.NewProviderPublishMessage(data)
	body, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error("failed to marshal publish message",
			"msg_id", data.MsgID, "error", err)
		return
	}

	if err := s.mqPublisher.Publish(ctx, mqTransactionURL, queueName, body); err != nil {
		s.logger.Error("failed to publish to downstream rabbitmq",
			"msg_id", data.MsgID, "queue_name", queueName,
			"mq_transaction", mqTransactionURL, "error", err)
		return
	}

	s.logger.Info("published to downstream rabbitmq",
		"msg_id", data.MsgID, "queue_name", queueName,
		"mq_transaction", mqTransactionURL)
}

// logInsertTransaction inserts a PROCESSING transaction record. Non-blocking: errors are logged only.
func (s *ConsumerServiceImpl) logInsertTransaction(ctx context.Context, rec contractsvc.TransactionRecord) {
	if s.transactionLogger == nil {
		return
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.transactionLogger.InsertTransaction(dbCtx, rec); err != nil {
		s.logger.Error("failed to insert transaction log", "msg_id", rec.MsgID, "error", err)
	}
}

// logUpdateStatus updates the transaction status. Non-blocking: errors are logged only.
func (s *ConsumerServiceImpl) logUpdateStatus(ctx context.Context, msgID string, status string) {
	if s.transactionLogger == nil {
		return
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.transactionLogger.UpdateTransactionStatus(dbCtx, msgID, status); err != nil {
		s.logger.Error("failed to update transaction status", "msg_id", msgID, "status", status, "error", err)
	}
}

// logInsertSyncResponse inserts a SYNC response record. Non-blocking: errors are logged only.
func (s *ConsumerServiceImpl) logInsertSyncResponse(ctx context.Context, rec contractsvc.ResponseRecord) {
	if s.transactionLogger == nil {
		return
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := s.transactionLogger.InsertSyncResponse(dbCtx, rec); err != nil {
		s.logger.Error("failed to insert sync response", "msg_id", rec.MsgID, "error", err)
	}
}

// logCheckDuplicateSyncResponse checks for duplicate SYNC responses and logs a warning. Non-blocking.
func (s *ConsumerServiceImpl) logCheckDuplicateSyncResponse(ctx context.Context, msgID string) {
	if s.transactionLogger == nil {
		return
	}
	dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	responses, err := s.transactionLogger.GetResponsesByMsgID(dbCtx, msgID)
	if err != nil {
		s.logger.Error("failed to check duplicate responses", "msg_id", msgID, "error", err)
		return
	}
	syncCount := 0
	for _, r := range responses {
		if r.StatusCode != "ERROR" { // count non-error SYNC responses
			syncCount++
		}
	}
	if syncCount > 1 {
		s.logger.Warn("duplicate SYNC responses detected", "msg_id", msgID, "sync_count", syncCount)
	}
}

// retryCheckStatus launches an asynchronous goroutine to perform retry check order status
// when RC PPS is 9 (pending). This is non-blocking so the consumer can continue processing
// other messages while retries happen in the background.
func (s *ConsumerServiceImpl) retryCheckStatus(ctx context.Context, payload consumePayload, msgID string, queueName string, requestedAt time.Time, originalErr error) {
	s.logger.Info("retryCheckStatus starting async retry",
		"queue", queueName,
		"msisdn", payload.MSISDN,
		"mid", payload.MID,
		"msgid", msgID,
	)
	go s.retryCheckStatusSync(context.Background(), payload, msgID, queueName, requestedAt, originalErr)
}

// retryCheckStatusPaketData launches an asynchronous goroutine to perform retry check order status
// for paket data flow. Unlike retryCheckStatus (used by pulsa), this goroutine checks the transaction
// status in the database before calling the Check Order Status API, avoiding unnecessary API calls
// if a callback has already resolved the transaction.
func (s *ConsumerServiceImpl) retryCheckStatusPaketData(ctx context.Context, payload consumePayload, msgID string, queueName string, requestedAt time.Time, originalTransactionID string, serialNumber string) {
	s.logger.Info("retryCheckStatusPaketData starting async retry",
		"queue", queueName,
		"msisdn", payload.MSISDN,
		"mid", payload.MID,
		"msgid", msgID,
	)
	go s.retryCheckStatusPaketDataSync(context.Background(), payload, msgID, queueName, requestedAt, originalTransactionID, serialNumber)
}

// retryCheckStatusPaketDataSync performs the actual retry check order status loop for paket data.
// Unlike retryCheckStatusSync (pulsa), this method checks the transaction status in the database
// before each API call, avoiding unnecessary Check Order Status API calls if a callback has
// already resolved the transaction.
func (s *ConsumerServiceImpl) retryCheckStatusPaketDataSync(ctx context.Context, payload consumePayload, msgID string, queueName string, requestedAt time.Time, originalTransactionID string, serialNumber string) {
	// If retryConfig is nil, treat as failed immediately (no retry).
	if s.retryConfig == nil {
		s.logger.Warn("retryCheckStatusPaketDataSync retryConfig is nil, treating as FAILED without retry",
			"queue", queueName,
			"msisdn", payload.MSISDN,
			"mid", payload.MID,
			"msgid", msgID,
		)
		s.logUpdateStatus(ctx, msgID, "FAILED")
		s.logCheckDuplicateSyncResponse(ctx, msgID)

		msgIDInt := 0
		if n, err := strconv.Atoi(msgID); err == nil {
			msgIDInt = n
		}
		statusToBe := StatusToBeCancel
		messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, "", "")
		s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
			MsgID:             msgIDInt,
			StatusToBe:        statusToBe,
			SerialNumber:      "",
			ClientNumber:      payload.MSISDN,
			Nominal:           fmt.Sprintf("%d", payload.Amount),
			ConversationID:    msgID,
			MessageToCustomer: messageToCustomer,
			AdditionalMessage: "pending: max retry reached",
			QueueName:         queueName,
		})
		return
	}

	maxAttempts := s.retryConfig.MaxAttempts
	waitDuration := s.retryConfig.WaitDuration

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		time.Sleep(waitDuration)

		// Step 1: Check transaction status in database before API call.
		if s.transactionLogger != nil {
			dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
			dbStatus, dbErr := s.transactionLogger.GetTransactionStatusByMsgID(dbCtx, msgID)
			cancel()

			if dbErr != nil {
				s.logger.Error("retryCheckStatusPaketDataSync DB status check failed, falling back to API call",
					"queue", queueName,
					"msgid", msgID,
					"attempt", attempt,
					"max_attempts", maxAttempts,
					"error", dbErr,
				)
			} else {
				upperStatus := strings.ToUpper(strings.TrimSpace(dbStatus))
				if upperStatus == "SUCCESS" || upperStatus == "FAILED" {
					s.logger.Info("retryCheckStatusPaketDataSync transaction already resolved in DB, stopping",
						"queue", queueName,
						"msisdn", payload.MSISDN,
						"mid", payload.MID,
						"msgid", msgID,
						"attempt", attempt,
						"db_status", upperStatus,
					)
					return
				}
			}
		}

		// Step 2: Call Check Order Status API.
		s.logger.Info("retryCheckStatusPaketDataSync calling CheckOrderStatus API",
			"queue", queueName,
			"msisdn", payload.MSISDN,
			"mid", payload.MID,
			"msgid", msgID,
			"attempt", attempt,
			"max_attempts", maxAttempts,
		)

		checkResp, checkErr := telkomsel.CheckOrderStatusOnConsume(
			ctx,
			payload.MSISDN,
			payload.MID,
			queueName,
			msgID,
			originalTransactionID,
			serialNumber,
		)

		if checkErr != nil {
			s.logger.Error("retryCheckStatusPaketDataSync check order status failed",
				"queue", queueName,
				"msisdn", payload.MSISDN,
				"mid", payload.MID,
				"msgid", msgID,
				"attempt", attempt,
				"error", checkErr,
			)
			var rejectedErr *telkomsel.RejectedError
			if errors.As(checkErr, &rejectedErr) {
				s.logger.Warn("retryCheckStatusPaketDataSync request rejected, treating as FAILED without retry",
					"queue", queueName,
					"msgid", msgID,
					"attempt", attempt,
					"status_code", rejectedErr.StatusCode,
					"support_id", rejectedErr.SupportID,
				)
				s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, checkErr, "REJECTED")
				return
			}
			// If we still got a parsed response, resolve RC PPS.
			if checkResp != nil {
				if strings.TrimSpace(checkResp.Transaction.StatusCode) == "" {
					s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, fmt.Errorf("missing telkomsel status_code"), "")
					return
				}
				rcPPS := telkomsel.ResolveRCPPS(ctx, checkResp.HTTPStatusCode, checkResp.Transaction.StatusCode)
				s.logger.Info("retryCheckStatusPaketDataSync resolved RC PPS from error response",
					"queue", queueName,
					"msgid", msgID,
					"attempt", attempt,
					"status_code", checkResp.Transaction.StatusCode,
					"rc_pps", rcPPS,
				)
				switch rcPPS {
				case 0:
					s.logger.Info("retryCheckStatusPaketDataSync SUCCESS (from error response)",
						"queue", queueName,
						"msgid", msgID,
						"attempt", attempt,
					)
					s.logUpdateStatus(ctx, msgID, "SUCCESS")
					s.logCheckDuplicateSyncResponse(ctx, msgID)

					msgIDInt := 0
					if n, err := strconv.Atoi(msgID); err == nil {
						msgIDInt = n
					}
					statusToBe := StatusToBeFinish
					sn := ""
					if checkResp.TransactionStatus != nil {
						sn = checkResp.TransactionStatus.SerialNumber
					}
					messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, sn, checkResp.Transaction.StatusCode)
					s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
						MsgID:             msgIDInt,
						StatusToBe:        statusToBe,
						SerialNumber:      sn,
						ClientNumber:      payload.MSISDN,
						Nominal:           fmt.Sprintf("%d", payload.Amount),
						ConversationID:    msgID,
						MessageToCustomer: messageToCustomer,
						QueueName:         queueName,
					})
					return

				case 1:
					s.logger.Info("retryCheckStatusPaketDataSync FAILED (RC 1 from error response)",
						"queue", queueName,
						"msgid", msgID,
						"attempt", attempt,
					)
					s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, checkErr, checkResp.Transaction.StatusCode)
					return

				case 9:
					continue
				}
			}
			// No parsed response — continue to next iteration
			continue
		}

		if checkResp == nil {
			s.logger.Warn("retryCheckStatusPaketDataSync nil response, continuing retry",
				"queue", queueName,
				"msgid", msgID,
				"attempt", attempt,
			)
			continue
		}

		// Step 3: Resolve RC PPS from successful response.
		if strings.TrimSpace(checkResp.Transaction.StatusCode) == "" {
			s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, fmt.Errorf("missing telkomsel status_code"), "")
			return
		}
		rcPPS := telkomsel.ResolveRCPPS(ctx, checkResp.HTTPStatusCode, checkResp.Transaction.StatusCode)
		s.logger.Info("retryCheckStatusPaketDataSync resolved RC PPS",
			"queue", queueName,
			"msgid", msgID,
			"attempt", attempt,
			"status_code", checkResp.Transaction.StatusCode,
			"rc_pps", rcPPS,
		)

		switch rcPPS {
		case 0:
			s.logger.Info("retryCheckStatusPaketDataSync SUCCESS",
				"queue", queueName,
				"msgid", msgID,
				"attempt", attempt,
			)
			s.logUpdateStatus(ctx, msgID, "SUCCESS")
			s.logCheckDuplicateSyncResponse(ctx, msgID)

			msgIDInt := 0
			if n, err := strconv.Atoi(msgID); err == nil {
				msgIDInt = n
			}
			statusToBe := StatusToBeFinish
			sn := ""
			if checkResp.TransactionStatus != nil {
				sn = checkResp.TransactionStatus.SerialNumber
			}
			messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, sn, checkResp.Transaction.StatusCode)
			s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
				MsgID:             msgIDInt,
				StatusToBe:        statusToBe,
				SerialNumber:      sn,
				ClientNumber:      payload.MSISDN,
				Nominal:           fmt.Sprintf("%d", payload.Amount),
				ConversationID:    msgID,
				MessageToCustomer: messageToCustomer,
				QueueName:         queueName,
			})
			return

		case 1:
			s.logger.Info("retryCheckStatusPaketDataSync FAILED (RC 1)",
				"queue", queueName,
				"msgid", msgID,
				"attempt", attempt,
			)
			s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, nil, checkResp.Transaction.StatusCode)
			return

		case 9:
			s.logger.Info("retryCheckStatusPaketDataSync still pending (RC 9)",
				"queue", queueName,
				"msgid", msgID,
				"attempt", attempt,
			)
			continue
		}
	}

	// Max retry reached and still pending — process as failed.
	s.logger.Warn("retryCheckStatusPaketDataSync max attempts reached, treating as PENDING",
		"queue", queueName,
		"msisdn", payload.MSISDN,
		"mid", payload.MID,
		"msgid", msgID,
		"max_attempts", maxAttempts,
	)
	//s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, nil, "")
}

// retryCheckStatusSync performs the actual retry check order status loop synchronously.
// It loops up to retryConfig.MaxAttempts times, sleeping retryConfig.WaitDuration between attempts.
// If a definitive RC (0 or 1) is received, it processes accordingly and returns.
// If retryConfig is nil, the transaction is treated as failed immediately.
func (s *ConsumerServiceImpl) retryCheckStatusSync(ctx context.Context, payload consumePayload, msgID string, queueName string, requestedAt time.Time, originalErr error) {
	// If retryConfig is nil, treat as failed immediately (no retry).
	if s.retryConfig == nil {
		s.logger.Warn("retryConfig is nil, treating as FAILED without retry",
			"queue", queueName,
			"msisdn", payload.MSISDN,
			"mid", payload.MID,
			"msgid", msgID,
		)
		errStatusCode := ""
		var bizErr *telkomsel.BusinessError
		if errors.As(originalErr, &bizErr) {
			errStatusCode = bizErr.Code
		}
		s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, originalErr, errStatusCode)
		return
	}

	maxAttempts := s.retryConfig.MaxAttempts
	waitDuration := s.retryConfig.WaitDuration

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		s.logger.Info("retryCheckStatus attempt",
			"queue", queueName,
			"msisdn", payload.MSISDN,
			"mid", payload.MID,
			"msgid", msgID,
			"attempt", attempt,
			"max_attempts", maxAttempts,
		)

		time.Sleep(waitDuration)

		checkResp, checkErr := telkomsel.CheckOrderStatusOnConsume(
			ctx,
			payload.MSISDN,
			payload.MID,
			queueName,
			msgID,
			msgID, // originalTransactionID — use msgID as our transaction reference
			"",    // serialNumber — not available in this context
		)

		if checkErr != nil {
			s.logger.Error("retryCheckStatus check order status failed",
				"queue", queueName,
				"msisdn", payload.MSISDN,
				"mid", payload.MID,
				"msgid", msgID,
				"attempt", attempt,
				"error", checkErr,
			)
			var rejectedErr *telkomsel.RejectedError
			if errors.As(checkErr, &rejectedErr) {
				s.logger.Warn("retryCheckStatus request rejected, treating as FAILED without retry",
					"queue", queueName,
					"msgid", msgID,
					"attempt", attempt,
					"status_code", rejectedErr.StatusCode,
					"support_id", rejectedErr.SupportID,
				)
				s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, checkErr, "REJECTED")
				return
			}
			// If we still got a parsed response, resolve RC PPS instead of blindly retrying.
			if checkResp != nil {
				if strings.TrimSpace(checkResp.Transaction.StatusCode) == "" {
					s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, fmt.Errorf("missing telkomsel status_code"), "")
					return
				}
				rcPPS := telkomsel.ResolveRCPPS(ctx, checkResp.HTTPStatusCode, checkResp.Transaction.StatusCode)
				s.logger.Info("retryCheckStatus resolved RC PPS from error response",
					"queue", queueName,
					"msgid", msgID,
					"attempt", attempt,
					"status_code", checkResp.Transaction.StatusCode,
					"rc_pps", rcPPS,
				)
				switch rcPPS {
				case 0:
					s.logger.Info("retryCheckStatus SUCCESS (from error response)",
						"queue", queueName,
						"msgid", msgID,
						"attempt", attempt,
					)
					s.logUpdateStatus(ctx, msgID, "SUCCESS")
					s.logCheckDuplicateSyncResponse(ctx, msgID)

					msgIDInt := 0
					if n, err := strconv.Atoi(msgID); err == nil {
						msgIDInt = n
					}

					productType := strings.ToLower(strings.TrimSpace(payload.ProductType))
					statusToBe := StatusToBeFinish
					if productType == "data" || productType == "paket data" {
						statusToBe = StatusToBeProcess
					}

					serialNumber := ""
					if checkResp.TransactionStatus != nil {
						serialNumber = checkResp.TransactionStatus.SerialNumber
					}
					messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, serialNumber, checkResp.Transaction.StatusCode)
					s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
						MsgID:             msgIDInt,
						StatusToBe:        statusToBe,
						SerialNumber:      serialNumber,
						ClientNumber:      payload.MSISDN,
						Nominal:           fmt.Sprintf("%d", payload.Amount),
						ConversationID:    msgID,
						MessageToCustomer: messageToCustomer,
						QueueName:         queueName,
					})
					return

				case 1:
					s.logger.Info("retryCheckStatus FAILED (RC 1 from error response)",
						"queue", queueName,
						"msgid", msgID,
						"attempt", attempt,
					)
					s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, checkErr, checkResp.Transaction.StatusCode)
					return

				case 9:
					// Still pending — continue retry
					continue
				}
			}
			// No parsed response — treat as pending, continue retry
			continue
		}

		if checkResp == nil {
			s.logger.Warn("retryCheckStatus nil response, continuing retry",
				"queue", queueName,
				"msgid", msgID,
				"attempt", attempt,
			)
			continue
		}

		if strings.TrimSpace(checkResp.Transaction.StatusCode) == "" {
			s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, fmt.Errorf("missing telkomsel status_code"), "")
			return
		}
		rcPPS := telkomsel.ResolveRCPPS(ctx, checkResp.HTTPStatusCode, checkResp.Transaction.StatusCode)
		s.logger.Info("retryCheckStatus resolved RC PPS",
			"queue", queueName,
			"msgid", msgID,
			"attempt", attempt,
			"status_code", checkResp.Transaction.StatusCode,
			"rc_pps", rcPPS,
		)

		switch rcPPS {
		case 0:
			// SUCCESS
			s.logger.Info("retryCheckStatus SUCCESS",
				"queue", queueName,
				"msgid", msgID,
				"attempt", attempt,
			)
			s.logUpdateStatus(ctx, msgID, "SUCCESS")
			s.logCheckDuplicateSyncResponse(ctx, msgID)

			msgIDInt := 0
			if n, err := strconv.Atoi(msgID); err == nil {
				msgIDInt = n
			}

			// Determine statusToBe based on product type
			productType := strings.ToLower(strings.TrimSpace(payload.ProductType))
			statusToBe := StatusToBeFinish
			if productType == "data" || productType == "paket data" {
				statusToBe = StatusToBeProcess
			}

			serialNumber := ""
			if checkResp.TransactionStatus != nil {
				serialNumber = checkResp.TransactionStatus.SerialNumber
			}
			messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, serialNumber, checkResp.Transaction.StatusCode)
			s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
				MsgID:             msgIDInt,
				StatusToBe:        statusToBe,
				SerialNumber:      serialNumber,
				ClientNumber:      payload.MSISDN,
				Nominal:           fmt.Sprintf("%d", payload.Amount),
				ConversationID:    msgID,
				MessageToCustomer: messageToCustomer,
				QueueName:         queueName,
			})
			return

		case 1:
			// FAILED
			s.logger.Info("retryCheckStatus FAILED (RC 1)",
				"queue", queueName,
				"msgid", msgID,
				"attempt", attempt,
			)
			s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, nil, checkResp.Transaction.StatusCode)
			return

		case 9:
			// Still pending — continue retry
			s.logger.Info("retryCheckStatus still pending (RC 9)",
				"queue", queueName,
				"msgid", msgID,
				"attempt", attempt,
			)
			continue
		}
	}

	// Max retry reached and still RC 9 — process as failed
	s.logger.Warn("retryCheckStatus max attempts reached, treating as PENDING",
		"queue", queueName,
		"msisdn", payload.MSISDN,
		"mid", payload.MID,
		"msgid", msgID,
		"max_attempts", maxAttempts,
	)

	// errStatusCode := ""
	// var bizErr *telkomsel.BusinessError
	// if errors.As(originalErr, &bizErr) {
	// 	errStatusCode = bizErr.Code
	// }
	// s.processRetryFailed(ctx, payload, msgID, queueName, requestedAt, originalErr, errStatusCode)
}

// processRetryFailed handles the failed outcome of a retry check status flow.
func (s *ConsumerServiceImpl) processRetryFailed(ctx context.Context, payload consumePayload, msgID string, queueName string, requestedAt time.Time, originalErr error, statusCode string) {
	s.logUpdateStatus(ctx, msgID, "FAILED")
	s.logCheckDuplicateSyncResponse(ctx, msgID)

	msgIDInt := 0
	if n, err := strconv.Atoi(msgID); err == nil {
		msgIDInt = n
	}
	statusToBe := StatusToBeCancel
	serialNumber := ""
	additionalMsg := "pending: max retry reached"
	if originalErr != nil {
		additionalMsg = originalErr.Error()
	}
	messageToCustomer := util.GenerateMessage(payload.Amount, payload.MSISDN, requestedAt, statusToBe, serialNumber, statusCode)
	s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
		MsgID:             msgIDInt,
		StatusToBe:        statusToBe,
		SerialNumber:      serialNumber,
		ClientNumber:      payload.MSISDN,
		Nominal:           fmt.Sprintf("%d", payload.Amount),
		ConversationID:    msgID,
		MessageToCustomer: messageToCustomer,
		AdditionalMessage: additionalMsg,
		QueueName:         queueName,
	})
}

func awaitDrain(waitGroup *sync.WaitGroup, timeout time.Duration, logger contractsvc.Logger) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		waitGroup.Wait()
	}()

	select {
	case <-done:
	case <-time.After(timeout):
		logger.Warn("consumer shutdown timeout reached", "timeout", timeout.String())
	}
}
