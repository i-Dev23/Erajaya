# Unit Test Report — pps-services-gateway-smb

## Ringkasan

| Package | Tests | Pass | Fail | Coverage Target |
|---------|-------|------|------|-----------------|
| `internal/usecase/plntoken` | 13 | ✅ 13 | 0 | Usecase logic |
| `internal/util` | 20 | ✅ 20 | 0 | RC mapping + trxid |
| `pkg/smb` | 11 | ✅ 11 | 0 | HTTP client SMB API |
| `internal/infrastructure/smbclient` | 4 | ✅ 4 | 0 | Adapter layer |
| `internal/infrastructure/mqpublisher` | 3 | ✅ 3 | 0 | Message format |
| **Total** | **51** | **✅ 51** | **0** | |

## Cara Jalankan

```bash
# Semua test
go test -v -count=1 ./...

# Usecase saja
go test -v -count=1 ./internal/usecase/plntoken/

# Util saja
go test -v -count=1 ./internal/util/

# Dengan coverage
go test -cover ./...

# Coverage HTML report
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
```

---

## Test Detail: `internal/usecase/plntoken`

File: `pln_token_usecase_test.go`

### ProcessTransaction (6 test cases)

| # | Test Name | Skenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestProcessTransaction_InquirySuccess_PaymentSuccess` | Inquiry OK → Payment OK | Status=SUCCESS, StatusToBe=F, Token ada |
| 2 | `TestProcessTransaction_InquiryError` | Inquiry network error (timeout) | Status=FAILED, StatusToBe=C |
| 3 | `TestProcessTransaction_InquiryNonSuccess` | Inquiry return code 94 (Error Inquiry Data) | Status=FAILED, StatusToBe=C |
| 4 | `TestProcessTransaction_PaymentFailed` | Inquiry OK → Payment return code 93 | Status=FAILED, StatusToBe=C |
| 5 | `TestProcessTransaction_PaymentPending` | Inquiry OK → Payment return code 28 (timeout) | Status=PENDING, NeedRetry=true, RefID ada |
| 6 | `TestProcessTransaction_PaymentNetworkError` | Inquiry OK → Payment network error | Status=PENDING, NeedRetry=true |

### RetryAdvice (7 test cases)

| # | Test Name | Skenario | Expected |
|---|-----------|----------|----------|
| 7 | `TestRetryAdvice_SuccessOnFirstAttempt` | Advice langsung return 00 | Status=SUCCESS, Token ada |
| 8 | `TestRetryAdvice_FailedOnFirstAttempt` | Advice langsung return 93 | Status=FAILED, StatusToBe=C |
| 9 | `TestRetryAdvice_SuccessOnThirdAttempt` | 2x pending (28) → 3rd attempt sukses (00) | Status=SUCCESS, attempt=3 |
| 10 | `TestRetryAdvice_ExhaustedAllRetries` | Semua attempt return 68 (pending) | Status=FAILED (retry habis) |
| 11 | `TestRetryAdvice_NilRetryConfig` | RetryConfig = nil | Status=FAILED langsung |
| 12 | `TestRetryAdvice_NetworkErrorThenSuccess` | 1st attempt network error → 2nd sukses | Status=SUCCESS, Token ada |
| 13 | `TestRetryAdvice_ContextCancelled` | Context di-cancel sebelum retry | Status=FAILED |

---

## Test Detail: `internal/util`

File: `rc_test.go`

### ResolveRCPPS (10 test cases)

| # | Input | Expected RC | Keterangan |
|---|-------|-------------|------------|
| 1 | `"00"` | 0 | Success |
| 2 | `"28"` | 9 | Pending timeout |
| 3 | `"68"` | 9 | Pending timeout |
| 4 | `""` | 9 | Empty = pending |
| 5 | `"93"` | 1 | Error Payment |
| 6 | `"94"` | 1 | Error Inquiry Data |
| 7 | `"97"` | 1 | Error Client Data |
| 8 | `"98"` | 1 | Error Parameter |
| 9 | `"99"` | 1 | Error Credential |
| 10 | `"XYZ"` | 1 | Unknown = failed |

### StatusToBeFromRC (4 test cases)

| # | Input RC | Expected | Keterangan |
|---|----------|----------|------------|
| 1 | 0 | `"F"` | Final success |
| 2 | 1 | `"C"` | Cancel/failed |
| 3 | 9 | `"S"` | Still processing |
| 4 | 99 | `"C"` | Unknown = failed |

---

## Teknik Testing yang Dipakai

### Mock Pattern
- `mockSMBClient` — mock interface `SMBClient` dengan function fields
- `mockLogger` — no-op logger (gak print apa-apa saat test)
- Gak pakai library mock external (testify/mock) — cukup pakai Go native

### Table-Driven Tests
- `TestResolveRCPPS` dan `TestStatusToBeFromRC` pakai pattern table-driven
- Setiap case punya `name`, `input`, `expected`

### Retry Testing
- `RetryAdvice` test pakai `WaitDuration: 10ms` supaya test cepat
- Test context cancellation pakai `context.WithCancel` + cancel langsung
- Test "success on Nth attempt" pakai counter di closure

---

## Yang Belum Di-test

| Package | Alasan |
|---------|--------|
| `infrastructure/rabbitmq` | Butuh RabbitMQ running (integration test) |
| `infrastructure/postgres` | Butuh PostgreSQL running (integration test) |

> Semua yang bisa di-unit-test tanpa external dependency sudah di-test.
> Sisanya butuh integration test dengan infra yang jalan (RabbitMQ, Postgres).

---

## Test Detail: `pkg/smb` (HTTP Client)

File: `pln_token_test.go`

Teknik: pakai `httptest.NewServer` untuk mock HTTP server SMB API.

| # | Test Name | Skenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestInquiryPLNToken_Success` | Server return 00 + data lengkap | Parse response benar, data ada |
| 2 | `TestInquiryPLNToken_ErrorResponse` | Server return 94 (Error Inquiry) | ResponseCode=94, Data=nil |
| 3 | `TestInquiryPLNToken_ServerDown` | Server unreachable | Error non-nil |
| 4 | `TestInquiryPLNToken_InvalidJSON` | Server return bukan JSON | Error non-nil, rawBody tetap ada |
| 5 | `TestPaymentPLNToken_Success` | Server return 00 + token | Token ada, ResponseCode=00 |
| 6 | `TestPaymentPLNToken_Pending` | Server return 28 (pending) | ResponseCode=28 |
| 7 | `TestPaymentPLNToken_ServerDown` | Server unreachable | Error non-nil |
| 8 | `TestAdvicePLNToken_Success` | Server return 00 + token | Token ada |
| 9 | `TestAdvicePLNToken_StillPending` | Server return 68 (pending) | ResponseCode=68 |
| 10 | `TestAdvicePLNToken_ServerDown` | Server unreachable | Error non-nil |
| 11 | `TestGenerateSignature` | Same input = same output, different input = different | MD5 32 chars, deterministic |

---

## Test Detail: `internal/infrastructure/smbclient` (Adapter)

File: `adapter_test.go`

Teknik: pakai `httptest.NewServer` + real `smb.Client` → test adapter end-to-end.

| # | Test Name | Skenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestAdapter_InquiryPLNToken_Success` | Full flow: adapter → smb.Client → mock server | Data ter-mapping ke contract response |
| 2 | `TestAdapter_PaymentPLNToken_Success` | Full flow payment | Token + SerialNumber ter-mapping |
| 3 | `TestAdapter_AdvicePLNToken_Success` | Full flow advice | Token ter-mapping |
| 4 | `TestAdapter_InquiryPLNToken_ServerError` | Server unreachable | Error non-nil |

---

## Test Detail: `internal/infrastructure/mqpublisher` (Message Format)

File: `message_test.go`

| # | Test Name | Skenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestNewProviderPublishMessage` | Create message | Source=PROVIDER, data fields benar |
| 2 | `TestProviderPublishMessage_JSONFormat` | Marshal → unmarshal roundtrip | JSON punya field "source" dan "data" |
| 3 | `TestProviderPublishData_JSONFieldNames` | Cek semua JSON field names | 10 fields: msg_id, status_to_be, dll |

---

## Test Detail: `internal/util` (Tambahan)

File: `trxid_test.go`

| # | Test Name | Skenario | Expected |
|---|-----------|----------|----------|
| 1 | `TestGenerateTransactionID/normal` | mid + msgID ada | Contains SMB-, mid, msgID, date |
| 2 | `TestGenerateTransactionID/empty_mid` | mid kosong | Contains "UNKNOWN" |
| 3 | `TestGenerateTransactionID/empty_msgID` | msgID kosong | Contains "0" |
| 4 | `TestGenerateTransactionID/both_empty` | Dua-duanya kosong | Contains "UNKNOWN" dan "0" |
| 5 | `TestGenerateTransactionID_Uniqueness` | Timestamp beda | ID berbeda |
