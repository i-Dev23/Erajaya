package repository

import (
	"encoding/json"
	"pps-services-consumer/model"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestExtractSourceValidJSON tests extracting source from valid JSON.
func TestExtractSourceValidJSON(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
		wantErr  bool
	}{
		{
			name:     "PRE-ORDER source",
			input:    `{"source":"PRE-ORDER","data":{}}`,
			expected: "PRE-ORDER",
			wantErr:  false,
		},
		{
			name:     "PROVIDER source",
			input:    `{"source":"PROVIDER","data":{}}`,
			expected: "PROVIDER",
			wantErr:  false,
		},
		{
			name:     "lowercase source normalized to uppercase",
			input:    `{"source":"pre-order"}`,
			expected: "PRE-ORDER",
			wantErr:  false,
		},
		{
			name:     "source with whitespace trimmed",
			input:    `{"source":" PROVIDER "}`,
			expected: "PROVIDER",
			wantErr:  false,
		},
		{
			name:     "empty source",
			input:    `{"source":""}`,
			expected: "",
			wantErr:  false,
		},
		{
			name:     "invalid JSON",
			input:    `{invalid json}`,
			expected: "",
			wantErr:  true,
		},
		{
			name:     "missing source field",
			input:    `{"data":{}}`,
			expected: "",
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source, err := extractSource(tt.input)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, tt.expected, source)
			}
		})
	}
}

// TestProsesDataFIFOSourceRouting tests that source routing selects the correct path.
// Property 1: Source routing memilih path pemrosesan yang benar
func TestProsesDataFIFOSourceRouting(t *testing.T) {
	tests := []struct {
		name           string
		data           string
		expectedResult sourceResult
		description    string
	}{
		{
			name:           "PRE-ORDER routes to processPreOrder",
			data:           `{"source":"PRE-ORDER","data":{"msgid":123}}`,
			expectedResult: resultOK,
			description:    "PRE-ORDER source should route to processPreOrder path",
		},
		{
			name:           "PREORDER routes to processPreOrder (alias)",
			data:           `{"source":"PREORDER","data":{"msgid":123}}`,
			expectedResult: resultOK,
			description:    "PREORDER source should be treated as PRE-ORDER for backward compatibility",
		},
		{
			name:           "PROVIDER with invalid ProviderMessage JSON routes to resultNackDiscard",
			data:           `{"source":"PROVIDER","msg_id":"invalid"}`,
			expectedResult: resultNackDiscard,
			description:    "PROVIDER with invalid ProviderMessage JSON should be discarded",
		},
		{
			name:           "Unknown source falls back to legacy",
			data:           `{"source":"UNKNOWN","data":{}}`,
			expectedResult: resultOK,
			description:    "Unknown source should fallback to legacy flow",
		},
		{
			name:           "Empty source falls back to legacy",
			data:           `{"source":"","data":{}}`,
			expectedResult: resultOK,
			description:    "Empty source should fallback to legacy flow",
		},
		{
			name:           "Invalid JSON falls back to legacy",
			data:           `{invalid json}`,
			expectedResult: resultOK,
			description:    "Invalid JSON should fallback to legacy flow",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := ProsesDataFIFO(tt.data, "amqp://localhost")
			assert.Equal(t, tt.expectedResult, result, tt.description)
		})
	}
}

// TestProviderMessageJSONRoundTrip tests JSON serialization/deserialization of ProviderMessage.
// Property 5: Round-trip serialisasi ProviderMessage
func TestProviderMessageJSONRoundTrip(t *testing.T) {
	original := model.ProviderMessage{
		MsgId:                  12345,
		StatusToBe:             "SUCCESS",
		SerialNumber:           "SN123",
		ClientNumber:           "08123456789",
		Nominal:                "50000",
		OriginalConversationID: "CONV001",
		ConversationID:         "CONV002",
		MessageToCustomer:      "Transaksi berhasil",
		AdditionalMessage:      "Pulsa telah dikirim",
		QueueName:              "telkomsel-queue",
		Source:                 "PROVIDER",
	}

	// Marshal to JSON
	jsonBytes, err := json.Marshal(original)
	require.NoError(t, err)

	// Unmarshal back
	var restored model.ProviderMessage
	err = json.Unmarshal(jsonBytes, &restored)
	require.NoError(t, err)

	// Verify all fields match
	assert.Equal(t, original.MsgId, restored.MsgId)
	assert.Equal(t, original.StatusToBe, restored.StatusToBe)
	assert.Equal(t, original.SerialNumber, restored.SerialNumber)
	assert.Equal(t, original.ClientNumber, restored.ClientNumber)
	assert.Equal(t, original.Nominal, restored.Nominal)
	assert.Equal(t, original.OriginalConversationID, restored.OriginalConversationID)
	assert.Equal(t, original.ConversationID, restored.ConversationID)
	assert.Equal(t, original.MessageToCustomer, restored.MessageToCustomer)
	assert.Equal(t, original.AdditionalMessage, restored.AdditionalMessage)
	assert.Equal(t, original.QueueName, restored.QueueName)
	assert.Equal(t, original.Source, restored.Source)
}

// TestProviderMessageJSONTags verifies that JSON tags match expected field names.
func TestProviderMessageJSONTags(t *testing.T) {
	msg := model.ProviderMessage{
		MsgId:                  999,
		StatusToBe:             "FAILED",
		SerialNumber:           "SN999",
		ClientNumber:           "08999999999",
		Nominal:                "100000",
		OriginalConversationID: "ORIG999",
		ConversationID:         "CONV999",
		MessageToCustomer:      "Gagal",
		AdditionalMessage:      "Silakan coba lagi",
		QueueName:              "unipin-queue",
		Source:                 "PROVIDER",
	}

	jsonBytes, err := json.Marshal(msg)
	require.NoError(t, err)

	var data map[string]interface{}
	err = json.Unmarshal(jsonBytes, &data)
	require.NoError(t, err)

	// Verify JSON field names match the tags
	assert.Equal(t, float64(999), data["msg_id"])
	assert.Equal(t, "FAILED", data["status_to_be"])
	assert.Equal(t, "SN999", data["serial_number"])
	assert.Equal(t, "08999999999", data["client_number"])
	assert.Equal(t, "100000", data["nominal"])
	assert.Equal(t, "ORIG999", data["original_conversation_id"])
	assert.Equal(t, "CONV999", data["conversation_id"])
	assert.Equal(t, "Gagal", data["message_to_customer"])
	assert.Equal(t, "Silakan coba lagi", data["additional_message"])
	assert.Equal(t, "unipin-queue", data["queue_name"])
	assert.Equal(t, "PROVIDER", data["source"])
}

// TestPublishProviderRequestJSONTags verifies JSON tags for PublishProviderRequest.
// Property 2: Mapping PRE-ORDER ke PublishRequest mempertahankan semua field
func TestPublishProviderRequestJSONTags(t *testing.T) {
	req := model.PublishProviderRequest{
		MsgID:        "MSG123",
		ClientNumber: "08123456789",
		IMSI:         "310410123456789",
		RemarkIMSI:   "IMSI_REMARK",
		MID:          "MID123",
		StoreID:      "EAR-M087",
		QueueName:    "telkomsel-queue",
		TypeVoucher:  "PULSA",
		VoucherCode:  "PROD001",
		Command:      "SELL",
		Provider:     "TELKOMSEL",
		QTransaction: "amqp://publisher-mq",
	}

	jsonBytes, err := json.Marshal(req)
	require.NoError(t, err)

	var data map[string]interface{}
	err = json.Unmarshal(jsonBytes, &data)
	require.NoError(t, err)

	// Verify all fields are present in JSON
	assert.Equal(t, "MSG123", data["msgid"])
	assert.Equal(t, "08123456789", data["clientNumber"])
	assert.Equal(t, "310410123456789", data["imsi"])
	assert.Equal(t, "IMSI_REMARK", data["remarkImsi"])
	assert.Equal(t, "MID123", data["mid"])
	assert.Equal(t, "EAR-M087", data["storeId"])
	assert.Equal(t, "telkomsel-queue", data["queueName"])
	assert.Equal(t, "PULSA", data["typeVoucher"])
	assert.Equal(t, "PROD001", data["voucherCode"])
	assert.Equal(t, "SELL", data["command"])
	assert.Equal(t, "TELKOMSEL", data["provider"])
	assert.Equal(t, "amqp://publisher-mq", data["MQTransaction"])
}

// TestPublishProviderRequestRoundTrip tests JSON round-trip for PublishProviderRequest.
func TestPublishProviderRequestRoundTrip(t *testing.T) {
	original := model.PublishProviderRequest{
		MsgID:        "MSG456",
		ClientNumber: "08987654321",
		IMSI:         "310410987654321",
		RemarkIMSI:   "REMARK456",
		MID:          "MID456",
		StoreID:      "2002",
		QueueName:    "unipin-queue",
		TypeVoucher:  "VOUCHER",
		VoucherCode:  "PROD002",
		Command:      "SELL2",
		Provider:     "UNIPIN",
		QTransaction: "amqp://publisher-mq-2",
	}

	jsonBytes, err := json.Marshal(original)
	require.NoError(t, err)

	var restored model.PublishProviderRequest
	err = json.Unmarshal(jsonBytes, &restored)
	require.NoError(t, err)

	assert.Equal(t, original.MsgID, restored.MsgID)
	assert.Equal(t, original.ClientNumber, restored.ClientNumber)
	assert.Equal(t, original.IMSI, restored.IMSI)
	assert.Equal(t, original.RemarkIMSI, restored.RemarkIMSI)
	assert.Equal(t, original.MID, restored.MID)
	assert.Equal(t, original.StoreID, restored.StoreID)
	assert.Equal(t, original.QueueName, restored.QueueName)
	assert.Equal(t, original.TypeVoucher, restored.TypeVoucher)
	assert.Equal(t, original.VoucherCode, restored.VoucherCode)
	assert.Equal(t, original.Command, restored.Command)
	assert.Equal(t, original.Provider, restored.Provider)
	assert.Equal(t, original.QTransaction, restored.QTransaction)
}
