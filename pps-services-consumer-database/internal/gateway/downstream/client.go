package downstream

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/rs/zerolog"
	"github.com/sony/gobreaker"
	"github.com/spf13/viper"

	"pps-services-consumer-database/internal/model"
)

type DownstreamClient struct {
	httpClient *http.Client
	config     *viper.Viper
	log        zerolog.Logger
	failedCB   *gobreaker.CircuitBreaker // Circuit breaker untuk failed handler
}

func NewDownstreamClient(config *viper.Viper, log zerolog.Logger) *DownstreamClient {
	timeout := config.GetInt("downstream.publisher_database.timeout")
	if timeout <= 0 {
		timeout = 30
	}

	client := &DownstreamClient{
		httpClient: &http.Client{Timeout: time.Duration(timeout) * time.Second},
		config:     config,
		log:        log,
	}

	// Setup circuit breaker untuk failed handler
	client.failedCB = gobreaker.NewCircuitBreaker(gobreaker.Settings{
		Name:        "failed-handler-cb",
		MaxRequests: uint32(config.GetInt("circuit_breaker.downstream.max_requests")),
		Interval:    time.Duration(config.GetInt("circuit_breaker.downstream.interval")) * time.Second,
		Timeout:     time.Duration(config.GetInt("circuit_breaker.downstream.timeout")) * time.Second,
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			t := config.GetInt("circuit_breaker.downstream.failure_threshold")
			if t <= 0 {
				t = 3
			}
			return int(counts.ConsecutiveFailures) >= t
		},
		OnStateChange: func(name string, from gobreaker.State, to gobreaker.State) {
			log.Warn().Msgf("CB '%s': %s -> %s", name, from, to)
		},
	})

	return client
}

// SendOrderResult POSTs the combined order data to the downstream API.
func (c *DownstreamClient) SendOrderResult(ctx context.Context, req *model.DownstreamRequest) error {
	body, err := json.Marshal(req)
	if err != nil {
		return fmt.Errorf("marshal downstream request: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, c.config.GetString("downstream.url"), bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("downstream request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		return fmt.Errorf("downstream returned status %d", resp.StatusCode)
	}

	c.log.Debug().Msgf("Downstream call successful for msg_id %d", req.MsgID)
	return nil
}

// // ForwardToFailedHandler mengirim data transaksi gagal ke failed handler service.
// // Menggunakan circuit breaker + exponential backoff retry.
// func (c *DownstreamClient) ForwardToFailedHandler(ctx context.Context, req *model.FailedCallbackRequest) error {
// 	baseURL := c.config.GetString("downstream.failed_handler.base_url")
// 	apiKey := c.config.GetString("downstream.failed_handler.api_key")
// 	url := baseURL + "/api/transactions/failed-callbacks"

// 	_, err := c.failedCB.Execute(func() (any, error) {
// 		return nil, c.postWithRetry(ctx, url, apiKey, req, "failed_handler")
// 	})
// 	return err
// }

// // postWithRetry melakukan HTTP POST dengan exponential backoff retry.
// // configKey digunakan untuk mengambil konfigurasi retry dari Viper.
// func (c *DownstreamClient) postWithRetry(ctx context.Context, url, apiKey string, payload any, configKey string) error {
// 	body, err := json.Marshal(payload)
// 	if err != nil {
// 		return fmt.Errorf("marshal payload: %w", err)
// 	}

// 	// Konfigurasi exponential backoff dari config
// 	initialInterval := c.config.GetInt(fmt.Sprintf("downstream.%s.retry.initial_interval", configKey))
// 	maxInterval := c.config.GetInt(fmt.Sprintf("downstream.%s.retry.max_interval", configKey))
// 	maxElapsed := c.config.GetInt(fmt.Sprintf("downstream.%s.retry.max_elapsed_time", configKey))

// 	if initialInterval <= 0 {
// 		initialInterval = 1
// 	}
// 	if maxInterval <= 0 {
// 		maxInterval = 15
// 	}
// 	if maxElapsed <= 0 {
// 		maxElapsed = 60
// 	}

// 	bo := backoff.NewExponentialBackOff()
// 	bo.InitialInterval = time.Duration(initialInterval) * time.Second
// 	bo.MaxInterval = time.Duration(maxInterval) * time.Second

// 	operation := func() (struct{}, error) {
// 		postErr := c.doPost(ctx, url, apiKey, body)
// 		if postErr != nil {
// 			c.log.Warn().Err(postErr).Msgf("POST to %s failed, retrying...", url)
// 			return struct{}{}, postErr
// 		}
// 		return struct{}{}, nil
// 	}

// 	_, err = backoff.Retry(ctx, operation,
// 		backoff.WithBackOff(bo),
// 		backoff.WithMaxElapsedTime(time.Duration(maxElapsed)*time.Second),
// 	)
// 	if err != nil {
// 		c.log.Error().Err(err).Msgf("POST to %s gave up after retries", url)
// 		return fmt.Errorf("post to %s failed after retries: %w", url, err)
// 	}

// 	return nil
// }

// // doPost melakukan satu kali HTTP POST request.
// func (c *DownstreamClient) doPost(ctx context.Context, url, apiKey string, body []byte) error {
// 	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
// 	if err != nil {
// 		return fmt.Errorf("create request: %w", err)
// 	}

// 	req.Header.Set("Content-Type", "application/json")
// 	req.Header.Set("X-API-Key", apiKey)

// 	resp, err := c.httpClient.Do(req)
// 	if err != nil {
// 		return fmt.Errorf("http do: %w", err)
// 	}
// 	defer resp.Body.Close()

// 	// Baca response body untuk logging
// 	respBody, _ := io.ReadAll(resp.Body)

// 	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
// 		c.log.Debug().Msgf("POST %s -> %d: %s", url, resp.StatusCode, string(respBody))
// 		return nil
// 	}

// 	return fmt.Errorf("POST %s returned %d: %s", url, resp.StatusCode, string(respBody))
// }
