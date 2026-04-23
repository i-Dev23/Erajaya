# Rencana Implementasi: UniPin Transaction Flow

## Overview

Mengimplementasikan alur transaksi lengkap untuk dua product type: **unipin-game** (baru) dan **unipin-voucher** (update parsing). Semua perubahan terpusat di `internal/infrastructure/rabbitmq/consumer_service.go`. Infrastruktur downstream (`forwardCallback`, `MQPublisher`, UniPin client) sudah tersedia dan digunakan apa adanya.

## Tasks

- [x] 1. Tambahkan field Command pada consumePayload
  - [x] 1.1 Modifikasi struct `consumePayload` di `internal/infrastructure/rabbitmq/consumer_service.go`
    - Tambahkan field `Command string` ke struct `consumePayload`
    - Di method `UnmarshalJSON`, tambahkan parsing: `p.Command = parseString(getAny(raw, "command"))`
    - Mengikuti pola field lain: case-insensitive lookup via `getAny`, lalu `parseString` yang sudah melakukan `strings.TrimSpace`
    - _Requirements: 1.1, 1.2, 1.3_

  - [ ]* 1.2 Tulis property test untuk parsing Command (Property 1: Parsing Command dengan Delimiter)
    - **Property 1: Parsing Command dengan Delimiter Mengekstrak Bagian yang Benar**
    - **Validates: Requirements 1.2, 1.3, 2.1, 8.1, 8.2**
    - Untuk sembarang string `command` yang mengandung tepat satu `*`, `strings.SplitN(command, "*", 2)` harus menghasilkan dua bagian yang benar. Setelah `TrimSpace`, `part1 + "*" + part2` harus merekonstruksi command yang di-trim
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: unipin-transaction-flow, Property 1: Parsing Command dengan Delimiter Mengekstrak Bagian yang Benar`

- [x] 2. Implementasi method processGame — parsing dan validasi input
  - [x] 2.1 Tambahkan method `processGame` pada `ConsumerServiceImpl` di `consumer_service.go`
    - Signature: `func (s *ConsumerServiceImpl) processGame(ctx context.Context, payload *consumePayload, queueName, msgID string)`
    - Implementasikan parsing Command: `strings.SplitN(payload.Command, "*", 2)` → `gameCode`, `denominationID`
    - Validasi: Command tidak kosong, ada delimiter `*`, `gameCode` tidak kosong setelah trim, `denominationID` tidak kosong setelah trim
    - Setiap validasi gagal → log error, `forwardCallback` FAILED, return
    - Implementasikan parsing MSISDN: `json.Unmarshal([]byte(payload.MSISDN), &fields)` → `map[string]any`
    - Validasi: MSISDN tidak kosong, JSON valid, map tidak kosong (`len(fields) == 0`)
    - Setiap validasi gagal → log error, `forwardCallback` FAILED, return
    - Log info setelah parsing berhasil: `queue_name`, `msisdn`, `mid`, `msgid`, `game_code`, `denomination_id`
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 3.1, 3.2, 3.3, 3.4, 9.1_

  - [ ]* 2.2 Tulis property test untuk MSISDN JSON round-trip (Property 2)
    - **Property 2: MSISDN JSON Round-Trip Mempertahankan Fields**
    - **Validates: Requirements 3.1, 3.4, 4.1**
    - Untuk sembarang `map[string]any` (key string, value string), setelah `json.Marshal` → `json.Unmarshal`, hasilnya harus identik dengan input asli
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: unipin-transaction-flow, Property 2: MSISDN JSON Round-Trip Mempertahankan Fields`

  - [ ]* 2.3 Tulis unit test untuk parsing error di processGame
    - `TestProcessGame_EmptyCommand_ForwardsFailed` — Command kosong (Req 2.2)
    - `TestProcessGame_NoDelimiter_ForwardsFailed` — Command tanpa `*` (Req 2.2)
    - `TestProcessGame_EmptyGameCode_ForwardsFailed` — gameCode kosong setelah trim (Req 2.3)
    - `TestProcessGame_EmptyDenominationID_ForwardsFailed` — denominationID kosong setelah trim (Req 2.4)
    - `TestProcessGame_EmptyMSISDN_ForwardsFailed` — MSISDN kosong (Req 3.2)
    - `TestProcessGame_InvalidMSISDNJSON_ForwardsFailed` — JSON invalid (Req 3.2)
    - `TestProcessGame_EmptyMSISDNMap_ForwardsFailed` — map kosong (Req 3.3)
    - _Requirements: 2.2, 2.3, 2.4, 3.2, 3.3_

- [x] 3. Implementasi method processGame — pemanggilan API dan fallback
  - [x] 3.1 Lanjutkan implementasi `processGame` — bagian ValidateUser, CreateOrder, dan fallback
    - Panggil `s.unipinClient.ValidateUser(ctx, ValidateUserRequest{GameCode: gameCode, Fields: fields})`
    - Sukses (`Status = 1`): simpan `ValidationToken`, log info dengan `game_code`, `msgid`, `username`, `status`
    - Gagal (`Status ≠ 1`): log error, `forwardCallback` FAILED dengan `message = resp.Reason`, return
    - TechnicalError: log error, `forwardCallback` FAILED dengan `message = err.Error()`, return
    - Panggil `s.unipinClient.CreateOrder(ctx, CreateOrderRequest{GameCode: gameCode, ValidationToken: validationToken, ReferenceNo: msgID, DenominationID: denominationID})`
    - Sukses (`Status = 1`): `forwardCallback` SUCCESS dengan `serialNumber = resp.ReferenceNo`, `message = resp.Reason`
    - Gagal (`Status ≠ 1`): `forwardCallback` FAILED dengan `serialNumber = resp.ReferenceNo`, `message = resp.Reason`
    - Timeout (`TechnicalError` + `DeadlineExceeded`): log warn, fallback ke `processOrderInquiry`
    - Error teknis lain: `forwardCallback` FAILED dengan `message = err.Error()`
    - Log info di setiap tahap sesuai Requirement 9
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 5.1, 5.2, 5.3, 5.4, 5.5, 9.2, 9.3, 9.4, 9.5_

  - [ ]* 3.2 Tulis property test untuk parameter mapping CreateOrder (Property 3)
    - **Property 3: Parameter CreateOrder Terpetakan dengan Benar dari Input**
    - **Validates: Requirements 5.1**
    - Untuk sembarang kombinasi `gameCode`, `denominationID`, `validationToken`, dan `msgID`, `CreateOrderRequest` harus memiliki field yang tepat sesuai input
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: unipin-transaction-flow, Property 3: Parameter CreateOrder Terpetakan dengan Benar dari Input`

  - [ ]* 3.3 Tulis unit test untuk alur API di processGame
    - `TestProcessGame_HappyPath_Success` — alur lengkap sukses (Req 4.1, 5.1, 5.2)
    - `TestProcessGame_ValidateUserFailed_ForwardsFailed` — ValidateUser gagal (Req 4.3)
    - `TestProcessGame_ValidateUserTechError_ForwardsFailed` — ValidateUser error teknis (Req 4.4)
    - `TestProcessGame_CreateOrderFailed_ForwardsFailed` — CreateOrder gagal (Req 5.3)
    - `TestProcessGame_CreateOrderTimeout_FallbackInquiry` — CreateOrder timeout → fallback (Req 5.4)
    - `TestProcessGame_CreateOrderTechError_ForwardsFailed` — CreateOrder error non-timeout (Req 5.5)
    - _Requirements: 4.1, 4.3, 4.4, 5.1, 5.2, 5.3, 5.4, 5.5_

- [x] 4. Implementasi method processOrderInquiry
  - [x] 4.1 Tambahkan method `processOrderInquiry` pada `ConsumerServiceImpl` di `consumer_service.go`
    - Signature: `func (s *ConsumerServiceImpl) processOrderInquiry(ctx context.Context, payload *consumePayload, queueName, msgID, referenceNo string)`
    - Panggil `s.unipinClient.OrderInquiry(ctx, referenceNo)`
    - Sukses (`Status = 1`): `forwardCallback` SUCCESS dengan `serialNumber = resp.ReferenceNo`, `message = resp.Reason`
    - Gagal (`Status ≠ 1`): `forwardCallback` FAILED dengan `serialNumber = resp.ReferenceNo`, `message = resp.Reason`
    - Error: `forwardCallback` FAILED dengan `message = err.Error()`
    - Log info/warn sesuai Requirement 9.6, 9.7
    - Mengikuti pola `processVoucherInquiry` yang sudah ada
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 9.6, 9.7_

  - [ ]* 4.2 Tulis unit test untuk processOrderInquiry
    - `TestProcessOrderInquiry_Success_ForwardsSuccess` — inquiry sukses (Req 6.2)
    - `TestProcessOrderInquiry_Failed_ForwardsFailed` — inquiry gagal (Req 6.3)
    - `TestProcessOrderInquiry_Error_ForwardsFailed` — inquiry error (Req 6.4)
    - _Requirements: 6.2, 6.3, 6.4_

- [x] 5. Checkpoint — Verifikasi alur unipin-game
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

- [x] 6. Tambahkan case "unipin-game" di processMessage switch
  - [x] 6.1 Modifikasi method `processMessage` di `consumer_service.go`
    - Tambahkan case `"unipin-game"` pada switch `productType` yang memanggil `s.processGame(ctx, &payload, queueName, msgID)`
    - Letakkan sebelum case `"unipin-direct-topup"` (atau setelah `"unipin-voucher"`)
    - Hapus/ganti log warning "not yet implemented" untuk product type ini
    - _Requirements: 7.1, 7.2, 7.3_

  - [ ]* 6.2 Tulis unit test untuk routing processMessage
    - `TestProcessMessage_UnipinGame_RoutesToProcessGame` — verifikasi routing ke processGame (Req 7.1, 7.2)
    - _Requirements: 7.1, 7.2_

- [x] 7. Update processVoucher — parsing Command menggantikan ProductCode
  - [x] 7.1 Modifikasi method `processVoucher` di `consumer_service.go`
    - Ganti `denominationCode := strings.TrimSpace(payload.ProductCode)` dengan parsing dari `Command`
    - Parse Command: `strings.SplitN(payload.Command, "*", 2)` → `parts`
    - Validasi: `len(parts) < 2` → log error, `forwardCallback` FAILED, return
    - `denominationCode := strings.TrimSpace(parts[1])` — jika kosong → log error, `forwardCallback` FAILED, return
    - Sisa alur voucher (VoucherRequest → timeout fallback → VoucherInquiry) tetap tidak berubah
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

  - [ ]* 7.2 Tulis unit test untuk update processVoucher
    - `TestProcessVoucher_ParsesCommandForDenominationCode` — parsing Command berhasil (Req 8.1, 8.2)
    - `TestProcessVoucher_EmptyCommand_ForwardsFailed` — Command kosong (Req 8.3)
    - `TestProcessVoucher_EmptyDenominationCode_ForwardsFailed` — denominationCode kosong (Req 8.4)
    - `TestProcessVoucher_FullFlow_Preserved` — alur voucher tetap utuh setelah perubahan (Req 8.5)
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

- [ ] 8. Tambahkan property test untuk fault tolerance (Property 4)
  - [ ]* 8.1 Tulis property test untuk toleransi kegagalan (Property 4)
    - **Property 4: Setiap Error Menghasilkan Forward FAILED tanpa Menghentikan Consumer**
    - **Validates: Requirements 10.1**
    - Untuk sembarang error pada tahap apapun (parsing Command, parsing MSISDN, ValidateUser, CreateOrder non-timeout, OrderInquiry), consumer harus selalu memanggil `forwardCallback` dengan `statusToBe = "FAILED"` dan tidak boleh panic
    - Gunakan table-driven test dengan generated error conditions
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: unipin-transaction-flow, Property 4: Setiap Error Menghasilkan Forward FAILED tanpa Menghentikan Consumer`

- [x] 9. Checkpoint akhir — Pastikan semua test lulus dan codebase bersih
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Jalankan `go vet ./...` untuk memastikan tidak ada issue
  - Pastikan logging lengkap di setiap tahap alur unipin-game (Req 9.1–9.7)
  - Pastikan semua error di-forward sebagai FAILED dan consumer tidak berhenti (Req 10.1, 10.2)
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

## Catatan

- Task bertanda `*` bersifat opsional dan dapat dilewati untuk MVP yang lebih cepat
- Setiap task mereferensikan requirements spesifik untuk traceability
- Checkpoint memastikan validasi inkremental di setiap tahap
- Property test menggunakan `testing/quick` (stdlib Go) dengan minimum 100 iterasi
- Semua perubahan terpusat di satu file: `internal/infrastructure/rabbitmq/consumer_service.go`
- Infrastruktur downstream (`forwardCallback`, `MQPublisher`, UniPin client) sudah tersedia dan tidak perlu dimodifikasi
- Pola timeout detection mengikuti pattern existing di `processVoucher`: `errors.As(err, &techErr) && errors.Is(techErr.Cause, context.DeadlineExceeded)`
