package rabbitmq

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"pps-services-gateway-unipin/internal/config"
	contractsvc "pps-services-gateway-unipin/internal/domain/contract/service"
	"pps-services-gateway-unipin/internal/infrastructure/mqpublisher"
	"pps-services-gateway-unipin/internal/util"
	"pps-services-gateway-unipin/pkg/unipin"

	amqp "github.com/rabbitmq/amqp091-go"
)

type consumePayload struct {
	Amount      int
	StockType   string
	ProductCode string
	ProductID   string
	ProductType string
	MID         string
	StoreID     string
	QueueName   string

	// Legacy field — previously used as either MSISDN or JSON fields (for in-game validate user).
	MSISDN string

	// New (per consumer expectation)
	ClientNumber string
	IMSI         string
	RemarkImsi   string
	TypeVoucher  string
	VoucherCode  string
	Provider     string

	MsgID         string
	CallbackURL   string
	MQTransaction string
	Command       string
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
	p.ProductType = parseString(getAny(raw, "product_type", "productType"))
	p.MID = parseString(getAny(raw, "mid"))
	p.StoreID = parseString(getAny(raw, "store_id", "storeId", "storeID"))
	p.QueueName = parseString(getAny(raw, "queue_name", "queueName"))

	p.ClientNumber = parseString(getAny(raw, "clientNumber", "client_number"))
	p.IMSI = parseString(getAny(raw, "imsi"))
	p.RemarkImsi = parseString(getAny(raw, "remarkImsi", "remark_imsi"))
	p.TypeVoucher = parseString(getAny(raw, "typeVoucher", "type_voucher"))
	p.VoucherCode = parseString(getAny(raw, "voucherCode", "voucher_code"))
	p.Provider = parseString(getAny(raw, "provider"))

	p.MSISDN = parseString(getAny(raw, "msisdn"))
	p.MsgID = parseString(getAny(raw, "msgid", "msgID", "msgId"))
	p.CallbackURL = parseString(getAny(raw, "callback_url", "callbackUrl", "callbackURL"))
	p.MQTransaction = parseString(getAny(raw, "mq_transaction", "mqTransaction", "MQTransaction"))
	p.Command = parseString(getAny(raw, "command"))

	envMQ := strings.TrimSpace(os.Getenv("MQ_TRANSACTION_URL_UNIPIN"))
	if envMQ == "" {
		envMQ = strings.TrimSpace(os.Getenv("MQ_TRANSACTION_URL"))
	}
	if envMQ != "" {
		p.MQTransaction = envMQ
	}

	return nil
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
	// Fallback: case-insensitive match (e.g. "Command" vs "command").
	for mk, v := range m {
		for _, k := range keys {
			if strings.EqualFold(mk, k) {
				return v
			}
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

func looksLikeJSONValue(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{") || strings.HasPrefix(s, "[")
}

func looksLikeJSONObject(s string) bool {
	s = strings.TrimSpace(s)
	return strings.HasPrefix(s, "{")
}

func parseJSONMap(s string) (map[string]any, error) {
	dec := json.NewDecoder(bytes.NewReader([]byte(s)))
	dec.UseNumber()
	var fields map[string]any
	if err := dec.Decode(&fields); err != nil {
		return nil, err
	}
	return fields, nil
}

const (
	initialReconnectDelay = 1 * time.Second
	maxReconnectDelay     = 30 * time.Second
)

const (
	// StatusToBe constants for downstream MQ publish messages.
	StatusToBeFinish  = "F" // transaksi selesai sukses
	StatusToBeCancel  = "C" // transaksi gagal / dibatalkan
	StatusToBeProcess = "S" // transaksi masih diproses
)

func normalizeStatusToBe(raw string) string {
	s := strings.ToUpper(strings.TrimSpace(raw))
	switch s {
	case "F", "FINISH", "FINISHED", "SUCCESS":
		return StatusToBeFinish
	case "C", "CANCEL", "CANCELED", "CANCELLED", "FAILED", "FAIL":
		return StatusToBeCancel
	case "S", "PROCESS", "PROCESSING", "PENDING":
		return StatusToBeProcess
	default:
		// Unknown status: return as-is to avoid silently changing semantics.
		return strings.TrimSpace(raw)
	}
}

// jsonMarshal is a package-level variable to allow test override for error simulation.
var jsonMarshal = json.Marshal

// amqpDial is a package-level variable to allow test override for connection simulation.
var amqpDial = func(url string) (amqpConnection, error) {
	conn, err := amqp.Dial(url)
	if err != nil {
		return nil, err
	}
	return &realAMQPConn{conn: conn}, nil
}

// amqpConnection abstracts *amqp.Connection for testability.
type amqpConnection interface {
	Channel() (amqpChannel, error)
	Close() error
}

// amqpChannel abstracts *amqp.Channel for testability.
type amqpChannel interface {
	QueueDeclare(name string, durable, autoDelete, exclusive, noWait bool, args amqp.Table) (amqp.Queue, error)
	Qos(prefetchCount, prefetchSize int, global bool) error
	Consume(queue, consumer string, autoAck, exclusive, noLocal, noWait bool, args amqp.Table) (<-chan amqp.Delivery, error)
	NotifyClose(receiver chan *amqp.Error) chan *amqp.Error
	Cancel(consumer string, noWait bool) error
	Close() error
}

// realAMQPConn wraps *amqp.Connection to satisfy amqpConnection interface.
type realAMQPConn struct {
	conn *amqp.Connection
}

func (r *realAMQPConn) Channel() (amqpChannel, error) { return r.conn.Channel() }
func (r *realAMQPConn) Close() error                  { return r.conn.Close() }

var _ contractsvc.ConsumerService = (*ConsumerServiceImpl)(nil)

type ConsumerServiceImpl struct {
	cfg          *config.Config
	unipinClient *unipin.Client
	logger       contractsvc.Logger
	mqPublisher  contractsvc.MQPublisher
}

func NewConsumerServiceImpl(cfg *config.Config, unipinClient *unipin.Client, logger contractsvc.Logger) *ConsumerServiceImpl {
	return &ConsumerServiceImpl{cfg: cfg, unipinClient: unipinClient, logger: logger}
}

// SetMQPublisher menyuntikkan MQ publisher untuk forwarding status transaksi.
func (s *ConsumerServiceImpl) SetMQPublisher(pub contractsvc.MQPublisher) {
	s.mqPublisher = pub
}

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
	conn, err := amqpDial(s.cfg.RabbitMQURL)
	if err != nil {
		return fmt.Errorf("failed to connect rabbitmq: %w", err)
	}
	defer conn.Close()
	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("failed to open rabbitmq channel: %w", err)
	}
	defer ch.Close()
	if _, err = ch.QueueDeclare(s.cfg.QueueName, true, false, false, false, nil); err != nil {
		return fmt.Errorf("failed to declare queue: %w", err)
	}
	if err = ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("failed to set qos: %w", err)
	}
	deliveries, err := ch.Consume(s.cfg.QueueName, s.cfg.ConsumerTag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("failed to register consumer: %w", err)
	}
	notifyClose := make(chan *amqp.Error, 1)
	ch.NotifyClose(notifyClose)
	s.logger.Info("rabbitmq consumer started", "queue", s.cfg.QueueName, "consumerTag", s.cfg.ConsumerTag)
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for delivery := range deliveries {
			s.logger.Info("message received", "queue", s.cfg.QueueName, "messageId", delivery.MessageId, "correlationId", delivery.CorrelationId, "routingKey", delivery.RoutingKey, "payload", string(delivery.Body))
			s.processMessage(ctx, &delivery)
			if ackErr := delivery.Ack(false); ackErr != nil {
				s.logger.Error("failed to ack message", "error", ackErr)
			}
		}
	}()
	select {
	case <-ctx.Done():
		_ = ch.Cancel(s.cfg.ConsumerTag, false)
		awaitDrain(&wg, s.cfg.ReadTimeout, s.logger)
		return nil
	case closeErr := <-notifyClose:
		awaitDrain(&wg, s.cfg.ReadTimeout, s.logger)
		if closeErr == nil {
			return fmt.Errorf("rabbitmq channel closed")
		}
		return fmt.Errorf("rabbitmq channel closed unexpectedly: %w", closeErr)
	}
}

func (s *ConsumerServiceImpl) processMessage(ctx context.Context, delivery *amqp.Delivery) {
	var payload consumePayload
	if err := json.Unmarshal(delivery.Body, &payload); err != nil {
		s.logger.Error("failed to parse message payload", "queue", s.cfg.QueueName, "messageId", delivery.MessageId, "error", err)
		return
	}
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
		"client_number", payload.ClientNumber,
		"type_voucher", payload.TypeVoucher,
		"provider", payload.Provider,
		"msgid", msgID,
	)

	txType, txTypeSource := resolveTxType(payload.TypeVoucher, payload.ProductType)
	if txTypeSource == "product_type" {
		s.logger.Warn("type_voucher missing, using product_type fallback", "queue", queueName, "product_type", payload.ProductType, "msgid", msgID)
	}

	switch txType {
	case "GAME-VOUCHER":
		s.processVoucher(ctx, &payload, queueName, msgID)
	case "GAME-DIRECT-TOP-UP", "GAME-DIRECT-TO-UP":
		s.processGame(ctx, &payload, queueName, msgID)
	default:
		s.logger.Warn("unsupported tx type",
			"queue", queueName,
			"client_number", payload.ClientNumber,
			"msisdn", payload.MSISDN,
			"mid", payload.MID,
			"msgid", msgID,
			"type_voucher", payload.TypeVoucher,
			"product_type", payload.ProductType,
			"resolved_tx_type", txType,
		)
	}
}

func resolveTxType(typeVoucher string, productType string) (resolved string, source string) {
	resolved = strings.ToUpper(strings.TrimSpace(typeVoucher))
	if resolved != "" {
		switch strings.ToLower(resolved) {
		case "unipin-voucher":
			return "GAME-VOUCHER", "type_voucher"
		case "unipin-game":
			return "GAME-DIRECT-TOP-UP", "type_voucher"
		default:
			return resolved, "type_voucher"
		}
	}

	pt := strings.ToLower(strings.TrimSpace(productType))
	switch pt {
	case "unipin-voucher":
		return "GAME-VOUCHER", "product_type"
	case "unipin-game":
		return "GAME-DIRECT-TOP-UP", "product_type"
	default:
		return "", ""
	}
}

func (s *ConsumerServiceImpl) processVoucher(ctx context.Context, payload *consumePayload, queueName, msgID string) {
	command := strings.TrimSpace(payload.Command)
	if command == "" {
		s.logger.Error("voucher request skipped: command is empty",
			"queue_name", queueName, "client_number", payload.ClientNumber, "msisdn", payload.MSISDN, "mid", payload.MID, "msgid", msgID)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", "command is empty")
		return
	}

	// Format voucher (new): vouchercode*denomination_voucher_code*msisdn
	// Backward compatible (old): vouchercode*denomination_voucher_code
	parts := strings.SplitN(command, "*", 3)
	if len(parts) < 2 {
		s.logger.Error("voucher request skipped: command missing delimiter *",
			"queue_name", queueName, "client_number", payload.ClientNumber, "msisdn", payload.MSISDN, "mid", payload.MID, "msgid", msgID, "command", command)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", "command missing delimiter *")
		return
	}

	voucherCode := strings.TrimSpace(parts[0])
	denominationCode := strings.TrimSpace(parts[1])
	msisdnFromCommand := ""
	if len(parts) == 3 {
		msisdnFromCommand = strings.TrimSpace(parts[2])
		if msisdnFromCommand != "" {
			payload.ClientNumber = msisdnFromCommand
		}
	}

	if denominationCode == "" {
		s.logger.Error("voucher request skipped: denomination_code is empty after parsing command",
			"queue_name", queueName, "voucher_code", voucherCode, "client_number", payload.ClientNumber, "msisdn", payload.MSISDN, "mid", payload.MID, "msgid", msgID, "command", command)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", "denomination_code is empty")
		return
	}

	referenceNo := msgID
	if referenceNo == "" {
		s.logger.Error("voucher request skipped: msgid is empty", "queue", queueName, "client_number", payload.ClientNumber, "msisdn", payload.MSISDN, "mid", payload.MID)
		return
	}

	s.logger.Info("processing voucher request",
		"queue", queueName,
		"voucher_code", voucherCode,
		"denomination_code", denominationCode,
		"reference_no", referenceNo,
		"client_number", payload.ClientNumber,
		"msisdn", payload.MSISDN,
		"mid", payload.MID,
		"msgid", msgID,
	)

	resp, err := s.unipinClient.VoucherRequest(ctx, unipin.VoucherRequestReq{DenominationCode: denominationCode, Quantity: 1, ReferenceNo: referenceNo})
	if err != nil {
		var techErr *unipin.TechnicalError
		if errors.As(err, &techErr) && errors.Is(techErr.Cause, context.DeadlineExceeded) {
			s.logger.Warn("voucher request timeout, falling back to inquiry",
				"queue", queueName,
				"voucher_code", voucherCode,
				"denomination_code", denominationCode,
				"reference_no", referenceNo,
				"client_number", payload.ClientNumber,
				"msisdn", payload.MSISDN,
				"mid", payload.MID,
				"msgid", msgID,
				"error", err,
			)

			maxAttempts := s.cfg.RetryMaxAttempts
			if maxAttempts <= 0 {
				maxAttempts = 5
			}
			retryWait := s.cfg.RetryWait
			go s.retryVoucherInquiry(ctx, payload, queueName, msgID, referenceNo, maxAttempts, retryWait)
			return
		}

		if resp != nil {
			s.logger.Error("voucher request failed",
				"queue", queueName,
				"voucher_code", voucherCode,
				"denomination_code", denominationCode,
				"reference_no", referenceNo,
				"status", resp.Status,
				"reason", resp.Reason,
				"error", err,
			)
			s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", resp.ReferenceNo, unipin.ResolveStatusCode(resp.Status, resp.Error), unipin.ResolveReason(resp.Reason, resp.Error))
			return
		}

		s.logger.Error("voucher request failed",
			"queue", queueName,
			"voucher_code", voucherCode,
			"denomination_code", denominationCode,
			"reference_no", referenceNo,
			"error", err,
		)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", err.Error())
		return
	}

	s.logger.Info("voucher request response",
		"queue", queueName,
		"voucher_code", voucherCode,
		"reference_no", resp.ReferenceNo,
		"order", resp.Order,
		"total_amount", resp.TotalAmount,
		"status", resp.Status,
		"reason", unipin.ResolveReason(resp.Reason, resp.Error),
		"items_count", len(resp.Items),
	)

	statusToBe := "FAILED"
	if resp.Status == 1 {
		statusToBe = "SUCCESS"
	}
	// Use resp.ReferenceNo as serial_number like existing implementation
	s.forwardCallback(ctx, payload, queueName, msgID, statusToBe, resp.ReferenceNo, unipin.ResolveStatusCode(resp.Status, resp.Error), unipin.ResolveReason(resp.Reason, resp.Error))
}

func (s *ConsumerServiceImpl) retryVoucherInquiry(ctx context.Context, payload *consumePayload, queueName, msgID, referenceNo string, maxAttempts int, retryWait time.Duration) {
	lastReason := ""
	var lastErr error

	serialNumber := referenceNo
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 && retryWait > 0 {
			select {
			case <-ctx.Done():
				s.logger.Info("voucher inquiry retry stopped", "reason", "context canceled", "msgid", msgID, "reference_no", referenceNo)
				return
			case <-time.After(retryWait):
			}
		}

		resp, err := s.unipinClient.VoucherInquiry(ctx, referenceNo)
		if err == nil {
			if resp == nil {
				lastErr = fmt.Errorf("voucher inquiry returned nil response")
				continue
			}

			switch resp.Status {
			case 1:
				// Success: stop retry and publish.
				s.forwardCallback(ctx, payload, queueName, msgID, "SUCCESS", resp.ReferenceNo, unipin.ResolveStatusCode(resp.Status, resp.Error), unipin.ResolveReason(resp.Reason, resp.Error))
				return
			case 0:
				// Failed: stop retry and publish.
				s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", resp.ReferenceNo, unipin.ResolveStatusCode(resp.Status, resp.Error), unipin.ResolveReason(resp.Reason, resp.Error))
				return
			default:
				// Still pending / non-final status.
				lastReason = strings.TrimSpace(unipin.ResolveReason(resp.Reason, resp.Error))
				s.logger.Info("voucher inquiry still pending",
					"attempt", attempt,
					"max_attempts", maxAttempts,
					"queue_name", queueName,
					"msgid", msgID,
					"reference_no", referenceNo,
					"status", resp.Status,
					"reason", resp.Reason,
				)
				continue
			}
		}

		lastErr = err
		if resp == nil {
			s.logger.Warn("voucher inquiry retry failed",
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"queue_name", queueName,
				"msgid", msgID,
				"reference_no", referenceNo,
				"error", err,
			)
			continue
		}

		if r := strings.TrimSpace(resp.ReferenceNo); r != "" {
			serialNumber = r
		}
		lastReason = strings.TrimSpace(unipin.ResolveReason(resp.Reason, resp.Error))

		switch resp.Status {
		case 0:
			s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", serialNumber, unipin.ResolveStatusCode(resp.Status, resp.Error), unipin.ResolveReason(resp.Reason, resp.Error))
			return
		default:
			// Still pending / non-final status.
			s.logger.Info("voucher inquiry still pending",
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"queue_name", queueName,
				"msgid", msgID,
				"reference_no", referenceNo,
				"status", resp.Status,
				"reason", resp.Reason,
				"error", err,
			)
		}
	}

	finalMsg := fmt.Sprintf("voucher inquiry still pending after %d attempts", maxAttempts)
	if lastReason != "" {
		finalMsg = fmt.Sprintf("voucher inquiry still pending after %d attempts: %s", maxAttempts, lastReason)
	} else if lastErr != nil {
		finalMsg = fmt.Sprintf("voucher inquiry still pending after %d attempts: %v", maxAttempts, lastErr)
	}

	// After max retry: publish failed/cancel so it doesn't become a silent transaction.
	s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", serialNumber, "", finalMsg)
}

func (s *ConsumerServiceImpl) processVoucherInquiry(ctx context.Context, payload *consumePayload, queueName, msgID, referenceNo string) {
	inquiryResp, err := s.unipinClient.VoucherInquiry(ctx, referenceNo)
	if err != nil {
		if inquiryResp != nil {
			s.logger.Error("voucher inquiry failed",
				"queue", queueName,
				"reference_no", referenceNo,
				"status", inquiryResp.Status,
				"reason", inquiryResp.Reason,
				"error", err,
			)
			s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", inquiryResp.ReferenceNo, unipin.ResolveStatusCode(inquiryResp.Status, inquiryResp.Error), unipin.ResolveReason(inquiryResp.Reason, inquiryResp.Error))
			return
		}
		s.logger.Error("voucher inquiry failed", "queue", queueName, "reference_no", referenceNo, "error", err)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", referenceNo, "", err.Error())
		return
	}

	s.logger.Info("voucher inquiry response",
		"queue", queueName,
		"reference_no", inquiryResp.ReferenceNo,
		"order", inquiryResp.Order,
		"total_amount", inquiryResp.TotalAmount,
		"status", inquiryResp.Status,
		"reason", unipin.ResolveReason(inquiryResp.Reason, inquiryResp.Error),
		"items_count", len(inquiryResp.Items),
	)

	statusToBe := "FAILED"
	if inquiryResp.Status == 1 {
		statusToBe = "SUCCESS"
	}
	s.forwardCallback(ctx, payload, queueName, msgID, statusToBe, inquiryResp.ReferenceNo, unipin.ResolveStatusCode(inquiryResp.Status, inquiryResp.Error), unipin.ResolveReason(inquiryResp.Reason, inquiryResp.Error))
}

func (s *ConsumerServiceImpl) processGame(ctx context.Context, payload *consumePayload, queueName, msgID string) {
	// Direct topup command format (new): gamecode*denomination_id*fields_json
	// Backward compatible (old): gamecode*denomination_id, fields_json in payload.MSISDN
	command := strings.TrimSpace(payload.Command)
	if command == "" {
		s.logger.Error("game request skipped: command is empty",
			"queue_name", queueName, "client_number", payload.ClientNumber, "msisdn", payload.MSISDN, "mid", payload.MID, "msgid", msgID)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", "command is empty")
		return
	}

	parts := strings.SplitN(command, "*", 3)
	if len(parts) < 2 {
		s.logger.Error("game request skipped: command missing delimiter *",
			"queue_name", queueName, "client_number", payload.ClientNumber, "msisdn", payload.MSISDN, "mid", payload.MID, "msgid", msgID, "command", command)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", "command missing delimiter *")
		return
	}

	gameCode := strings.TrimSpace(parts[0])
	if gameCode == "" {
		s.logger.Error("game request skipped: game_code is empty after parsing command",
			"queue_name", queueName, "client_number", payload.ClientNumber, "msisdn", payload.MSISDN, "mid", payload.MID, "msgid", msgID, "command", command)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", "game_code is empty")
		return
	}

	denominationID := strings.TrimSpace(parts[1])
	if denominationID == "" {
		s.logger.Error("game request skipped: denomination_id is empty after parsing command",
			"queue_name", queueName, "client_number", payload.ClientNumber, "msisdn", payload.MSISDN, "mid", payload.MID, "msgid", msgID, "command", command)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", "denomination_id is empty")
		return
	}

	fieldsJSON := ""
	if len(parts) == 3 {
		candidate := strings.TrimSpace(parts[2])
		// Treat 3rd segment as fields only when it looks like a JSON object.
		// Otherwise, keep backward-compat behavior where extra '*' stays in denomination_id.
		if looksLikeJSONObject(candidate) {
			fieldsJSON = candidate
		} else if candidate != "" {
			denominationID = strings.TrimSpace(parts[1] + "*" + parts[2])
		}
	}
	if fieldsJSON == "" {
		fieldsJSON = strings.TrimSpace(payload.MSISDN)
	}

	if fieldsJSON == "" {
		s.logger.Error("game request skipped: fields is empty",
			"queue_name", queueName, "mid", payload.MID, "msgid", msgID)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", "fields is empty")
		return
	}

	fields, err := parseJSONMap(fieldsJSON)
	if err != nil {
		s.logger.Error("game request skipped: fields is not valid JSON object",
			"queue_name", queueName,
			"fields", fieldsJSON,
			"mid", payload.MID,
			"msgid", msgID,
			"error", err)
		// Fallback: if command had a 3rd segment but it's not JSON, try legacy MSISDN.
		if len(parts) == 3 {
			legacy := strings.TrimSpace(payload.MSISDN)
			if legacy != "" && looksLikeJSONObject(legacy) {
				if f2, err2 := parseJSONMap(legacy); err2 == nil {
					fields = f2
					err = nil
				}
			}
		}
		if err != nil {
			s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", "fields is not valid JSON object: "+err.Error())
			return
		}
	}

	if len(fields) == 0 {
		s.logger.Error("game request skipped: fields JSON has no fields",
			"queue_name", queueName, "mid", payload.MID, "msgid", msgID)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", "fields JSON has no fields")
		return
	}

	s.logger.Info("processing game request",
		"queue_name", queueName,
		"client_number", payload.ClientNumber,
		"mid", payload.MID,
		"msgid", msgID,
		"game_code", gameCode,
		"denomination_id", denominationID,
		"fields_count", len(fields),
	)

	// 1) Validate user (status=1 success, otherwise fail)
	validateResp, err := s.unipinClient.ValidateUser(ctx, unipin.ValidateUserRequest{GameCode: gameCode, Fields: fields})
	if err != nil {
		s.logger.Error("validate user failed",
			"queue_name", queueName, "game_code", gameCode, "msgid", msgID, "error", err)
		if validateResp != nil {
			s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", unipin.ResolveStatusCode(validateResp.Status, validateResp.Error), unipin.ResolveReason(validateResp.Reason, validateResp.Error))
		} else {
			s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", err.Error())
		}
		return
	}

	validationToken := validateResp.ValidationToken
	s.logger.Info("validate user success",
		"game_code", gameCode, "msgid", msgID, "username", validateResp.Username, "status", validateResp.Status)

	// 2) Create order (status=1 success, status=0 fail, otherwise inquiry)
	orderResp, err := s.unipinClient.CreateOrder(ctx, unipin.CreateOrderRequest{
		GameCode:        gameCode,
		ValidationToken: validationToken,
		ReferenceNo:     msgID,
		DenominationID:  denominationID,
	})
	if err != nil {
		var techErr *unipin.TechnicalError
		if errors.As(err, &techErr) && errors.Is(techErr.Cause, context.DeadlineExceeded) {
			s.logger.Warn("create order timeout, falling back to order inquiry",
				"queue_name", queueName, "game_code", gameCode, "msgid", msgID,
				"reference_no", msgID, "error", err)
			s.processOrderInquiry(ctx, payload, queueName, msgID, msgID)
			return
		}

		if orderResp != nil {
			s.logger.Warn("create order returned non-success status",
				"queue_name", queueName,
				"game_code", gameCode,
				"msgid", msgID,
				"reference_no", orderResp.ReferenceNo,
				"status", orderResp.Status,
				"reason", orderResp.Reason,
				"error", err,
			)

			if orderResp.Status == 0 {
				s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", orderResp.ReferenceNo, unipin.ResolveStatusCode(orderResp.Status, orderResp.Error), unipin.ResolveReason(orderResp.Reason, orderResp.Error))
				return
			}

			// Status selain 1/0: wajib order inquiry
			referenceNo := msgID
			if strings.TrimSpace(orderResp.ReferenceNo) != "" {
				referenceNo = strings.TrimSpace(orderResp.ReferenceNo)
			}
			s.processOrderInquiry(ctx, payload, queueName, msgID, referenceNo)
			return
		}

		s.logger.Error("create order failed",
			"queue_name", queueName, "game_code", gameCode, "msgid", msgID, "error", err)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", "", "", err.Error())
		return
	}

	s.logger.Info("create order response",
		"reference_no", orderResp.ReferenceNo,
		"transaction_number", orderResp.TransactionNumber,
		"status", orderResp.Status,
		"reason", orderResp.Reason,
	)

	s.forwardCallback(ctx, payload, queueName, msgID, "SUCCESS", orderResp.ReferenceNo, unipin.ResolveStatusCode(orderResp.Status, orderResp.Error), unipin.ResolveReason(orderResp.Reason, orderResp.Error))
}

func (s *ConsumerServiceImpl) processOrderInquiry(ctx context.Context, payload *consumePayload, queueName, msgID, referenceNo string) {
	inquiryResp, err := s.unipinClient.OrderInquiry(ctx, referenceNo)
	if err != nil {
		if inquiryResp != nil {
			s.logger.Warn("order inquiry returned non-success status",
				"queue_name", queueName,
				"msgid", msgID,
				"reference_no", referenceNo,
				"transaction_number", inquiryResp.TransactionNumber,
				"status", inquiryResp.Status,
				"reason", inquiryResp.Reason,
				"error", err,
			)

			switch inquiryResp.Status {
			case 0:
				s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", inquiryResp.ReferenceNo, unipin.ResolveStatusCode(inquiryResp.Status, inquiryResp.Error), unipin.ResolveReason(inquiryResp.Reason, inquiryResp.Error))
			default:
				// Status selain 1/0: transaksi pending → publish processing + retry async sampai final.
				serialNumber := strings.TrimSpace(inquiryResp.ReferenceNo)
				if serialNumber == "" {
					serialNumber = strings.TrimSpace(referenceNo)
				}

				s.logger.Info("order inquiry pending, scheduling retry",
					"queue_name", queueName,
					"msgid", msgID,
					"reference_no", referenceNo,
					"status", inquiryResp.Status,
					"reason", inquiryResp.Reason,
				)

				// Publish intermediate processing state so downstream can trace the transaction.
				s.forwardCallback(ctx, payload, queueName, msgID, StatusToBeProcess, serialNumber, unipin.ResolveStatusCode(inquiryResp.Status, inquiryResp.Error), unipin.ResolveReason(inquiryResp.Reason, inquiryResp.Error))

				maxAttempts := s.cfg.RetryMaxAttempts
				if maxAttempts <= 0 {
					maxAttempts = 5
				}
				retryWait := s.cfg.RetryWait
				go s.retryOrderInquiry(ctx, payload, queueName, msgID, referenceNo, serialNumber, maxAttempts, retryWait)
			}
			return
		}

		s.logger.Error("order inquiry error",
			"queue_name", queueName, "msgid", msgID, "reference_no", referenceNo, "error", err)
		s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", referenceNo, "", err.Error())
		return
	}

	s.logger.Info("order inquiry response",
		"reference_no", inquiryResp.ReferenceNo,
		"transaction_number", inquiryResp.TransactionNumber,
		"status", inquiryResp.Status,
		"reason", inquiryResp.Reason,
	)
	s.forwardCallback(ctx, payload, queueName, msgID, "SUCCESS", inquiryResp.ReferenceNo, unipin.ResolveStatusCode(inquiryResp.Status, inquiryResp.Error), unipin.ResolveReason(inquiryResp.Reason, inquiryResp.Error))
}

func (s *ConsumerServiceImpl) retryOrderInquiry(ctx context.Context, payload *consumePayload, queueName, msgID, referenceNo, serialNumber string, maxAttempts int, retryWait time.Duration) {
	lastReason := ""
	var lastErr error

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		if attempt > 1 && retryWait > 0 {
			select {
			case <-ctx.Done():
				s.logger.Info("order inquiry retry stopped", "reason", "context canceled", "msgid", msgID, "reference_no", referenceNo)
				return
			case <-time.After(retryWait):
			}
		}

		resp, err := s.unipinClient.OrderInquiry(ctx, referenceNo)
		if err == nil {
			if resp == nil {
				lastErr = fmt.Errorf("order inquiry returned nil response")
				continue
			}
			s.forwardCallback(ctx, payload, queueName, msgID, "SUCCESS", resp.ReferenceNo, unipin.ResolveStatusCode(resp.Status, resp.Error), unipin.ResolveReason(resp.Reason, resp.Error))
			return
		}

		lastErr = err
		if resp == nil {
			s.logger.Warn("order inquiry retry failed",
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"queue_name", queueName,
				"msgid", msgID,
				"reference_no", referenceNo,
				"error", err,
			)
			continue
		}

		if r := strings.TrimSpace(resp.ReferenceNo); r != "" {
			serialNumber = r
		}
		lastReason = strings.TrimSpace(resp.Reason)

		switch resp.Status {
		case 0:
			s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", serialNumber, unipin.ResolveStatusCode(resp.Status, resp.Error), unipin.ResolveReason(resp.Reason, resp.Error))
			return
		default:
			// Still pending.
			s.logger.Info("order inquiry still pending",
				"attempt", attempt,
				"max_attempts", maxAttempts,
				"queue_name", queueName,
				"msgid", msgID,
				"reference_no", referenceNo,
				"status", resp.Status,
				"reason", resp.Reason,
				"error", err,
			)
		}
	}

	finalMsg := fmt.Sprintf("order inquiry still pending after %d attempts", maxAttempts)
	if lastReason != "" {
		finalMsg = fmt.Sprintf("order inquiry still pending after %d attempts: %s", maxAttempts, lastReason)
	} else if lastErr != nil {
		finalMsg = fmt.Sprintf("order inquiry still pending after %d attempts: %v", maxAttempts, lastErr)
	}

	// After max retry: publish failed/cancel so it doesn't become a silent transaction.
	s.forwardCallback(ctx, payload, queueName, msgID, "FAILED", serialNumber, "", finalMsg)
}

// forwardCallback mempublikasikan status transaksi ke RabbitMQ tujuan.
// Non-blocking: error di-log saja, tidak menghentikan alur utama.
func (s *ConsumerServiceImpl) forwardCallback(ctx context.Context, payload *consumePayload, queueName, msgID, statusToBe, serialNumber, statusCode, additionalMessage string) {
	if s.mqPublisher == nil {
		s.logger.Warn("mq publisher not initialized, skipping publish",
			"msgid", msgID, "queue_name", queueName)
		return
	}

	statusToBe = normalizeStatusToBe(statusToBe)

	msgIDInt := 0
	if n, err := strconv.Atoi(msgID); err == nil {
		msgIDInt = n
	}

	correlationID := strings.TrimSpace(msgID)
	if correlationID == "" {
		correlationID = strings.TrimSpace(serialNumber)
	}

	clientNumber := strings.TrimSpace(payload.ClientNumber)
	if clientNumber == "" {
		msisdn := strings.TrimSpace(payload.MSISDN)
		if msisdn != "" && !looksLikeJSONValue(msisdn) {
			clientNumber = msisdn
		}
	}

	messageToCustomer := util.GenerateMessage(payload.Amount, clientNumber, time.Now(), statusToBe, serialNumber, statusCode)

	msg := mqpublisher.NewProviderPublishMessage(mqpublisher.ProviderPublishData{
		MsgID:                  msgIDInt,
		StatusToBe:             statusToBe,
		SerialNumber:           serialNumber,
		ClientNumber:           clientNumber,
		Nominal:                fmt.Sprintf("%d", payload.Amount),
		OriginalConversationID: correlationID,
		ConversationID:         correlationID,
		MessageToCustomer:      messageToCustomer,
		AdditionalMessage:      additionalMessage,
		QueueName:              queueName,
	})

	body, err := jsonMarshal(msg)
	if err != nil {
		s.logger.Error("failed to marshal publish message",
			"msgid", msgID, "error", err)
		return
	}

	if err := s.mqPublisher.Publish(ctx, payload.MQTransaction, queueName, body); err != nil {
		s.logger.Error("failed to publish to downstream rabbitmq",
			"msgid", msgID, "queue_name", queueName,
			"mq_transaction", payload.MQTransaction, "error", err)
		return
	}

	s.logger.Info("published to downstream rabbitmq",
		"msgid", msgID, "queue_name", queueName,
		"mq_transaction", payload.MQTransaction)
}

func awaitDrain(wg *sync.WaitGroup, timeout time.Duration, logger contractsvc.Logger) {
	done := make(chan struct{})
	go func() {
		defer close(done)
		wg.Wait()
	}()
	select {
	case <-done:
	case <-time.After(timeout):
		logger.Warn("consumer shutdown timeout reached", "timeout", timeout.String())
	}
}
