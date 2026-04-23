package rabbitmq

import (
	"encoding/json"
	"fmt"
	"testing"
)

// ---------------------------------------------------------------------------
// getAny
// ---------------------------------------------------------------------------

func TestGetAny(t *testing.T) {
	tests := []struct {
		name string
		m    map[string]any
		keys []string
		want any
	}{
		{"nil map returns nil", nil, []string{"a"}, nil},
		{"missing key returns nil", map[string]any{"x": 1}, []string{"a", "b"}, nil},
		{"first key matches", map[string]any{"a": "v1", "b": "v2"}, []string{"a", "b"}, "v1"},
		{"second key matches", map[string]any{"b": "v2"}, []string{"a", "b"}, "v2"},
		{"no keys returns nil", map[string]any{"a": 1}, []string{}, nil},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := getAny(tc.m, tc.keys...)
			if got != tc.want {
				t.Errorf("getAny() = %v, want %v", got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseString
// ---------------------------------------------------------------------------

func TestParseString(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  string
	}{
		{"nil returns empty", nil, ""},
		{"string value", "hello", "hello"},
		{"string with spaces trimmed", "  hello  ", "hello"},
		{"json.Number", json.Number("42"), "42"},
		{"float64 integer", float64(99), "99"},
		{"float64 fractional", float64(3.14), "3.14"},
		{"bool value uses default fmt", true, "true"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseString(tc.input)
			if got != tc.want {
				t.Errorf("parseString(%v) = %q, want %q", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// parseInt
// ---------------------------------------------------------------------------

func TestParseInt(t *testing.T) {
	tests := []struct {
		name  string
		input any
		want  int
	}{
		{"nil returns 0", nil, 0},
		{"json.Number integer", json.Number("42"), 42},
		{"json.Number float", json.Number("3.9"), 3},
		{"float64", float64(7.8), 7},
		{"string integer", "123", 123},
		{"string with spaces", " 456 ", 456},
		{"non-numeric string returns 0", "abc", 0},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := parseInt(tc.input)
			if got != tc.want {
				t.Errorf("parseInt(%v) = %d, want %d", tc.input, got, tc.want)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// consumePayload.UnmarshalJSON – snake_case field names
// ---------------------------------------------------------------------------

func TestUnmarshalJSON_SnakeCase(t *testing.T) {
	input := `{
		"amount": 15000,
		"stock_type": "BULK",
		"product_code": "PC001",
		"product_id": "PID001",
		"product_type": "pulsa",
		"mid": "MID123",
		"store_id": "STORE1",
		"queue_name": "q1",
		"msisdn": "628123456789",
		"msgid": "MSG001",
		"callback_url": "http://example.com/cb",
		"type_voucher": "pulsa",
		"command": "",
		"mq_transaction": "amqp://localhost"
	}`

	var p consumePayload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	assertField(t, "Amount", p.Amount, 15000)
	assertField(t, "StockType", p.StockType, "BULK")
	assertField(t, "ProductCode", p.ProductCode, "PC001")
	assertField(t, "ProductID", p.ProductID, "PID001")
	assertField(t, "ProductType", p.ProductType, "pulsa")
	assertField(t, "MID", p.MID, "MID123")
	assertField(t, "StoreID", p.StoreID, "STORE1")
	assertField(t, "QueueName", p.QueueName, "q1")
	assertField(t, "MSISDN", p.MSISDN, "628123456789")
	assertField(t, "MsgID", p.MsgID, "MSG001")
	assertField(t, "CallbackURL", p.CallbackURL, "http://example.com/cb")
	assertField(t, "TypeVoucher", p.TypeVoucher, "pulsa")
	assertField(t, "MQTransaction", p.MQTransaction, "amqp://localhost")
}

// ---------------------------------------------------------------------------
// consumePayload.UnmarshalJSON – camelCase field names
// ---------------------------------------------------------------------------

func TestUnmarshalJSON_CamelCase(t *testing.T) {
	input := `{
		"amount": 20000,
		"stockType": "FIXED",
		"productCode": "PC002",
		"productId": "PID002",
		"productType": "paket data",
		"mid": "MID456",
		"storeId": "STORE2",
		"queueName": "q2",
		"msisdn": "628987654321",
		"msgID": "MSG002",
		"callbackUrl": "http://example.com/cb2",
		"typeVoucher": "paket data",
		"command": "",
		"mqTransaction": "amqp://remote"
	}`

	var p consumePayload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	assertField(t, "Amount", p.Amount, 20000)
	assertField(t, "StockType", p.StockType, "FIXED")
	assertField(t, "ProductCode", p.ProductCode, "PC002")
	assertField(t, "ProductID", p.ProductID, "PID002")
	assertField(t, "ProductType", p.ProductType, "data")
	assertField(t, "MID", p.MID, "MID456")
	assertField(t, "StoreID", p.StoreID, "STORE2")
	assertField(t, "QueueName", p.QueueName, "q2")
	assertField(t, "MSISDN", p.MSISDN, "628987654321")
	assertField(t, "MsgID", p.MsgID, "MSG002")
	assertField(t, "CallbackURL", p.CallbackURL, "http://example.com/cb2")
	assertField(t, "TypeVoucher", p.TypeVoucher, "paket data")
	assertField(t, "MQTransaction", p.MQTransaction, "amqp://remote")
}

// ---------------------------------------------------------------------------
// consumePayload.UnmarshalJSON – amount as JSON string
// ---------------------------------------------------------------------------

func TestUnmarshalJSON_AmountAsString(t *testing.T) {
	input := `{"amount": "25000", "mid": "M1", "msisdn": "628111111111"}`

	var p consumePayload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	assertField(t, "Amount", p.Amount, 25000)
}

// ---------------------------------------------------------------------------
// consumePayload.UnmarshalJSON – msisdn fallback to clientNumber
// ---------------------------------------------------------------------------

func TestUnmarshalJSON_MSISDNFallbackToClientNumber(t *testing.T) {
	input := `{"msisdn": "", "clientNumber": "628222222222", "mid": "M1"}`

	var p consumePayload
	if err := json.Unmarshal([]byte(input), &p); err != nil {
		t.Fatalf("UnmarshalJSON error: %v", err)
	}

	assertField(t, "MSISDN", p.MSISDN, "628222222222")
}

// ---------------------------------------------------------------------------
// consumePayload.UnmarshalJSON – typeVoucher derives productType
// ---------------------------------------------------------------------------

func TestUnmarshalJSON_TypeVoucherDerivesProductType(t *testing.T) {
	tests := []struct {
		name        string
		typeVoucher string
		wantType    string
	}{
		{"pulsa", "pulsa", "pulsa"},
		{"paket data", "paket data", "data"},
		{"data", "data", "data"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`{"typeVoucher": %q, "mid": "M1", "msisdn": "628111111111"}`, tc.typeVoucher)

			var p consumePayload
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				t.Fatalf("UnmarshalJSON error: %v", err)
			}

			assertField(t, "ProductType", p.ProductType, tc.wantType)
		})
	}
}

// ---------------------------------------------------------------------------
// parseCommand – pulsa format
// ---------------------------------------------------------------------------

func TestParseCommand_Pulsa(t *testing.T) {
	tests := []struct {
		name      string
		command   string
		wantCode  string
		wantAmt   int
		wantStock string
	}{
		{"basic bulk", "SP15*15000*BULK", "SP15", 15000, "BULK"},
		{"fixed type", "SP40*40000*FIXED", "SP40", 40000, "FIXED"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`{"command": %q, "typeVoucher": "pulsa", "mid": "M1", "msisdn": "628111111111"}`, tc.command)

			var p consumePayload
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				t.Fatalf("UnmarshalJSON error: %v", err)
			}

			assertField(t, "ProductCode", p.ProductCode, tc.wantCode)
			assertField(t, "Amount", p.Amount, tc.wantAmt)
			assertField(t, "StockType", p.StockType, tc.wantStock)
		})
	}
}

// ---------------------------------------------------------------------------
// parseCommand – data format
// ---------------------------------------------------------------------------

func TestParseCommand_Data(t *testing.T) {
	tests := []struct {
		name       string
		command    string
		wantCode   string
		wantAmt    int
		wantProdID string
		wantStock  string
	}{
		{"bulk vas akuisisi", "PP10*5000*00061469*BULK_VALUE_VAS_AKUISISI", "PP10", 5000, "00061469", "BULK_VALUE_VAS_AKUISISI"},
		{"bulk vas", "PP20*20000*00045463*BULK_VALUE_VAS", "PP20", 20000, "00045463", "BULK_VALUE_VAS"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			input := fmt.Sprintf(`{"command": %q, "typeVoucher": "data", "mid": "M1", "msisdn": "628111111111"}`, tc.command)

			var p consumePayload
			if err := json.Unmarshal([]byte(input), &p); err != nil {
				t.Fatalf("UnmarshalJSON error: %v", err)
			}

			assertField(t, "ProductCode", p.ProductCode, tc.wantCode)
			assertField(t, "Amount", p.Amount, tc.wantAmt)
			assertField(t, "ProductID", p.ProductID, tc.wantProdID)
			assertField(t, "StockType", p.StockType, tc.wantStock)
		})
	}
}

// ---------------------------------------------------------------------------
// helper
// ---------------------------------------------------------------------------

func assertField[T comparable](t *testing.T, name string, got, want T) {
	t.Helper()
	if got != want {
		t.Errorf("%s = %v, want %v", name, got, want)
	}
}
