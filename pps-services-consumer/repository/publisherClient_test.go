package repository

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"pps-services-consumer/constanta"
	"pps-services-consumer/model"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestCallPublisherProviderSuccess tests successful HTTP call to publisher-provider.
func TestCallPublisherProviderSuccess(t *testing.T) {
	// Create a test server that returns 200 OK
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		assert.Equal(t, http.MethodPost, r.Method)
		assert.Equal(t, "/api/v1/publish", r.URL.Path)
		assert.Equal(t, "application/json", r.Header.Get("Content-Type"))

		var req model.PublishProviderRequest
		err := json.NewDecoder(r.Body).Decode(&req)
		require.NoError(t, err)

		assert.Equal(t, "MSG123", req.MsgID)
		assert.Equal(t, "08123456789", req.ClientNumber)

		w.WriteHeader(http.StatusOK)
		w.Write([]byte(`{"status":"ok"}`))
	}))
	defer server.Close()

	// Set env var to point to test server
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_BASE_URL, server.URL)
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_TIMEOUT, "5")
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_MAX_RETRIES, "1")

	req := model.PublishProviderRequest{
		MsgID:        "MSG123",
		ClientNumber: "08123456789",
		IMSI:         "310410123456789",
		RemarkIMSI:   "REMARK",
		MID:          "MID123",
		StoreID:      "1001",
		QueueName:    "queue1",
		TypeVoucher:  "PULSA",
		VoucherCode:  "PROD001",
		Command:      "SELL",
		Provider:     "TELKOMSEL",
		QTransaction: "amqp://mq",
	}

	err := CallPublisherProvider(req)
	assert.NoError(t, err)
}

// TestCallPublisherProviderMissingBaseURL tests error when base URL is not set.
func TestCallPublisherProviderMissingBaseURL(t *testing.T) {
	os.Unsetenv(constanta.OS_ENV_PUBLISHER_PROVIDER_BASE_URL)

	req := model.PublishProviderRequest{
		MsgID:        "MSG123",
		ClientNumber: "08123456789",
	}

	err := CallPublisherProvider(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "PUBLISHER_PROVIDER_BASE_URL")
}

// TestCallPublisherProviderRetry tests exponential backoff retry on failure.
func TestCallPublisherProviderRetry(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		if attemptCount < 3 {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("error"))
		} else {
			w.WriteHeader(http.StatusOK)
			w.Write([]byte(`{"status":"ok"}`))
		}
	}))
	defer server.Close()

	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_BASE_URL, server.URL)
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_TIMEOUT, "5")
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_MAX_RETRIES, "3")

	req := model.PublishProviderRequest{
		MsgID:        "MSG123",
		ClientNumber: "08123456789",
	}

	err := CallPublisherProvider(req)
	assert.NoError(t, err)
	assert.Equal(t, 3, attemptCount)
}

// TestCallPublisherProviderMaxRetriesExceeded tests failure after max retries.
func TestCallPublisherProviderMaxRetriesExceeded(t *testing.T) {
	attemptCount := 0
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attemptCount++
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("persistent error"))
	}))
	defer server.Close()

	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_BASE_URL, server.URL)
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_TIMEOUT, "1")
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_MAX_RETRIES, "2")

	req := model.PublishProviderRequest{
		MsgID:        "MSG123",
		ClientNumber: "08123456789",
	}

	err := CallPublisherProvider(req)
	assert.Error(t, err)
	assert.Equal(t, 2, attemptCount)
}

// TestCallPublisherProviderNon2xxStatus tests error on non-2xx response.
func TestCallPublisherProviderNon2xxStatus(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("invalid request"))
	}))
	defer server.Close()

	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_BASE_URL, server.URL)
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_TIMEOUT, "5")
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_MAX_RETRIES, "1")

	req := model.PublishProviderRequest{
		MsgID:        "MSG123",
		ClientNumber: "08123456789",
	}

	err := CallPublisherProvider(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "400")
}

// TestCallPublisherProviderTimeout tests timeout handling.
func TestCallPublisherProviderTimeout(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(3 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_BASE_URL, server.URL)
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_TIMEOUT, "1") // 1 second timeout
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_MAX_RETRIES, "1")

	// Reinitialize httpClient with new timeout
	initHTTPClient()

	req := model.PublishProviderRequest{
		MsgID:        "MSG123",
		ClientNumber: "08123456789",
	}

	err := CallPublisherProvider(req)
	assert.Error(t, err)
}

// TestGetMaxRetriesDefault tests default max retries value.
func TestGetMaxRetriesDefault(t *testing.T) {
	os.Unsetenv(constanta.OS_ENV_PUBLISHER_PROVIDER_MAX_RETRIES)
	retries := getMaxRetries()
	assert.Equal(t, 3, retries)
}

// TestGetMaxRetriesFromEnv tests reading max retries from env var.
func TestGetMaxRetriesFromEnv(t *testing.T) {
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_MAX_RETRIES, "5")
	retries := getMaxRetries()
	assert.Equal(t, 5, retries)
}

// TestGetMaxRetriesInvalidValue tests invalid max retries value defaults to 3.
func TestGetMaxRetriesInvalidValue(t *testing.T) {
	os.Setenv(constanta.OS_ENV_PUBLISHER_PROVIDER_MAX_RETRIES, "invalid")
	retries := getMaxRetries()
	assert.Equal(t, 3, retries)
}

// TestPublishProviderRequestMarshal tests marshaling PublishProviderRequest to JSON.
func TestPublishProviderRequestMarshal(t *testing.T) {
	req := model.PublishProviderRequest{
		MsgID:        "MSG789",
		ClientNumber: "08555555555",
		IMSI:         "310410555555555",
		RemarkIMSI:   "REMARK789",
		MID:          "MID789",
		StoreID:      "3003",
		QueueName:    "queue3",
		TypeVoucher:  "DATA",
		VoucherCode:  "PROD003",
		Command:      "SELL3",
		Provider:     "UNIPIN",
		QTransaction: "amqp://mq3",
	}

	jsonBytes, err := json.Marshal(req)
	require.NoError(t, err)

	var data map[string]interface{}
	err = json.Unmarshal(jsonBytes, &data)
	require.NoError(t, err)

	// Verify all fields are present
	assert.Equal(t, "MSG789", data["msgid"])
	assert.Equal(t, "08555555555", data["clientNumber"])
	assert.Equal(t, "310410555555555", data["imsi"])
	assert.Equal(t, "REMARK789", data["remarkImsi"])
	assert.Equal(t, "MID789", data["mid"])
	assert.Equal(t, "3003", data["storeId"])
	assert.Equal(t, "queue3", data["queueName"])
	assert.Equal(t, "DATA", data["typeVoucher"])
	assert.Equal(t, "PROD003", data["voucherCode"])
	assert.Equal(t, "SELL3", data["command"])
	assert.Equal(t, "UNIPIN", data["provider"])
	assert.Equal(t, "amqp://mq3", data["MQTransaction"])
}
