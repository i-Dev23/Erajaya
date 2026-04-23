package repository

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"pps-services-consumer/constanta"
	"pps-services-consumer/model"
	"pps-services-consumer/util"
	log "pps-services-consumer/util"
	"strconv"
	"time"
)

var httpClient = &http.Client{
	Transport: &http.Transport{
		MaxIdleConns:        100,
		MaxIdleConnsPerHost: 50,
		IdleConnTimeout:     90 * time.Second,
	},
	Timeout: 30 * time.Second,
}

// initHTTPClient mengatur timeout dari env var jika tersedia.
func initHTTPClient() {
	timeoutStr := os.Getenv(constanta.OS_ENV_PUBLISHER_PROVIDER_TIMEOUT)
	if timeoutStr != "" {
		if t, err := strconv.Atoi(timeoutStr); err == nil && t > 0 {
			httpClient.Timeout = time.Duration(t) * time.Second
		}
	}
}

func init() {
	initHTTPClient()
}

// getMaxRetries membaca max retries dari env var, default 3.
func getMaxRetries() int {
	s := os.Getenv(constanta.OS_ENV_PUBLISHER_PROVIDER_MAX_RETRIES)
	if s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			return n
		}
	}
	return 3
}

// CallPublisherProvider mengirim HTTP POST ke pps-services-publisher-provider /api/v1/publish.
// Menggunakan exponential backoff retry (initial 1s, max 10s, max N attempts dari env).
func CallPublisherProvider(req model.PublishProviderRequest) error {
	baseURL := os.Getenv(constanta.OS_ENV_PUBLISHER_PROVIDER_BASE_URL)
	if baseURL == "" {
		return fmt.Errorf("PUBLISHER_PROVIDER_BASE_URL is not set")
	}

	url := baseURL + "/api/v1/publish"
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal PublishProviderRequest: %w", err)
	}

	maxRetries := getMaxRetries()
	backoff := time.Second // initial 1s

	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		lastErr = doPostPublisher(url, body)
		if lastErr == nil {
			log.Printf("CallPublisherProvider success, msgid: %s, attempt: %d", req.MsgID, attempt)
			return nil
		}

		log.Printf("CallPublisherProvider attempt %d/%d failed, msgid: %s, error: %s",
			attempt, maxRetries, req.MsgID, lastErr.Error())

		if attempt < maxRetries {
			time.Sleep(backoff)
			backoff = backoff * 2
			if backoff > 10*time.Second {
				backoff = 10 * time.Second
			}
		}
	}

	errMsg := fmt.Sprintf("CallPublisherProvider failed after %d retries, msgid: %s, error: %s",
		maxRetries, req.MsgID, lastErr.Error())
	util.ComposeMessageTelegramNotification(errMsg)
	log.Println(errMsg)
	return lastErr
}

// doPostPublisher melakukan satu kali HTTP POST request ke publisher-provider.
func doPostPublisher(url string, body []byte) error {
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("http do: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}

	return fmt.Errorf("POST %s returned %d: %s", url, resp.StatusCode, string(respBody))
}
