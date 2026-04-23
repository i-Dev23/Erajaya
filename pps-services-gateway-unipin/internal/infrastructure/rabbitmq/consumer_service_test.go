package rabbitmq

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"pps-services-gateway-unipin/internal/config"
	"pps-services-gateway-unipin/pkg/unipin"

	amqp "github.com/rabbitmq/amqp091-go"
)

// ─── Mocks ───

type mockLogger struct {
	infos  []string
	warns  []string
	errors []string
}

func (m *mockLogger) Info(msg string, _ ...any)  { m.infos = append(m.infos, msg) }
func (m *mockLogger) Warn(msg string, _ ...any)  { m.warns = append(m.warns, msg) }
func (m *mockLogger) Error(msg string, _ ...any) { m.errors = append(m.errors, msg) }

type mockMQPublisher struct {
	calls   []publishCall
	callsCh chan publishCall
	err     error
}

type publishCall struct {
	mqTransactionURL string
	queueName        string
	body             []byte
}

func (m *mockMQPublisher) Publish(_ context.Context, mqTransactionURL, queueName string, body []byte) error {
	call := publishCall{mqTransactionURL, queueName, body}
	m.calls = append(m.calls, call)
	if m.callsCh != nil {
		select {
		case m.callsCh <- call:
		default:
		}
	}
	return m.err
}

// ─── Helpers ───

func newTestClient(t *testing.T, serverURL string) *unipin.Client {
	t.Helper()
	c, err := unipin.NewClient(serverURL, "partner", "secret", 5*time.Second, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	return c
}

func newService(t *testing.T, serverURL string) (*ConsumerServiceImpl, *mockLogger, *mockMQPublisher) {
	t.Helper()
	logger := &mockLogger{}
	pub := &mockMQPublisher{}
	pub.callsCh = make(chan publishCall, 32)
	cfg := &config.Config{QueueName: "test-queue", ConsumerTag: "test-tag", ReadTimeout: 5 * time.Second}
	svc := NewConsumerServiceImpl(cfg, newTestClient(t, serverURL), logger)
	svc.SetMQPublisher(pub)
	return svc, logger, pub
}

func waitForPublishCalls(t *testing.T, pub *mockMQPublisher, want int, timeout time.Duration) {
	t.Helper()
	if pub.callsCh == nil {
		t.Fatalf("callsCh is nil; mockMQPublisher must be initialized with a channel")
	}
	deadline := time.NewTimer(timeout)
	defer deadline.Stop()
	for i := 0; i < want; i++ {
		select {
		case <-pub.callsCh:
		case <-deadline.C:
			t.Fatalf("timeout waiting for %d publish calls (got %d)", want, i)
		}
	}
}

func publishedDataFromCall(t *testing.T, call publishCall) map[string]any {
	t.Helper()
	var msg map[string]any
	if err := json.Unmarshal(call.body, &msg); err != nil {
		t.Fatalf("unmarshal published body: %v", err)
	}
	data, ok := msg["data"].(map[string]any)
	if !ok {
		t.Fatal("published message missing 'data' field")
	}
	return data
}

func lastPublished(t *testing.T, pub *mockMQPublisher) map[string]any {
	t.Helper()
	if len(pub.calls) == 0 {
		t.Fatal("expected at least one publish call")
	}
	var msg map[string]any
	if err := json.Unmarshal(pub.calls[len(pub.calls)-1].body, &msg); err != nil {
		t.Fatalf("unmarshal published body: %v", err)
	}
	return msg
}

func publishedData(t *testing.T, pub *mockMQPublisher) map[string]any {
	t.Helper()
	msg := lastPublished(t, pub)
	data, ok := msg["data"].(map[string]any)
	if !ok {
		t.Fatal("published message missing 'data' field")
	}
	return data
}

func makeDelivery(body []byte) *amqp.Delivery {
	return &amqp.Delivery{Body: body, MessageId: "delivery-msg-1"}
}

// ─── consumePayload UnmarshalJSON Tests ───

func TestConsumePayload_ParsesAllFields(t *testing.T) {
	raw := `{
		"amount": 50000,
		"stock_type": "DIGITAL",
		"product_code": "PC001",
		"product_id": "PID001",
		"product_type": "unipin-game",
		"mid": "MID001",
		"store_id": "STORE001",
		"queue_name": "test.queue",
		"msisdn": "08123456789",
		"msgid": "MSG001",
		"callback_url": "http://callback",
		"mq_transaction": "amqp://localhost",
		"command": "MLBB*123"
	}`
	var p consumePayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Amount != 50000 {
		t.Errorf("Amount: got %d, want 50000", p.Amount)
	}
	if p.ProductType != "unipin-game" {
		t.Errorf("ProductType: got %q, want %q", p.ProductType, "unipin-game")
	}
	if p.MQTransaction != "amqp://localhost" {
		t.Errorf("MQTransaction: got %q, want %q", p.MQTransaction, "amqp://localhost")
	}
	if p.Command != "MLBB*123" {
		t.Errorf("Command: got %q, want %q", p.Command, "MLBB*123")
	}
}

func TestConsumePayload_CommandCaseInsensitive(t *testing.T) {
	raw := `{"Command": " GAME*100 "}`
	var p consumePayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Command != "GAME*100" {
		t.Fatalf("Command: got %q, want %q", p.Command, "GAME*100")
	}
}

func TestConsumePayload_TrimsWhitespace(t *testing.T) {
	raw := `{"command": "  MLBB*123  ", "msisdn": "  08123  "}`
	var p consumePayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Command != "MLBB*123" {
		t.Errorf("Command not trimmed: got %q", p.Command)
	}
	if p.MSISDN != "08123" {
		t.Errorf("MSISDN not trimmed: got %q", p.MSISDN)
	}
}

func TestConsumePayload_EmptyJSON(t *testing.T) {
	var p consumePayload
	if err := json.Unmarshal([]byte(`{}`), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Command != "" {
		t.Errorf("Command should be empty, got %q", p.Command)
	}
	if p.Amount != 0 {
		t.Errorf("Amount should be 0, got %d", p.Amount)
	}
}

func TestConsumePayload_InvalidJSON(t *testing.T) {
	var p consumePayload
	err := json.Unmarshal([]byte(`not json`), &p)
	if err == nil {
		t.Fatal("expected error for invalid JSON")
	}
}

func TestConsumePayload_NumericAmount(t *testing.T) {
	raw := `{"amount": "12345"}`
	var p consumePayload
	if err := json.Unmarshal([]byte(raw), &p); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if p.Amount != 12345 {
		t.Errorf("Amount: got %d, want 12345", p.Amount)
	}
}

// ─── getAny / parseString / parseInt Tests ───

func TestGetAny_NilMap(t *testing.T) {
	if v := getAny(nil, "key"); v != nil {
		t.Errorf("expected nil, got %v", v)
	}
}

func TestGetAny_FirstKeyMatch(t *testing.T) {
	m := map[string]any{"a": 1, "b": 2}
	if v := getAny(m, "a", "b"); v != 1 {
		t.Errorf("expected 1, got %v", v)
	}
}

func TestGetAny_FallbackKey(t *testing.T) {
	m := map[string]any{"b": 2}
	if v := getAny(m, "a", "b"); v != 2 {
		t.Errorf("expected 2, got %v", v)
	}
}

func TestGetAny_NoMatch(t *testing.T) {
	m := map[string]any{"c": 3}
	if v := getAny(m, "a", "b"); v != nil {
		t.Errorf("expected nil, got %v", v)
	}
}

func TestParseString_Nil(t *testing.T) {
	if s := parseString(nil); s != "" {
		t.Errorf("expected empty, got %q", s)
	}
}

func TestParseString_String(t *testing.T) {
	if s := parseString("  hello  "); s != "hello" {
		t.Errorf("expected %q, got %q", "hello", s)
	}
}

func TestParseString_Float64Integer(t *testing.T) {
	if s := parseString(float64(42)); s != "42" {
		t.Errorf("expected %q, got %q", "42", s)
	}
}

func TestParseString_Float64Decimal(t *testing.T) {
	s := parseString(float64(3.14))
	if s != "3.14" {
		t.Errorf("expected %q, got %q", "3.14", s)
	}
}

func TestParseString_JSONNumber(t *testing.T) {
	if s := parseString(json.Number("999")); s != "999" {
		t.Errorf("expected %q, got %q", "999", s)
	}
}

func TestParseString_OtherType(t *testing.T) {
	if s := parseString(true); s != "true" {
		t.Errorf("expected %q, got %q", "true", s)
	}
}

func TestParseInt_Nil(t *testing.T) {
	if n := parseInt(nil); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestParseInt_JSONNumber(t *testing.T) {
	if n := parseInt(json.Number("42")); n != 42 {
		t.Errorf("expected 42, got %d", n)
	}
}

func TestParseInt_JSONNumberFloat(t *testing.T) {
	if n := parseInt(json.Number("3.7")); n != 3 {
		t.Errorf("expected 3, got %d", n)
	}
}

func TestParseInt_JSONNumberInvalid(t *testing.T) {
	if n := parseInt(json.Number("abc")); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestParseInt_Float64(t *testing.T) {
	if n := parseInt(float64(99)); n != 99 {
		t.Errorf("expected 99, got %d", n)
	}
}

func TestParseInt_String(t *testing.T) {
	if n := parseInt("  77  "); n != 77 {
		t.Errorf("expected 77, got %d", n)
	}
}

func TestParseInt_InvalidString(t *testing.T) {
	if n := parseInt("abc"); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

func TestParseInt_UnsupportedType(t *testing.T) {
	if n := parseInt(true); n != 0 {
		t.Errorf("expected 0, got %d", n)
	}
}

// ─── processMessage Routing Tests ───

func TestProcessMessage_InvalidJSON(t *testing.T) {
	svc, logger, _ := newService(t, "http://unused")
	svc.processMessage(context.Background(), makeDelivery([]byte(`not json`)))
	if len(logger.errors) == 0 {
		t.Error("expected error log for invalid JSON")
	}
}

func TestProcessMessage_UnipinVoucher_RoutesToProcessVoucher(t *testing.T) {
	// Voucher with empty command → should hit processVoucher and fail on empty command
	body := `{"product_type":"unipin-voucher","command":"","msgid":"MSG1","mq_transaction":"amqp://localhost"}`
	svc, logger, pub := newService(t, "http://unused")
	svc.processMessage(context.Background(), makeDelivery([]byte(body)))
	// processVoucher should log error about empty command
	found := false
	for _, e := range logger.errors {
		if e == "voucher request skipped: command is empty" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected processVoucher to be called (empty command error)")
	}
	if len(pub.calls) == 0 {
		t.Error("expected forwardCallback FAILED to be published")
	}
}

func TestProcessMessage_UnipinGame_RoutesToProcessGame(t *testing.T) {
	body := `{"product_type":"unipin-game","command":"","msgid":"MSG1","mq_transaction":"amqp://localhost"}`
	svc, logger, pub := newService(t, "http://unused")
	svc.processMessage(context.Background(), makeDelivery([]byte(body)))
	found := false
	for _, e := range logger.errors {
		if e == "game request skipped: command is empty" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected processGame to be called (empty command error)")
	}
	if len(pub.calls) == 0 {
		t.Error("expected forwardCallback FAILED to be published")
	}
}

func TestProcessMessage_UnipinDirectTopup_UnsupportedType(t *testing.T) {
	body := `{"product_type":"unipin-direct-topup","msgid":"MSG1"}`
	svc, logger, _ := newService(t, "http://unused")
	svc.processMessage(context.Background(), makeDelivery([]byte(body)))
	found := false
	for _, w := range logger.warns {
		if w == "unsupported tx type" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning for unsupported tx type")
	}
}

func TestProcessMessage_UnsupportedType_LogsWarning(t *testing.T) {
	body := `{"product_type":"unknown-type","msgid":"MSG1"}`
	svc, logger, _ := newService(t, "http://unused")
	svc.processMessage(context.Background(), makeDelivery([]byte(body)))
	found := false
	for _, w := range logger.warns {
		if w == "unsupported tx type" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning for unsupported tx type")
	}
}

func TestProcessMessage_UsesPayloadQueueName(t *testing.T) {
	body := `{"product_type":"unipin-game","command":"","msgid":"MSG1","queue_name":"custom.queue","mq_transaction":"amqp://localhost"}`
	svc, _, pub := newService(t, "http://unused")
	svc.processMessage(context.Background(), makeDelivery([]byte(body)))
	if len(pub.calls) > 0 {
		data := publishedData(t, pub)
		if qn, ok := data["queue_name"].(string); ok && qn != "custom.queue" {
			t.Errorf("expected queue_name %q, got %q", "custom.queue", qn)
		}
	}
}

func TestProcessMessage_FallbackMsgID(t *testing.T) {
	// msgid empty → falls back to delivery.MessageId
	body := `{"product_type":"unipin-game","command":"","mq_transaction":"amqp://localhost"}`
	svc, _, pub := newService(t, "http://unused")
	d := &amqp.Delivery{Body: []byte(body), MessageId: "fallback-id"}
	svc.processMessage(context.Background(), d)
	if len(pub.calls) > 0 {
		data := publishedData(t, pub)
		// The msgid should be "fallback-id" from delivery
		if mid, ok := data["msg_id"]; ok {
			// msg_id is int in the published message, fallback-id is non-numeric → 0
			if mid != float64(0) {
				t.Logf("msg_id: %v (type %T)", mid, mid)
			}
		}
		if data["conversation_id"] != "fallback-id" {
			t.Errorf("conversation_id: got %v, want fallback-id", data["conversation_id"])
		}
		if data["original_conversation_id"] != "fallback-id" {
			t.Errorf("original_conversation_id: got %v, want fallback-id", data["original_conversation_id"])
		}
	}
}

// ─── forwardCallback Tests ───

func TestForwardCallback_NilPublisher_LogsWarning(t *testing.T) {
	logger := &mockLogger{}
	cfg := &config.Config{QueueName: "q", ReadTimeout: 5 * time.Second}
	svc := NewConsumerServiceImpl(cfg, nil, logger)
	// mqPublisher is nil by default
	payload := &consumePayload{MSISDN: "08123", Amount: 1000, MQTransaction: "amqp://localhost"}
	svc.forwardCallback(context.Background(), payload, "q", "1", "SUCCESS", "SN1", "", "ok")
	found := false
	for _, w := range logger.warns {
		if w == "mq publisher not initialized, skipping publish" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected warning about nil publisher")
	}
}

func TestForwardCallback_PublishesCorrectData(t *testing.T) {
	logger := &mockLogger{}
	pub := &mockMQPublisher{}
	cfg := &config.Config{QueueName: "q", ReadTimeout: 5 * time.Second}
	svc := NewConsumerServiceImpl(cfg, nil, logger)
	svc.SetMQPublisher(pub)

	payload := &consumePayload{MSISDN: "08123", Amount: 50000, MQTransaction: "amqp://host"}
	svc.forwardCallback(context.Background(), payload, "target-queue", "123", "SUCCESS", "SN-001", "1", "completed")

	if len(pub.calls) != 1 {
		t.Fatalf("expected 1 publish call, got %d", len(pub.calls))
	}
	if pub.calls[0].mqTransactionURL != "amqp://host" {
		t.Errorf("mqTransactionURL: got %q", pub.calls[0].mqTransactionURL)
	}
	if pub.calls[0].queueName != "target-queue" {
		t.Errorf("queueName: got %q", pub.calls[0].queueName)
	}

	data := publishedData(t, pub)
	if data["status_to_be"] != "F" {
		t.Errorf("status_to_be: got %v, want F", data["status_to_be"])
	}
	if data["serial_number"] != "SN-001" {
		t.Errorf("serial_number: got %v", data["serial_number"])
	}
	if data["client_number"] != "08123" {
		t.Errorf("client_number: got %v", data["client_number"])
	}
	if data["nominal"] != "50000" {
		t.Errorf("nominal: got %v", data["nominal"])
	}
	msg, _ := data["message_to_customer"].(string)
	if !strings.Contains(msg, "Pengisian Voucher sebesar 50000") {
		t.Errorf("message_to_customer missing amount prefix, got %q", msg)
	}
	if !strings.Contains(msg, "telah berhasil dengan no ref <SN-001>") {
		t.Errorf("message_to_customer missing ref, got %q", msg)
	}
	if data["additional_message"] != "completed" {
		t.Errorf("additional_message: got %v, want completed", data["additional_message"])
	}
	// msg_id should be 123 (parsed from string)
	if data["msg_id"] != float64(123) {
		t.Errorf("msg_id: got %v", data["msg_id"])
	}
	if data["conversation_id"] != "123" {
		t.Errorf("conversation_id: got %v, want 123", data["conversation_id"])
	}
	if data["original_conversation_id"] != "123" {
		t.Errorf("original_conversation_id: got %v, want 123", data["original_conversation_id"])
	}
}

func TestForwardCallback_NonNumericMsgID(t *testing.T) {
	logger := &mockLogger{}
	pub := &mockMQPublisher{}
	cfg := &config.Config{QueueName: "q", ReadTimeout: 5 * time.Second}
	svc := NewConsumerServiceImpl(cfg, nil, logger)
	svc.SetMQPublisher(pub)

	payload := &consumePayload{MQTransaction: "amqp://host"}
	svc.forwardCallback(context.Background(), payload, "q", "non-numeric", "FAILED", "", "", "err")

	data := publishedData(t, pub)
	if data["msg_id"] != float64(0) {
		t.Errorf("msg_id should be 0 for non-numeric, got %v", data["msg_id"])
	}
	if data["conversation_id"] != "non-numeric" {
		t.Errorf("conversation_id: got %v, want non-numeric", data["conversation_id"])
	}
	if data["original_conversation_id"] != "non-numeric" {
		t.Errorf("original_conversation_id: got %v, want non-numeric", data["original_conversation_id"])
	}
}

func TestForwardCallback_PublishError_LogsError(t *testing.T) {
	logger := &mockLogger{}
	pub := &mockMQPublisher{err: fmt.Errorf("connection refused")}
	cfg := &config.Config{QueueName: "q", ReadTimeout: 5 * time.Second}
	svc := NewConsumerServiceImpl(cfg, nil, logger)
	svc.SetMQPublisher(pub)

	payload := &consumePayload{MQTransaction: "amqp://host"}
	svc.forwardCallback(context.Background(), payload, "q", "1", "FAILED", "", "", "err")

	found := false
	for _, e := range logger.errors {
		if e == "failed to publish to downstream rabbitmq" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error log for publish failure")
	}
}

func TestForwardCallback_SourceIsProvider(t *testing.T) {
	logger := &mockLogger{}
	pub := &mockMQPublisher{}
	cfg := &config.Config{QueueName: "q", ReadTimeout: 5 * time.Second}
	svc := NewConsumerServiceImpl(cfg, nil, logger)
	svc.SetMQPublisher(pub)

	payload := &consumePayload{MQTransaction: "amqp://host"}
	svc.forwardCallback(context.Background(), payload, "q", "1", "SUCCESS", "", "", "ok")

	msg := lastPublished(t, pub)
	if msg["source"] != "PROVIDER" {
		t.Errorf("source: got %v, want PROVIDER", msg["source"])
	}
}

// ─── processGame Tests ───

// gameHandlers holds overridable HTTP handlers for the mock UniPin server.
type gameHandlers struct {
	validateUser http.HandlerFunc
	createOrder  http.HandlerFunc
	orderInquiry http.HandlerFunc
}

func defaultGameHandlers() gameHandlers {
	return gameHandlers{
		validateUser: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status": 1, "reason": "success",
				"username": "TestUser", "validation_token": "tok-123",
			})
		},
		createOrder: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status": 1, "reason": "order created",
				"reference_no": "REF-001", "transaction_number": "TXN-001",
				"amount": 50000, "currency": "IDR", "item_name": "100 Diamonds",
			})
		},
		orderInquiry: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status": 1, "reason": "order found",
				"reference_no": "REF-001", "transaction_number": "TXN-001",
			})
		},
	}
}

func newGameMockServer(t *testing.T, opts ...func(h *gameHandlers)) *httptest.Server {
	t.Helper()
	h := defaultGameHandlers()
	for _, opt := range opts {
		opt(&h)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/in-game-topup/user/validate", h.validateUser)
	mux.HandleFunc("/in-game-topup/order/create", h.createOrder)
	mux.HandleFunc("/in-game-topup/order/inquiry", h.orderInquiry)
	return httptest.NewServer(mux)
}

func gamePayload(command, msisdn string) *consumePayload {
	return &consumePayload{
		Command:       command,
		MSISDN:        msisdn,
		MID:           "MID1",
		MsgID:         "MSG1",
		Amount:        50000,
		MQTransaction: "amqp://localhost",
		ProductType:   "unipin-game",
	}
}

func TestProcessGame_HappyPath_Success(t *testing.T) {
	server := newGameMockServer(t)
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processGame(context.Background(), gamePayload("MLBB*123", `{"userid":"789","zone":"ID"}`), "q", "MSG1")

	if len(pub.calls) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(pub.calls))
	}
	data := publishedData(t, pub)
	if data["status_to_be"] != "F" {
		t.Errorf("status_to_be: got %v, want F", data["status_to_be"])
	}
	if data["serial_number"] != "REF-001" {
		t.Errorf("serial_number: got %v, want REF-001", data["serial_number"])
	}
}

func TestProcessGame_EmptyCommand_ForwardsFailed(t *testing.T) {
	svc, logger, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("", `{"userid":"1"}`), "q", "MSG1")

	if len(pub.calls) != 1 {
		t.Fatalf("expected 1 publish, got %d", len(pub.calls))
	}
	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
	if len(logger.errors) == 0 || logger.errors[0] != "game request skipped: command is empty" {
		t.Errorf("expected error log about empty command, got %v", logger.errors)
	}
}

func TestProcessGame_NoDelimiter_ForwardsFailed(t *testing.T) {
	svc, logger, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("MLBB123", `{"userid":"1"}`), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
	if len(logger.errors) == 0 || logger.errors[0] != "game request skipped: command missing delimiter *" {
		t.Errorf("expected delimiter error, got %v", logger.errors)
	}
}

func TestProcessGame_EmptyGameCode_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("*123", `{"userid":"1"}`), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_EmptyDenominationID_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("MLBB*", `{"userid":"1"}`), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_EmptyMSISDN_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("MLBB*123", ""), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_InvalidMSISDNJSON_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("MLBB*123", "not-json"), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_EmptyMSISDNMap_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("MLBB*123", "{}"), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_ValidateUserFailed_ForwardsFailed(t *testing.T) {
	server := newGameMockServer(t, func(h *gameHandlers) {
		h.validateUser = func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": 0, "reason": "invalid user"})
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processGame(context.Background(), gamePayload("MLBB*123", `{"userid":"bad"}`), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
	msg, _ := data["message_to_customer"].(string)
	if !strings.Contains(msg, "GAGAL") {
		t.Errorf("message_to_customer should indicate failure, got %q", msg)
	}
	if !strings.Contains(msg, "Status Code : 0") {
		t.Errorf("message_to_customer should include status code, got %q", msg)
	}
	if data["additional_message"] != "invalid user" {
		t.Errorf("additional_message: got %v, want invalid user", data["additional_message"])
	}
}

func TestProcessGame_ValidateUserTechError_ForwardsFailed(t *testing.T) {
	server := newGameMockServer(t, func(h *gameHandlers) {
		h.validateUser = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server error"))
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processGame(context.Background(), gamePayload("MLBB*123", `{"userid":"1"}`), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_CreateOrderFailed_ForwardsFailed(t *testing.T) {
	server := newGameMockServer(t, func(h *gameHandlers) {
		h.createOrder = func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": 0, "reason": "insufficient balance", "reference_no": "REF-FAIL"})
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processGame(context.Background(), gamePayload("MLBB*123", `{"userid":"1"}`), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
	if data["serial_number"] != "REF-FAIL" {
		t.Errorf("serial_number: got %v", data["serial_number"])
	}
	msg, _ := data["message_to_customer"].(string)
	if !strings.Contains(msg, "GAGAL") {
		t.Errorf("message_to_customer should indicate failure, got %q", msg)
	}
	if !strings.Contains(msg, "Status Code : 0") {
		t.Errorf("message_to_customer should include status code, got %q", msg)
	}
	if data["additional_message"] != "insufficient balance" {
		t.Errorf("additional_message: got %v, want insufficient balance", data["additional_message"])
	}
}

func TestProcessGame_CreateOrderTechError_ForwardsFailed(t *testing.T) {
	server := newGameMockServer(t, func(h *gameHandlers) {
		h.createOrder = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("internal error"))
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processGame(context.Background(), gamePayload("MLBB*123", `{"userid":"1"}`), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_CreateOrderTimeout_FallbackInquiry(t *testing.T) {
	server := newGameMockServer(t, func(h *gameHandlers) {
		h.createOrder = func(w http.ResponseWriter, r *http.Request) {
			// Delay longer than client timeout but respond eventually so server can close
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		}
	})
	defer server.Close()

	// Use a very short timeout client
	logger := &mockLogger{}
	pub := &mockMQPublisher{}
	cfg := &config.Config{QueueName: "q", ConsumerTag: "tag", ReadTimeout: 5 * time.Second}
	client, err := unipin.NewClient(server.URL, "partner", "secret", 200*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("create client: %v", err)
	}
	svc := NewConsumerServiceImpl(cfg, client, logger)
	svc.SetMQPublisher(pub)

	svc.processGame(context.Background(), gamePayload("MLBB*123", `{"userid":"1"}`), "q", "MSG1")

	// Should fallback to OrderInquiry and get success
	if len(pub.calls) == 0 {
		t.Fatal("expected publish call from OrderInquiry fallback")
	}
	data := publishedData(t, pub)
	if data["status_to_be"] != "F" {
		t.Errorf("status_to_be: got %v, want F (from inquiry fallback)", data["status_to_be"])
	}

	// Check that timeout warning was logged
	foundWarn := false
	for _, w := range logger.warns {
		if w == "create order timeout, falling back to order inquiry" {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Error("expected timeout warning log")
	}
}

// ─── processOrderInquiry Tests ───

func TestProcessOrderInquiry_Success(t *testing.T) {
	server := newGameMockServer(t)
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	payload := gamePayload("MLBB*123", `{"userid":"1"}`)
	svc.processOrderInquiry(context.Background(), payload, "q", "MSG1", "REF-001")

	data := publishedData(t, pub)
	if data["status_to_be"] != "F" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
	if data["serial_number"] != "REF-001" {
		t.Errorf("serial_number: got %v", data["serial_number"])
	}
}

func TestProcessOrderInquiry_BusinessError_ForwardsFailed(t *testing.T) {
	server := newGameMockServer(t, func(h *gameHandlers) {
		h.orderInquiry = func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": 0, "reason": "order not found", "reference_no": "REF-002"})
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	payload := gamePayload("MLBB*123", `{"userid":"1"}`)
	svc.processOrderInquiry(context.Background(), payload, "q", "MSG1", "REF-002")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
	if data["serial_number"] != "REF-002" {
		t.Errorf("serial_number: got %v", data["serial_number"])
	}
	msg, _ := data["message_to_customer"].(string)
	if !strings.Contains(msg, "GAGAL") {
		t.Errorf("message_to_customer should indicate failure, got %q", msg)
	}
	if !strings.Contains(msg, "Status Code : 0") {
		t.Errorf("message_to_customer should include status code, got %q", msg)
	}
	if data["additional_message"] != "order not found" {
		t.Errorf("additional_message: got %v, want order not found", data["additional_message"])
	}
}

func TestProcessOrderInquiry_TechError_ForwardsFailed(t *testing.T) {
	server := newGameMockServer(t, func(h *gameHandlers) {
		h.orderInquiry = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("server down"))
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	payload := gamePayload("MLBB*123", `{"userid":"1"}`)
	svc.processOrderInquiry(context.Background(), payload, "q", "MSG1", "REF-003")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
	// serialNumber should be referenceNo for tech errors
	if data["serial_number"] != "REF-003" {
		t.Errorf("serial_number: got %v, want REF-003", data["serial_number"])
	}
}

func TestProcessOrderInquiry_Pending_RetriesUntilSuccess(t *testing.T) {
	var calls int
	server := newGameMockServer(t, func(h *gameHandlers) {
		h.orderInquiry = func(w http.ResponseWriter, r *http.Request) {
			calls++
			w.Header().Set("Content-Type", "application/json")
			if calls < 3 {
				json.NewEncoder(w).Encode(map[string]any{"status": 2, "reason": "processing", "reference_no": "REF-PEND", "transaction_number": "TXN-001"})
				return
			}
			json.NewEncoder(w).Encode(map[string]any{"status": 1, "reason": "done", "reference_no": "REF-PEND", "transaction_number": "TXN-001"})
		}
	})
	defer server.Close()

	svc, _, pub := newService(t, server.URL)
	svc.cfg.RetryMaxAttempts = 5
	svc.cfg.RetryWait = 0

	payload := gamePayload("MLBB*123", `{"userid":"1"}`)
	svc.processOrderInquiry(context.Background(), payload, "q", "MSG1", "REF-PEND")

	// Expect 2 publishes: processing (S) then success (F)
	waitForPublishCalls(t, pub, 2, 2*time.Second)
	if len(pub.calls) < 2 {
		t.Fatalf("expected >=2 publish calls, got %d", len(pub.calls))
	}

	first := publishedDataFromCall(t, pub.calls[0])
	if first["status_to_be"] != "S" {
		t.Errorf("first status_to_be: got %v, want S", first["status_to_be"])
	}

	second := publishedDataFromCall(t, pub.calls[1])
	if second["status_to_be"] != "F" {
		t.Errorf("second status_to_be: got %v, want F", second["status_to_be"])
	}
}

func TestProcessOrderInquiry_Pending_MaxRetry_PublishesFailed(t *testing.T) {
	server := newGameMockServer(t, func(h *gameHandlers) {
		h.orderInquiry = func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": 2, "reason": "still processing", "reference_no": "REF-PEND", "transaction_number": "TXN-001"})
		}
	})
	defer server.Close()

	svc, _, pub := newService(t, server.URL)
	svc.cfg.RetryMaxAttempts = 2
	svc.cfg.RetryWait = 0

	payload := gamePayload("MLBB*123", `{"userid":"1"}`)
	svc.processOrderInquiry(context.Background(), payload, "q", "MSG1", "REF-PEND")

	// Expect 2 publishes: processing (S) then final failed/cancel (C)
	waitForPublishCalls(t, pub, 2, 2*time.Second)
	if len(pub.calls) < 2 {
		t.Fatalf("expected >=2 publish calls, got %d", len(pub.calls))
	}

	first := publishedDataFromCall(t, pub.calls[0])
	if first["status_to_be"] != "S" {
		t.Errorf("first status_to_be: got %v, want S", first["status_to_be"])
	}

	second := publishedDataFromCall(t, pub.calls[1])
	if second["status_to_be"] != "C" {
		t.Errorf("second status_to_be: got %v, want C", second["status_to_be"])
	}
}

// ─── processVoucher Tests ───

type voucherHandlers struct {
	voucherRequest http.HandlerFunc
	voucherInquiry http.HandlerFunc
}

func defaultVoucherHandlers() voucherHandlers {
	return voucherHandlers{
		voucherRequest: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status": 1, "reason": "success", "reference_no": "VREF-001",
				"order": "ORD-001", "total_amount": 100000,
				"items": []map[string]string{{"serial_1": "S1", "serial_2": "S2"}},
			})
		},
		voucherInquiry: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{
				"status": 1, "reason": "found", "reference_no": "VREF-001",
				"order": "ORD-001", "total_amount": 100000,
				"items": []map[string]string{{"serial_1": "S1", "serial_2": "S2"}},
			})
		},
	}
}

func newVoucherMockServer(t *testing.T, opts ...func(h *voucherHandlers)) *httptest.Server {
	t.Helper()
	h := defaultVoucherHandlers()
	for _, opt := range opts {
		opt(&h)
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/voucher/request", h.voucherRequest)
	mux.HandleFunc("/voucher/inquiry", h.voucherInquiry)
	return httptest.NewServer(mux)
}

func voucherPayload(command, msgID string) *consumePayload {
	return &consumePayload{
		Command:       command,
		MSISDN:        "08123456789",
		MID:           "MID1",
		MsgID:         msgID,
		Amount:        100000,
		MQTransaction: "amqp://localhost",
		ProductType:   "unipin-voucher",
	}
}

func TestProcessVoucher_HappyPath_Success(t *testing.T) {
	server := newVoucherMockServer(t)
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processVoucher(context.Background(), voucherPayload("STEAM*STEAM-100K", "MSG1"), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "F" {
		t.Errorf("status_to_be: got %v, want F (Status=1)", data["status_to_be"])
	}
	if data["serial_number"] != "VREF-001" {
		t.Errorf("serial_number: got %v, want VREF-001", data["serial_number"])
	}
}

func TestProcessVoucher_EmptyCommand_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processVoucher(context.Background(), voucherPayload("", "MSG1"), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessVoucher_NoDelimiter_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processVoucher(context.Background(), voucherPayload("STEAM100K", "MSG1"), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessVoucher_EmptyDenominationCode_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processVoucher(context.Background(), voucherPayload("STEAM*", "MSG1"), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessVoucher_EmptyMsgID_ReturnsEarly(t *testing.T) {
	svc, logger, pub := newService(t, "http://unused")
	svc.processVoucher(context.Background(), voucherPayload("STEAM*CODE", ""), "q", "")

	// Should return early without publishing (no forwardCallback for empty msgid)
	if len(pub.calls) != 0 {
		t.Errorf("expected no publish for empty msgid, got %d", len(pub.calls))
	}
	found := false
	for _, e := range logger.errors {
		if e == "voucher request skipped: msgid is empty" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error log about empty msgid")
	}
}

func TestProcessVoucher_VoucherRequestFailed_ForwardsFailed(t *testing.T) {
	server := newVoucherMockServer(t, func(h *voucherHandlers) {
		h.voucherRequest = func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": 0, "reason": "out of stock"})
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processVoucher(context.Background(), voucherPayload("STEAM*CODE", "MSG1"), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessVoucher_VoucherRequestTimeout_FallbackInquiry(t *testing.T) {
	server := newVoucherMockServer(t, func(h *voucherHandlers) {
		h.voucherRequest = func(w http.ResponseWriter, r *http.Request) {
			select {
			case <-r.Context().Done():
			case <-time.After(5 * time.Second):
			}
		}
	})
	defer server.Close()

	logger := &mockLogger{}
	pub := &mockMQPublisher{}
	pub.callsCh = make(chan publishCall, 32)
	cfg := &config.Config{QueueName: "q", ConsumerTag: "tag", ReadTimeout: 5 * time.Second}
	client, _ := unipin.NewClient(server.URL, "partner", "secret", 200*time.Millisecond, slog.New(slog.NewTextHandler(io.Discard, nil)))
	svc := NewConsumerServiceImpl(cfg, client, logger)
	svc.SetMQPublisher(pub)

	svc.processVoucher(context.Background(), voucherPayload("STEAM*CODE", "MSG1"), "q", "MSG1")

	// Should fallback to VoucherInquiry (async via goroutine)
	waitForPublishCalls(t, pub, 1, 2*time.Second)
	data := publishedData(t, pub)
	if data["status_to_be"] != "F" {
		t.Errorf("status_to_be: got %v, want F (Status=1)", data["status_to_be"])
	}
}

func TestProcessVoucher_VoucherRequestTechError_ForwardsFailed(t *testing.T) {
	server := newVoucherMockServer(t, func(h *voucherHandlers) {
		h.voucherRequest = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processVoucher(context.Background(), voucherPayload("STEAM*CODE", "MSG1"), "q", "MSG1")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessVoucher_NonZeroStatus_ForwardsFailed(t *testing.T) {
	server := newVoucherMockServer(t, func(h *voucherHandlers) {
		h.voucherRequest = func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": 2, "reason": "partial", "reference_no": "VREF-002"})
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processVoucher(context.Background(), voucherPayload("STEAM*CODE", "MSG1"), "q", "MSG1")

	data := publishedData(t, pub)
	// Status != 0 means FAILED in voucher flow (different from game flow where Status != 1 is error)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

// ─── processVoucherInquiry Tests ───

func TestProcessVoucherInquiry_Success(t *testing.T) {
	server := newVoucherMockServer(t)
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	payload := voucherPayload("STEAM*CODE", "MSG1")
	svc.processVoucherInquiry(context.Background(), payload, "q", "MSG1", "VREF-001")

	data := publishedData(t, pub)
	if data["status_to_be"] != "F" {
		t.Errorf("status_to_be: got %v, want F (Status=1)", data["status_to_be"])
	}
}

func TestProcessVoucherInquiry_Error_ForwardsFailed(t *testing.T) {
	server := newVoucherMockServer(t, func(h *voucherHandlers) {
		h.voucherInquiry = func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	payload := voucherPayload("STEAM*CODE", "MSG1")
	svc.processVoucherInquiry(context.Background(), payload, "q", "MSG1", "VREF-001")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessVoucherInquiry_NonZeroStatus_ForwardsFailed(t *testing.T) {
	server := newVoucherMockServer(t, func(h *voucherHandlers) {
		h.voucherInquiry = func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"status": 3, "reason": "expired", "reference_no": "VREF-003"})
		}
	})
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	payload := voucherPayload("STEAM*CODE", "MSG1")
	svc.processVoucherInquiry(context.Background(), payload, "q", "MSG1", "VREF-003")

	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

// ─── SetMQPublisher / NewConsumerServiceImpl Tests ───

func TestNewConsumerServiceImpl(t *testing.T) {
	logger := &mockLogger{}
	cfg := &config.Config{QueueName: "q", ReadTimeout: 5 * time.Second}
	svc := NewConsumerServiceImpl(cfg, nil, logger)
	if svc == nil {
		t.Fatal("expected non-nil service")
	}
	if svc.mqPublisher != nil {
		t.Error("mqPublisher should be nil initially")
	}
}

func TestSetMQPublisher(t *testing.T) {
	logger := &mockLogger{}
	cfg := &config.Config{QueueName: "q", ReadTimeout: 5 * time.Second}
	svc := NewConsumerServiceImpl(cfg, nil, logger)
	pub := &mockMQPublisher{}
	svc.SetMQPublisher(pub)
	if svc.mqPublisher == nil {
		t.Error("mqPublisher should be set")
	}
}

// ─── awaitDrain Tests ───

func TestAwaitDrain_CompletesBeforeTimeout(t *testing.T) {
	logger := &mockLogger{}
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
	}()
	time.Sleep(10 * time.Millisecond) // let goroutine finish
	awaitDrain(&wg, 1*time.Second, logger)
	if len(logger.warns) > 0 {
		t.Error("should not have timeout warning")
	}
}

func TestAwaitDrain_TimesOut(t *testing.T) {
	logger := &mockLogger{}
	var wg sync.WaitGroup
	wg.Add(1)
	// Don't call wg.Done() — will timeout
	awaitDrain(&wg, 50*time.Millisecond, logger)
	found := false
	for _, w := range logger.warns {
		if w == "consumer shutdown timeout reached" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected timeout warning")
	}
	wg.Done() // cleanup
}

// ─── Additional edge case tests for coverage ───

func TestProcessMessage_CorrelationIdFallback(t *testing.T) {
	// msgid empty, messageId empty → falls back to correlationId
	body := `{"product_type":"unipin-game","command":"","mq_transaction":"amqp://localhost"}`
	svc, _, pub := newService(t, "http://unused")
	d := &amqp.Delivery{Body: []byte(body), CorrelationId: "corr-id"}
	svc.processMessage(context.Background(), d)
	if len(pub.calls) == 0 {
		t.Fatal("expected publish call")
	}
}

func TestProcessGame_WhitespaceCommand_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("   ", `{"userid":"1"}`), "q", "MSG1")
	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_WhitespaceGameCode_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("  *123", `{"userid":"1"}`), "q", "MSG1")
	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_WhitespaceDenomID_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("MLBB*  ", `{"userid":"1"}`), "q", "MSG1")
	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_MSISDNArray_ForwardsFailed(t *testing.T) {
	// Valid JSON but not an object — should fail on unmarshal to map
	svc, _, pub := newService(t, "http://unused")
	svc.processGame(context.Background(), gamePayload("MLBB*123", `[1,2,3]`), "q", "MSG1")
	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessGame_CommandWithMultipleDelimiters(t *testing.T) {
	// "MLBB*123*extra" → SplitN with 2 → ["MLBB", "123*extra"]
	server := newGameMockServer(t)
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processGame(context.Background(), gamePayload("MLBB*123*extra", `{"userid":"1"}`), "q", "MSG1")
	if len(pub.calls) == 0 {
		t.Fatal("expected publish call")
	}
	data := publishedData(t, pub)
	if data["status_to_be"] != "F" {
		t.Errorf("status_to_be: got %v, want F", data["status_to_be"])
	}
}

func TestProcessVoucher_CommandWithMultipleDelimiters(t *testing.T) {
	server := newVoucherMockServer(t)
	defer server.Close()
	svc, _, pub := newService(t, server.URL)

	svc.processVoucher(context.Background(), voucherPayload("STEAM*CODE*extra", "MSG1"), "q", "MSG1")
	if len(pub.calls) == 0 {
		t.Fatal("expected publish call")
	}
}

func TestProcessVoucher_WhitespaceCommand_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processVoucher(context.Background(), voucherPayload("   ", "MSG1"), "q", "MSG1")
	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

func TestProcessVoucher_WhitespaceDenomCode_ForwardsFailed(t *testing.T) {
	svc, _, pub := newService(t, "http://unused")
	svc.processVoucher(context.Background(), voucherPayload("STEAM*  ", "MSG1"), "q", "MSG1")
	data := publishedData(t, pub)
	if data["status_to_be"] != "C" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
}

// ─── forwardCallback: json.Marshal error path ───

func TestForwardCallback_MarshalError_LogsError(t *testing.T) {
	// Override jsonMarshal to simulate error
	original := jsonMarshal
	jsonMarshal = func(v any) ([]byte, error) {
		return nil, fmt.Errorf("marshal failed")
	}
	defer func() { jsonMarshal = original }()

	logger := &mockLogger{}
	pub := &mockMQPublisher{}
	cfg := &config.Config{QueueName: "q", ReadTimeout: 5 * time.Second}
	svc := NewConsumerServiceImpl(cfg, nil, logger)
	svc.SetMQPublisher(pub)

	payload := &consumePayload{MQTransaction: "amqp://host"}
	svc.forwardCallback(context.Background(), payload, "q", "1", "FAILED", "", "", "err")

	// Should NOT publish (marshal failed before publish)
	if len(pub.calls) != 0 {
		t.Errorf("expected 0 publish calls, got %d", len(pub.calls))
	}

	found := false
	for _, e := range logger.errors {
		if e == "failed to marshal publish message" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected error log for marshal failure")
	}
}

// ─── Start / consumeSession Tests ───

func TestStart_ContextAlreadyCancelled_ReturnsNil(t *testing.T) {
	svc, logger, _ := newService(t, "http://unused")
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	err := svc.Start(ctx)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	found := false
	for _, i := range logger.infos {
		if i == "rabbitmq consumer stopped" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'rabbitmq consumer stopped' info log")
	}
}

func TestStart_DialFails_RetriesThenContextCancel(t *testing.T) {
	original := amqpDial
	callCount := 0
	amqpDial = func(url string) (amqpConnection, error) {
		callCount++
		return nil, fmt.Errorf("connection refused")
	}
	defer func() { amqpDial = original }()

	svc, logger, _ := newService(t, "http://unused")
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(150 * time.Millisecond)
		cancel()
	}()

	err := svc.Start(ctx)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if callCount < 1 {
		t.Errorf("expected at least 1 dial attempt, got %d", callCount)
	}
	foundWarn := false
	for _, w := range logger.warns {
		if w == "rabbitmq consume session ended, attempting recovery" {
			foundWarn = true
			break
		}
	}
	if !foundWarn {
		t.Error("expected recovery warning log")
	}
}

func TestStart_DialFailsMultipleTimes_ReconnectDelayIncreases(t *testing.T) {
	original := amqpDial
	callCount := 0
	amqpDial = func(url string) (amqpConnection, error) {
		callCount++
		return nil, fmt.Errorf("connection refused")
	}
	defer func() { amqpDial = original }()

	svc, _, _ := newService(t, "http://unused")
	ctx, cancel := context.WithCancel(context.Background())

	// Let it retry a few times (initial=1s, so cancel after ~2.5s to get 2 retries)
	go func() {
		time.Sleep(2500 * time.Millisecond)
		cancel()
	}()

	err := svc.Start(ctx)
	if err != nil {
		t.Errorf("expected nil error, got %v", err)
	}
	if callCount < 2 {
		t.Errorf("expected at least 2 dial attempts, got %d", callCount)
	}
}

func TestConsumeSession_DialError_ReturnsError(t *testing.T) {
	original := amqpDial
	amqpDial = func(url string) (amqpConnection, error) {
		return nil, fmt.Errorf("dial failed")
	}
	defer func() { amqpDial = original }()

	svc, _, _ := newService(t, "http://unused")
	err := svc.consumeSession(context.Background())
	if err == nil {
		t.Fatal("expected error from consumeSession")
	}
	if !strings.Contains(err.Error(), "failed to connect rabbitmq") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestStart_SessionReturnsNil_StopsGracefully(t *testing.T) {
	original := amqpDial
	amqpDial = func(url string) (amqpConnection, error) {
		return nil, fmt.Errorf("simulated")
	}
	defer func() { amqpDial = original }()

	svc, logger, _ := newService(t, "http://unused")
	ctx, cancel := context.WithCancel(context.Background())

	// Cancel during the reconnect wait
	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	err := svc.Start(ctx)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	foundStop := false
	for _, i := range logger.infos {
		if i == "rabbitmq consumer stopped" {
			foundStop = true
			break
		}
	}
	if !foundStop {
		t.Error("expected stop log")
	}
}

// ─── consumeSession with fake AMQP server ───

// mockAMQPConn implements amqpConnection for testing.
type mockAMQPConn struct {
	channelErr error
	ch         amqpChannel
	closed     bool
}

func (m *mockAMQPConn) Channel() (amqpChannel, error) {
	if m.channelErr != nil {
		return nil, m.channelErr
	}
	if m.ch != nil {
		return m.ch, nil
	}
	return nil, fmt.Errorf("no channel configured")
}

func (m *mockAMQPConn) Close() error {
	m.closed = true
	return nil
}

// mockAMQPChannel implements amqpChannel for testing.
type mockAMQPChannel struct {
	queueDeclareErr error
	qosErr          error
	consumeErr      error
	deliveries      chan amqp.Delivery
	notifyCloseCh   chan *amqp.Error
}

func (m *mockAMQPChannel) QueueDeclare(string, bool, bool, bool, bool, amqp.Table) (amqp.Queue, error) {
	return amqp.Queue{}, m.queueDeclareErr
}
func (m *mockAMQPChannel) Qos(int, int, bool) error { return m.qosErr }
func (m *mockAMQPChannel) Consume(string, string, bool, bool, bool, bool, amqp.Table) (<-chan amqp.Delivery, error) {
	if m.consumeErr != nil {
		return nil, m.consumeErr
	}
	if m.deliveries == nil {
		m.deliveries = make(chan amqp.Delivery)
	}
	return m.deliveries, nil
}
func (m *mockAMQPChannel) NotifyClose(receiver chan *amqp.Error) chan *amqp.Error {
	if m.notifyCloseCh != nil {
		// Forward: when notifyCloseCh receives, relay to the receiver channel
		go func() {
			for err := range m.notifyCloseCh {
				receiver <- err
			}
		}()
	}
	return receiver
}
func (m *mockAMQPChannel) Cancel(string, bool) error { return nil }
func (m *mockAMQPChannel) Close() error              { return nil }

func TestConsumeSession_ChannelError_ReturnsError(t *testing.T) {
	original := amqpDial
	amqpDial = func(url string) (amqpConnection, error) {
		return &mockAMQPConn{channelErr: fmt.Errorf("channel refused")}, nil
	}
	defer func() { amqpDial = original }()

	svc, _, _ := newService(t, "http://unused")
	err := svc.consumeSession(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to open rabbitmq channel") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConsumeSession_QueueDeclareError_ReturnsError(t *testing.T) {
	original := amqpDial
	amqpDial = func(url string) (amqpConnection, error) {
		return &mockAMQPConn{ch: &mockAMQPChannel{queueDeclareErr: fmt.Errorf("queue error")}}, nil
	}
	defer func() { amqpDial = original }()

	svc, _, _ := newService(t, "http://unused")
	err := svc.consumeSession(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to declare queue") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConsumeSession_QosError_ReturnsError(t *testing.T) {
	original := amqpDial
	amqpDial = func(url string) (amqpConnection, error) {
		return &mockAMQPConn{ch: &mockAMQPChannel{qosErr: fmt.Errorf("qos error")}}, nil
	}
	defer func() { amqpDial = original }()

	svc, _, _ := newService(t, "http://unused")
	err := svc.consumeSession(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to set qos") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConsumeSession_ConsumeError_ReturnsError(t *testing.T) {
	original := amqpDial
	amqpDial = func(url string) (amqpConnection, error) {
		return &mockAMQPConn{ch: &mockAMQPChannel{consumeErr: fmt.Errorf("consume error")}}, nil
	}
	defer func() { amqpDial = original }()

	svc, _, _ := newService(t, "http://unused")
	err := svc.consumeSession(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "failed to register consumer") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConsumeSession_ContextCancel_ReturnsNil(t *testing.T) {
	original := amqpDial
	deliveries := make(chan amqp.Delivery)
	amqpDial = func(url string) (amqpConnection, error) {
		return &mockAMQPConn{ch: &mockAMQPChannel{deliveries: deliveries}}, nil
	}
	defer func() { amqpDial = original }()

	svc, logger, _ := newService(t, "http://unused")
	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(deliveries) // unblock the delivery goroutine
		cancel()
	}()

	err := svc.consumeSession(ctx)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}
	found := false
	for _, i := range logger.infos {
		if i == "rabbitmq consumer started" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'rabbitmq consumer started' log")
	}
}

func TestConsumeSession_ChannelClosed_ReturnsError(t *testing.T) {
	original := amqpDial
	notifyCh := make(chan *amqp.Error, 1)
	deliveries := make(chan amqp.Delivery)
	amqpDial = func(url string) (amqpConnection, error) {
		return &mockAMQPConn{ch: &mockAMQPChannel{notifyCloseCh: notifyCh, deliveries: deliveries}}, nil
	}
	defer func() { amqpDial = original }()

	svc, _, _ := newService(t, "http://unused")

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(deliveries) // unblock the delivery goroutine
		notifyCh <- nil   // channel closed without error
	}()

	err := svc.consumeSession(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rabbitmq channel closed") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConsumeSession_ChannelClosedWithError_ReturnsError(t *testing.T) {
	original := amqpDial
	notifyCh := make(chan *amqp.Error, 1)
	deliveries := make(chan amqp.Delivery)
	amqpDial = func(url string) (amqpConnection, error) {
		return &mockAMQPConn{ch: &mockAMQPChannel{notifyCloseCh: notifyCh, deliveries: deliveries}}, nil
	}
	defer func() { amqpDial = original }()

	svc, _, _ := newService(t, "http://unused")

	go func() {
		time.Sleep(50 * time.Millisecond)
		close(deliveries) // unblock the delivery goroutine
		notifyCh <- &amqp.Error{Code: 320, Reason: "connection forced"}
	}()

	err := svc.consumeSession(context.Background())
	if err == nil {
		t.Fatal("expected error")
	}
	if !strings.Contains(err.Error(), "rabbitmq channel closed unexpectedly") {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestConsumeSession_ProcessesDelivery(t *testing.T) {
	original := amqpDial
	deliveries := make(chan amqp.Delivery, 1)
	mockCh := &mockAMQPChannel{deliveries: deliveries}
	amqpDial = func(url string) (amqpConnection, error) {
		return &mockAMQPConn{ch: mockCh}, nil
	}
	defer func() { amqpDial = original }()

	svc, logger, _ := newService(t, "http://unused")
	ctx, cancel := context.WithCancel(context.Background())

	// Send a delivery then cancel
	go func() {
		deliveries <- amqp.Delivery{
			Body:      []byte(`{"product_type":"unknown","msgid":"TEST1"}`),
			MessageId: "TEST1",
		}
		time.Sleep(50 * time.Millisecond)
		close(deliveries)
		cancel()
	}()

	err := svc.consumeSession(ctx)
	if err != nil {
		t.Errorf("expected nil, got %v", err)
	}

	found := false
	for _, i := range logger.infos {
		if i == "message received" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected 'message received' log")
	}
}
