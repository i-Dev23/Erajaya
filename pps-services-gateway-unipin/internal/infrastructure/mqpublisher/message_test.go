package mqpublisher

import (
	"encoding/json"
	"testing"
)

func TestNewProviderPublishMessage_SourceIsProvider(t *testing.T) {
	msg := NewProviderPublishMessage(ProviderPublishData{MsgID: 1})
	if msg.Source != "PROVIDER" {
		t.Errorf("Source: got %q, want PROVIDER", msg.Source)
	}
}

func TestNewProviderPublishMessage_DataPreserved(t *testing.T) {
	data := ProviderPublishData{
		MsgID:             123,
		StatusToBe:        "SUCCESS",
		SerialNumber:      "SN-001",
		ClientNumber:      "08123",
		Nominal:           "50000",
		ConversationID:    "CONV-001",
		MessageToCustomer: "completed",
		QueueName:         "test-queue",
	}
	msg := NewProviderPublishMessage(data)
	if msg.Data.MsgID != 123 {
		t.Errorf("MsgID: got %d", msg.Data.MsgID)
	}
	if msg.Data.StatusToBe != "SUCCESS" {
		t.Errorf("StatusToBe: got %q", msg.Data.StatusToBe)
	}
	if msg.Data.SerialNumber != "SN-001" {
		t.Errorf("SerialNumber: got %q", msg.Data.SerialNumber)
	}
	if msg.Data.QueueName != "test-queue" {
		t.Errorf("QueueName: got %q", msg.Data.QueueName)
	}
}

func TestProviderPublishMessage_JSONSerialization(t *testing.T) {
	msg := NewProviderPublishMessage(ProviderPublishData{
		MsgID:      42,
		StatusToBe: "FAILED",
		QueueName:  "q",
	})
	b, err := json.Marshal(msg)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var parsed map[string]any
	if err := json.Unmarshal(b, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if parsed["source"] != "PROVIDER" {
		t.Errorf("source: got %v", parsed["source"])
	}
	data, ok := parsed["data"].(map[string]any)
	if !ok {
		t.Fatal("missing data field")
	}
	if data["status_to_be"] != "FAILED" {
		t.Errorf("status_to_be: got %v", data["status_to_be"])
	}
	if data["msg_id"] != float64(42) {
		t.Errorf("msg_id: got %v", data["msg_id"])
	}
}

func TestProviderPublishData_EmptyFields(t *testing.T) {
	msg := NewProviderPublishMessage(ProviderPublishData{})
	if msg.Source != "PROVIDER" {
		t.Errorf("Source: got %q", msg.Source)
	}
	if msg.Data.MsgID != 0 {
		t.Errorf("MsgID should be 0, got %d", msg.Data.MsgID)
	}
}
