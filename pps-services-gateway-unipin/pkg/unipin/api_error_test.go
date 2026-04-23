package unipin

import (
	"encoding/json"
	"testing"
)

func TestResolveReason_FallbackToAPIErrorMessage(t *testing.T) {
	reason := ResolveReason("", &APIError{Message: "Internal error", ErrorCode: 999})
	if reason != "Internal error (error_code=999)" {
		t.Fatalf("unexpected reason: %q", reason)
	}
}

func TestUnmarshal_Status0WithErrorObject(t *testing.T) {
	body := []byte(`{"error":{"message":"Internal error","error_code":999},"status":0}`)

	var resp CreateOrderResponse
	if err := json.Unmarshal(body, &resp); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}
	if resp.Status != 0 {
		t.Fatalf("expected status=0, got %d", resp.Status)
	}
	if resp.Error == nil || resp.Error.Message != "Internal error" || resp.Error.ErrorCode != 999 {
		t.Fatalf("expected error message+code, got %#v", resp.Error)
	}

	resolved := ResolveReason(resp.Reason, resp.Error)
	if resolved != "Internal error (error_code=999)" {
		t.Fatalf("unexpected resolved reason: %q", resolved)
	}
}
