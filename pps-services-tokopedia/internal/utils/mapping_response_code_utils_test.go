package utils

import "testing"

func TestMapOracleErrorToResponseCode(t *testing.T) {
	tests := []struct {
		name            string
		oracleMessage   string
		expectedCode    string
		expectedMessage string
	}{
		{
			name:            "Signature invalid component",
			oracleMessage:   "Error 02 : Signature tidak benar, komponen tidak syah.",
			expectedCode:    "32",
			expectedMessage: "Invalid signature",
		},
		{
			name:            "Signature mismatch",
			oracleMessage:   "Error 01 : Signature tidak benar.",
			expectedCode:    "32",
			expectedMessage: "Invalid signature",
		},
		{
			name:            "Product not mapped",
			oracleMessage:   "Produk belum dimapping",
			expectedCode:    "14",
			expectedMessage: "Ineligible Product",
		},
		{
			name:            "IP not configured",
			oracleMessage:   "Sell Error 02 : IP belum di setting.",
			expectedCode:    "60",
			expectedMessage: "Access is not allowed",
		},
		{
			name:            "IP mismatch with dynamic IP",
			oracleMessage:   "Sell Error 02 : IP Anda 192.168.1.1 tidak sama dengan Settingan ",
			expectedCode:    "60",
			expectedMessage: "Access is not allowed",
		},
		{
			name:            "Queue not ready",
			oracleMessage:   "Sell Error 02 : queue setting is not ready",
			expectedCode:    "60",
			expectedMessage: "Access is not allowed",
		},
		{
			name:            "Web transaction not allowed",
			oracleMessage:   "Anda tidak diperkenankan transaksi lewat WEB, hubungi admin kami untuk merubah profile anda.",
			expectedCode:    "60",
			expectedMessage: "Access is not allowed",
		},
		{
			name:            "Product not available for account",
			oracleMessage:   "Kode Voucher tidak tersedia untuk Account Anda.",
			expectedCode:    "14",
			expectedMessage: "Ineligible Product",
		},
		{
			name:            "Price not configured",
			oracleMessage:   "4. Harga jual belum di setting untuk anda.",
			expectedCode:    "14",
			expectedMessage: "Ineligible Product",
		},
		{
			name:            "Insufficient deposit",
			oracleMessage:   "Deposit anda kurang untuk memenuhi penjualan.",
			expectedCode:    "43",
			expectedMessage: "Insufficient balance",
		},
		{
			name:            "Stock voucher not available with dynamic voucher code",
			oracleMessage:   "Stock Voucher PLN20 habis, transaksi kami batalkan.",
			expectedCode:    "14",
			expectedMessage: "Ineligible Product",
		},
		{
			name:            "Fee not configured",
			oracleMessage:   "Fee Amount belum disetting",
			expectedCode:    "14",
			expectedMessage: "Ineligible Product",
		},
		{
			name:            "Transaction not found with dynamic transaction number",
			oracleMessage:   "No Transaksi TRX12345 tidak ditemukan.",
			expectedCode:    "12",
			expectedMessage: "Transaction not found",
		},
		{
			name:            "Product code not found",
			oracleMessage:   "Kode voucher is not found",
			expectedCode:    "14",
			expectedMessage: "Ineligible Product",
		},
		{
			name:            "Unknown error - should return default",
			oracleMessage:   "Some unknown error from Oracle",
			expectedCode:    "99",
			expectedMessage: "Other error",
		},
		{
			name:            "Case insensitive exact match - uppercase",
			oracleMessage:   "ERROR 02 : SIGNATURE TIDAK BENAR, KOMPONEN TIDAK SYAH.",
			expectedCode:    "32",
			expectedMessage: "Invalid signature",
		},
		{
			name:            "Case insensitive exact match - lowercase",
			oracleMessage:   "error 02 : signature tidak benar, komponen tidak syah.",
			expectedCode:    "32",
			expectedMessage: "Invalid signature",
		},
		{
			name:            "Case insensitive partial match - uppercase",
			oracleMessage:   "TIDAK TERSEDIA UNTUK ACCOUNT ANDA.",
			expectedCode:    "14",
			expectedMessage: "Ineligible Product",
		},
		{
			name:            "Case insensitive partial match - lowercase",
			oracleMessage:   "tidak tersedia untuk account anda.",
			expectedCode:    "14",
			expectedMessage: "Ineligible Product",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, message := MapOracleErrorToResponseCode(tt.oracleMessage)

			if code != tt.expectedCode {
				t.Errorf("MapOracleErrorToResponseCode() code = %v, want %v", code, tt.expectedCode)
			}

			if message != tt.expectedMessage {
				t.Errorf("MapOracleErrorToResponseCode() message = %v, want %v", message, tt.expectedMessage)
			}
		})
	}
}

func TestMapUltimaErrorMessageToResponseCode(t *testing.T) {
	tests := []struct {
		name            string
		ultimaMessage   string
		expectedCode    string
		expectedMessage string
	}{
		{
			name:            "Exact match uppercase",
			ultimaMessage:   "YANG ANDA MASUKKAN SALAH",
			expectedCode:    "20",
			expectedMessage: "Unregistered number",
		},
		{
			name:            "Exact match lowercase",
			ultimaMessage:   "yang anda masukkan salah",
			expectedCode:    "20",
			expectedMessage: "Unregistered number",
		},
		{
			name:            "Exact match mixed case",
			ultimaMessage:   "Yang Anda Masukkan Salah",
			expectedCode:    "20",
			expectedMessage: "Unregistered number",
		},
		{
			name:            "Partial match uppercase",
			ultimaMessage:   "CUT-OFF HARI INI",
			expectedCode:    "63",
			expectedMessage: "Biller maintenance",
		},
		{
			name:            "Partial match lowercase",
			ultimaMessage:   "cut-off hari ini",
			expectedCode:    "63",
			expectedMessage: "Biller maintenance",
		},
		{
			name:            "Partial match mixed case",
			ultimaMessage:   "Cut-Off Hari Ini",
			expectedCode:    "63",
			expectedMessage: "Biller maintenance",
		},
		{
			name:            "Unknown error - should return default",
			ultimaMessage:   "Some unknown error from Ultima",
			expectedCode:    "99",
			expectedMessage: "Other error",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			code, message := MapUltimaErrorMessageToResponseCode(tt.ultimaMessage)

			if code != tt.expectedCode {
				t.Errorf("MapUltimaErrorMessageToResponseCode() code = %v, want %v", code, tt.expectedCode)
			}

			if message != tt.expectedMessage {
				t.Errorf("MapUltimaErrorMessageToResponseCode() message = %v, want %v", message, tt.expectedMessage)
			}
		})
	}
}

func TestGetResponseCode(t *testing.T) {
	tests := []struct {
		name        string
		code        string
		wantCode    string
		wantMessage string
		wantOk      bool
	}{
		{
			name:        "Success code",
			code:        "00",
			wantCode:    "00",
			wantMessage: "Success",
			wantOk:      true,
		},
		{
			name:        "Invalid signature code",
			code:        "32",
			wantCode:    "32",
			wantMessage: "Invalid signature",
			wantOk:      true,
		},
		{
			name:        "Insufficient balance code",
			code:        "43",
			wantCode:    "43",
			wantMessage: "Insufficient balance",
			wantOk:      true,
		},
		{
			name:        "Non-existent code",
			code:        "999",
			wantCode:    "",
			wantMessage: "",
			wantOk:      false,
		},
		{
			name:        "Other error code 99",
			code:        "99",
			wantCode:    "99",
			wantMessage: "Other error",
			wantOk:      true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			rc, ok := GetResponseCode(tt.code)

			if ok != tt.wantOk {
				t.Errorf("GetResponseCode() ok = %v, want %v", ok, tt.wantOk)
			}

			if ok {
				if rc.Code != tt.wantCode {
					t.Errorf("GetResponseCode() code = %v, want %v", rc.Code, tt.wantCode)
				}

				if rc.Message != tt.wantMessage {
					t.Errorf("GetResponseCode() message = %v, want %v", rc.Message, tt.wantMessage)
				}
			}
		})
	}
}
