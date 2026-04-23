package utils

import "strings"

type ResponseCode struct {
	Code        string
	Message     string
	Description string
	Behavior    string
	Expected    string
}

// ResponseCodeMap maps response code strings to their corresponding ResponseCode struct.
var ResponseCodeMap = map[string]ResponseCode{
	// --- Success ---
	"00": {"00", "Success", "Transaction is success", "Success", "Success"},

	// --- Pending/On Process ---
	"01": {"01", "On process", "The transaction is still in progress, need to do check status", "Pending", "Pending"},
	"15": {"15", "Already Redeemed", "Tried to void already active or redeemed transaction", "Pending", "Pending"},
	"40": {"40", "Timeout", "Timeout from partner’s side", "Pending", "Pending"},
	"41": {"41", "Biller timeout", "Timeout from biller’s side", "Pending", "Pending"},

	// --- Failed/Retry ---
	"02": {"02", "Failed", "Transaction is failed", "Failed and Retry", "Failed and Retry"},
	"12": {"12", "Transaction not found", "No transaction was found for given Tokopedia ref_id", "Failed and Retry", "Failed and Retry"},
	"13": {"13", "Duplicate transaction ${ref_id}", "Found duplicate transaction with same Tokopedia ref_id", "Failed and Retry", "Failed and Retry"},
	"42": {"42", "Invalid parameter", "Missing required parameter", "Failed and Retry", "Pending"},
	"43": {"43", "Insufficient balance", "Balance is not enough to do a transaction", "Failed and Retry", "Failed and Retry"},
	"50": {"50", "Product out of stock", "Product is out of stock", "Failed and Retry", "Failed and Retry"},
	"51": {"51", "Product closed", "Product is closed", "Failed and Retry", "Failed and Retry"},
	"52": {"52", "Invalid product", "Wrong product code sent", "Failed and Retry", "Failed and Retry"},
	"60": {"60", "Access is not allowed", "Tokopedia’s IP is blocked or no API access is allowed", "Failed and Retry", "Pending"},
	"61": {"61", "Server maintenance", "Partner’s server ongoing maintenance", "Failed and Retry", "Pending"},
	"62": {"62", "Server error", "Internal error on Partner’s server", "Failed and Retry", "Pending"},
	"63": {"63", "Biller maintenance", "Biller’s server ongoing maintenance", "Failed and Retry", "Pending"},
	"64": {"64", "Biller error", "Internal error on Biller’s server", "Failed and Retry", "Pending"},

	// --- Bill Related Errors ---
	"10": {"10", "Bill already paid", "Bill already paid by the user, no new bill generated yet", "Failed and Refund", "Failed and Refund"},
	"11": {"11", "Bill not available", "No bill found for the given client identifier number", "Failed and Refund", "Failed and Refund"},

	// --- Product/Eligibility Errors ---
	"14": {"14", "Ineligible Product", "Product is not eligible to given user or client number", "Failed and Refund", "Pending"},

	// --- User/Account Errors ---
	"20": {"20", "Unregistered number", "Invalid client identifier number", "Failed and Refund", "Failed and Refund"},
	"21": {"21", "Number blocked", "The number is blocked or blacklisted", "Failed and Refund", "Failed and Refund"},
	"22": {"22", "Online payment blocked", "Cannot do payment on the online channel, need to do offline payment method on the biller", "Failed and Refund", "Failed and Refund"},
	"23": {"23", "Limit exceeded", "User had exceeded top-up or payment limit", "Failed and Refund", "Failed and Refund"},

	// --- Authentication/Authorization Errors ---
	"30": {"30", "Invalid token", "Wrong or expired token sent", "Failed and Refund", "Pending"},
	"31": {"31", "Invalid credential", "Wrong credential sent", "Failed and Refund", "Pending"},
	"32": {"32", "Invalid signature", "Cannot validate payload with given signature", "Failed and Refund", "Pending"},
	"33": {"33", "Decryption failed", "Failed to do decryption with given key", "Failed and Refund", "Pending"},

	// --- Transaction/Amount Errors ---
	"44": {"44", "Invalid transaction amount", "Wrong amount sent or does not match with user existing bill", "Failed and Refund", "Failed and Refund"},

	// --- Errors [part PPS]---
	"500": {"500", "Internal Server Error", "Internal server error", "Failed and Retry", "Pending"},
	"401": {"401", "Unauthorized", "Unauthorized", "Failed and Retry", "Pending"},
	"99":  {"99", "Other error", "Error message not found in mapping database", "Failed and Refund", "Pending"},
}

// GetResponseCode returns the ResponseCode struct for a given code.
// If the code is not found, returns a zero-value ResponseCode and false.
func GetResponseCode(code string) (ResponseCode, bool) {
	rc, ok := ResponseCodeMap[code]
	return rc, ok
}

// UltimaErrorMessageMap maps Ultima error messages to Tokopedia response codes
// if error message not found in mapping, returns "64" (Biller error) as default.
var UltimaErrorMessageMap = map[string]string{
	"yang anda masukkan salah":    "20", // Unregistered number
	"kwh melebihi batas maksimum": "21", // Number blocked
	"no payment":                  "22", // Online payment blocked
	"failed system (timeout)":     "41", // Biller timeout
	"sistem sedang kendala":       "64", // Biller error
	"cut-off":                     "63", // Biller maintenance (partial match for dynamic cutoff error message)
}

// MapUltimaErrorMessageToResponseCode maps Ultima error message to Tokopedia response code and message.
// Returns response code and response message.
// If message not found in mapping, returns "64" (Biller error) as default.
func MapUltimaErrorMessageToResponseCode(ultimaErrorMessage string) (string, string) {
	// Try exact match first
	if code, ok := UltimaErrorMessageMap[strings.ToLower(ultimaErrorMessage)]; ok {
		if rc, found := GetResponseCode(code); found {
			return rc.Code, rc.Message
		}
	}

	// Try partial/contains match for dynamic error messages
	// Sort patterns by length (longest first) to match most specific pattern
	var matchedCode string
	var maxLength int

	ultimaLower := strings.ToLower(ultimaErrorMessage)
	for pattern, code := range UltimaErrorMessageMap {
		if strings.Contains(ultimaLower, pattern) && len(pattern) > maxLength {
			matchedCode = code
			maxLength = len(pattern)
		}
	}

	if matchedCode != "" {
		if rc, found := GetResponseCode(matchedCode); found {
			return rc.Code, rc.Message
		}
	}

	// Default to other error if not found in mapping
	return "99", "Other error"
}

// OracleErrorMessageMap maps Oracle error messages to Tokopedia response codes
// Supports both exact match and partial matching for dynamic messages
var OracleErrorMessageMap = map[string]string{

	"h2h - error :": "61", // Server maintenance (partial match for dynamic cutoff error message)

	// Authentication & Signature Errors
	"error 02 : signature tidak benar, komponen tidak syah.": "32", // Invalid signature (changed from 20 to 32)
	"error 01 : signature tidak benar.":                      "32", // Invalid signature

	// Product Mapping & Availability Errors
	"produk belum dimapping":             "14", // Ineligible Product
	"kode voucher is not found":          "14", // Ineligible Product
	"kode voucher":                       "14", // Ineligible Product (partial match for dynamic voucher)
	"tidak tersedia untuk account anda.": "14", // Ineligible Product (partial match for dynamic voucher)
	"fee amount":                         "14", // Ineligible Product (partial match for dynamic voucher)
	"belum disetting":                    "14", // Ineligible Product (partial match for dynamic voucher)

	// IP & Access Errors
	"sell error 02 : ip belum di setting.":         "60", // Access is not allowed
	"ip anda":                                      "60", // Access is not allowed (partial match for dynamic IP)
	"tidak sama dengan settingan":                  "60", // Access is not allowed (partial match)
	"anda tidak diperkenankan transaksi lewat web": "60", // Access is not allowed (partial match)

	// Queue & Member Errors
	"sell error 02 : queue setting is not ready": "60", // Access is not allowed (changed from 20 to 60)

	// Price & Stock Errors
	"harga jual belum di setting untuk anda.": "14", // Ineligible Product (changed from 50 to 14)
	"stock voucher":                  "14", // Ineligible Product (changed from 52 to 14)
	"habis, transaksi kami batalkan": "14", // Ineligible Product (changed from 52 to 14)

	// Balance Errors
	"deposit anda kurang untuk memenuhi penjualan.": "43", // Insufficient balance

	// Transaction Errors
	"no transaksi":    "12", // Transaction not found (partial match for dynamic txn number)
	"tidak ditemukan": "12", // Transaction not found (partial match)
}

// MapOracleErrorToResponseCode maps Oracle error message to Tokopedia response code and message.
// Returns response code and response message.
// If message not found in mapping, returns "62" (Server error) as default.
func MapOracleErrorToResponseCode(oracleErrorMessage string) (string, string) {
	// Try exact match first
	if code, ok := OracleErrorMessageMap[strings.ToLower(oracleErrorMessage)]; ok {
		if rc, found := GetResponseCode(code); found {
			return rc.Code, rc.Message
		}
	}

	// Try partial/contains match for dynamic error messages
	// Sort patterns by length (longest first) to match most specific pattern
	var matchedCode string
	var maxLength int

	for pattern, code := range OracleErrorMessageMap {
		if contains(oracleErrorMessage, pattern) && len(pattern) > maxLength {
			matchedCode = code
			maxLength = len(pattern)
		}
	}

	if matchedCode != "" {
		if rc, found := GetResponseCode(matchedCode); found {
			return rc.Code, rc.Message
		}
	}

	// Default to other error if not found in mapping
	return "99", "Other error"
}

// contains checks if string s contains substring substr (case-insensitive)
func contains(s, substr string) bool {
	return strings.Contains(strings.ToLower(s), strings.ToLower(substr))
}

// indexOf returns the index of substring in string, -1 if not found
func indexOf(s, substr string) int {
	if len(substr) == 0 {
		return 0
	}
	if len(s) < len(substr) {
		return -1
	}
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return i
		}
	}
	return -1
}
