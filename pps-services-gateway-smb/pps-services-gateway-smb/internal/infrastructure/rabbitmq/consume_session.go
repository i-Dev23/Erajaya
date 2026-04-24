package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
	"pps-services-gateway-smb/internal/infrastructure/mqpublisher"
	"pps-services-gateway-smb/internal/usecase/plntoken"
	"pps-services-gateway-smb/internal/util"

	amqp "github.com/rabbitmq/amqp091-go"
)

func (s *ConsumerServiceImpl) consumeSession(ctx context.Context) error {
	conn, err := amqp.Dial(s.cfg.RabbitMQURL)
	if err != nil {
		return fmt.Errorf("dial rabbitmq: %w", err)
	}
	defer conn.Close()

	ch, err := conn.Channel()
	if err != nil {
		return fmt.Errorf("open channel: %w", err)
	}
	defer ch.Close()

	q, err := ch.QueueDeclare(s.cfg.QueueName, true, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("declare queue %s: %w", s.cfg.QueueName, err)
	}

	if err := ch.Qos(1, 0, false); err != nil {
		return fmt.Errorf("set qos: %w", err)
	}

	deliveries, err := ch.Consume(q.Name, s.cfg.ConsumerTag, false, false, false, false, nil)
	if err != nil {
		return fmt.Errorf("start consuming: %w", err)
	}

	s.logger.Info("consumer started", "queue", s.cfg.QueueName, "consumer_tag", s.cfg.ConsumerTag)

	var wg sync.WaitGroup
	for {
		select {
		case <-ctx.Done():
			wg.Wait()
			return ctx.Err()
		case d, ok := <-deliveries:
			if !ok {
				wg.Wait()
				return fmt.Errorf("delivery channel closed")
			}
			s.processDelivery(ctx, d, &wg)
		}
	}
}

func (s *ConsumerServiceImpl) processDelivery(ctx context.Context, d amqp.Delivery, wg *sync.WaitGroup) {
	defer func() { _ = d.Ack(false) }()

	var payload consumePayload
	if err := json.Unmarshal(d.Body, &payload); err != nil {
		s.logger.Error("failed to parse payload", "error", err, "body", string(d.Body))
		return
	}

	msgID := payload.MsgID
	if msgID == "" {
		msgID = d.MessageId
	}
	if msgID == "" {
		msgID = d.CorrelationId
	}

	queueName := payload.QueueName
	if queueName == "" {
		queueName = s.cfg.QueueName
	}

	s.logger.Info("processing message",
		"msg_id", msgID,
		"client_number", payload.ClientNumber,
		"product_code", payload.ProductCode,
		"product_type", payload.ProductType,
		"amount", payload.Amount,
		"mq_transaction", payload.MQTransaction)

	productType := strings.ToLower(strings.TrimSpace(payload.ProductType))
	switch productType {
	case "pln_token", "pln token", "pln-token", "plntoken":
		s.handlePLNToken(ctx, payload, msgID, queueName, wg)
	default:
		s.logger.Warn("unsupported product type, skipping",
			"product_type", payload.ProductType,
			"msg_id", msgID)
	}
}

func (s *ConsumerServiceImpl) handlePLNToken(ctx context.Context, payload consumePayload, msgID string, queueName string, wg *sync.WaitGroup) {
	ourTrxID := util.GenerateTransactionID(payload.MID, msgID, timeNow())

	s.logInsertTransaction(ctx, contractsvc.TransactionRecord{
		MsgID:         msgID,
		OurTrxID:      ourTrxID,
		ClientNumber:  payload.ClientNumber,
		MID:           payload.MID,
		ProductType:   "pln_token",
		ProductCode:   payload.ProductCode,
		Amount:        payload.Amount,
		QueueName:     queueName,
		MQTransaction: payload.MQTransaction,
	})

	uc := plntoken.NewUsecase(s.smbClient, s.retryConfig, s.logger)
	result, _ := uc.ProcessTransaction(ctx, payload.ClientNumber, payload.ProductCode, msgID, payload.Amount)

	switch result.Status {
	case "SUCCESS":
		s.logUpdateStatus(ctx, msgID, "SUCCESS")
		s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
			MsgID:             parseMsgIDToInt(msgID),
			StatusToBe:        result.StatusToBe,
			SerialNumber:      result.Token,
			ClientNumber:      payload.ClientNumber,
			Nominal:           result.Nominal,
			ConversationID:    result.ConversationID,
			MessageToCustomer: result.Message,
			QueueName:         queueName,
		})

	case "FAILED":
		s.logUpdateStatus(ctx, msgID, "FAILED")
		s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
			MsgID:             parseMsgIDToInt(msgID),
			StatusToBe:        result.StatusToBe,
			ClientNumber:      payload.ClientNumber,
			Nominal:           result.Nominal,
			ConversationID:    result.ConversationID,
			MessageToCustomer: result.Message,
			QueueName:         queueName,
		})

	case "PENDING":
		wg.Add(1)
		go func() {
			defer wg.Done()
			adviceResult := uc.RetryAdvice(ctx, payload.ClientNumber, result.RefID, msgID, payload.Amount)

			if adviceResult.Status == "SUCCESS" {
				s.logUpdateStatus(ctx, msgID, "SUCCESS")
			} else {
				s.logUpdateStatus(ctx, msgID, "FAILED")
			}

			s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
				MsgID:             parseMsgIDToInt(msgID),
				StatusToBe:        adviceResult.StatusToBe,
				SerialNumber:      adviceResult.Token,
				ClientNumber:      payload.ClientNumber,
				Nominal:           adviceResult.Nominal,
				ConversationID:    adviceResult.ConversationID,
				MessageToCustomer: adviceResult.Message,
				QueueName:         queueName,
			})
		}()
	}
}

func (s *ConsumerServiceImpl) publishToDownstream(ctx context.Context, mqTransactionURL, queueName string, data mqpublisher.ProviderPublishData) {
	if s.mqPublisher == nil {
		s.logger.Warn("mqPublisher is nil, skipping downstream publish", "msg_id", data.MsgID)
		return
	}
	if strings.TrimSpace(mqTransactionURL) == "" {
		s.logger.Warn("mq_transaction URL is empty, skipping downstream publish", "msg_id", data.MsgID)
		return
	}

	msg := mqpublisher.NewProviderPublishMessage(data)
	body, err := json.Marshal(msg)
	if err != nil {
		s.logger.Error("failed to marshal downstream message", "error", err, "msg_id", data.MsgID)
		return
	}

	if err := s.mqPublisher.Publish(ctx, mqTransactionURL, queueName, body); err != nil {
		s.logger.Error("failed to publish to downstream", "error", err, "msg_id", data.MsgID, "queue", queueName)
		return
	}

	s.logger.Info("published to downstream",
		"msg_id", data.MsgID, "status_to_be", data.StatusToBe, "queue", queueName)
}

func (s *ConsumerServiceImpl) logInsertTransaction(ctx context.Context, rec contractsvc.TransactionRecord) {
	if s.transactionLogger == nil {
		return
	}
	if err := s.transactionLogger.InsertTransaction(ctx, rec); err != nil {
		s.logger.Error("failed to insert transaction log", "error", err, "msg_id", rec.MsgID)
	}
}

func (s *ConsumerServiceImpl) logUpdateStatus(ctx context.Context, msgID string, status string) {
	if s.transactionLogger == nil {
		return
	}
	if err := s.transactionLogger.UpdateTransactionStatus(ctx, msgID, status); err != nil {
		s.logger.Error("failed to update transaction status", "error", err, "msg_id", msgID, "status", status)
	}
}

func (s *ConsumerServiceImpl) logInsertSyncResponse(ctx context.Context, rec contractsvc.ResponseRecord) {
	if s.transactionLogger == nil {
		return
	}
	if err := s.transactionLogger.InsertSyncResponse(ctx, rec); err != nil {
		s.logger.Error("failed to insert sync response", "error", err, "msg_id", rec.MsgID)
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
	if v == nil {
		return ""
	}
	switch val := v.(type) {
	case string:
		return val
	case json.Number:
		return val.String()
	case float64:
		return strconv.FormatFloat(val, 'f', -1, 64)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func parseInt(v any) int {
	if v == nil {
		return 0
	}
	switch val := v.(type) {
	case json.Number:
		n, _ := val.Int64()
		return int(n)
	case float64:
		return int(val)
	case string:
		n, _ := strconv.Atoi(val)
		return n
	default:
		return 0
	}
}

func parseMsgIDToInt(msgID string) int {
	n, _ := strconv.Atoi(msgID)
	return n
}

var timeNow = func() time.Time { return time.Now() }
