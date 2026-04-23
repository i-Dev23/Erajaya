package telkomsel

import (
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"
)

func TestConsumePulsa_Live_FromEnvExample(t *testing.T) {
	if os.Getenv("TELKOMSEL_LIVE_TEST") != "1" {
		t.Skip("set TELKOMSEL_LIVE_TEST=1 to enable live test")
	}

	// This test hits the real Telkomsel endpoint.
	// Secrets must come from your local environment (do not commit them into .env.example).
	required := []string{
		"BASE_URL",
		"CHANNEL_ID",
		"SECRET_KEY",
		"API_KEY",
		"THIRD_PARTY_ID",
		"THIRD_PARTY_PASSWORD",
		"ENCRYPTION_KEY",
		"DELIVERY_CHANNEL",
		"swpps",
	}
	for _, k := range required {
		if os.Getenv(k) == "" {
			t.Skipf("missing env %s; set required Telkomsel env vars to run live test", k)
		}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Use a likely-invalid amount to avoid unintended real recharge success.
	amount := 1

	resp, err := InitiateRegularRechargeOnConsume(
		ctx,
		"081000000000", // dummy MSISDN; will be normalized to 62xxxxxxxxxxx
		"swpps",
		"live.queue",
		"live-msg-1",
		amount,
		StockTypeFixed,
	)

	if resp != nil {
		prettyResp, _ := json.MarshalIndent(resp, "", "  ")
		t.Logf("Telkomsel response JSON (live):\n%s", string(prettyResp))
	}

	if err != nil {
		t.Logf("live call returned error: %v", err)
		return
	}
}
