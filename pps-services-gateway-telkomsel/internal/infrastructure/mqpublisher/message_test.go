package mqpublisher

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestNewProviderPublishMessage(t *testing.T) {
	tests := []struct {
		name string
		data ProviderPublishData
	}{
		{
			name: "all fields populated",
			data: ProviderPublishData{
				MsgID:                  123,
				StatusToBe:             "SUCCESS",
				SerialNumber:           "SN-001",
				ClientNumber:           "628123456789",
				Nominal:                "10000",
				OriginalConversationID: "orig-conv-001",
				ConversationID:         "conv-001",
				MessageToCustomer:      "Transaction successful",
				AdditionalMessage:      "Thank you",
				QueueName:              "downstream-queue",
			},
		},
		{
			name: "minimal fields",
			data: ProviderPublishData{
				MsgID:      1,
				StatusToBe: "FAILED",
			},
		},
		{
			name: "zero value data",
			data: ProviderPublishData{},
		},
		{
			name: "large msg_id and long strings",
			data: ProviderPublishData{
				MsgID:                  999999,
				StatusToBe:             "PENDING",
				SerialNumber:           strings.Repeat("A", 50),
				ClientNumber:           "628999999999",
				Nominal:                "1000000",
				OriginalConversationID: "orig-conv-long-id-12345",
				ConversationID:         "conv-long-id-12345",
				MessageToCustomer:      "A longer message to the customer",
				AdditionalMessage:      "Extra info here",
				QueueName:              "another-queue",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewProviderPublishMessage(tt.data)

			if msg.Source != "PROVIDER" {
				t.Errorf("Source = %q, want %q", msg.Source, "PROVIDER")
			}
			if msg.Data != tt.data {
				t.Errorf("Data = %+v, want %+v", msg.Data, tt.data)
			}
		})
	}
}

func TestProviderPublishMessageJSONSerialization(t *testing.T) {
	tests := []struct {
		name         string
		data         ProviderPublishData
		wantKeys     []string
		wantContains []string
	}{
		{
			name: "all fields serialized with correct keys",
			data: ProviderPublishData{
				MsgID:                  42,
				StatusToBe:             "SUCCESS",
				SerialNumber:           "SN-100",
				ClientNumber:           "628111222333",
				Nominal:                "25000",
				OriginalConversationID: "orig-abc",
				ConversationID:         "conv-abc",
				MessageToCustomer:      "Done",
				AdditionalMessage:      "Info",
				QueueName:              "my-queue",
			},
			wantKeys: []string{
				`"source":"PROVIDER"`,
				`"msg_id":42`,
				`"status_to_be":"SUCCESS"`,
				`"serial_number":"SN-100"`,
				`"client_number":"628111222333"`,
				`"nominal":"25000"`,
				`"original_conversation_id":"orig-abc"`,
				`"conversation_id":"conv-abc"`,
				`"message_to_customer":"Done"`,
				`"additional_message":"Info"`,
				`"queue_name":"my-queue"`,
			},
		},
		{
			name: "zero value fields present in JSON",
			data: ProviderPublishData{},
			wantKeys: []string{
				`"source":"PROVIDER"`,
				`"msg_id":0`,
				`"status_to_be":""`,
				`"serial_number":""`,
				`"client_number":""`,
				`"nominal":""`,
				`"original_conversation_id":""`,
				`"conversation_id":""`,
				`"message_to_customer":""`,
				`"additional_message":""`,
				`"queue_name":""`,
			},
		},
		{
			name: "partial fields",
			data: ProviderPublishData{
				MsgID:        7,
				ClientNumber: "628555666777",
				Nominal:      "50000",
			},
			wantKeys: []string{
				`"source":"PROVIDER"`,
				`"msg_id":7`,
				`"client_number":"628555666777"`,
				`"nominal":"50000"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			msg := NewProviderPublishMessage(tt.data)
			b, err := json.Marshal(msg)
			if err != nil {
				t.Fatalf("json.Marshal failed: %v", err)
			}

			jsonStr := string(b)
			for _, key := range tt.wantKeys {
				if !strings.Contains(jsonStr, key) {
					t.Errorf("JSON output missing %s\ngot: %s", key, jsonStr)
				}
			}

			// Verify round-trip: unmarshal back and compare
			var decoded ProviderPublishMessage
			if err := json.Unmarshal(b, &decoded); err != nil {
				t.Fatalf("json.Unmarshal failed: %v", err)
			}
			if decoded.Source != "PROVIDER" {
				t.Errorf("decoded Source = %q, want %q", decoded.Source, "PROVIDER")
			}
			if decoded.Data != tt.data {
				t.Errorf("decoded Data = %+v, want %+v", decoded.Data, tt.data)
			}
		})
	}
}
