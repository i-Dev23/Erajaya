# Rencana Implementasi: Telkomsel Callback Endpoint

## Overview

Implementasi HTTP callback endpoint ke `pps-services-gateway-telkomsel` menggunakan gofiber untuk menerima notifikasi asinkron dari Telkomsel terkait fulfillment VAS Recharge (paket data). HTTP server berjalan bersamaan dengan RabbitMQ consumer dalam satu proses, dikoordinasikan oleh `errgroup`.

## Tasks

- [x] 1. Tambahkan dependency gofiber dan errgroup
  - Jalankan `go get github.com/gofiber/fiber/v2` untuk menambahkan gofiber ke `go.mod`
  - Jalankan `go get golang.org/x/sync/errgroup` untuk memastikan errgroup tersedia sebagai direct dependency (saat ini hanya indirect via pgx)
  - Verifikasi `go.mod` sudah berisi kedua dependency
  - _Requirements: 1.3_

- [ ] 2. Tambahkan `GetTransactionByOurTrxID` ke interface dan implementasi TransactionLogger
  - [x] 2.1 Tambahkan method `GetTransactionByOurTrxID` ke interface `TransactionLogger`
    - Di `internal/domain/contract/service/transaction_logger.go`, tambahkan method:
      ```go
      GetTransactionByOurTrxID(ctx context.Context, ourTrxID string) (*TransactionRecord, error)
      ```
    - Method ini mengambil satu baris dari `telkomsel_transaction` berdasarkan `our_trx_id`
    - Mengembalikan `*TransactionRecord` dan error jika tidak ditemukan
    - _Requirements: 4.1, 4.2_

  - [x] 2.2 Implementasikan `GetTransactionByOurTrxID` di `PostgresTransactionLogger`
    - Di `internal/infrastructure/postgres/transaction_logger.go`, tambahkan:
      - Konstanta SQL `getTransactionByOurTrxIDSQL` dengan query `SELECT ... FROM telkomsel_transaction WHERE our_trx_id = $1 LIMIT 1`
      - Method `GetTransactionByOurTrxID` yang menggunakan `QueryRowContext` dan `Scan` ke `TransactionRecord`
      - Handle `sql.NullString` untuk field nullable (`product_id`, `stock_type`, `mq_transaction`)
    - _Requirements: 4.1, 4.2_

  - [x] 2.3 Tambahkan index `our_trx_id` ke migration statements
    - Di `internal/infrastructure/postgres/transaction_logger.go`, tambahkan DDL ke `migrationStatements`:
      ```sql
      CREATE INDEX IF NOT EXISTS idx_telkomsel_transaction_our_trx_id
          ON telkomsel_transaction (our_trx_id);
      ```
    - Index diperlukan agar lookup `WHERE our_trx_id = $1` tidak melakukan full table scan
    - _Requirements: 4.1_

  - [ ]* 2.4 Tulis unit test untuk `GetTransactionByOurTrxID`
    - `TestGetTransactionByOurTrxID_RoundTrip` — insert transaksi lalu lookup berdasarkan `our_trx_id`, verifikasi semua field cocok
    - `TestGetTransactionByOurTrxID_NotFound` — lookup `our_trx_id` yang tidak ada, verifikasi error dikembalikan
    - _Requirements: 4.1, 4.2_

- [x] 3. Tambahkan `CallbackServerConfig` ke config
  - Di `internal/config/config.go`, tambahkan:
    - Struct `CallbackServerConfig` dengan field `Port int`
    - Fungsi `LoadCallbackServer() (*CallbackServerConfig, error)` yang membaca `CALLBACK_PORT` dari environment variable
    - Default port `8080` jika `CALLBACK_PORT` kosong
    - Return error jika `CALLBACK_PORT` berisi nilai non-numerik
  - _Requirements: 1.1, 1.2, 8.1, 8.2, 8.3_

  - [ ]* 3.1 Tulis property test untuk `LoadCallbackServer` (Property 6)
    - **Property 6: CALLBACK_PORT Non-Numerik Menghasilkan Error**
    - **Validates: Requirements 8.3**
    - Untuk sembarang string yang bukan angka valid, `LoadCallbackServer` harus mengembalikan error
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: telkomsel-callback-endpoint, Property 6: CALLBACK_PORT Non-Numerik Menghasilkan Error`

  - [ ]* 3.2 Tulis unit test untuk `LoadCallbackServer`
    - `TestLoadCallbackServer_DefaultPort` — tanpa `CALLBACK_PORT`, verifikasi port default 8080
    - `TestLoadCallbackServer_CustomPort` — set `CALLBACK_PORT=9090`, verifikasi port 9090
    - `TestLoadCallbackServer_NonNumericPort_ReturnsError` — set `CALLBACK_PORT=abc`, verifikasi error
    - _Requirements: 8.1, 8.2, 8.3_

- [x] 4. Checkpoint — Verifikasi interface dan config
  - Pastikan `PostgresTransactionLogger` masih mengimplementasikan interface `TransactionLogger` (compile-time check)
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

- [ ] 5. Buat CallbackHandler
  - [x] 5.1 Buat file `internal/handler/callback_handler.go`
    - Definisikan struct `CallbackQuery` dengan field dan tag `query:` untuk gofiber query parsing:
      - `TransactionID`, `OrganizationCode`, `ServiceID`, `Status`, `Message`, `SerialNumber`
    - Definisikan struct `CallbackResponse` dengan field `Status` dan `Message` (JSON tags)
    - Definisikan struct `CallbackHandler` dengan field:
      - `logger contractsvc.Logger`
      - `transactionLogger contractsvc.TransactionLogger` (nil jika Postgres tidak dikonfigurasi)
      - `downstreamClient *downstream.DownstreamClient` (nil jika downstream tidak dikonfigurasi)
      - `queueName string`
    - Implementasikan `NewCallbackHandler(logger, transactionLogger, downstreamClient, queueName) *CallbackHandler`
    - _Requirements: 2.1, 2.2_

  - [x] 5.2 Implementasikan method `Handle(c *fiber.Ctx) error` di `CallbackHandler`
    - Parse query parameter menggunakan `c.QueryParser(&query)`
    - Validasi parameter mandatory: `transaction_id`, `organization_code`, `service_id`, `status`, `message` — return 400 jika kosong
    - Validasi `status` hanya menerima `"SUCCESS"` atau `"FAILED"` — return 400 jika tidak valid
    - Validasi panjang `organization_code` (6-13 karakter) — return 400 jika tidak valid
    - Validasi panjang `service_id` (13 karakter) — return 400 jika tidak valid
    - URL-decode `message` menggunakan `url.QueryUnescape`
    - Log info: callback diterima dengan `transaction_id`, `organization_code`, `service_id`, `status`, `message`
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 3.1, 3.2, 3.3_

  - [x] 5.3 Implementasikan lookup transaksi dan pencatatan ke database di `Handle`
    - Panggil `GetTransactionByOurTrxID(ctx, transaction_id)` dengan context deadline 5 detik
    - Jika berhasil: gunakan `rec.MsgID` sebagai `msgID`, gunakan `rec.QueueName` sebagai `queueName` (fallback ke `h.queueName`)
    - Jika gagal: log warning, gunakan `transaction_id` sebagai fallback `msgID`
    - Bangun `ResponseRecord` sesuai mapping di design.md:
      - `StatusCode`: `"0"` jika status SUCCESS, `"1"` jika FAILED
      - `RawPayload`: seluruh query parameter dalam format JSON
    - Panggil `InsertCallbackResponse(ctx, rec)` dengan context deadline 5 detik
    - Jika gagal: log error, lanjut ke forwarding
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 10.1, 10.3, 10.4_

  - [x] 5.4 Implementasikan forwarding ke downstream di `Handle`
    - Jika `downstreamClient` nil: log warning, skip forwarding
    - Bangun `CallbackRequest` sesuai mapping di design.md:
      - `MsgId`: `strconv.Atoi(msgID)` (0 jika gagal parse)
      - `StatusToBe`: `status` dari query parameter
      - `SerialNumber`: `serial_number` dari query parameter
      - `ClientNumber`: `service_id` dari query parameter
      - `ConversationID`: `transaction_id` dari query parameter
      - `MessageToCustomer`: `message` (URL-decoded)
      - `QueueName`: queue name dari handler
      - `Source`: `"PROVIDER"`
    - Panggil `ForwardToPublisher(ctx, callbackReq)`
    - Jika gagal: log error
    - Return HTTP 200 OK dengan `{"status": "OK", "message": "Callback received"}`
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 6.1, 7.1, 7.2, 7.3_

  - [ ]* 5.5 Tulis property test untuk CallbackHandler (Property 1 & 2)
    - **Property 1: Valid Callback Menghasilkan HTTP 200**
    - **Validates: Requirements 2.3, 6.1**
    - Untuk sembarang kombinasi query parameter valid, handler harus mengembalikan HTTP 200 OK dengan body `{"status":"OK","message":"Callback received"}`
    - **Property 2: Invalid Callback Menghasilkan HTTP 400**
    - **Validates: Requirements 2.4, 2.5, 3.1, 3.2**
    - Untuk sembarang kombinasi parameter invalid (mandatory kosong, status bukan SUCCESS/FAILED, organization_code panjang invalid, service_id panjang bukan 13), handler harus mengembalikan HTTP 400
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: telkomsel-callback-endpoint, Property 1: Valid Callback Menghasilkan HTTP 200`
    - Tag: `// Feature: telkomsel-callback-endpoint, Property 2: Invalid Callback Menghasilkan HTTP 400`

  - [ ]* 5.6 Tulis property test untuk URL-decoding dan mapping (Property 3, 4, 5)
    - **Property 3: URL-Decoding Message Round-Trip**
    - **Validates: Requirements 3.3**
    - Untuk sembarang string message yang di-URL-encode, nilai message yang diproses handler harus sama dengan string asli
    - **Property 4: Mapping Callback ke ResponseRecord**
    - **Validates: Requirements 4.1, 4.2**
    - Untuk sembarang callback valid, `InsertCallbackResponse` dipanggil dengan `ResponseRecord` yang field-nya sesuai mapping di design.md
    - **Property 5: Mapping Callback ke CallbackRequest (Downstream)**
    - **Validates: Requirements 5.1, 5.2**
    - Untuk sembarang callback valid, `ForwardToPublisher` dipanggil dengan `CallbackRequest` yang field-nya sesuai mapping di design.md
    - Tag: `// Feature: telkomsel-callback-endpoint, Property 3: URL-Decoding Message Round-Trip`
    - Tag: `// Feature: telkomsel-callback-endpoint, Property 4: Mapping Callback ke ResponseRecord`
    - Tag: `// Feature: telkomsel-callback-endpoint, Property 5: Mapping Callback ke CallbackRequest`

  - [ ]* 5.7 Tulis unit test untuk error handling dan edge case
    - `TestHandle_DBFailure_StillReturns200` — mock `InsertCallbackResponse` return error, verifikasi tetap HTTP 200 (Req 4.4)
    - `TestHandle_DownstreamFailure_StillReturns200` — mock `ForwardToPublisher` return error, verifikasi tetap HTTP 200 (Req 5.4)
    - `TestHandle_NilTransactionLogger_StillReturns200` — `transactionLogger = nil`, verifikasi tetap HTTP 200 (Req 10.3)
    - `TestHandle_NilDownstreamClient_StillReturns200` — `downstreamClient = nil`, verifikasi tetap HTTP 200 (Req 5.3)
    - `TestHandle_LookupFails_FallbackMsgID` — mock `GetTransactionByOurTrxID` return error, verifikasi `transaction_id` digunakan sebagai fallback `msgID`
    - `TestHandle_ExactResponseBody` — verifikasi body response persis `{"status":"OK","message":"Callback received"}` dan `{"status":"ERROR","message":"..."}` (Req 6.1, 6.2)
    - _Requirements: 4.4, 5.3, 5.4, 6.1, 6.2, 10.1, 10.2, 10.3_

- [ ] 6. Checkpoint — Verifikasi CallbackHandler
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

- [ ] 7. Buat HTTP Server setup
  - [x] 7.1 Buat file `internal/http/server.go`
    - Definisikan struct `Server` dengan field `app *fiber.App`, `config ServerConfig`, `logger contractsvc.Logger`
    - Definisikan struct `ServerConfig` dengan field `Port int`
    - Implementasikan `NewServer(cfg ServerConfig, callbackHandler *handler.CallbackHandler, logger contractsvc.Logger) *Server`:
      - Buat `fiber.New()` dengan konfigurasi default
      - Tambahkan `recover.New()` middleware untuk menangkap panic
      - Daftarkan route `GET /callback/ext` ke `callbackHandler.Handle`
    - Implementasikan `Listen(ctx context.Context) error`:
      - Start server di goroutine dengan `app.Listen(fmt.Sprintf(":%d", cfg.Port))`
      - Tunggu context cancel, lalu panggil `app.Shutdown()` untuk graceful shutdown
      - Return error jika `Listen` gagal
    - _Requirements: 1.1, 1.3, 1.4, 1.5, 2.1, 6.3, 9.4_

- [x] 8. Modifikasi `main.go` — jalankan HTTP server bersamaan dengan consumer via errgroup
  - Di `cmd/app/main.go`, tambahkan:
    - Import `golang.org/x/sync/errgroup`, `pps-services-gateway-telkomsel/internal/handler`, `pps-services-gateway-telkomsel/internal/http`
    - Setelah inisialisasi consumer, transactionLogger, dan downstreamClient:
      - Panggil `config.LoadCallbackServer()` untuk membaca konfigurasi callback server
      - Buat `CallbackHandler` via `handler.NewCallbackHandler(logger, txLogger, downstreamClient, cfg.QueueName)`
      - Buat `Server` via `http.NewServer(callbackCfg, callbackHandler, logger)`
    - Ganti pemanggilan langsung `consumer.Start(ctx)` dengan `errgroup`:
      ```go
      g, gCtx := errgroup.WithContext(ctx)
      g.Go(func() error { return consumer.Start(gCtx) })
      g.Go(func() error { return httpServer.Listen(gCtx) })
      if err := g.Wait(); err != nil { ... }
      ```
    - Jika `LoadCallbackServer` gagal: log error dan `os.Exit(1)`
  - _Requirements: 1.1, 1.5, 1.6, 9.1, 9.2, 9.3_

- [x] 9. Checkpoint akhir — Pastikan semua test lulus
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Jalankan `go vet ./...` untuk memastikan tidak ada issue
  - Pastikan semua test lulus dengan `go test ./...`
  - Tanyakan ke user jika ada pertanyaan sebelum selesai.

## Catatan

- Task bertanda `*` bersifat opsional dan dapat dilewati untuk MVP yang lebih cepat
- Setiap task mereferensikan requirements spesifik untuk traceability
- Checkpoint memastikan validasi inkremental di setiap tahap
- Property test memvalidasi correctness properties universal dari design document
- Unit test memvalidasi contoh spesifik dan edge case
- `our_trx_id` dari callback Telkomsel perlu di-lookup ke `telkomsel_transaction` untuk mendapatkan `msg_id` asli
