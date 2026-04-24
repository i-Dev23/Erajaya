package mqpublisher

import (
	"encoding/json"
	"testing"
)

func TestNewProviderPublishMessage(t *testing.T) {
	data := ProviderPublishData{
		MsgID:             12345,
		StatusToBe:        "F",
		SerialNumber:      "TOKEN-123",
		ClientNumber:      "12345678901",
		Nominal:           "52500",
		ConversationID:    "SMB-MID-123-20260422",
		MessageToCustomer: "Token PLN: TOKEN-123",
		QueueName:         "queue_smb",
	}

	msg := NewProviderPublishMessage(data)

	if msg.Source != "PROVIDER" {
		t.Errorf("expected source PROVIDER, got %s", msg.Source)
	}
	if msg.Data.MsgID != 12345 {
		t.Errorf("expected msg_id 12345, got %d", msg.Data.MsgID)
	}
	if msg.Data.StatusToBe != "F" {
		t.Errorf("expected F, got %s", msg.Data.StatusToBe)
	}
	if msg.Data.SerialNumber != "TOKEN-123" {
		t.Errorf("expected TOKEN-123, got %s", msg.Data.SerialNumber)
	}
}

func TestProviderPublishMessage_JSONFormat(t *testing.T) {
	msg := NewProviderPublishMessage(ProviderPublishData{
		MsgID:             99,
		StatusToBe:        "C",
		ClientNumber:      "999",
		Nominal:           "50000",
		MessageToCustomer: "Gagal",
		QueueName:         "q1",
	})

	body, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal error: %v", err)
	}

	// Verify it can be unmarshalled back
	var parsed ProviderPublishMessage
	if err := json.Unmarshal(body, &parsed); err != nil {
		t.Fatalf("unmarshal error: %v", err)
	}

	if parsed.Source != "PROVIDER" {
		t.Errorf("expected PROVIDER, got %s", parsed.Source)
	}
	if parsed.Data.MsgID != 99 {
		t.Errorf("expected 99, got %d", parsed.Data.MsgID)
	}
	if parsed.Data.StatusToBe != "C" {
		t.Errorf("expected C, got %s", parsed.Data.StatusToBe)
	}

	// Verify JSON has correct field names
	var raw map[string]json.RawMessage
	json.Unmarshal(body, &raw)

	if _, ok := raw["source"]; !ok {
		t.Error("JSON missing 'source' field")
	}
	if _, ok := raw["data"]; !ok {
		t.Error("JSON missing 'data' field")
	}
}

func TestProviderPublishData_JSONFieldNames(t *testing.T) {
	data := ProviderPublishData{
		MsgID:                  1,
		StatusToBe:             "F",
		SerialNumber:           "SN",
		ClientNumber:           "CN",
		Nominal:                "100",
		OriginalConversationID: "OCI",
		ConversationID:         "CI",
		MessageToCustomer:      "MSG",
		AdditionalMessage:      "ADD",
		QueueName:              "QN",
	}

	body, _ := json.Marshal(data)
	var raw map[string]json.RawMessage
	json.Unmarshal(body, &raw)

	expectedFields := []string{
		"msg_id", "status_to_be", "serial_number", "client_number",
		"nominal", "original_conversation_id", "conversation_id",
		"message_to_customer", "additional_message", "queue_name",
	}

	for _, field := range expectedFields {
		if _, ok := raw[field]; !ok {
			t.Errorf("JSON missing field: %s", field)
		}
	}
}
