package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"time"

	"pps-services-tokopedia/internal/domain"
)

type ultimaService struct {
	httpClient *http.Client
	baseURL    string
	logger     Logger
}

// NewUltimaService creates a new UltimaService instance
func NewUltimaService(logger Logger) domain.UltimaService {
	baseURL := os.Getenv("ULTIMA_GATEWAY_BASE_URL")
	if baseURL == "" {
		baseURL = "http://paymentservices-evs.local:9449/h2h-ultima/api/v1"
	}

	return &ultimaService{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		baseURL: baseURL,
		logger:  logger,
	}
}

// CheckIdPlnUltima calls the Ultima API to check PLN customer ID
func (u *ultimaService) CheckIdPlnUltima(ctx context.Context, req *domain.UltimaCheckIdPlnRequestDomain) (*domain.UltimaBaseResponseDomain, *domain.PLNTransactionInquiry, error) {
	u.logger.Info("Starting CheckIdPlnUltima request for idPel: %s, idTrx: %s", req.IdPel, req.IdTrx)

	// Prepare request body
	reqBody, err := json.Marshal(req)
	if err != nil {
		u.logger.Error("Failed to marshal request body: %v", err)
		return nil, nil, fmt.Errorf("failed to marshal request body: %w", err)
	}

	// Create HTTP request
	url := fmt.Sprintf("%s/checkIdPlnUltima", u.baseURL)
	httpReq, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(reqBody))
	if err != nil {
		u.logger.Error("Failed to create HTTP request: %v", err)
		return nil, nil, fmt.Errorf("failed to create HTTP request: %w", err)
	}

	// Set headers
	httpReq.Header.Set("Content-Type", "application/json")

	// Log request details
	u.logger.Info("Making request to: %s", url)
	u.logger.Debug("Request body: %s", string(reqBody))

	// Make HTTP request
	resp, err := u.httpClient.Do(httpReq)
	if err != nil {
		u.logger.Error("HTTP request failed: %v", err)
		return nil, nil, fmt.Errorf("HTTP request failed: %w", err)
	}
	defer resp.Body.Close()

	// Read response body
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		u.logger.Error("Failed to read response body: %v", err)
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	u.logger.Info("Received response with status: %d", resp.StatusCode)
	u.logger.Debug("Response body: %s", string(respBody))

	// Parse response
	var ultimaResp domain.UltimaBaseResponseDomain
	if err := json.Unmarshal(respBody, &ultimaResp); err != nil {
		u.logger.Error("Failed to unmarshal response: %v", err)
		return nil, nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// Check HTTP status code
	if resp.StatusCode != http.StatusOK {
		u.logger.Error("API returned non-200 status: %d, response: %s", resp.StatusCode, string(respBody))
		return &ultimaResp, nil, fmt.Errorf("API returned status %d: %v", resp.StatusCode, ultimaResp.HttpResponseBody)
	}

	parsedData := ParsePLNTransaction(ultimaResp.HttpResponseBody.Msg)

	u.logger.Info("CheckIdPlnUltima completed successfully for idPel: %s", req.IdPel)
	return &ultimaResp, parsedData, nil
}

// Ping checks if the Ultima service is reachable
func (u *ultimaService) Ping(ctx context.Context) error {
	// Parse baseURL to extract host
	parsedURL, err := url.Parse(u.baseURL)
	if err != nil {
		return fmt.Errorf("failed to parse baseURL: %w", err)
	}

	// Extract hostname and port
	host := parsedURL.Host

	// Set timeout for ping request
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	// Use TCP dial to check if host is reachable
	dialer := net.Dialer{}
	conn, err := dialer.DialContext(pingCtx, "tcp", host)
	if err != nil {
		return fmt.Errorf("Ultima service unreachable: %w", err)
	}
	defer conn.Close()

	return nil
}

func ParsePLNTransaction(input string) *domain.PLNTransactionInquiry {
	result := domain.PLNTransactionInquiry{}

	// Map regex untuk tiap field
	patterns := map[string]*regexp.Regexp{
		"meter": regexp.MustCompile(`NO_METER:([0-9]+)`),
		"idpel": regexp.MustCompile(`IDPEL:([0-9]+)`),
		"nama":  regexp.MustCompile(`NAMA:([^@]+)`),
		"tarif": regexp.MustCompile(`TARIF_DAYA:([^@]+)`),
	}

	if match := patterns["meter"].FindStringSubmatch(input); len(match) > 1 {
		result.MeterNumber = match[1]
	}
	if match := patterns["idpel"].FindStringSubmatch(input); len(match) > 1 {
		result.IDPelanggan = match[1]
	}
	if match := patterns["nama"].FindStringSubmatch(input); len(match) > 1 {
		result.Name = match[1]
	}
	if match := patterns["tarif"].FindStringSubmatch(input); len(match) > 1 {
		result.TarifDaya = match[1]
	}

	return &result
}
