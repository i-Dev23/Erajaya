package telkomsel

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"
)

// ---------------------------------------------------------------------------
// NewClient
// ---------------------------------------------------------------------------

func TestNewClient_ValidParams(t *testing.T) {
	c, err := NewClient("https://api.example.com", "ch1", "secret", "apikey", 5*time.Second, slog.Default())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil Client")
	}
}

func TestNewClient_EmptyRequiredParams(t *testing.T) {
	tests := []struct {
		name      string
		baseURL   string
		channelID string
		secretKey string
		apiKey    string
		timeout   time.Duration
		wantErr   string
	}{
		{name: "empty baseURL", baseURL: "", channelID: "ch", secretKey: "sk", apiKey: "ak", timeout: 5 * time.Second, wantErr: "baseURL is required"},
		{name: "whitespace baseURL", baseURL: "   ", channelID: "ch", secretKey: "sk", apiKey: "ak", timeout: 5 * time.Second, wantErr: "baseURL is required"},
		{name: "empty channelID", baseURL: "https://api.example.com", channelID: "", secretKey: "sk", apiKey: "ak", timeout: 5 * time.Second, wantErr: "channelID is required"},
		{name: "empty secretKey", baseURL: "https://api.example.com", channelID: "ch", secretKey: "", apiKey: "ak", timeout: 5 * time.Second, wantErr: "secretKey is required"},
		{name: "empty apiKey", baseURL: "https://api.example.com", channelID: "ch", secretKey: "sk", apiKey: "", timeout: 5 * time.Second, wantErr: "apiKey is required"},
		{name: "zero timeout", baseURL: "https://api.example.com", channelID: "ch", secretKey: "sk", apiKey: "ak", timeout: 0, wantErr: "timeout must be greater than zero"},
		{name: "negative timeout", baseURL: "https://api.example.com", channelID: "ch", secretKey: "sk", apiKey: "ak", timeout: -1 * time.Second, wantErr: "timeout must be greater than zero"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			c, err := NewClient(tc.baseURL, tc.channelID, tc.secretKey, tc.apiKey, tc.timeout, nil)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if c != nil {
				t.Fatal("expected nil Client on error")
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

func TestNewClient_NilLoggerUsesDefault(t *testing.T) {
	c, err := NewClient("https://api.example.com", "ch", "sk", "ak", 5*time.Second, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c == nil {
		t.Fatal("expected non-nil Client")
	}
	if c.logger == nil {
		t.Fatal("expected non-nil logger (should default)")
	}
}

func TestNewClient_TrimsTrailingSlash(t *testing.T) {
	c, err := NewClient("https://api.example.com/", "ch", "sk", "ak", 5*time.Second, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if c.baseURL != "https://api.example.com" {
		t.Fatalf("baseURL = %q, want trailing slash trimmed", c.baseURL)
	}
}

// ---------------------------------------------------------------------------
// validateInitiateRegularRechargeRequest
// ---------------------------------------------------------------------------

func validInitiateRegularRechargeRequest() InitiateRegularRechargeRequest {
	return InitiateRegularRechargeRequest{
		Transaction: InitiateRegularRechargeTransaction{
			TransactionID: "TX00000000000000000000001",
			Channel:       "tp",
		},
		Service: InitiateRegularRechargeService{
			OrganizationCode: "705009",
			ServiceID:        "6281234567890",
		},
		Recharge: InitiateRegularRechargeRecharge{
			Amount:    10000,
			StockType: StockTypeFixed,
			Element1:  "encrypted-element1",
		},
		MerchantProfile: InitiateRegularRechargeMerchantProfile{
			ThirdPartyID:       "NGRS",
			ThirdPartyPassword: "pass123",
			DeliveryChannel:    "SMS",
		},
	}
}

func TestValidateInitiateRegularRechargeRequest_Valid(t *testing.T) {
	err := validateInitiateRegularRechargeRequest(validInitiateRegularRechargeRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateInitiateRegularRechargeRequest_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*InitiateRegularRechargeRequest)
		wantErr string
	}{
		{
			name:    "empty transaction_id",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.Transaction.TransactionID = "" },
			wantErr: "transaction.transaction_id is required",
		},
		{
			name:    "transaction_id > 25 chars",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.Transaction.TransactionID = strings.Repeat("A", 26) },
			wantErr: "transaction.transaction_id must be at most 25 characters",
		},
		{
			name:    "empty channel",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.Transaction.Channel = "" },
			wantErr: "transaction.channel is required",
		},
		{
			name:    "empty organization_code",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.Service.OrganizationCode = "" },
			wantErr: "service.organization_code is required",
		},
		{
			name:    "empty service_id",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.Service.ServiceID = "" },
			wantErr: "service.service_id is required",
		},
		{
			name:    "service_id not starting with 62",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.Service.ServiceID = "0812345678901" },
			wantErr: "service.service_id must start with 62",
		},
		{
			name:    "zero amount",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.Recharge.Amount = 0 },
			wantErr: "recharge.amount must be greater than zero",
		},
		{
			name:    "negative amount",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.Recharge.Amount = -100 },
			wantErr: "recharge.amount must be greater than zero",
		},
		{
			name:    "empty stock_type",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.Recharge.StockType = "" },
			wantErr: "recharge.stock_type is required",
		},
		{
			name:    "empty element1",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.Recharge.Element1 = "" },
			wantErr: "recharge.element1 is required",
		},
		{
			name:    "empty third_party_id",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.MerchantProfile.ThirdPartyID = "" },
			wantErr: "merchant_profile.third_party_id is required",
		},
		{
			name:    "empty third_party_password",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.MerchantProfile.ThirdPartyPassword = "" },
			wantErr: "merchant_profile.third_party_password is required",
		},
		{
			name:    "empty delivery_channel",
			mutate:  func(r *InitiateRegularRechargeRequest) { r.MerchantProfile.DeliveryChannel = "" },
			wantErr: "merchant_profile.delivery_channel is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validInitiateRegularRechargeRequest()
			tc.mutate(&req)
			err := validateInitiateRegularRechargeRequest(req)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateOrderDealerRequest
// ---------------------------------------------------------------------------

func validOrderDealerRequest() OrderDealerRequest {
	return OrderDealerRequest{
		Transaction: OrderDealerTransaction{
			TransactionID: "TX00000000000000000000001",
			Channel:       "tp",
		},
		Service: OrderDealerService{
			OrganizationCode: "705009",
			ServiceID:        "6281234567890",
		},
		Order: OrderDealerOrder{
			ProductID: "PROD001",
			StockType: StockTypeBulk,
			Element1:  "encrypted-element1",
		},
		MerchantProfile: OrderDealerMerchantProfile{
			ThirdPartyID:       "NGRS",
			ThirdPartyPassword: "pass123",
			DeliveryChannel:    "SMS",
		},
	}
}

func TestValidateOrderDealerRequest_Valid(t *testing.T) {
	err := validateOrderDealerRequest(validOrderDealerRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateOrderDealerRequest_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*OrderDealerRequest)
		wantErr string
	}{
		{
			name:    "empty transaction_id",
			mutate:  func(r *OrderDealerRequest) { r.Transaction.TransactionID = "" },
			wantErr: "transaction.transaction_id is required",
		},
		{
			name:    "transaction_id > 25 chars",
			mutate:  func(r *OrderDealerRequest) { r.Transaction.TransactionID = strings.Repeat("B", 26) },
			wantErr: "transaction.transaction_id must be at most 25 characters",
		},
		{
			name:    "empty channel",
			mutate:  func(r *OrderDealerRequest) { r.Transaction.Channel = "" },
			wantErr: "transaction.channel is required",
		},
		{
			name:    "empty organization_code",
			mutate:  func(r *OrderDealerRequest) { r.Service.OrganizationCode = "" },
			wantErr: "service.organization_code is required",
		},
		{
			name:    "empty service_id",
			mutate:  func(r *OrderDealerRequest) { r.Service.ServiceID = "" },
			wantErr: "service.service_id is required",
		},
		{
			name:    "service_id not starting with 62",
			mutate:  func(r *OrderDealerRequest) { r.Service.ServiceID = "0812345678901" },
			wantErr: "service.service_id must start with 62",
		},
		{
			name:    "empty product_id",
			mutate:  func(r *OrderDealerRequest) { r.Order.ProductID = "" },
			wantErr: "order.product_id is required",
		},
		{
			name:    "empty stock_type",
			mutate:  func(r *OrderDealerRequest) { r.Order.StockType = "" },
			wantErr: "order.stock_type is required",
		},
		{
			name:    "empty element1",
			mutate:  func(r *OrderDealerRequest) { r.Order.Element1 = "" },
			wantErr: "order.element1 is required",
		},
		{
			name:    "empty third_party_id",
			mutate:  func(r *OrderDealerRequest) { r.MerchantProfile.ThirdPartyID = "" },
			wantErr: "merchant_profile.third_party_id is required",
		},
		{
			name:    "empty third_party_password",
			mutate:  func(r *OrderDealerRequest) { r.MerchantProfile.ThirdPartyPassword = "" },
			wantErr: "merchant_profile.third_party_password is required",
		},
		{
			name:    "empty delivery_channel",
			mutate:  func(r *OrderDealerRequest) { r.MerchantProfile.DeliveryChannel = "" },
			wantErr: "merchant_profile.delivery_channel is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validOrderDealerRequest()
			tc.mutate(&req)
			err := validateOrderDealerRequest(req)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateBrowseOfferRequest
// ---------------------------------------------------------------------------

func validBrowseOfferRequest() BrowseOfferRequest {
	return BrowseOfferRequest{
		TransactionID:    "TX00000000000000000000001",
		Channel:          "tp",
		OrganizationCode: "705009",
		ServiceID:        "6281234567890",
		ProductID:        "PROD001",
		Version:          "v2",
	}
}

func TestValidateBrowseOfferRequest_Valid(t *testing.T) {
	err := validateBrowseOfferRequest(validBrowseOfferRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateBrowseOfferRequest_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*BrowseOfferRequest)
		wantErr string
	}{
		{
			name:    "empty transaction_id",
			mutate:  func(r *BrowseOfferRequest) { r.TransactionID = "" },
			wantErr: "transaction_id is required",
		},
		{
			name:    "transaction_id > 25 chars",
			mutate:  func(r *BrowseOfferRequest) { r.TransactionID = strings.Repeat("C", 26) },
			wantErr: "transaction_id must be at most 25 characters",
		},
		{
			name:    "empty channel",
			mutate:  func(r *BrowseOfferRequest) { r.Channel = "" },
			wantErr: "channel is required",
		},
		{
			name:    "empty organization_code",
			mutate:  func(r *BrowseOfferRequest) { r.OrganizationCode = "" },
			wantErr: "organization_code is required",
		},
		{
			name:    "empty service_id",
			mutate:  func(r *BrowseOfferRequest) { r.ServiceID = "" },
			wantErr: "service_id is required",
		},
		{
			name:    "service_id not starting with 62",
			mutate:  func(r *BrowseOfferRequest) { r.ServiceID = "0812345678901" },
			wantErr: "service_id must start with 62",
		},
		{
			name:    "empty product_id",
			mutate:  func(r *BrowseOfferRequest) { r.ProductID = "" },
			wantErr: "product_id is required",
		},
		{
			name:    "empty version",
			mutate:  func(r *BrowseOfferRequest) { r.Version = "" },
			wantErr: "version is required",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validBrowseOfferRequest()
			tc.mutate(&req)
			err := validateBrowseOfferRequest(req)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// validateCheckOrderStatusRequest
// ---------------------------------------------------------------------------

func validCheckOrderStatusRequest() CheckOrderStatusRequest {
	return CheckOrderStatusRequest{
		TransactionID:         "TX00000000000000000000001",
		OriginalTransactionID: "ORIG0000000000000000001",
		ServiceID:             "6281234567890",
	}
}

func TestValidateCheckOrderStatusRequest_Valid(t *testing.T) {
	err := validateCheckOrderStatusRequest(validCheckOrderStatusRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateCheckOrderStatusRequest_Invalid(t *testing.T) {
	tests := []struct {
		name    string
		mutate  func(*CheckOrderStatusRequest)
		wantErr string
	}{
		{
			name:    "empty transaction_id",
			mutate:  func(r *CheckOrderStatusRequest) { r.TransactionID = "" },
			wantErr: "transaction_id is required",
		},
		{
			name:    "transaction_id > 25 chars",
			mutate:  func(r *CheckOrderStatusRequest) { r.TransactionID = strings.Repeat("D", 26) },
			wantErr: "transaction_id must be at most 25 characters",
		},
		{
			name:    "empty original_transaction_id",
			mutate:  func(r *CheckOrderStatusRequest) { r.OriginalTransactionID = "" },
			wantErr: "original_transaction_id is required",
		},
		{
			name:    "empty service_id",
			mutate:  func(r *CheckOrderStatusRequest) { r.ServiceID = "" },
			wantErr: "service_id is required",
		},
		{
			name:    "service_id not starting with 62",
			mutate:  func(r *CheckOrderStatusRequest) { r.ServiceID = "0812345678901" },
			wantErr: "service_id must start with 62",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			req := validCheckOrderStatusRequest()
			tc.mutate(&req)
			err := validateCheckOrderStatusRequest(req)
			if err == nil {
				t.Fatalf("expected error containing %q, got nil", tc.wantErr)
			}
			if !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("error %q does not contain %q", err.Error(), tc.wantErr)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// buildBrowseOfferURL
// ---------------------------------------------------------------------------

func TestBuildBrowseOfferURL(t *testing.T) {
	req := validBrowseOfferRequest()
	got, err := buildBrowseOfferURL("https://api.example.com/esb/v1/modern/offer/dealer", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	q := u.Query()
	checks := map[string]string{
		"transaction_id":    req.TransactionID,
		"channel":           req.Channel,
		"organization_code": req.OrganizationCode,
		"service_id":        req.ServiceID,
		"product_id":        req.ProductID,
		"version":           req.Version,
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("query param %q = %q, want %q", key, got, want)
		}
	}

	if u.Path != "/esb/v1/modern/offer/dealer" {
		t.Errorf("path = %q, want /esb/v1/modern/offer/dealer", u.Path)
	}
}

// ---------------------------------------------------------------------------
// buildCheckOrderStatusURL
// ---------------------------------------------------------------------------

func TestBuildCheckOrderStatusURL(t *testing.T) {
	req := CheckOrderStatusRequest{
		TransactionID:         "TX001",
		OriginalTransactionID: "ORIG001",
		SerialNumber:          "SN001",
		ServiceID:             "6281234567890",
	}
	got, err := buildCheckOrderStatusURL("https://api.example.com/esb/v1/modern/dealer/order/status", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	q := u.Query()
	checks := map[string]string{
		"transaction_id":          "TX001",
		"original_transaction_id": "ORIG001",
		"serial_number":           "SN001",
		"service_id":              "6281234567890",
	}
	for key, want := range checks {
		if got := q.Get(key); got != want {
			t.Errorf("query param %q = %q, want %q", key, got, want)
		}
	}
}

func TestBuildCheckOrderStatusURL_OmitsEmptySerialNumber(t *testing.T) {
	req := CheckOrderStatusRequest{
		TransactionID:         "TX001",
		OriginalTransactionID: "ORIG001",
		SerialNumber:          "",
		ServiceID:             "6281234567890",
	}
	got, err := buildCheckOrderStatusURL("https://api.example.com/status", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	u, err := url.Parse(got)
	if err != nil {
		t.Fatalf("failed to parse URL: %v", err)
	}

	if u.Query().Get("serial_number") != "" {
		t.Error("expected serial_number to be omitted when empty")
	}
}

// ---------------------------------------------------------------------------
// API method tests with httptest.NewServer
// ---------------------------------------------------------------------------

// testClient creates a Client pointing at the given httptest server URL.
func testClient(t *testing.T, serverURL string) *Client {
	t.Helper()
	c, err := NewClient(serverURL, "tp", "secret", "apikey", 5*time.Second, slog.Default())
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

// ---------------------------------------------------------------------------
// InitiateRegularRecharge
// ---------------------------------------------------------------------------

func TestInitiateRegularRecharge_HTTP200_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "00000",
				"status_desc": "SUCCESS"
			},
			"serial_number": "SN12345"
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	resp, err := client.InitiateRegularRecharge(context.Background(), validInitiateRegularRechargeRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Transaction.StatusCode != "00000" {
		t.Fatalf("status_code = %q, want %q", resp.Transaction.StatusCode, "00000")
	}
	if resp.SerialNumber != "SN12345" {
		t.Fatalf("serial_number = %q, want %q", resp.SerialNumber, "SN12345")
	}
}

func TestInitiateRegularRecharge_HTTP200_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "10001",
				"status_desc": "Insufficient balance"
			},
			"serial_number": ""
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	resp, err := client.InitiateRegularRecharge(context.Background(), validInitiateRegularRechargeRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var bErr *BusinessError
	if !errors.As(err, &bErr) {
		t.Fatalf("expected *BusinessError, got %T: %v", err, err)
	}
	if bErr.Code != "10001" {
		t.Fatalf("BusinessError.Code = %q, want %q", bErr.Code, "10001")
	}
	if bErr.Message != "Insufficient balance" {
		t.Fatalf("BusinessError.Message = %q, want %q", bErr.Message, "Insufficient balance")
	}
	// Response should still be returned alongside the error
	if resp != nil && resp.Transaction.StatusCode != "10001" {
		t.Fatalf("resp.Transaction.StatusCode = %q, want %q", resp.Transaction.StatusCode, "10001")
	}
}

func TestInitiateRegularRecharge_HTTP500_TechnicalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	// Set maxRetries to 0 to avoid retries for this specific test
	client.maxRetries = 0

	_, err := client.InitiateRegularRecharge(context.Background(), validInitiateRegularRechargeRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var tErr *TechnicalError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TechnicalError, got %T: %v", err, err)
	}
	if tErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("TechnicalError.StatusCode = %d, want %d", tErr.StatusCode, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// BrowseOffer
// ---------------------------------------------------------------------------

func TestBrowseOffer_HTTP200_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "00000",
				"status_desc": "Success"
			},
			"product": {
				"id": "PROD001",
				"commercial_name": "Combo 50 GB",
				"allowance_description": "Internet 4G 20 GB",
				"product_length": "30 Days",
				"price": "120000",
				"stock_type": "BULK"
			}
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	resp, err := client.BrowseOffer(context.Background(), validBrowseOfferRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Transaction.StatusCode != "00000" {
		t.Fatalf("status_code = %q, want %q", resp.Transaction.StatusCode, "00000")
	}
	if resp.Product == nil {
		t.Fatal("expected non-nil product")
	}
	if resp.Product.ID != "PROD001" {
		t.Fatalf("product.id = %q, want %q", resp.Product.ID, "PROD001")
	}
	if resp.Product.CommercialName != "Combo 50 GB" {
		t.Fatalf("product.commercial_name = %q, want %q", resp.Product.CommercialName, "Combo 50 GB")
	}
	if resp.Product.Price != "120000" {
		t.Fatalf("product.price = %q, want %q", resp.Product.Price, "120000")
	}
}

func TestBrowseOffer_HTTP400_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "30RV-0001",
				"status_desc": "Could not get eligible product information"
			}
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	resp, err := client.BrowseOffer(context.Background(), validBrowseOfferRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var bErr *BusinessError
	if !errors.As(err, &bErr) {
		t.Fatalf("expected *BusinessError, got %T: %v", err, err)
	}
	if bErr.Code != "30RV-0001" {
		t.Fatalf("BusinessError.Code = %q, want %q", bErr.Code, "30RV-0001")
	}
	// Response should be returned alongside the error
	if resp == nil {
		t.Fatal("expected non-nil response alongside BusinessError")
	}
	if resp.Transaction.StatusCode != "30RV-0001" {
		t.Fatalf("resp.Transaction.StatusCode = %q, want %q", resp.Transaction.StatusCode, "30RV-0001")
	}
}

// ---------------------------------------------------------------------------
// OrderDealer
// ---------------------------------------------------------------------------

func TestOrderDealer_HTTP200_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "00000",
				"status_desc": "SUCCESS"
			},
			"service": {
				"organization_code": "705009",
				"service_id": "6281234567890"
			},
			"order": {
				"product_id": "PROD001",
				"stock_type": "BULK",
				"element1": "enc",
				"ap_flag": "Y"
			},
			"merchant_profile": {
				"merchant_signature": "",
				"partner_mid": "",
				"third_party_id": "NGRS",
				"third_party_password": "pass",
				"fund_source": "",
				"address": "",
				"postcode": "",
				"district": "",
				"store_id": "",
				"city": "",
				"coordinate": "",
				"delivery_channel": "SMS",
				"transmission_date": ""
			}
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	resp, err := client.OrderDealer(context.Background(), validOrderDealerRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Transaction.StatusCode != "00000" {
		t.Fatalf("status_code = %q, want %q", resp.Transaction.StatusCode, "00000")
	}
	if resp.Order.ProductID != "PROD001" {
		t.Fatalf("order.product_id = %q, want %q", resp.Order.ProductID, "PROD001")
	}
}

// ---------------------------------------------------------------------------
// CheckOrderStatus
// ---------------------------------------------------------------------------

func TestCheckOrderStatus_HTTP200_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "00000",
				"status_desc": "SUCCESS"
			},
			"transaction_status": {
				"original_transaction_id": "ORIG0000000000000000001",
				"serial_number": "SN999",
				"status": "COMPLETED"
			}
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	resp, err := client.CheckOrderStatus(context.Background(), validCheckOrderStatusRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response")
	}
	if resp.Transaction.StatusCode != "00000" {
		t.Fatalf("status_code = %q, want %q", resp.Transaction.StatusCode, "00000")
	}
	if resp.TransactionStatus == nil {
		t.Fatal("expected non-nil transaction_status")
	}
	if resp.TransactionStatus.Status != "COMPLETED" {
		t.Fatalf("transaction_status.status = %q, want %q", resp.TransactionStatus.Status, "COMPLETED")
	}
}

func TestCheckOrderStatus_HTTP400_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "40RV-0002",
				"status_desc": "Transaction not found"
			}
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	resp, err := client.CheckOrderStatus(context.Background(), validCheckOrderStatusRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var bErr *BusinessError
	if !errors.As(err, &bErr) {
		t.Fatalf("expected *BusinessError, got %T: %v", err, err)
	}
	if bErr.Code != "40RV-0002" {
		t.Fatalf("BusinessError.Code = %q, want %q", bErr.Code, "40RV-0002")
	}
	if resp == nil {
		t.Fatal("expected non-nil response alongside BusinessError")
	}
	if resp.Transaction.StatusCode != "40RV-0002" {
		t.Fatalf("resp.Transaction.StatusCode = %q, want %q", resp.Transaction.StatusCode, "40RV-0002")
	}
}

// ---------------------------------------------------------------------------
// Retry behavior
// ---------------------------------------------------------------------------

func TestRetry_HTTP500_RetriesUpToMaxRetries(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 2

	_, err := client.InitiateRegularRecharge(context.Background(), validInitiateRegularRechargeRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var tErr *TechnicalError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TechnicalError, got %T: %v", err, err)
	}

	// maxRetries=2 means 1 initial + 2 retries = 3 total attempts
	expectedCalls := 3
	if callCount != expectedCalls {
		t.Fatalf("callCount = %d, want %d (1 initial + %d retries)", callCount, expectedCalls, client.maxRetries)
	}
}

func TestRetry_BusinessError_NoRetry(t *testing.T) {
	callCount := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "10001",
				"status_desc": "Insufficient balance"
			},
			"serial_number": ""
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 2

	_, err := client.InitiateRegularRecharge(context.Background(), validInitiateRegularRechargeRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var bErr *BusinessError
	if !errors.As(err, &bErr) {
		t.Fatalf("expected *BusinessError, got %T: %v", err, err)
	}

	// BusinessError should not trigger retries — only 1 call
	if callCount != 1 {
		t.Fatalf("callCount = %d, want 1 (no retries for BusinessError)", callCount)
	}
}

func TestRetry_RequestRejected_NoRetry(t *testing.T) {
	callCount := 0
	rejectedHTML := `<html><head><title>Request Rejected</title></head><body>The requested URL was rejected. Please consult with your administrator.<br><br>Your support ID is: 9849749215759513006<br><br><a href='javascript:history.back();'>[Go Back]</a></body></html>`
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		callCount++
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(rejectedHTML))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 2

	_, err := client.InitiateRegularRecharge(context.Background(), validInitiateRegularRechargeRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var rErr *RejectedError
	if !errors.As(err, &rErr) {
		t.Fatalf("expected *RejectedError, got %T: %v", err, err)
	}
	if rErr.SupportID != "9849749215759513006" {
		t.Fatalf("RejectedError.SupportID = %q, want %q", rErr.SupportID, "9849749215759513006")
	}

	// RejectedError must not trigger retries — only 1 call
	if callCount != 1 {
		t.Fatalf("callCount = %d, want 1 (no retries for RejectedError)", callCount)
	}
}

// ---------------------------------------------------------------------------
// Request header verification
// ---------------------------------------------------------------------------

func TestRequestHeaders_AreSetCorrectly(t *testing.T) {
	var capturedHeaders http.Header
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		capturedHeaders = r.Header.Clone()
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "00000",
				"status_desc": "SUCCESS"
			},
			"serial_number": "SN123"
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "test-api-key-value")
	t.Setenv("SECRET_KEY", "test-secret-key-value")

	client := testClient(t, srv.URL)
	_, err := client.InitiateRegularRecharge(context.Background(), validInitiateRegularRechargeRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify api-key header
	if got := capturedHeaders.Get("api-key"); got != "apikey" {
		t.Fatalf("api-key header = %q, want %q", got, "apikey")
	}

	// Verify Channel-Id header
	if got := capturedHeaders.Get("Channel-Id"); got != "tp" {
		t.Fatalf("Channel-Id header = %q, want %q", got, "tp")
	}

	// Verify Timestamp header is non-empty
	if got := capturedHeaders.Get("Timestamp"); got == "" {
		t.Fatal("Timestamp header is empty, expected non-empty")
	}

	// Verify External-Transaction-Id header matches request transaction_id
	if got := capturedHeaders.Get("External-Transaction-Id"); got != "TX00000000000000000000001" {
		t.Fatalf("External-Transaction-Id header = %q, want %q", got, "TX00000000000000000000001")
	}

	// Verify x-signature header is non-empty (32-char hex)
	sig := capturedHeaders.Get("x-signature")
	if sig == "" {
		t.Fatal("x-signature header is empty, expected non-empty")
	}
	if len(sig) != 32 {
		t.Fatalf("x-signature length = %d, want 32", len(sig))
	}

	// Verify Content-Type for POST requests
	if got := capturedHeaders.Get("Content-Type"); got != "application/json" {
		t.Fatalf("Content-Type header = %q, want %q", got, "application/json")
	}
}

// ---------------------------------------------------------------------------
// OrderDealer – error paths
// ---------------------------------------------------------------------------

func TestOrderDealer_HTTP500_TechnicalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 0

	_, err := client.OrderDealer(context.Background(), validOrderDealerRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var tErr *TechnicalError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TechnicalError, got %T: %v", err, err)
	}
	if tErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("TechnicalError.StatusCode = %d, want %d", tErr.StatusCode, http.StatusInternalServerError)
	}
}

func TestOrderDealer_HTTP200_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "20001",
				"status_desc": "Order failed"
			}
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	resp, err := client.OrderDealer(context.Background(), validOrderDealerRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var bErr *BusinessError
	if !errors.As(err, &bErr) {
		t.Fatalf("expected *BusinessError, got %T: %v", err, err)
	}
	if bErr.Code != "20001" {
		t.Fatalf("BusinessError.Code = %q, want %q", bErr.Code, "20001")
	}
	if resp == nil {
		t.Fatal("expected non-nil response alongside BusinessError")
	}
}

func TestOrderDealer_HTTP400_TechnicalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte(`Bad Request`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 0

	_, err := client.OrderDealer(context.Background(), validOrderDealerRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var tErr *TechnicalError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TechnicalError, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// CheckOrderStatus – error paths
// ---------------------------------------------------------------------------

func TestCheckOrderStatus_HTTP500_TechnicalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 0

	_, err := client.CheckOrderStatus(context.Background(), validCheckOrderStatusRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var tErr *TechnicalError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TechnicalError, got %T: %v", err, err)
	}
	if tErr.StatusCode != http.StatusInternalServerError {
		t.Fatalf("TechnicalError.StatusCode = %d, want %d", tErr.StatusCode, http.StatusInternalServerError)
	}
}

// ---------------------------------------------------------------------------
// BrowseOffer – error paths
// ---------------------------------------------------------------------------

func TestBrowseOffer_HTTP500_TechnicalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 0

	_, err := client.BrowseOffer(context.Background(), validBrowseOfferRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var tErr *TechnicalError
	if !errors.As(err, &tErr) {
		t.Fatalf("expected *TechnicalError, got %T: %v", err, err)
	}
}

// ---------------------------------------------------------------------------
// WithAPILogger
// ---------------------------------------------------------------------------

func TestWithAPILogger(t *testing.T) {
	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client, err := NewClient("http://localhost", "tp", "secret", "apikey", 5*time.Second, nil, WithAPILogger(nil))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

// ---------------------------------------------------------------------------
// TechnicalError.Error branches
// ---------------------------------------------------------------------------

func TestTechnicalError_Error(t *testing.T) {
	tests := []struct {
		name string
		err  TechnicalError
		want string
	}{
		{
			name: "status and cause",
			err:  TechnicalError{StatusCode: 500, Cause: fmt.Errorf("server down")},
			want: "technical error: status=500 cause=server down",
		},
		{
			name: "status only",
			err:  TechnicalError{StatusCode: 502},
			want: "technical error: status=502",
		},
		{
			name: "cause only",
			err:  TechnicalError{Cause: fmt.Errorf("timeout")},
			want: "technical error: timeout",
		},
		{
			name: "no status no cause",
			err:  TechnicalError{},
			want: "technical error",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := tc.err.Error()
			if got != tc.want {
				t.Errorf("Error() = %q, want %q", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// logAPICall with APILogger
// ---------------------------------------------------------------------------

type mockAPILogger struct {
	logCalls []APICallLog
}

func (m *mockAPILogger) Log(_ context.Context, entry APICallLog) {
	m.logCalls = append(m.logCalls, entry)
}

func TestInitiateRegularRecharge_WithAPILogger_LogsCall(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "00000",
				"status_desc": "SUCCESS"
			},
			"serial_number": "SN12345"
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	logger := &mockAPILogger{}
	client, err := NewClient(srv.URL, "tp", "secret", "apikey", 5*time.Second, slog.Default(), WithAPILogger(logger))
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}

	_, err = client.InitiateRegularRecharge(context.Background(), validInitiateRegularRechargeRequest())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Give the goroutine a moment to complete
	time.Sleep(50 * time.Millisecond)

	if len(logger.logCalls) == 0 {
		t.Error("expected at least 1 API log call")
	}
}

// ---------------------------------------------------------------------------
// Retry backoff paths
// ---------------------------------------------------------------------------

func TestOrderDealer_HTTP500_RetriesAndReturnsError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 2

	_, err := client.OrderDealer(context.Background(), validOrderDealerRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	// Should have attempted maxRetries + 1 times
	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestCheckOrderStatus_HTTP500_RetriesAndReturnsError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 2

	_, err := client.CheckOrderStatus(context.Background(), validCheckOrderStatusRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestBrowseOffer_HTTP500_RetriesAndReturnsError(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.WriteHeader(http.StatusInternalServerError)
		_, _ = w.Write([]byte(`Internal Server Error`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 2

	_, err := client.BrowseOffer(context.Background(), validBrowseOfferRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", attempts)
	}
}

func TestOrderDealer_BusinessError_NoRetry(t *testing.T) {
	attempts := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		attempts++
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "20001",
				"status_desc": "Order failed"
			}
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	client.maxRetries = 2

	_, err := client.OrderDealer(context.Background(), validOrderDealerRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	if attempts != 1 {
		t.Fatalf("expected 1 attempt (no retry on BusinessError), got %d", attempts)
	}
}

func TestBrowseOffer_HTTP200_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "30RV-0002",
				"status_desc": "Product not found"
			}
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	resp, err := client.BrowseOffer(context.Background(), validBrowseOfferRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var bErr *BusinessError
	if !errors.As(err, &bErr) {
		t.Fatalf("expected *BusinessError, got %T: %v", err, err)
	}
	if bErr.Code != "30RV-0002" {
		t.Fatalf("BusinessError.Code = %q, want %q", bErr.Code, "30RV-0002")
	}
	if resp == nil {
		t.Fatal("expected non-nil response alongside BusinessError")
	}
}

func TestCheckOrderStatus_HTTP200_BusinessError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{
			"transaction": {
				"transaction_id": "TX00000000000000000000001",
				"channel": "tp",
				"status_code": "40001",
				"status_desc": "Transaction not found"
			}
		}`))
	}))
	defer srv.Close()

	t.Setenv("API_KEY", "apikey")
	t.Setenv("SECRET_KEY", "secret")

	client := testClient(t, srv.URL)
	resp, err := client.CheckOrderStatus(context.Background(), validCheckOrderStatusRequest())
	if err == nil {
		t.Fatal("expected error, got nil")
	}

	var bErr *BusinessError
	if !errors.As(err, &bErr) {
		t.Fatalf("expected *BusinessError, got %T: %v", err, err)
	}
	if resp == nil {
		t.Fatal("expected non-nil response alongside BusinessError")
	}
}
