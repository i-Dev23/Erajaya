# Implementation Plan: Error Mapping

## Overview

Implementasi fitur Error Mapping secara bertahap: mulai dari database migration, repository interface + implementasi PostgreSQL, retry config, helper function `ResolveRCPPS`, integrasi consumer flow, perubahan minimal callback handler, wiring di `main.go`, dan testing. Setiap task membangun di atas task sebelumnya sehingga tidak ada kode yang orphan.

## Tasks

- [x] 1. Database Migration
  - [x] 1.1 Buat file `database/migrations/003_create_telkomsel_error_mapping.up.sql`
    - CREATE TABLE IF NOT EXISTS `telkomsel_error_mapping` dengan kolom: `id` SERIAL PRIMARY KEY, `http_status_code` INTEGER NOT NULL, `esb_status_code` VARCHAR(20) NOT NULL, `rc_pps` INTEGER NOT NULL CHECK (rc_pps IN (0, 1, 9)), `description` VARCHAR(255), `created_at` TIMESTAMPTZ DEFAULT NOW(), `updated_at` TIMESTAMPTZ DEFAULT NOW()
    - Tambahkan UNIQUE constraint pada `(http_status_code, esb_status_code)`
    - INSERT 11 baris seed data menggunakan `INSERT ... ON CONFLICT DO NOTHING` sesuai tabel mapping di requirements 5.3
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 5.1, 5.3_

  - [x] 1.2 Buat file `database/migrations/003_create_telkomsel_error_mapping.down.sql`
    - DROP TABLE IF EXISTS `telkomsel_error_mapping`
    - _Requirements: 5.2, 5.4_

  - [x] 1.3 Tambahkan migration statement ke `internal/infrastructure/postgres/transaction_logger.go`
    - Tambahkan CREATE TABLE dan INSERT seed data ke array `migrationStatements` agar migrasi juga dijalankan saat startup aplikasi (pola yang sudah ada)
    - _Requirements: 1.4, 1.5_

- [x] 2. Repository Interface dan Implementasi PostgreSQL
  - [x] 2.1 Buat file `internal/domain/contract/service/error_mapping_repository.go`
    - Definisikan interface `ErrorMappingRepository` dengan method `GetResponseCode(ctx context.Context, httpStatusCode int, esbStatusCode string) (int, error)`
    - _Requirements: 2.1, 2.3_

  - [x] 2.2 Buat file `internal/infrastructure/postgres/error_mapping_repository.go`
    - Implementasikan `ErrorMappingRepositoryImpl` struct dengan field `db *sql.DB` dan `logger contractsvc.Logger`
    - Implementasikan `NewErrorMappingRepositoryImpl(db *sql.DB, logger contractsvc.Logger) *ErrorMappingRepositoryImpl`
    - Implementasikan method `GetResponseCode`: SELECT `rc_pps` FROM `telkomsel_error_mapping` WHERE `http_status_code` = $1 AND `esb_status_code` = $2. Return 9 jika `sql.ErrNoRows` atau error lainnya
    - _Requirements: 2.1, 3.1, 3.2, 3.3, 3.5_

  - [x] 2.3 Tambahkan method `DB() *sql.DB` pada `PostgresTransactionLogger` di `internal/infrastructure/postgres/transaction_logger.go`
    - Method ini mengembalikan `l.db` untuk sharing connection pool dengan `ErrorMappingRepositoryImpl`
    - _Requirements: 3.4_

  - [ ]* 2.4 Write property tests untuk `ErrorMappingRepositoryImpl`
    - **Property 1: Round-trip lookup seed data** — Untuk setiap baris seed data, `GetResponseCode(ctx, httpStatusCode, esbStatusCode)` harus mengembalikan `rc_pps` yang sesuai
    - **Validates: Requirements 3.1, 7.1**

  - [ ]* 2.5 Write property test: default RC PPS untuk kombinasi tidak dikenal
    - **Property 2: Default RC PPS** — Untuk kombinasi `httpStatusCode` dan `esbStatusCode` yang TIDAK ada di tabel, `GetResponseCode` harus mengembalikan 9
    - **Validates: Requirements 3.2, 4.5**

  - [ ]* 2.6 Write property test: invariant output RC PPS
    - **Property 3: Invariant output** — Untuk kombinasi apapun, hasil `GetResponseCode` harus selalu bernilai 0, 1, atau 9
    - **Validates: Requirements 7.2**

- [x] 3. Checkpoint — Pastikan semua test pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 4. RetryConfig
  - [x] 4.1 Tambahkan struct `RetryConfig` dan function `LoadRetryConfig()` di `internal/config/config.go`
    - `RetryConfig` memiliki field `MaxAttempts int` dan `WaitDuration time.Duration`
    - `LoadRetryConfig()` membaca `RETRY_MAX_ATTEMPTS` (default 4) dan `RETRY_WAIT_SECONDS` (default 10) dari environment variable
    - Return error jika nilai bukan integer positif (nol, negatif, non-numerik)
    - _Requirements: 8.1, 8.2, 8.3, 8.4, 8.5_

  - [ ]* 4.2 Write property test: parsing konfigurasi retry valid
    - **Property 5: Parsing konfigurasi retry valid** — Untuk integer positif yang di-set sebagai env var, `LoadRetryConfig()` harus mengembalikan `RetryConfig` dengan nilai yang sesuai
    - **Validates: Requirements 8.1, 8.2**

  - [ ]* 4.3 Write property test: penolakan konfigurasi retry invalid
    - **Property 6: Penolakan konfigurasi retry invalid** — Untuk string yang bukan integer positif, `LoadRetryConfig()` harus mengembalikan error
    - **Validates: Requirements 8.3, 8.4**

- [x] 5. Constants StatusToBe
  - [x] 5.1 Tambahkan constants `statusToBe` di `internal/infrastructure/rabbitmq/consumer_service.go` atau file terpisah
    - `StatusToBeFinish  = "F"` — transaksi selesai sukses (pulsa)
    - `StatusToBeCancel  = "C"` — transaksi gagal / dibatalkan
    - `StatusToBeProcess = "S"` — transaksi masih diproses (paket data, menunggu callback)
    - Ganti semua hardcoded string `"F"`, `"C"`, `"S"` di `consumer_service.go` dan `callback_handler.go` dengan constants ini

- [x] 6. Helper Function ResolveRCPPS
  - [x] 6.1 Buat file `pkg/telkomsel/error_mapping.go`
    - Definisikan interface `ErrorMappingResolver` dengan method `GetResponseCode(ctx, httpStatusCode, esbStatusCode)`
    - Buat package-level variable `errorMappingResolver ErrorMappingResolver`
    - Implementasikan `SetErrorMappingResolver(r ErrorMappingResolver)` untuk inject resolver saat startup
    - Implementasikan `ResolveRCPPS(ctx context.Context, httpStatusCode int, esbStatusCode string) int` — panggil resolver, return 9 jika resolver nil atau error
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

  - [ ]* 6.2 Write unit tests untuk `ResolveRCPPS`
    - Test `ResolveRCPPS` dengan mock resolver: HTTP 200 + "00000" → 0, HTTP 400 + "20001" → 1, HTTP 500 + "40000" → 9
    - Test `ResolveRCPPS` saat resolver nil → return 9
    - _Requirements: 4.2, 4.3, 4.4, 4.5_

- [x] 7. Integrasi Consumer Flow
  - [x] 7.1 Modifikasi `ConsumerServiceImpl` di `internal/infrastructure/rabbitmq/consumer_service.go`
    - Tambah field `retryConfig *config.RetryConfig` pada struct `ConsumerServiceImpl`
    - Tambah method `SetRetryConfig(cfg *config.RetryConfig)`
    - _Requirements: 8.6_

  - [x] 7.2 Ganti logika hardcoded di consumer flow pulsa
    - Pada case `"pulsa"`: ganti pengecekan `resp.Transaction.StatusCode == "00000"` dengan `telkomsel.ResolveRCPPS(ctx, 200, resp.Transaction.StatusCode)`
    - RC 0 → `statusToBe = StatusToBeFinish`, log SUCCESS, publish downstream
    - RC 1 → `statusToBe = StatusToBeCancel`, log FAILED, publish downstream
    - RC 9 → panggil `retryCheckStatus` (task 7.4)
    - Untuk error case (`callErr != nil`): extract HTTP status code dari `TechnicalError.StatusCode` atau default 400 untuk `BusinessError`, lalu panggil `ResolveRCPPS`
    - _Requirements: 6.1, 6.2, 6.3, 6.4_

  - [x] 7.3 Ganti logika hardcoded di consumer flow paket data
    - Pada case `"paket data"` setelah OrderDealer: ganti pengecekan `orderResp.Transaction.StatusCode == "00000"` dengan `telkomsel.ResolveRCPPS(ctx, 200, orderResp.Transaction.StatusCode)`
    - RC 0 → `statusToBe = StatusToBeProcess` (paket data tetap "S" karena menunggu callback), log PROCESSING
    - RC 1 → `statusToBe = StatusToBeCancel`, log FAILED, publish downstream
    - RC 9 → panggil `retryCheckStatus`
    - _Requirements: 6.1, 6.2, 6.3, 6.4_

  - [x] 7.4 Implementasikan method `retryCheckStatus` pada `ConsumerServiceImpl`
    - Loop maksimal `retryConfig.MaxAttempts` kali dengan `time.Sleep(retryConfig.WaitDuration)` antar percobaan
    - Setiap iterasi: panggil `telkomsel.CheckOrderStatusOnConsume`, lalu `telkomsel.ResolveRCPPS` dari hasilnya
    - Jika RC 0 → return sukses, jika RC 1 → return gagal, jika RC 9 → lanjut retry
    - Setelah max retry tercapai dan masih RC 9 → proses sebagai gagal
    - Log setiap attempt dengan detail attempt number
    - _Requirements: 6.4, 6.5, 8.6_

  - [ ]* 7.5 Write property test: retry check status berhenti pada RC PPS definitif
    - **Property 4: Retry berhenti pada RC PPS definitif** — Jika salah satu hasil bernilai 0 atau 1 sebelum MaxAttempts, retry harus berhenti. Jika semua 9 sampai MaxAttempts, transaksi diproses sebagai gagal
    - **Validates: Requirements 6.4, 6.5**

- [x] 8. Checkpoint — Pastikan semua test pass
  - Ensure all tests pass, ask the user if questions arise.

- [x] 9. Callback Handler (perubahan minimal)
  - [x] 9.1 Review dan update `internal/handler/callback_handler.go`
    - Callback dari Telkomsel sudah membawa status final (SUCCESS/FAILED), sehingga tidak perlu lookup error mapping
    - Ganti hardcoded `statusToBe = "F"` dan `statusToBe = "C"` dengan constants `StatusToBeFinish` dan `StatusToBeCancel`
    - Pastikan tidak ada hardcoded StatusCode check yang perlu diganti
    - _Requirements: 6.1 (callback handler dikecualikan dari error mapping lookup)_

- [x] 10. Wiring di main.go
  - [x] 10.1 Modifikasi `cmd/app/main.go` untuk inisialisasi error mapping
    - Load `RetryConfig` via `config.LoadRetryConfig()`, fail-fast jika error
    - Buat `ErrorMappingRepositoryImpl` menggunakan `pgLogger.DB()` dan `logger`
    - Panggil `telkomsel.SetErrorMappingResolver(errorMappingRepo)` untuk inject resolver
    - Panggil `consumer.SetRetryConfig(retryConfig)` untuk inject retry config ke consumer
    - _Requirements: 3.4, 8.5, 8.6_

- [x] 11. Final Checkpoint — Pastikan semua test pass
  - Ensure all tests pass, ask the user if questions arise.

## Notes

- Task bertanda `*` bersifat opsional dan bisa di-skip untuk MVP lebih cepat
- Setiap task mereferensikan requirements spesifik untuk traceability
- Checkpoint memastikan validasi inkremental di setiap tahap
- Property tests memvalidasi correctness properties universal dari design document
- Method `GetRCPPS` sudah di-rename menjadi `GetResponseCode` di seluruh task
- Helper function tetap bernama `ResolveRCPPS` (tidak berubah)
- statusToBe constants: `StatusToBeFinish = "F"`, `StatusToBeCancel = "C"`, `StatusToBeProcess = "S"` — menggantikan hardcoded string di consumer dan callback handler
- Callback handler tidak memerlukan error mapping karena menerima status final dari Telkomsel
