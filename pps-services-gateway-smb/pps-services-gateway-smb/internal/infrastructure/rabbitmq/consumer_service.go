package rabbitmq

import (
	"bytes"
	"context"
	"encoding/json"
	"time"

	"pps-services-gateway-smb/internal/config"
	contractsvc "pps-services-gateway-smb/internal/domain/contract/service"
)

type consumePayload struct {
	Amount        int
	ProductCode   string
	ProductType   string
	MID           string
	QueueName     string
	ClientNumber  string
	MsgID         string
	Command       string
	MQTransaction string
}

func (p *consumePayload) UnmarshalJSON(data []byte) error {
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()

	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		return err
	}

	p.Amount = parseInt(getAny(raw, "amount"))
	p.ProductCode = parseString(getAny(raw, "product_code", "productCode"))
	p.ProductType = parseString(getAny(raw, "product_type", "productType"))
	p.MID = parseString(getAny(raw, "mid"))
	p.QueueName = parseString(getAny(raw, "queue_name", "queueName"))
	p.ClientNumber = parseString(getAny(raw, "client_number", "clientNumber", "msisdn"))
	p.MsgID = parseString(getAny(raw, "msgid", "msgID", "msg_id"))
	p.Command = parseString(getAny(raw, "command"))
	p.MQTransaction = parseString(getAny(raw, "MQTransaction", "mqTransaction", "mq_transaction"))

	return nil
}

type ConsumerServiceImpl struct {
	cfg               *config.Config
	logger            contractsvc.Logger
	mqPublisher       contractsvc.MQPublisher
	smbClient         contractsvc.SMBClient
	retryConfig       *config.RetryConfig
	transactionLogger contractsvc.TransactionLogger
}

func NewConsumerServiceImpl(cfg *config.Config, logger contractsvc.Logger) *ConsumerServiceImpl {
	return &ConsumerServiceImpl{cfg: cfg, logger: logger}
}

func (s *ConsumerServiceImpl) SetMQPublisher(pub contractsvc.MQPublisher) { s.mqPublisher = pub }

func (s *ConsumerServiceImpl) SetSMBClient(client contractsvc.SMBClient) { s.smbClient = client }

func (s *ConsumerServiceImpl) SetRetryConfig(cfg *config.RetryConfig) { s.retryConfig = cfg }

func (s *ConsumerServiceImpl) SetTransactionLogger(tl contractsvc.TransactionLogger) {
	s.transactionLogger = tl
}

func (s *ConsumerServiceImpl) Start(ctx context.Context) error {
	backoff := time.Second
	maxBackoff := 30 * time.Second

	for {
		select {
		case <-ctx.Done():
			s.logger.Info("consumer stopped by context cancellation")
			return ctx.Err()
		default:
		}

		err := s.consumeSession(ctx)
		if ctx.Err() != nil {
			return ctx.Err()
		}

		s.logger.Error("consumer session ended, reconnecting", "error", err, "backoff", backoff.String())
		time.Sleep(backoff)
		backoff = min(backoff*2, maxBackoff)
	}
}
