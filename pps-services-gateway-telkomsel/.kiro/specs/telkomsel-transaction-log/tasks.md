# Rencana Implementasi: Telkomsel Transaction Log

## Overview

Implementasi lapisan persistensi Postgres ke `pps-services-gateway-telkomsel` melalui komponen `TransactionLogger`. Setiap transaksi yang diproses oleh `ConsumerServiceImpl` akan dicatat ke dua tabel Postgres: `telkomsel_transaction` dan `telkomsel_transaction_response`.

## Tasks

- [x] 1. Tambahkan dependency pgx/v5 dan definisikan interface TransactionLogger
  - Jalankan `go get github.com/jackc/pgx/v5` untuk menambahkan driver Postgres ke `go.mod`
  - Buat file `internal/domain/contract/service/transaction_logger.go` berisi:
    - Struct `TransactionRecord` dengan field: `MsgID`, `OurTrxID`, `MSISDN`, `MID`, `ProductType`, `ProductID`, `Amount`, `StockType`, `QueueName`, `MQTransaction`
    - Struct `ResponseRecord` dengan field: `MsgID`, `OurTrxID`, `TelkomselTrxID`, `StatusCode`, `StatusDesc`, `RequestPayload json.RawMessage`, `RawPayload json.RawMessage`, `RequestedAt time.Time`, `ResponseLatencyMs int64`
    - Interface `TransactionLogger` dengan method: `InsertTransaction`, `UpdateTransactionStatus`, `InsertSyncResponse`, `InsertCallbackResponse`, `GetResponsesByMsgID`, `RunMigration`, `Close`
  - _Requirements: 1.5, 8.1_

- [x] 2. Implementasi PostgresTransactionLogger — inisialisasi dan migration
  - [x] 2.1 Buat file `internal/infrastructure/postgres/transaction_logger.go`
    - Definisikan struct `PostgresTransactionLogger` dengan field `db *sql.DB` dan `logger contractsvc.Logger`
    - Implementasikan `NewTransactionLogger(dsn string, logger contractsvc.Logger)` yang membuka koneksi pgx/v5 stdlib, mengatur pool (MaxOpenConns=5, MaxIdleConns=2, ConnMaxLifetime=5m, ConnMaxIdleTime=2m), dan melakukan `db.PingContext`
    - Implementasikan `Close()` yang menutup connection pool
    - _Requirements: 1.1, 1.5_

  - [x] 2.2 Implementasikan `RunMigration` di `PostgresTransactionLogger`
    - Definisikan DDL `CREATE TABLE IF NOT EXISTS telkomsel_transaction` sesuai skema design.md (termasuk kolom: `mq_transaction`, `processing_at`, `success_at`, `failed_at`, `first_requested_at`, `last_response_at`)
    - Definisikan DDL `CREATE TABLE IF NOT EXISTS telkomsel_transaction_response` sesuai skema design.md (termasuk kolom: `request_payload JSONB`, `requested_at TIMESTAMPTZ`, `response_latency_ms INTEGER`)
    - Definisikan DDL `CREATE INDEX IF NOT EXISTS idx_telkomsel_transaction_response_msg_id` sesuai requirements 2.3
    - Jalankan ketiga DDL dalam satu transaksi database (`BeginTx` → `ExecContext` × 3 → `Commit`)
    - _Requirements: 2.1, 2.2, 2.3, 2.4_

  - [ ]* 2.3 Tulis property test untuk RunMigration (Property 1: Migration Idempoten)
    - **Property 1: Migration Idempoten**
    - **Validates: Requirements 2.4**
    - Panggil `RunMigration` N kali berurutan (N di-generate secara acak, min 2), verifikasi semua berhasil tanpa error dan struktur tabel identik setelah setiap pemanggilan
    - Gunakan `testing/quick` atau `gopter` dengan minimum 100 iterasi
    - Tag: `// Feature: telkomsel-transaction-log, Property 1: Migration Idempoten`

- [x] 3. Implementasi operasi CRUD di PostgresTransactionLogger
  - [x] 3.1 Implementasikan `InsertTransaction`
    - Gunakan query `INSERT INTO telkomsel_transaction ... ON CONFLICT (msg_id) DO NOTHING`
    - Isi semua kolom dari `TransactionRecord` termasuk `mq_transaction` dan `first_requested_at`, set `status = 'PROCESSING'`
    - Gunakan `ExecContext` dengan context yang diberikan
    - _Requirements: 3.1, 3.2, 3.3_

  - [ ]* 3.2 Tulis property test untuk InsertTransaction (Property 2 & 3)
    - **Property 2: Insert Transaction Round-Trip**
    - **Validates: Requirements 3.1, 3.2**
    - Untuk sembarang `TransactionRecord` valid, setelah `InsertTransaction`, `SELECT` berdasarkan `msg_id` harus mengembalikan tepat satu baris dengan semua field identik dan `status = 'PROCESSING'`
    - **Property 3: Insert Transaction Idempoten (ON CONFLICT DO NOTHING)**
    - **Validates: Requirements 3.3**
    - Panggil `InsertTransaction` dua kali dengan `msg_id` yang sama, verifikasi tidak ada error dan hanya ada satu baris di tabel
    - Tag: `// Feature: telkomsel-transaction-log, Property 2: Insert Transaction Round-Trip`
    - Tag: `// Feature: telkomsel-transaction-log, Property 3: Insert Transaction Idempoten`

  - [x] 3.3 Implementasikan `UpdateTransactionStatus`
    - Gunakan query terpisah untuk SUCCESS dan FAILED sesuai design.md:
      - SUCCESS: `SET status = 'SUCCESS', success_at = NOW(), last_response_at = NOW(), updated_at = NOW()`
      - FAILED: `SET status = 'FAILED', failed_at = NOW(), last_response_at = NOW(), updated_at = NOW()`
    - Validasi bahwa `status` hanya menerima `'SUCCESS'` atau `'FAILED'`
    - _Requirements: 4.1, 4.2, 4.3_

  - [ ]* 3.4 Tulis property test untuk UpdateTransactionStatus (Property 4)
    - **Property 4: Update Status Round-Trip**
    - **Validates: Requirements 4.1, 4.2, 4.3**
    - Untuk sembarang `msg_id` yang sudah ada dan sembarang status valid, setelah `UpdateTransactionStatus`, `SELECT` harus mengembalikan `status` yang sesuai dan `updated_at >= created_at`
    - Tag: `// Feature: telkomsel-transaction-log, Property 4: Update Status Round-Trip`

  - [x] 3.5 Implementasikan `InsertSyncResponse`
    - Gunakan query `INSERT INTO telkomsel_transaction_response` dengan `response_type = 'SYNC'`
    - Isi semua kolom dari `ResponseRecord` termasuk `request_payload`, `requested_at`, `response_latency_ms`; `raw_payload` dan `request_payload` boleh NULL jika nil
    - Update `last_response_at` di `telkomsel_transaction` setelah insert response
    - _Requirements: 5.1, 5.2, 5.3_

  - [ ]* 3.6 Tulis property test untuk InsertSyncResponse (Property 5 & 6)
    - **Property 5: Insert Response Round-Trip**
    - **Validates: Requirements 5.1, 5.2**
    - Untuk sembarang `ResponseRecord` valid, setelah `InsertSyncResponse`, `SELECT` berdasarkan `id` harus mengembalikan baris dengan semua field identik dan `response_type = 'SYNC'`
    - **Property 6: Multiple Responses per msg_id**
    - **Validates: Requirements 7.1, 7.3**
    - Untuk sembarang `msg_id` dan N ≥ 2 pemanggilan `InsertSyncResponse`, `GetResponsesByMsgID` harus mengembalikan tepat N baris
    - Tag: `// Feature: telkomsel-transaction-log, Property 5: Insert Response Round-Trip`
    - Tag: `// Feature: telkomsel-transaction-log, Property 6: Multiple Responses per msg_id`

  - [x] 3.7 Implementasikan `InsertCallbackResponse`
    - Gunakan query `INSERT INTO telkomsel_transaction_response` dengan `response_type = 'CALLBACK'`
    - Setelah insert, panggil `UpdateTransactionStatus` berdasarkan `status_code` (jika `"0"` → `SUCCESS`, selainnya → `FAILED`)
    - _Requirements: 8.1, 8.2, 8.3_

  - [ ]* 3.8 Tulis property test untuk InsertCallbackResponse (Property 7)
    - **Property 7: Callback Response Type**
    - **Validates: Requirements 8.1, 8.2**
    - Untuk sembarang `ResponseRecord` valid, setelah `InsertCallbackResponse`, `GetResponsesByMsgID` harus mengembalikan setidaknya satu baris dengan `response_type = 'CALLBACK'`
    - Tag: `// Feature: telkomsel-transaction-log, Property 7: Callback Response Type`

  - [x] 3.9 Implementasikan `GetResponsesByMsgID`
    - Gunakan query `SELECT ... FROM telkomsel_transaction_response WHERE msg_id = $1 ORDER BY received_at ASC`
    - Kembalikan slice `[]ResponseRecord`
    - _Requirements: 7.3_

  - [ ]* 3.10 Tulis unit test untuk context cancellation (Property 8)
    - **Property 8: Context Cancellation Propagation**
    - **Validates: Requirements 9.3**
    - Untuk sembarang operasi database, jika context sudah di-cancel sebelum operasi dimulai, operasi harus segera mengembalikan error context
    - Tag: `// Feature: telkomsel-transaction-log, Property 8: Context Cancellation Propagation`

- [x] 4. Checkpoint — Verifikasi implementasi PostgresTransactionLogger
  - Pastikan semua method `PostgresTransactionLogger` mengimplementasikan interface `TransactionLogger` (tambahkan compile-time check: `var _ contractsvc.TransactionLogger = (*PostgresTransactionLogger)(nil)`)
  - Pastikan semua test unit/property lulus, tanyakan ke user jika ada pertanyaan.

- [x] 5. Modifikasi ConsumerServiceImpl — inject TransactionLogger
  - [x] 5.1 Tambahkan field dan setter ke `ConsumerServiceImpl` di `internal/infrastructure/rabbitmq/consumer_service.go`
    - Tambahkan field `transactionLogger contractsvc.TransactionLogger` ke struct `ConsumerServiceImpl`
    - Tambahkan method `SetTransactionLogger(tl contractsvc.TransactionLogger)`
    - _Requirements: 1.2_

  - [x] 5.2 Tambahkan helper methods non-blocking di `ConsumerServiceImpl`
    - Implementasikan `logInsertTransaction(ctx, rec)` — cek nil, buat `context.WithTimeout(ctx, 5s)`, panggil `InsertTransaction`, log error jika gagal
    - Implementasikan `logUpdateStatus(ctx, msgID, status)` — pola yang sama
    - Implementasikan `logInsertSyncResponse(ctx, rec)` — pola yang sama
    - Implementasikan `logCheckDuplicateSyncResponse(ctx, msgID)` — panggil `GetResponsesByMsgID`, hitung baris SYNC, log warning jika > 1
    - _Requirements: 9.1, 9.2, 9.3_

  - [x] 5.3 Integrasikan TransactionLogger ke alur pulsa di `consumeSession`
    - Setelah `msg_id` di-resolve, panggil `logInsertTransaction` dengan `TransactionRecord` yang diisi dari payload
    - Setelah `InitiateRegularRechargeOnConsume` berhasil, panggil `logInsertSyncResponse` lalu `logUpdateStatus` (SUCCESS jika `status_code == "0"`, FAILED selainnya)
    - Setelah `InitiateRegularRechargeOnConsume` gagal (error teknis), panggil `logInsertSyncResponse` (dengan `status_code = 'ERROR'`, `raw_payload = nil`) lalu `logUpdateStatus(FAILED)`
    - Panggil `logCheckDuplicateSyncResponse` setelah insert response untuk pulsa
    - _Requirements: 3.1, 3.2, 4.1, 4.2, 4.3, 5.1, 5.2, 5.3, 7.2_

  - [x] 5.4 Integrasikan TransactionLogger ke alur paket data di `consumeSession`
    - Setelah `msg_id` di-resolve, panggil `logInsertTransaction` dengan `our_trx_id` dari BrowseOffer call
    - Setelah `BrowseOfferOnConsume` berhasil, panggil `logInsertSyncResponse` untuk respons BrowseOffer
    - Jika `BrowseOfferOnConsume` gagal, panggil `logInsertSyncResponse` (ERROR) lalu `logUpdateStatus(FAILED)`
    - Setelah `OrderDealerOnConsume` berhasil/gagal, panggil `logInsertSyncResponse` untuk respons OrderDealer lalu `logUpdateStatus` sesuai `status_code`
    - _Requirements: 6.1, 6.2, 6.3, 6.4_

  - [ ]* 5.5 Tulis unit test untuk ConsumerServiceImpl dengan mock TransactionLogger
    - `TestConsumer_NilLogger_NoOp` — verifikasi Consumer berjalan normal tanpa panic saat `transactionLogger = nil` (Req 1.2)
    - `TestConsumer_DBFailure_NonBlocking` — verifikasi kegagalan DB tidak memblokir alur utama (Req 3.4, 4.4, 5.4)
    - `TestConsumer_BrowseOfferFail_StatusFailed` — verifikasi status diupdate FAILED saat BrowseOffer gagal (Req 6.4)
    - _Requirements: 1.2, 3.4, 4.4, 5.4, 6.4, 9.1_

- [x] 6. Modifikasi main.go — inisialisasi dan wiring TransactionLogger
  - Di `cmd/app/main.go`, setelah `config.Load()`, tambahkan blok kondisional:
    - Jika `cfg.PostgresDSN != ""`: panggil `postgres.NewTransactionLogger(cfg.PostgresDSN, logger)`, tangani error dengan `os.Exit(1)`
    - Panggil `txLogger.RunMigration(ctx)` dengan timeout 30 detik, tangani error dengan `os.Exit(1)`
    - Log `"database migration completed"`
    - Panggil `consumer.SetTransactionLogger(txLogger)`
    - Tambahkan `defer txLogger.Close()`
  - Tambahkan import `"pps-services-gateway-telkomsel/internal/infrastructure/postgres"` ke `main.go`
  - _Requirements: 1.1, 1.2, 1.3, 1.4_

- [x] 7. Checkpoint akhir — Pastikan semua test lulus
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Jalankan `go vet ./...` untuk memastikan tidak ada issue
  - Pastikan semua test unit lulus dengan `go test ./internal/...`
  - Tanyakan ke user jika ada pertanyaan sebelum selesai.

## Catatan

- Task bertanda `*` bersifat opsional dan dapat dilewati untuk MVP yang lebih cepat
- Setiap task mereferensikan requirements spesifik untuk traceability
- Property test membutuhkan koneksi Postgres nyata — gunakan tag build `integration` atau Docker untuk isolasi
- Helper methods non-blocking di `ConsumerServiceImpl` memastikan kegagalan DB tidak pernah menghentikan alur utama
- `our_trx_id` untuk paket data menggunakan `transactionID` dari BrowseOffer call (bukan OrderDealer)
