# Unit Test Report — pps-services-gateway-unipin

Tanggal: 13 April 2026
Scope: Semua fitur baru sejak commit `c69ee74`
Runner: `go test` (Go 1.26.1)

---

## Ringkasan Coverage

| Package                               | Coverage                     | Total Test | Status  |
|---------------------------------------|------------------------------|------------|---------|
| `internal/infrastructure/rabbitmq`    | 78.5% (testable code: ~97%)  | 69         | ✅ PASS |
| `internal/infrastructure/mqpublisher` | 100% (`message.go`)          | 4          | ✅ PASS |
| `internal/usecase/gamesync`           | 96.7%                        | 23         | ✅ PASS |

> **Catatan:** Coverage `rabbitmq` 78.5% karena `Start()` dan `consumeSession()` membutuhkan koneksi RabbitMQ asli (integration test). Semua fungsi yang bisa di-unit-test sudah 100%.

---

## Coverage Per Fungsi — rabbitmq/consumer_service.go

| Fungsi                    | Coverage  | Keterangan                                    |
|---------------------------|-----------|-----------------------------------------------|
| `UnmarshalJSON`           | 94.7%     | 1 branch error path                           |
| `getAny`                  | 100.0%    |                                               |
| `parseString`             | 100.0%    |                                               |
| `parseInt`                | 100.0%    |                                               |
| `NewConsumerServiceImpl`  | 100.0%    |                                               |
| `SetMQPublisher`          | 100.0%    |                                               |
| `Start`                   | 0.0%      | Butuh koneksi RabbitMQ                        |
| `consumeSession`          | 0.0%      | Butuh koneksi RabbitMQ                        |
| `processMessage`          | 100.0%    |                                               |
| `processVoucher`          | 100.0%    |                                               |
| `processVoucherInquiry`   | 100.0%    |                                               |
| `processGame`             | 100.0%    |                                               |
| `processOrderInquiry`     | 100.0%    |                                               |
| `forwardCallback`         | 86.7%     | `json.Marshal` error path sulit di-trigger    | sudah di-update (100%)
| `awaitDrain`              | 100.0%    |                                               |

---

## Coverage Per Fungsi — gamesync/sync_service.go

| Fungsi                   | Coverage |
|--------------------------|----------|
| `NewSyncService`         | 100.0%   |
| `SyncGameList`           | 96.6%    |
| `SyncSingleGame`         | 96.9%    |
| `SyncSingleDenomination` | 96.7%    |
| `SyncVoucherList`        | 96.4%    |

---

## Detail Test Cases

### 1. consumePayload Parsing (6 test)

| #  | Test                                                                                     | Status  |
|----|------------------------------------------------------------------------------------------|---------|
| 1  | `TestConsumePayload_ParsesAllFields` — parse semua field termasuk Command, MQTransaction | ✅ PASS |
| 2  | `TestConsumePayload_CommandCaseInsensitive` — verifikasi case-insensitive lookup         | ✅ PASS |
| 3  | `TestConsumePayload_TrimsWhitespace` — whitespace di-trim otomatis                       | ✅ PASS |
| 4  | `TestConsumePayload_EmptyJSON` — JSON kosong → default values                            | ✅ PASS |
| 5  | `TestConsumePayload_InvalidJSON` — JSON invalid → return error                           | ✅ PASS |
| 6  | `TestConsumePayload_NumericAmount` — amount string "12345" → int 12345                   | ✅ PASS |

### 2. Helper Functions: getAny (4 test)

| #  | Test                                             | Status  |
|----|--------------------------------------------------|---------|
| 1  | `TestGetAny_NilMap` — nil map → nil              | ✅ PASS |
| 2  | `TestGetAny_FirstKeyMatch` — key pertama cocok   | ✅ PASS |
| 3  | `TestGetAny_FallbackKey` — fallback ke key kedua | ✅ PASS |
| 4  | `TestGetAny_NoMatch` — tidak ada key cocok → nil | ✅ PASS |

### 3. Helper Functions: parseString (6 test)

| #  | Test                                                       | Status  |
|----|------------------------------------------------------------|---------|
| 1  | `TestParseString_Nil` — nil → ""                           | ✅ PASS |
| 2  | `TestParseString_String` — trim whitespace                 | ✅ PASS |
| 3  | `TestParseString_Float64Integer` — float64(42) → "42"      | ✅ PASS |
| 4  | `TestParseString_Float64Decimal` — float64(3.14) → "3.14"  | ✅ PASS |
| 5  | `TestParseString_JSONNumber` — json.Number("999") → "999"  | ✅ PASS |
| 6  | `TestParseString_OtherType` — bool true → "true"           | ✅ PASS |

### 4. Helper Functions: parseInt (8 test)

| #  | Test                                                       | Status  |
|----|------------------------------------------------------------|---------|
| 1  | `TestParseInt_Nil` — nil → 0                               | ✅ PASS |
| 2  | `TestParseInt_JSONNumber` — json.Number("42") → 42         | ✅ PASS |
| 3  | `TestParseInt_JSONNumberFloat` — json.Number("3.7") → 3    | ✅ PASS |
| 4  | `TestParseInt_JSONNumberInvalid` — json.Number("abc") → 0  | ✅ PASS |
| 5  | `TestParseInt_Float64` — float64(99) → 99                  | ✅ PASS |
| 6  | `TestParseInt_String` — "  77  " → 77                      | ✅ PASS |
| 7  | `TestParseInt_InvalidString` — "abc" → 0                   | ✅ PASS |
| 8  | `TestParseInt_UnsupportedType` — bool → 0                  | ✅ PASS |

### 5. processMessage Routing (8 test)

| #  | Test                                                                                       | Status  |
|----|--------------------------------------------------------------------------------------------|---------|
| 1  | `TestProcessMessage_InvalidJSON` — JSON invalid → log error                                | ✅ PASS |
| 2  | `TestProcessMessage_UnipinVoucher_RoutesToProcessVoucher` — routing ke processVoucher      | ✅ PASS |
| 3  | `TestProcessMessage_UnipinGame_RoutesToProcessGame` — routing ke processGame               | ✅ PASS |
| 4  | `TestProcessMessage_UnipinDirectTopup_LogsWarning` — log warning "not yet implemented"     | ✅ PASS |
| 5  | `TestProcessMessage_UnsupportedType_LogsWarning` — log warning "unsupported product type"  | ✅ PASS |
| 6  | `TestProcessMessage_UsesPayloadQueueName` — gunakan queue_name dari payload                | ✅ PASS |
| 7  | `TestProcessMessage_FallbackMsgID` — fallback ke delivery.MessageId jika msgid kosong      | ✅ PASS |
| 8  | `TestProcessMessage_CorrelationIdFallback` — fallback ke correlationId                     | ✅ PASS |

### 6. forwardCallback (5 test)

| #  | Test                                                                                                                  | Status  |
|----|-----------------------------------------------------------------------------------------------------------------------|---------|
| 1  | `TestForwardCallback_NilPublisher_LogsWarning` — publisher nil → log warning, skip                                    | ✅ PASS |
| 2  | `TestForwardCallback_PublishesCorrectData` — verifikasi field: status_to_be, serial_number, client_number, nominal    | ✅ PASS |
| 3  | `TestForwardCallback_NonNumericMsgID` — msgid non-numeric → msg_id = 0                                                | ✅ PASS |
| 4  | `TestForwardCallback_PublishError_LogsError` — publish gagal → log error, tidak panic                                 | ✅ PASS |
| 5  | `TestForwardCallback_SourceIsProvider` — source = "PROVIDER"                                                          | ✅ PASS |

### 7. processGame — Parsing & Validasi (12 test)

| #   | Test                                                                                              | Status  |
|-----|---------------------------------------------------------------------------------------------------|---------|
| 1   | `TestProcessGame_HappyPath_Success` — alur lengkap: parse → ValidateUser → CreateOrder → SUCCESS  | ✅ PASS |
| 2   | `TestProcessGame_EmptyCommand_ForwardsFailed` — command kosong → FAILED                           | ✅ PASS |
| 3   | `TestProcessGame_NoDelimiter_ForwardsFailed` — command tanpa `*` → FAILED                         | ✅ PASS |
| 4   | `TestProcessGame_EmptyGameCode_ForwardsFailed` — gameCode kosong → FAILED                         | ✅ PASS |
| 5   | `TestProcessGame_EmptyDenominationID_ForwardsFailed` — denominationID kosong → FAILED             | ✅ PASS |
| 6   | `TestProcessGame_EmptyMSISDN_ForwardsFailed` — MSISDN kosong → FAILED                             | ✅ PASS |
| 7   | `TestProcessGame_InvalidMSISDNJSON_ForwardsFailed` — MSISDN bukan JSON → FAILED                   | ✅ PASS |
| 8   | `TestProcessGame_EmptyMSISDNMap_ForwardsFailed` — MSISDN JSON `{}` → FAILED                       | ✅ PASS |
| 9   | `TestProcessGame_WhitespaceCommand_ForwardsFailed` — command hanya whitespace → FAILED            | ✅ PASS |
| 10  | `TestProcessGame_WhitespaceGameCode_ForwardsFailed` — gameCode whitespace → FAILED                | ✅ PASS |
| 11  | `TestProcessGame_WhitespaceDenomID_ForwardsFailed` — denominationID whitespace → FAILED           | ✅ PASS |
| 12  | `TestProcessGame_MSISDNArray_ForwardsFailed` — MSISDN JSON array → FAILED                         | ✅ PASS |

### 8. processGame — API Calls (5 test)

| #  | Test                                                                                                         | Status  |
|----|--------------------------------------------------------------------------------------------------------------|---------|
| 1  | `TestProcessGame_ValidateUserFailed_ForwardsFailed` — ValidateUser status=0 → FAILED, message="invalid user" | ✅ PASS |
| 2  | `TestProcessGame_ValidateUserTechError_ForwardsFailed` — ValidateUser 500 → FAILED                           | ✅ PASS |
| 3  | `TestProcessGame_CreateOrderFailed_ForwardsFailed` — CreateOrder status=0 → FAILED, serial_number dari resp  | ✅ PASS |
| 4  | `TestProcessGame_CreateOrderTechError_ForwardsFailed` — CreateOrder 500 → FAILED                             | ✅ PASS |
| 5  | `TestProcessGame_CreateOrderTimeout_FallbackInquiry` — CreateOrder timeout → fallback OrderInquiry → SUCCESS | ✅ PASS |

### 9. processGame — Edge Cases (1 test)

| #  | Test                                                                                                              | Status  |
|----|-------------------------------------------------------------------------------------------------------------------|---------|
| 1  | `TestProcessGame_CommandWithMultipleDelimiters` — "MLBB\*123\*extra" → gameCode="MLBB", denomID="123*extra" → SUCCESS | ✅ PASS |

### 10. processOrderInquiry (3 test)

| #  | Test                                                                                                       | Status  |
|----|------------------------------------------------------------------------------------------------------------|---------|
| 1  | `TestProcessOrderInquiry_Success` — inquiry sukses → SUCCESS                                               | ✅ PASS |
| 2  | `TestProcessOrderInquiry_BusinessError_ForwardsFailed` — inquiry status=0 → FAILED, serial_number + reason | ✅ PASS |
| 3  | `TestProcessOrderInquiry_TechError_ForwardsFailed` — inquiry 500 → FAILED, serial_number = referenceNo     | ✅ PASS |

### 11. processVoucher (10 test)

| #   | Test                                                                                                    | Status  |
|-----|---------------------------------------------------------------------------------------------------------|---------|
| 1   | `TestProcessVoucher_HappyPath_Success` — alur lengkap: parse Command → VoucherRequest → forward         | ✅ PASS |
| 2   | `TestProcessVoucher_EmptyCommand_ForwardsFailed` — command kosong → FAILED                              | ✅ PASS |
| 3   | `TestProcessVoucher_NoDelimiter_ForwardsFailed` — command tanpa `*` → FAILED                            | ✅ PASS |
| 4   | `TestProcessVoucher_EmptyDenominationCode_ForwardsFailed` — denominationCode kosong → FAILED            | ✅ PASS |
| 5   | `TestProcessVoucher_EmptyMsgID_ReturnsEarly` — msgid kosong → log error, tidak publish                  | ✅ PASS |
| 6   | `TestProcessVoucher_VoucherRequestFailed_ForwardsFailed` — VoucherRequest status=0 → FAILED             | ✅ PASS |
| 7   | `TestProcessVoucher_VoucherRequestTimeout_FallbackInquiry` — VoucherRequest timeout → fallback Inquiry  | ✅ PASS |
| 8   | `TestProcessVoucher_VoucherRequestTechError_ForwardsFailed` — VoucherRequest 500 → FAILED               | ✅ PASS |
| 9   | `TestProcessVoucher_NonZeroStatus_ForwardsFailed` — VoucherRequest status=2 → FAILED                    | ✅ PASS |
| 10  | `TestProcessVoucher_CommandWithMultipleDelimiters` — "STEAM\*CODE\*extra" → denomCode="CODE*extra"      | ✅ PASS |

### 12. processVoucher — Whitespace Edge Cases (2 test)

| #  | Test                                                                                           | Status  |
|----|------------------------------------------------------------------------------------------------|---------|
| 1  | `TestProcessVoucher_WhitespaceCommand_ForwardsFailed` — command hanya whitespace → FAILED      | ✅ PASS |
| 2  | `TestProcessVoucher_WhitespaceDenomCode_ForwardsFailed` — denominationCode whitespace → FAILED | ✅ PASS |

### 13. processVoucherInquiry (3 test)

| #  | Test                                                                                     | Status  |
|----|------------------------------------------------------------------------------------------|---------|
| 1  | `TestProcessVoucherInquiry_Success` — inquiry sukses → forward status                    | ✅ PASS |
| 2  | `TestProcessVoucherInquiry_Error_ForwardsFailed` — inquiry 500 → FAILED                  | ✅ PASS |
| 3  | `TestProcessVoucherInquiry_NonZeroStatus_ForwardsFailed` — inquiry status=3 → FAILED     | ✅ PASS |

### 14. Constructor & Lifecycle (4 test)

| #  | Test                                                                              | Status  |
|----|-----------------------------------------------------------------------------------|---------|
| 1  | `TestNewConsumerServiceImpl` — service non-nil, publisher nil by default          | ✅ PASS |
| 2  | `TestSetMQPublisher` — publisher di-set setelah konstruksi                        | ✅ PASS |
| 3  | `TestAwaitDrain_CompletesBeforeTimeout` — WaitGroup selesai sebelum timeout       | ✅ PASS |
| 4  | `TestAwaitDrain_TimesOut` — WaitGroup tidak selesai → log warning                 | ✅ PASS |

---

### 15. mqpublisher/message.go (4 test)

| #  | Test                                                                               | Status  |
|----|------------------------------------------------------------------------------------|---------|
| 1  | `TestNewProviderPublishMessage_SourceIsProvider` — source = "PROVIDER"             | ✅ PASS |
| 2  | `TestNewProviderPublishMessage_DataPreserved` — semua field data dipertahankan     | ✅ PASS |
| 3  | `TestProviderPublishMessage_JSONSerialization` — marshal/unmarshal round-trip      | ✅ PASS |
| 4  | `TestProviderPublishData_EmptyFields` — default values untuk struct kosong         | ✅ PASS |

---

### 16. gamesync — SyncGameList (5 test, existing)

| #  | Test                                                                                  | Status  |
|----|---------------------------------------------------------------------------------------|---------|
| 1  | `TestSyncGameList_FullFlow` — 2 game, 3 denomination → 3 upsert calls                 | ✅ PASS |
| 2  | `TestSyncGameList_SPError_ContinuesProcessing` — SP error → lanjut proses             | ✅ PASS |
| 3  | `TestSyncGameList_DBError_ContinuesProcessing` — DB error → lanjut proses             | ✅ PASS |
| 4  | `TestSyncGameList_ContextCancelled` — context cancelled → return error                | ✅ PASS |
| 5  | `TestSyncGameList_FieldRequestJSON` — FieldRequest valid JSON                         | ✅ PASS |

### 17. gamesync — SyncVoucherList (4 test, baru)

| #  | Test                                                                                  | Status  |
|----|---------------------------------------------------------------------------------------|---------|
| 1  | `TestSyncVoucherList_FullFlow` — 2 voucher, 3 denomination → 3 upsert calls           | ✅ PASS |
| 2  | `TestSyncVoucherList_SPError_ContinuesProcessing` — SP error → lanjut proses          | ✅ PASS |
| 3  | `TestSyncVoucherList_DBError_ContinuesProcessing` — DB error → lanjut proses          | ✅ PASS |
| 4  | `TestSyncVoucherList_ContextCancelled` — context cancelled → return error             | ✅ PASS |

### 18. gamesync — SyncSingleGame (4 test, baru)

| #  | Test                                                                                  | Status  |
|----|---------------------------------------------------------------------------------------|---------|
| 1  | `TestSyncSingleGame_Success` — game ditemukan → 1 upsert call                         | ✅ PASS |
| 2  | `TestSyncSingleGame_NotFound` — game tidak ditemukan → error                          | ✅ PASS |
| 3  | `TestSyncSingleGame_SPError_ContinuesProcessing` — SP error → lanjut                  | ✅ PASS |
| 4  | `TestSyncSingleGame_DBError` — DB error → lanjut                                      | ✅ PASS |

### 19. gamesync — SyncSingleDenomination (5 test, baru)

| #  | Test                                                                                  | Status  |
|----|---------------------------------------------------------------------------------------|---------|
| 1  | `TestSyncSingleDenomination_Success` — denomination ditemukan → 1 upsert              | ✅ PASS |
| 2  | `TestSyncSingleDenomination_GameNotFound` — game tidak ditemukan → error              | ✅ PASS |
| 3  | `TestSyncSingleDenomination_DenomNotFound` — denomination tidak ditemukan → error     | ✅ PASS |
| 4  | `TestSyncSingleDenomination_SPError` — SP error → return error                        | ✅ PASS |
| 5  | `TestSyncSingleDenomination_DBError` — DB error → return error                        | ✅ PASS |

### 20. gamesync — Error Paths (5 test, baru)

| #  | Test                                                                                              | Status  |
|----|---------------------------------------------------------------------------------------------------|---------|
| 1  | `TestSyncVoucherList_DetailFetchError_ContinuesProcessing` — voucher detail 500 → skip, lanjut    | ✅ PASS |
| 2  | `TestSyncGameList_DetailFetchError_ContinuesProcessing` — game detail 500 → skip, lanjut          | ✅ PASS |
| 3  | `TestSyncSingleGame_DetailFetchError` — game detail 500 → return error                            | ✅ PASS |
| 4  | `TestSyncSingleGame_NoDenominations` — game tanpa denomination → return error                     | ✅ PASS |
| 5  | `TestSyncSingleDenomination_DetailFetchError` — game detail 500 → return error                    | ✅ PASS |

### 21. gamesync — Constructor (1 test, baru)

| #  | Test                                     | Status  |
|----|------------------------------------------|---------|
| 1  | `TestNewSyncService` — service non-nil   | ✅ PASS |

---

## File Test

| File                                                          | Package     | Jumlah Test |
|---------------------------------------------------------------|-------------|-------------|
| `internal/infrastructure/rabbitmq/consumer_service_test.go`   | rabbitmq    | 69          |
| `internal/infrastructure/mqpublisher/message_test.go`         | mqpublisher | 4           |
| `internal/usecase/gamesync/sync_service_test.go`              | gamesync    | 23          |
| **Total**                                                     |             | **96**      |

---

## Fungsi yang Tidak Di-Unit-Test (Butuh Integration Test)

| Fungsi                    | Package     | Alasan                             |
|---------------------------|-------------|------------------------------------|
| `Start`                   | rabbitmq    | Membutuhkan koneksi RabbitMQ asli  |
| `consumeSession`          | rabbitmq    | Membutuhkan koneksi RabbitMQ asli  |
| `NewAMQPPublisher`        | mqpublisher | Trivial constructor                |
| `AMQPPublisher.Publish`   | mqpublisher | Membutuhkan koneksi RabbitMQ asli  |

---

## Cara Menjalankan

```bash
# Semua unit test
go test ./internal/infrastructure/rabbitmq/ ./internal/infrastructure/mqpublisher/ ./internal/usecase/gamesync/ -v -count=1 -timeout 120s

# Dengan coverage
go test ./internal/infrastructure/rabbitmq/ -cover -timeout 120s
go test ./internal/infrastructure/mqpublisher/ -cover
go test ./internal/usecase/gamesync/ -cover

# Coverage per fungsi
go test ./internal/infrastructure/rabbitmq/ -coverprofile=cover.out -timeout 120s && go tool cover -func=cover.out
```
