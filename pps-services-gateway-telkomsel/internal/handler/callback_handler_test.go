package handler

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
	"pps-services-gateway-telkomsel/internal/infrastructure/mqpublisher"
	"testing"

	"github.com/gofiber/fiber/v2"
)

// validCallbackQuery returns a query string with all required valid parameters.
func validCallbackQuery() string {
	return "transaction_id=TRX123&organization_code=ORG123&service_id=6200000000001&status=SUCCESS&message=OK"
}

func setupApp(h *CallbackHandler) *fiber.App {
	app := fiber.New()
	app.Get("/callback/ext", h.Handle)
	return app
}

func parseResponse(t *testing.T, resp *http.Response) CallbackResponse {
	t.Helper()
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("failed to read response body: %v", err)
	}
	var cr CallbackResponse
	if err := json.Unmarshal(body, &cr); err != nil {
		t.Fatalf("failed to unmarshal response: %v (body: %s)", err, string(body))
	}
	return cr
}

func TestHandle_ValidCallback_ReturnsOK(t *testing.T) {
	h := NewCallbackHandler(&mockLogger{}, nil, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+validCallbackQuery(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	cr := parseResponse(t, resp)
	if cr.Status != "OK" {
		t.Errorf("expected status OK, got %q", cr.Status)
	}
	if cr.Message != "Callback received" {
		t.Errorf("expected message 'Callback received', got %q", cr.Message)
	}
}

func TestHandle_MissingTransactionID_Returns400(t *testing.T) {
	h := NewCallbackHandler(&mockLogger{}, nil, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?organization_code=ORG123&service_id=6200000000001&status=SUCCESS&message=OK", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	cr := parseResponse(t, resp)
	if cr.Status != "ERROR" {
		t.Errorf("expected status ERROR, got %q", cr.Status)
	}
	if cr.Message != "transaction_id is required" {
		t.Errorf("expected message 'transaction_id is required', got %q", cr.Message)
	}
}

func TestHandle_OrgCodeTooShort_Returns400(t *testing.T) {
	h := NewCallbackHandler(&mockLogger{}, nil, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?transaction_id=TRX1&organization_code=AB&service_id=6200000000001&status=SUCCESS&message=OK", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	cr := parseResponse(t, resp)
	if cr.Status != "ERROR" {
		t.Errorf("expected status ERROR, got %q", cr.Status)
	}
}

func TestHandle_OrgCodeTooLong_Returns400(t *testing.T) {
	h := NewCallbackHandler(&mockLogger{}, nil, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?transaction_id=TRX1&organization_code=12345678901234&service_id=6200000000001&status=SUCCESS&message=OK", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestHandle_ServiceIDNot13Chars_Returns400(t *testing.T) {
	h := NewCallbackHandler(&mockLogger{}, nil, nil, "test-queue")
	app := setupApp(h)

	// service_id with 10 chars instead of 13
	req := httptest.NewRequest(http.MethodGet, "/callback/ext?transaction_id=TRX1&organization_code=ORG123&service_id=6200000001&status=SUCCESS&message=OK", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	cr := parseResponse(t, resp)
	if cr.Status != "ERROR" {
		t.Errorf("expected status ERROR, got %q", cr.Status)
	}
}

func TestHandle_InvalidStatus_Returns400(t *testing.T) {
	h := NewCallbackHandler(&mockLogger{}, nil, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?transaction_id=TRX1&organization_code=ORG123&service_id=6200000000001&status=PENDING&message=OK", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	cr := parseResponse(t, resp)
	if cr.Status != "ERROR" {
		t.Errorf("expected status ERROR, got %q", cr.Status)
	}
}

func TestHandle_MissingMessage_Returns400(t *testing.T) {
	h := NewCallbackHandler(&mockLogger{}, nil, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?transaction_id=TRX1&organization_code=ORG123&service_id=6200000000001&status=SUCCESS", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	cr := parseResponse(t, resp)
	if cr.Message != "message is required" {
		t.Errorf("expected 'message is required', got %q", cr.Message)
	}
}

func TestHandle_WithTransactionLogger_CallsLookupAndInsert(t *testing.T) {
	ml := &mockLogger{}
	mtl := &mockTransactionLogger{
		getByOurTrxIDResult: &contractsvc.TransactionRecord{
			MsgID:         "MSG-001",
			OurTrxID:      "TRX123",
			MQTransaction: "amqp://localhost",
			QueueName:     "downstream-queue",
		},
	}
	h := NewCallbackHandler(ml, mtl, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+validCallbackQuery(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify GetTransactionByOurTrxID was called
	if len(mtl.getByOurTrxIDCalls) != 1 {
		t.Fatalf("expected 1 GetTransactionByOurTrxID call, got %d", len(mtl.getByOurTrxIDCalls))
	}
	if mtl.getByOurTrxIDCalls[0] != "TRX123" {
		t.Errorf("expected lookup with TRX123, got %q", mtl.getByOurTrxIDCalls[0])
	}

	// Verify InsertCallbackResponse was called
	if len(mtl.insertCallbackCalls) != 1 {
		t.Fatalf("expected 1 InsertCallbackResponse call, got %d", len(mtl.insertCallbackCalls))
	}
	rec := mtl.insertCallbackCalls[0]
	if rec.MsgID != "MSG-001" {
		t.Errorf("expected MsgID MSG-001, got %q", rec.MsgID)
	}
	if rec.OurTrxID != "TRX123" {
		t.Errorf("expected OurTrxID TRX123, got %q", rec.OurTrxID)
	}
	// status SUCCESS -> statusCode "0"
	if rec.StatusCode != "0" {
		t.Errorf("expected StatusCode '0' for SUCCESS, got %q", rec.StatusCode)
	}
}

func TestHandle_WithMQPublisher_PublishesMessage(t *testing.T) {
	ml := &mockLogger{}
	mtl := &mockTransactionLogger{
		getByOurTrxIDResult: &contractsvc.TransactionRecord{
			MsgID:         "42",
			OurTrxID:      "TRX123",
			MQTransaction: "amqp://mq-host",
			QueueName:     "downstream-q",
		},
	}
	mp := &mockMQPublisher{}
	h := NewCallbackHandler(ml, mtl, mp, "default-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+validCallbackQuery(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify Publish was called
	if len(mp.publishCalls) != 1 {
		t.Fatalf("expected 1 Publish call, got %d", len(mp.publishCalls))
	}
	call := mp.publishCalls[0]
	if call.MQTransactionURL != "amqp://mq-host" {
		t.Errorf("expected MQTransactionURL 'amqp://mq-host', got %q", call.MQTransactionURL)
	}
	if call.QueueName != "downstream-q" {
		t.Errorf("expected QueueName 'downstream-q', got %q", call.QueueName)
	}

	// Verify the published body contains correct ProviderPublishMessage
	var msg mqpublisher.ProviderPublishMessage
	if err := json.Unmarshal(call.Body, &msg); err != nil {
		t.Fatalf("failed to unmarshal published body: %v", err)
	}
	if msg.Source != "PROVIDER" {
		t.Errorf("expected source PROVIDER, got %q", msg.Source)
	}
	if msg.Data.MsgID != 42 {
		t.Errorf("expected MsgID 42, got %d", msg.Data.MsgID)
	}
	if msg.Data.StatusToBe != "F" {
		t.Errorf("expected StatusToBe F, got %q", msg.Data.StatusToBe)
	}
	if msg.Data.ConversationID != "TRX123" {
		t.Errorf("expected ConversationID TRX123, got %q", msg.Data.ConversationID)
	}
	if msg.Data.QueueName != "downstream-q" {
		t.Errorf("expected QueueName 'downstream-q', got %q", msg.Data.QueueName)
	}
}

func TestHandle_TransactionLoggerLookupFails_UsesFallbackMsgID(t *testing.T) {
	ml := &mockLogger{}
	mtl := &mockTransactionLogger{
		getByOurTrxIDErr: errors.New("db connection failed"),
	}
	h := NewCallbackHandler(ml, mtl, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+validCallbackQuery(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// InsertCallbackResponse should still be called with transaction_id as fallback msg_id
	if len(mtl.insertCallbackCalls) != 1 {
		t.Fatalf("expected 1 InsertCallbackResponse call, got %d", len(mtl.insertCallbackCalls))
	}
	rec := mtl.insertCallbackCalls[0]
	if rec.MsgID != "TRX123" {
		t.Errorf("expected fallback MsgID 'TRX123', got %q", rec.MsgID)
	}

	// Verify warn was logged about the lookup failure
	if len(ml.warnCalls) == 0 {
		t.Error("expected a warn log about lookup failure")
	}
}

func TestHandle_NilPublisherAndNilLogger_ReturnsOK(t *testing.T) {
	ml := &mockLogger{}
	h := NewCallbackHandler(ml, nil, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+validCallbackQuery(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	cr := parseResponse(t, resp)
	if cr.Status != "OK" {
		t.Errorf("expected status OK, got %q", cr.Status)
	}
	if cr.Message != "Callback received" {
		t.Errorf("expected message 'Callback received', got %q", cr.Message)
	}
}

func TestHandle_InsertCallbackResponseError_LogsError(t *testing.T) {
	ml := &mockLogger{}
	mtl := &mockTransactionLogger{
		getByOurTrxIDResult: &contractsvc.TransactionRecord{
			MsgID:    "MSG-002",
			OurTrxID: "TRX123",
		},
		insertCallbackErr: errors.New("insert failed"),
	}
	h := NewCallbackHandler(ml, mtl, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+validCallbackQuery(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify error was logged
	found := false
	for _, call := range ml.errorCalls {
		if len(call) > 0 {
			if msg, ok := call[0].(string); ok && msg == "failed to insert callback response" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected error log about failed insert callback response")
	}
}

func TestHandle_PublishError_LogsError(t *testing.T) {
	ml := &mockLogger{}
	mtl := &mockTransactionLogger{
		getByOurTrxIDResult: &contractsvc.TransactionRecord{
			MsgID:         "42",
			OurTrxID:      "TRX123",
			MQTransaction: "amqp://mq-host",
			QueueName:     "downstream-q",
		},
	}
	mp := &mockMQPublisher{publishErr: errors.New("publish failed")}
	h := NewCallbackHandler(ml, mtl, mp, "default-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+validCallbackQuery(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify error was logged
	found := false
	for _, call := range ml.errorCalls {
		if len(call) > 0 {
			if msg, ok := call[0].(string); ok && msg == "failed to publish callback to downstream rabbitmq" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected error log about failed publish")
	}
}

func TestHandle_EmptyMQTransactionURL_SkipsPublish(t *testing.T) {
	ml := &mockLogger{}
	mtl := &mockTransactionLogger{
		getByOurTrxIDResult: &contractsvc.TransactionRecord{
			MsgID:         "42",
			OurTrxID:      "TRX123",
			MQTransaction: "", // empty URL
			QueueName:     "downstream-q",
		},
	}
	mp := &mockMQPublisher{}
	h := NewCallbackHandler(ml, mtl, mp, "default-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+validCallbackQuery(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	// Verify publish was NOT called
	if len(mp.publishCalls) != 0 {
		t.Fatalf("expected 0 Publish calls, got %d", len(mp.publishCalls))
	}

	// Verify warn about empty mq_transaction URL
	found := false
	for _, call := range ml.warnCalls {
		if len(call) > 0 {
			if msg, ok := call[0].(string); ok && msg == "mq_transaction URL not available, skipping callback publish" {
				found = true
				break
			}
		}
	}
	if !found {
		t.Error("expected warn log about empty mq_transaction URL")
	}
}

func TestHandle_FAILEDStatus_SetsStatusCode1(t *testing.T) {
	ml := &mockLogger{}
	mtl := &mockTransactionLogger{
		getByOurTrxIDResult: &contractsvc.TransactionRecord{
			MsgID:    "MSG-003",
			OurTrxID: "TRX456",
		},
	}
	h := NewCallbackHandler(ml, mtl, nil, "test-queue")
	app := setupApp(h)

	query := "transaction_id=TRX456&organization_code=ORG123&service_id=6200000000001&status=FAILED&message=SomethingWrong"
	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+query, nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(mtl.insertCallbackCalls) != 1 {
		t.Fatalf("expected 1 InsertCallbackResponse call, got %d", len(mtl.insertCallbackCalls))
	}
	rec := mtl.insertCallbackCalls[0]
	if rec.StatusCode != "1" {
		t.Errorf("expected StatusCode '1' for FAILED, got %q", rec.StatusCode)
	}
}

func TestHandle_MissingOrgCode_Returns400(t *testing.T) {
	h := NewCallbackHandler(&mockLogger{}, nil, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?transaction_id=TRX1&service_id=6200000000001&status=SUCCESS&message=OK", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	cr := parseResponse(t, resp)
	if cr.Message != "organization_code is required" {
		t.Errorf("expected 'organization_code is required', got %q", cr.Message)
	}
}

func TestHandle_MissingServiceID_Returns400(t *testing.T) {
	h := NewCallbackHandler(&mockLogger{}, nil, nil, "test-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?transaction_id=TRX1&organization_code=ORG123&status=SUCCESS&message=OK", nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
	cr := parseResponse(t, resp)
	if cr.Message != "service_id is required" {
		t.Errorf("expected 'service_id is required', got %q", cr.Message)
	}
}

func TestHandle_NonNumericMsgID_PublishesWithZeroMsgID(t *testing.T) {
	ml := &mockLogger{}
	mtl := &mockTransactionLogger{
		getByOurTrxIDResult: &contractsvc.TransactionRecord{
			MsgID:         "not-a-number",
			OurTrxID:      "TRX123",
			MQTransaction: "amqp://mq-host",
			QueueName:     "downstream-q",
		},
	}
	mp := &mockMQPublisher{}
	h := NewCallbackHandler(ml, mtl, mp, "default-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+validCallbackQuery(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(mp.publishCalls) != 1 {
		t.Fatalf("expected 1 Publish call, got %d", len(mp.publishCalls))
	}

	var msg mqpublisher.ProviderPublishMessage
	if err := json.Unmarshal(mp.publishCalls[0].Body, &msg); err != nil {
		t.Fatalf("failed to unmarshal published body: %v", err)
	}
	if msg.Data.MsgID != 0 {
		t.Errorf("expected MsgID 0 for non-numeric msgID, got %d", msg.Data.MsgID)
	}
}

func TestHandle_TransactionLookupReturnsEmptyQueueName_UsesDefaultQueue(t *testing.T) {
	ml := &mockLogger{}
	mtl := &mockTransactionLogger{
		getByOurTrxIDResult: &contractsvc.TransactionRecord{
			MsgID:         "42",
			OurTrxID:      "TRX123",
			MQTransaction: "amqp://mq-host",
			QueueName:     "", // empty queue name
		},
	}
	mp := &mockMQPublisher{}
	h := NewCallbackHandler(ml, mtl, mp, "default-queue")
	app := setupApp(h)

	req := httptest.NewRequest(http.MethodGet, "/callback/ext?"+validCallbackQuery(), nil)
	resp, err := app.Test(req, -1)
	if err != nil {
		t.Fatalf("app.Test failed: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	if len(mp.publishCalls) != 1 {
		t.Fatalf("expected 1 Publish call, got %d", len(mp.publishCalls))
	}
	if mp.publishCalls[0].QueueName != "default-queue" {
		t.Errorf("expected default queue name 'default-queue', got %q", mp.publishCalls[0].QueueName)
	}
}
