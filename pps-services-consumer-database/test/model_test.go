package test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"pps-services-consumer-database/internal/model"
)

// TestCallbackEvent_UnmarshalFromPublisher tests unmarshalling a JSON string matching publisher format.
func TestCallbackEvent_UnmarshalFromPublisher(t *testing.T) {
	raw := `{
		"id": "40444906",
		"queue_name": "biller-telkomsel-1",
		"headers": {"X-Action": ["topup"]},
		"body": {"source": "PROVIDER", "data": {"msg_id": 40444906, "status_to_be": "S"}}
	}`

	var event model.TransactionEvent
	err := json.Unmarshal([]byte(raw), &event)
	require.NoError(t, err)

	assert.Equal(t, "40444906", event.Id)
	assert.Equal(t, "biller-telkomsel-1", event.QueueName)
	assert.Equal(t, []string{"topup"}, event.Headers["X-Action"])
	assert.NotNil(t, event.Payload)
}

// TestCallbackEvent_BodyIsRawMessage verifies Body stays as raw JSON bytes.
func TestCallbackEvent_BodyIsRawMessage(t *testing.T) {
	event := model.TransactionEvent{
		Id:        "123",
		QueueName: "test-queue",
		Payload:   json.RawMessage(`{"key":"value"}`),
	}

	data, err := json.Marshal(event)
	require.NoError(t, err)

	var decoded model.TransactionEvent
	err = json.Unmarshal(data, &decoded)
	require.NoError(t, err)

	assert.Equal(t, "123", decoded.Id)
	assert.JSONEq(t, `{"key":"value"}`, string(decoded.Payload))
}

// TestCallbackBody_DecodeProvider decodes body with source=PROVIDER.
func TestCallbackBody_DecodeProvider(t *testing.T) {
	raw := `{"source": "PROVIDER", "data": {"msg_id": 100, "status_to_be": "S"}}`

	var cb model.TransactionPayload
	err := json.Unmarshal([]byte(raw), &cb)
	require.NoError(t, err)

	assert.Equal(t, model.SourceProvider, cb.Source)
	assert.NotNil(t, cb.Data)

	var payload model.TopupDataPayload
	err = json.Unmarshal(cb.Data, &payload)
	require.NoError(t, err)
	assert.Equal(t, 100, payload.MsgID)
	assert.Equal(t, "S", payload.StatusToBe)
}

// TestCallbackBody_DecodeOrder decodes body with source=ORDER.
func TestCallbackBody_DecodeOrder(t *testing.T) {
	raw := `{"source": "PRE_ORDER", "data": {"msgId": 200, "consumeStatus": "PENDING", "clientNumber": "08123"}}`

	var cb model.TransactionPayload
	err := json.Unmarshal([]byte(raw), &cb)
	require.NoError(t, err)

	assert.Equal(t, model.SourcePreOrder, cb.Source)

	var payload model.OrderPayload
	err = json.Unmarshal(cb.Data, &payload)
	require.NoError(t, err)
	assert.Equal(t, 200, payload.MsgID)
	assert.Equal(t, "PENDING", payload.ConsumeStatus)
	assert.Equal(t, "08123", payload.ClientNumber)
}

// TestTopupPayload_Unmarshal verifies TopupPayload fields.
func TestTopupPayload_Unmarshal(t *testing.T) {
	raw := `{
		"msg_id": 40444906,
		"status_to_be": "S",
		"serial_number": "SN123",
		"client_number": "08123",
		"nominal": "10000",
		"original_conversation_id": "OC1",
		"conversation_id": "C1",
		"message_to_customer": "OK",
		"additional_message": "extra",
		"queue_name": "biller-telkomsel-1"
	}`

	var p model.TopupDataPayload
	err := json.Unmarshal([]byte(raw), &p)
	require.NoError(t, err)

	assert.Equal(t, 40444906, p.MsgID)
	assert.Equal(t, "S", p.StatusToBe)
	assert.Equal(t, "SN123", p.SerialNumber)
	assert.Equal(t, "08123", p.ClientNumber)
	assert.Equal(t, "10000", p.Nominal)
	assert.Equal(t, "OC1", p.OriginalConversationID)
	assert.Equal(t, "C1", p.ConversationID)
	assert.Equal(t, "OK", p.MessageToCustomer)
	assert.Equal(t, "extra", p.AdditionalMessage)
	assert.Equal(t, "biller-telkomsel-1", p.QueueName)
}

// TestOrderPayload_Unmarshal verifies OrderPayload fields.
func TestOrderPayload_Unmarshal(t *testing.T) {
	raw := `{"msg_id": 300, "consume_status": "DONE", "client_number": "08999"}`

	var p model.OrderPayload
	err := json.Unmarshal([]byte(raw), &p)
	require.NoError(t, err)

	assert.Equal(t, 300, p.MsgID)
	assert.Equal(t, "DONE", p.ConsumeStatus)
	assert.Equal(t, "08999", p.ClientNumber)
}

// TestDownstreamRequest_Marshal verifies JSON output of DownstreamRequest.
func TestDownstreamRequest_Marshal(t *testing.T) {
	req := model.DownstreamRequest{
		MsgID:         "1",
		ClientNumber:  "08123",
		IMSI:          "imsi-val",
		RemarkIMSI:    "remark",
		MID:           "mid-val",
		StoreID:       10,
		QueueName:     "q1",
		TypeVoucher:   "tv",
		VoucherCode:   "VC001",
		BID:           5,
		TypeOfStock:   "stock",
		Provider:      "prov",
		MQTransaction: "mq-txn",
	}

	data, err := json.Marshal(req)
	require.NoError(t, err)

	var m map[string]any
	err = json.Unmarshal(data, &m)
	require.NoError(t, err)

	assert.Equal(t, float64(1), m["msg_id"])
	assert.Equal(t, "08123", m["client_number"])
	assert.Equal(t, "imsi-val", m["imsi"])
	assert.Equal(t, "VC001", m["voucher_code"])
	assert.Equal(t, "mq-txn", m["mq_transaction"])
}

// TestPreOrderResult_Fields verifies PreOrderResult struct.
func TestPreOrderResult_Fields(t *testing.T) {
	r := model.PreOrderResult{
		IMSI:        "imsi1",
		RemarkIMSI:  "remark1",
		MID:         "mid1",
		StoreID:     42,
		QueueName:   "q1",
		TypeVoucher: "tv1",
		BID:         7,
		TypeOfStock: "stock1",
		Provider:    "prov1",
	}

	assert.Equal(t, "imsi1", r.IMSI)
	assert.Equal(t, 42, r.StoreID)
	assert.Equal(t, 7, r.BID)
	assert.Equal(t, "prov1", r.Provider)
}

// TestSPResult_Fields tests SPResult struct fields.
func TestSPResult_Fields(t *testing.T) {
	result := model.SPResult{
		ID:      12345,
		Error:   0,
		Message: "Success",
	}

	assert.Equal(t, 12345, result.ID)
	assert.Equal(t, 0, result.Error)
	assert.Equal(t, "Success", result.Message)
}

// TestSPResult_WithError tests SPResult with error.
func TestSPResult_WithError(t *testing.T) {
	result := model.SPResult{
		ID:      0,
		Error:   99,
		Message: "Transaction not found",
	}

	assert.Equal(t, 0, result.ID)
	assert.Equal(t, 99, result.Error)
	assert.Equal(t, "Transaction not found", result.Message)
}
