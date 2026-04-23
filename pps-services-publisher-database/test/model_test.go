package test

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"

	"pps-services-publisher-database/internal/model"
)

// TestTopupPayload_GetMsgId verifies HasMsgIdAndQueueName interface — GetMsgId.
func TestTopupPayload_GetMsgId(t *testing.T) {
	p := model.TopupDataPayload{MsgID: "42", QueueName: "q"}
	var iface model.HasMsgIdAndQueueName = p // compile-time check
	assert.Equal(t, 42, iface.GetMsgID())
}

// TestTopupPayload_GetQueueName verifies HasMsgIdAndQueueName interface — GetQueueName.
func TestTopupPayload_GetQueueName(t *testing.T) {
	p := model.TopupDataPayload{MsgID: "1", QueueName: "biller-telkomsel-1"}
	var iface model.HasMsgIdAndQueueName = p
	assert.Equal(t, "biller-telkomsel-1", iface.GetQueueName())
}

// TestCallbackRequest_ToCallbackEvent verifies conversion preserves Id, QueueName, Body.
func TestCallbackRequest_ToCallbackEvent(t *testing.T) {
	req := &model.CallbackRequest[model.TopupDataPayload]{
		Source: "test-source",
		Data: model.TopupDataPayload{
			MsgID:                  "123",
			StatusToBe:             "0",
			ClientNumber:           "08112233445",
			OriginalConversationID: "ORIG-001",
			ConversationID:         "CONV-001",
			MessageToCustomer:      "OK",
			QueueName:              "biller-pln",
		},
	}

	event := req.ToCallbackEvent(nil)

	assert.Equal(t, "123", event.ID)
	assert.Equal(t, "biller-pln", event.QueueName)
	assert.Nil(t, event.Headers)

	// Body should be valid json.RawMessage containing the payload
	var body map[string]interface{}
	err := json.Unmarshal(event.Payload, &body)
	assert.Nil(t, err)
	assert.Equal(t, float64(123), body["msg_id"])
	assert.Equal(t, "biller-pln", body["queue_name"])
	assert.Equal(t, "CONV-001", body["conversation_id"])
}

// TestCallbackRequest_ToCallbackEvent_WithHeaders verifies headers are passed through.
func TestCallbackRequest_ToCallbackEvent_WithHeaders(t *testing.T) {
	headers := map[string][]string{
		"X-Action":  {"topup"},
		"X-Request": {"req-001"},
	}

	req := &model.CallbackRequest[model.TopupDataPayload]{
		Source: "src",
		Data: model.TopupDataPayload{
			MsgID:                  "1",
			StatusToBe:             "0",
			ClientNumber:           "08112233445",
			OriginalConversationID: "ORIG",
			ConversationID:         "CONV",
			MessageToCustomer:      "OK",
			QueueName:              "q1",
		},
	}

	event := req.ToCallbackEvent(headers)

	assert.Equal(t, "1", event.ID)
	assert.Equal(t, "q1", event.QueueName)
	assert.Equal(t, []string{"topup"}, event.Headers["X-Action"])
	assert.Equal(t, []string{"req-001"}, event.Headers["X-Request"])
}

// TestTopupPayload_Validation_Positive — valid payload passes validation.
func TestTopupPayload_Validation_Positive(t *testing.T) {
	validate := newTestValidator()

	req := model.CallbackRequest[model.TopupDataPayload]{
		Source: "test",
		Data: model.TopupDataPayload{
			MsgID:                  "1",
			StatusToBe:             "0",
			ClientNumber:           "08112233445",
			OriginalConversationID: "ORIG-001",
			ConversationID:         "CONV-001",
			MessageToCustomer:      "Payment successful",
			QueueName:              "biller-telkomsel-1",
		},
	}

	err := validate.Struct(req)
	assert.Nil(t, err, "Valid payload should pass validation")
}

// TestTopupPayload_Validation_Negative_EmptyMsgId — MsgId=0 fails validation.
func TestTopupPayload_Validation_Negative_EmptyMsgId(t *testing.T) {
	validate := newTestValidator()

	payload := model.TopupDataPayload{
		MsgID:                  "0",
		StatusToBe:             "0",
		ClientNumber:           "08112233445",
		OriginalConversationID: "ORIG-001",
		ConversationID:         "CONV-001",
		MessageToCustomer:      "OK",
		QueueName:              "biller-test",
	}

	err := validate.Struct(payload)
	assert.NotNil(t, err, "MsgId=0 should fail validation (required,min=1)")
}

// TestTopupPayload_Validation_Negative_EmptyQueueName — empty QueueName fails.
func TestTopupPayload_Validation_Negative_EmptyQueueName(t *testing.T) {
	validate := newTestValidator()

	payload := model.TopupDataPayload{
		MsgID:                  "1",
		StatusToBe:             "0",
		ClientNumber:           "08112233445",
		OriginalConversationID: "ORIG-001",
		ConversationID:         "CONV-001",
		MessageToCustomer:      "OK",
		QueueName:              "",
	}

	err := validate.Struct(payload)
	assert.NotNil(t, err, "Empty QueueName should fail validation")
}

// TestWebResponseSerialization tests WebResponse serialization.
func TestWebResponseSerialization(t *testing.T) {
	resp := model.WebResponse[string]{
		Data: "test",
	}
	assert.Equal(t, "test", resp.Data)
	assert.Empty(t, resp.Errors)
}

// TestHealthResponseFields tests HealthResponse fields.
func TestHealthResponseFields(t *testing.T) {
	resp := model.HealthResponse{
		Status: "healthy",
		Services: map[string]string{
			"oracle":   "healthy",
			"postgres": "healthy",
			"rabbitmq": "healthy",
		},
	}

	assert.Equal(t, "healthy", resp.Status)
	assert.Len(t, resp.Services, 3)
	assert.Equal(t, "healthy", resp.Services["oracle"])
}
