package model

import (
	"encoding/json"
	"math/rand"
	"reflect"
	"testing"
	"testing/quick"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// --- Sub-task 1.1: Unit tests for ProviderWrapperMessage ---

func TestProviderWrapperMessageJSONUnmarshal(t *testing.T) {
	// Requirements: 1.1, 1.2
	input := `{
		"source": "PROVIDER",
		"data": {
			"msg_id": 12345,
			"status_to_be": "SUCCESS",
			"serial_number": "SN123",
			"client_number": "08123456789",
			"nominal": "50000",
			"original_conversation_id": "ORIG001",
			"conversation_id": "CONV001",
			"message_to_customer": "Transaksi berhasil",
			"additional_message": "Pulsa telah dikirim",
			"queue_name": "telkomsel-queue"
		}
	}`

	var wrapper ProviderWrapperMessage
	err := json.Unmarshal([]byte(input), &wrapper)
	require.NoError(t, err)

	assert.Equal(t, "PROVIDER", wrapper.Source)
	assert.Equal(t, 12345, wrapper.Data.MsgId)
	assert.Equal(t, "SUCCESS", wrapper.Data.StatusToBe)
	assert.Equal(t, "SN123", wrapper.Data.SerialNumber)
	assert.Equal(t, "08123456789", wrapper.Data.ClientNumber)
	assert.Equal(t, "50000", wrapper.Data.Nominal)
	assert.Equal(t, "ORIG001", wrapper.Data.OriginalConversationID)
	assert.Equal(t, "CONV001", wrapper.Data.ConversationID)
	assert.Equal(t, "Transaksi berhasil", wrapper.Data.MessageToCustomer)
	assert.Equal(t, "Pulsa telah dikirim", wrapper.Data.AdditionalMessage)
	assert.Equal(t, "telkomsel-queue", wrapper.Data.QueueName)
}

func TestProviderWrapperMessageJSONMarshal(t *testing.T) {
	// Requirements: 1.1, 1.2
	wrapper := ProviderWrapperMessage{
		Source: "PROVIDER",
		Data: ProviderMessage{
			MsgId:                  99,
			StatusToBe:             "FAILED",
			SerialNumber:           "SN999",
			ClientNumber:           "08199999999",
			Nominal:                "100000",
			OriginalConversationID: "ORIG099",
			ConversationID:         "CONV099",
			MessageToCustomer:      "Gagal",
			AdditionalMessage:      "Coba lagi",
			QueueName:              "queue-test",
		},
	}

	data, err := json.Marshal(wrapper)
	require.NoError(t, err)

	var raw map[string]interface{}
	err = json.Unmarshal(data, &raw)
	require.NoError(t, err)

	assert.Equal(t, "PROVIDER", raw["source"])
	dataMap, ok := raw["data"].(map[string]interface{})
	require.True(t, ok, "data should be an object")
	assert.Equal(t, float64(99), dataMap["msg_id"])
	assert.Equal(t, "FAILED", dataMap["status_to_be"])
}

// --- Sub-task 1.2: Property test for round-trip serialization ---

// Feature: provider-message-wrapper-format, Property 1: Round-Trip Serialization ProviderWrapperMessage
// **Validates: Requirements 1.3**
func TestProviderWrapperMessageRoundTrip(t *testing.T) {
	f := func(
		msgId int, statusToBe, serialNumber, clientNumber, nominal string,
		origConvID, convID, msgToCustomer, additionalMsg, queueName, source string,
	) bool {
		original := ProviderWrapperMessage{
			Source: source,
			Data: ProviderMessage{
				MsgId:                  msgId,
				StatusToBe:             statusToBe,
				SerialNumber:           serialNumber,
				ClientNumber:           clientNumber,
				Nominal:                nominal,
				OriginalConversationID: origConvID,
				ConversationID:         convID,
				MessageToCustomer:      msgToCustomer,
				AdditionalMessage:      additionalMsg,
				QueueName:              queueName,
			},
		}

		data, err := json.Marshal(original)
		if err != nil {
			return false
		}

		var decoded ProviderWrapperMessage
		err = json.Unmarshal(data, &decoded)
		if err != nil {
			return false
		}

		return reflect.DeepEqual(original, decoded)
	}

	cfg := &quick.Config{
		MaxCount: 100,
		Rand:     rand.New(rand.NewSource(42)),
	}
	if err := quick.Check(f, cfg); err != nil {
		t.Errorf("Round-trip property failed: %v", err)
	}
}
