# Dokumen Desain: Telkomsel Transaction Log

## Overview

Fitur ini menambahkan lapisan persistensi Postgres ke `pps-services-gateway-telkomsel` melalui komponen baru bernama `TransactionLogger`. Komponen ini mencatat setiap transaksi yang diproses oleh `ConsumerServiceImpl` — mulai dari penerimaan pesan RabbitMQ, pemanggilan Telkomsel API, hingga penerimaan respons — ke dalam dua tabel Postgres.

Tujuan utama:
- **Auditability**: setiap transaksi meninggalkan jejak persisten yang bisa di-query.
- **Debugging**: raw payload respons Telkomsel tersimpan dalam format JSONB.
- **Deteksi duplikat**: struktur tabel memungkinkan identifikasi double-processing.
- **Kesiapan async callback**: skema dan interface sudah mendukung `response_type = 'CALLBACK'` untuk iterasi berikutnya.

Prinsip desain utama: **kegagalan database tidak boleh mengganggu alur utama**. Jika Postgres tidak tersedia, `ConsumerServiceImpl` tetap memproses pesan dan meneruskan callback ke `pps-services-publisher-database` secara normal.

---

## Architecture

### Alur Integrasi

```
RabbitMQ
   │
   ▼
ConsumerServiceImpl.consumeSession()
   │
   ├─► TransactionLogger.InsertTransaction(PROCESSING)   ──► telkomsel_transaction
   │
   ├─► telkomsel.InitiateRegularRechargeOnConsume()  (pulsa)
   │       │
   │       ├─► TransactionLogger.InsertSyncResponse()    ──► telkomsel_transaction_response
   │       └─► TransactionLogger.UpdateTransactionStatus(SUCCESS/FAILED)
   │
   ├─► telkomsel.BrowseOfferOnConsume()              (paket data)
   │       │
   │       ├─► TransactionLogger.InsertSyncResponse()    ──► telkomsel_transaction_response
   │       └─► [jika gagal] TransactionLogger.UpdateTransactionStatus(FAILED)
   │
   ├─► telkomsel.OrderDealerOnConsume()              (paket data, setelah BrowseOffer sukses)
   │       │
   │       ├─► TransactionLogger.InsertSyncResponse()    ──► telkomsel_transaction_response
   │       └─► TransactionLogger.UpdateTransactionStatus(SUCCESS/FAILED)
   │
   └─► downstream.ForwardToPublisher()
```

### Posisi Komponen dalam Codebase

```
internal/
  infrastructure/
    postgres/
      transaction_logger.go     ← implementasi TransactionLogger
      transaction_logger_test.go
    rabbitmq/
      consumer_service.go       ← dimodifikasi: inject TransactionLogger
  domain/
    contract/
      service/
        transaction_logger.go   ← interface TransactionLogger (baru)
cmd/
  app/
    main.go                     ← dimodifikasi: inisialisasi TransactionLogger
```

### Pola Inisialisasi (mengikuti pola DownstreamClient)

Sama seperti `DownstreamClient`, `TransactionLogger` diinisialisasi di `main.go` dan di-inject ke `ConsumerServiceImpl` via setter method. Jika `POSTGRES_DSN` kosong, `TransactionLogger` tidak diinisialisasi dan `ConsumerServiceImpl` berjalan tanpa pencatatan database (no-op path).

```
main.go
  │
  ├─► config.Load()                    → cfg.PostgresDSN
  ├─► postgres.NewTransactionLogger()  → logger (atau nil jika DSN kosong)
  ├─► consumer.SetTransactionLogger()
  └─► consumer.Start()
```

---

## Components and Interfaces

### Interface `TransactionLogger`

Ditempatkan di `internal/domain/contract/service/transaction_logger.go`:

```go
package service

import (
    "context"
    "encoding/json"
)

// TransactionRecord merepresentasikan data transaksi yang akan dicatat saat pesan RabbitMQ diterima.
type TransactionRecord struct {
    MsgID       string
    OurTrxID    string
    MSISDN      string
    MID         string
    ProductType string
    ProductID   string
    Amount      int
    StockType   string
    QueueName   string
    MQTransaction string // RabbitMQ URL untuk update status via MQ di next step
}

// ResponseRecord merepresentasikan data respons dari Telkomsel API.
type ResponseRecord struct {
    MsgID           string
    OurTrxID        string
    TelkomselTrxID  string
    StatusCode      string
    StatusDesc      string
    RequestPayload  json.RawMessage // request body yang dikirim ke Telkomsel
    RawPayload      json.RawMessage // response body dari Telkomsel (nil untuk error teknis)
    RequestedAt     time.Time       // kapan request dikirim ke Telkomsel
    ResponseLatencyMs int64         // durasi response dalam milliseconds (untuk SYNC)
}

// TransactionLogger mendefinisikan kontrak untuk pencatatan transaksi ke Postgres.
// Semua method bersifat non-blocking terhadap alur utama Consumer:
// jika Postgres tidak tersedia, implementasi harus log error dan return nil.
type TransactionLogger interface {
    // InsertTransaction menyisipkan baris baru ke telkomsel_transaction dengan status PROCESSING.
    // Menggunakan INSERT ... ON CONFLICT (msg_id) DO NOTHING untuk idempotence.
    InsertTransaction(ctx context.Context, rec TransactionRecord) error

    // UpdateTransactionStatus memperbarui kolom status dan updated_at pada baris dengan msg_id yang sesuai.
    // status harus berupa "SUCCESS" atau "FAILED".
    UpdateTransactionStatus(ctx context.Context, msgID string, status string) error

    // InsertSyncResponse menyisipkan baris ke telkomsel_transaction_response dengan response_type = 'SYNC'.
    InsertSyncResponse(ctx context.Context, rec ResponseRecord) error

    // InsertCallbackResponse menyisipkan baris ke telkomsel_transaction_response dengan response_type = 'CALLBACK'.
    // Juga memperbarui status di telkomsel_transaction berdasarkan status_code.
    InsertCallbackResponse(ctx context.Context, rec ResponseRecord) error

    // GetResponsesByMsgID mengambil semua baris telkomsel_transaction_response untuk msg_id tertentu.
    GetResponsesByMsgID(ctx context.Context, msgID string) ([]ResponseRecord, error)

    // RunMigration menjalankan DDL untuk membuat tabel dan index jika belum ada.
    // Bersifat idempoten — aman dipanggil berulang kali.
    RunMigration(ctx context.Context) error

    // Close menutup connection pool Postgres.
    Close()
}
```

### Struct `PostgresTransactionLogger`

Ditempatkan di `internal/infrastructure/postgres/transaction_logger.go`:

```go
package postgres

import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
    contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
)

// PostgresTransactionLogger mengimplementasikan contractsvc.TransactionLogger menggunakan pgx/v5 stdlib.
type PostgresTransactionLogger struct {
    db     *sql.DB
    logger contractsvc.Logger
}

// NewTransactionLogger membuat instance baru dengan connection pool pgx/v5.
// Mengembalikan error jika DSN tidak valid atau koneksi tidak bisa dibuka.
func NewTransactionLogger(dsn string, logger contractsvc.Logger) (*PostgresTransactionLogger, error)

// RunMigration menjalankan DDL idempoten untuk membuat tabel dan index.
func (l *PostgresTransactionLogger) RunMigration(ctx context.Context) error

// InsertTransaction menyisipkan baris PROCESSING ke telkomsel_transaction.
func (l *PostgresTransactionLogger) InsertTransaction(ctx context.Context, rec contractsvc.TransactionRecord) error

// UpdateTransactionStatus memperbarui status dan updated_at.
func (l *PostgresTransactionLogger) UpdateTransactionStatus(ctx context.Context, msgID string, status string) error

// InsertSyncResponse menyisipkan baris SYNC ke telkomsel_transaction_response.
func (l *PostgresTransactionLogger) InsertSyncResponse(ctx context.Context, rec contractsvc.ResponseRecord) error

// InsertCallbackResponse menyisipkan baris CALLBACK dan memperbarui status transaksi.
func (l *PostgresTransactionLogger) InsertCallbackResponse(ctx context.Context, rec contractsvc.ResponseRecord) error

// GetResponsesByMsgID mengambil semua response untuk msg_id tertentu.
func (l *PostgresTransactionLogger) GetResponsesByMsgID(ctx context.Context, msgID string) ([]contractsvc.ResponseRecord, error)

// Close menutup connection pool.
func (l *PostgresTransactionLogger) Close()
```

### Modifikasi `ConsumerServiceImpl`

Tambahkan field `transactionLogger` dan setter method:

```go
type ConsumerServiceImpl struct {
    cfg               *config.Config
    logger            contractsvc.Logger
    downstreamClient  *downstream.DownstreamClient
    transactionLogger contractsvc.TransactionLogger // nil jika POSTGRES_DSN tidak dikonfigurasi
}

func (s *ConsumerServiceImpl) SetTransactionLogger(tl contractsvc.TransactionLogger) {
    s.transactionLogger = tl
}
```

Helper method untuk memanggil TransactionLogger secara aman (no-op jika nil):

```go
func (s *ConsumerServiceImpl) logInsertTransaction(ctx context.Context, rec contractsvc.TransactionRecord) {
    if s.transactionLogger == nil {
        return
    }
    dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
    defer cancel()
    if err := s.transactionLogger.InsertTransaction(dbCtx, rec); err != nil {
        s.logger.Error("failed to insert transaction log", "msg_id", rec.MsgID, "error", err)
    }
}
```

---

## Data Models

### DDL: `telkomsel_transaction`

```sql
CREATE TABLE IF NOT EXISTS telkomsel_transaction (
    msg_id             VARCHAR     PRIMARY KEY,
    our_trx_id         VARCHAR     NOT NULL,
    msisdn             VARCHAR     NOT NULL,
    mid                VARCHAR     NOT NULL,
    product_type       VARCHAR     NOT NULL,
    product_id         VARCHAR,
    amount             INTEGER     NOT NULL,
    stock_type         VARCHAR,
    queue_name         VARCHAR     NOT NULL,
    mq_transaction     VARCHAR,
    status             VARCHAR     NOT NULL,
    processing_at      TIMESTAMPTZ NOT NULL DEFAULT NOW(), -- kapan status PROCESSING (= created_at)
    success_at         TIMESTAMPTZ,                        -- kapan status jadi SUCCESS
    failed_at          TIMESTAMPTZ,                        -- kapan status jadi FAILED
    first_requested_at TIMESTAMPTZ,                        -- kapan pertama kali hit Telkomsel API
    last_response_at   TIMESTAMPTZ,                        -- kapan response terakhir diterima
    created_at         TIMESTAMPTZ NOT NULL DEFAULT NOW(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
```

**Catatan desain:**
- `msg_id` sebagai PRIMARY KEY memastikan satu baris per pesan RabbitMQ.
- `INSERT ... ON CONFLICT (msg_id) DO NOTHING` digunakan untuk idempotence (Requirement 3.3).
- `status` menggunakan nilai string `'PROCESSING'`, `'SUCCESS'`, `'FAILED'`.
- `product_id`, `stock_type`, `mq_transaction` nullable karena tidak selalu tersedia.
- `processing_at` = waktu message diterima dan di-parse (trigger: sebelum hit Telkomsel API).
- `success_at` = diisi saat `UpdateTransactionStatus("SUCCESS")` dipanggil.
- `failed_at` = diisi saat `UpdateTransactionStatus("FAILED")` dipanggil — ada 4 kondisi:
  1. Pulsa: `InitiateRegularRecharge` return `StatusCode != "0"`
  2. Pulsa: `InitiateRegularRecharge` return error teknis (network/timeout)
  3. Paket data: `BrowseOffer` gagal (OrderDealer tidak dipanggil)
  4. Paket data: `OrderDealer` gagal atau return `StatusCode != "0"`
- `first_requested_at` = diisi saat pertama kali hit Telkomsel API.
- `last_response_at` = diisi setiap kali response diterima — untuk hitung async duration paket data.

### DDL: `telkomsel_transaction_response`

```sql
CREATE TABLE IF NOT EXISTS telkomsel_transaction_response (
    id                  BIGSERIAL   PRIMARY KEY,
    msg_id              VARCHAR     NOT NULL,
    our_trx_id          VARCHAR     NOT NULL,
    telkomsel_trx_id    VARCHAR,
    response_type       VARCHAR     NOT NULL,
    status_code         VARCHAR,
    status_desc         VARCHAR,
    request_payload     JSONB,                          -- request body yang dikirim ke Telkomsel
    raw_payload         JSONB,                          -- response body dari Telkomsel
    requested_at        TIMESTAMPTZ,                    -- kapan request dikirim
    response_latency_ms INTEGER,                        -- durasi response dalam ms (SYNC)
    received_at         TIMESTAMPTZ NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_telkomsel_transaction_response_msg_id
    ON telkomsel_transaction_response (msg_id);
```

**Catatan desain:**
- `id BIGSERIAL` sebagai surrogate key memungkinkan multiple baris per `msg_id` (paket data menghasilkan dua baris: BrowseOffer + OrderDealer).
- `response_type` berisi `'SYNC'` atau `'CALLBACK'`.
- `request_payload JSONB` — exact body yang dikirim ke Telkomsel, untuk debugging "apakah kita kirim parameter yang benar?".
- `raw_payload JSONB` nullable — diisi `NULL` untuk error teknis (Requirement 5.3).
- `telkomsel_trx_id` nullable — bisa kosong jika Telkomsel tidak mengembalikan transaction ID.
- `requested_at` — timestamp saat request dikirim, diisi sebelum HTTP call.
- `response_latency_ms` — `time.Since(requestedAt).Milliseconds()` setelah response diterima. Untuk CALLBACK, kolom ini NULL karena durasi diukur dari `telkomsel_transaction.first_requested_at` ke `last_response_at`.
- Index pada `msg_id` untuk mendukung query `GetResponsesByMsgID` dan deteksi duplikat.

### Mapping Field: Payload → Database

| Field DB (`telkomsel_transaction`) | Sumber |
|---|---|
| `msg_id` | `payload.MsgID` → fallback `delivery.MessageId` → `delivery.CorrelationId` |
| `our_trx_id` | `transactionID` yang di-generate oleh `buildTelkomselTransactionID()` sebelum API call |
| `msisdn` | `payload.MSISDN` |
| `mid` | `payload.MID` |
| `product_type` | `payload.ProductType` |
| `product_id` | `payload.ProductID` (kosong untuk pulsa) |
| `amount` | `payload.Amount` |
| `stock_type` | `payload.StockType` |
| `queue_name` | `queueName` (setelah fallback logic) |
| `mq_transaction` | `payload.MQTransaction` — RabbitMQ URL untuk update status via MQ |
| `status` | `'PROCESSING'` saat insert |
| `processing_at` | `NOW()` saat INSERT (default) |
| `success_at` | `NOW()` saat `UpdateTransactionStatus("SUCCESS")` dipanggil |
| `failed_at` | `NOW()` saat `UpdateTransactionStatus("FAILED")` dipanggil |
| `first_requested_at` | `time.Now()` sebelum pertama kali hit Telkomsel API |
| `last_response_at` | `NOW()` setiap kali response diterima dari Telkomsel |

| Field DB (`telkomsel_transaction_response`) | Sumber |
|---|---|
| `msg_id` | sama dengan `telkomsel_transaction.msg_id` |
| `our_trx_id` | `transactionID` yang dikirim ke Telkomsel |
| `telkomsel_trx_id` | `resp.Transaction.TransactionID` |
| `response_type` | `'SYNC'` untuk respons langsung, `'CALLBACK'` untuk async |
| `status_code` | `resp.Transaction.StatusCode` (atau `'ERROR'` untuk error teknis) |
| `status_desc` | `resp.Transaction.StatusDesc` (atau pesan error) |
| `request_payload` | exact JSON body yang dikirim ke Telkomsel API |
| `raw_payload` | seluruh body respons JSON (atau `NULL` untuk error teknis) |
| `requested_at` | `time.Now()` sebelum HTTP call ke Telkomsel |
| `response_latency_ms` | `time.Since(requestedAt).Milliseconds()` setelah response diterima |

---

## Implementation

### Connection Pooling

`PostgresTransactionLogger` menggunakan `database/sql` dengan driver `pgx/v5/stdlib`. Konfigurasi pool default:

```go
db, err := sql.Open("pgx", dsn)
if err != nil {
    return nil, fmt.Errorf("open postgres connection: %w", err)
}
db.SetMaxOpenConns(5)
db.SetMaxIdleConns(2)
db.SetConnMaxLifetime(5 * time.Minute)
db.SetConnMaxIdleTime(2 * time.Minute)
```

Pool kecil (max 5 koneksi) karena service ini adalah single-consumer dengan throughput rendah. Menggunakan `database/sql` (bukan pgx native pool) agar interface standar dan mudah di-mock untuk testing.

### Context dan Timeout

Setiap operasi database menggunakan context dengan deadline 5 detik yang diturunkan dari context Consumer:

```go
dbCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
defer cancel()
```

Jika context Consumer di-cancel (shutdown), operasi database yang sedang berjalan juga akan di-cancel.

### Migration saat Startup

`RunMigration` menjalankan kedua DDL statement dalam satu transaksi database:

```go
func (l *PostgresTransactionLogger) RunMigration(ctx context.Context) error {
    tx, err := l.db.BeginTx(ctx, nil)
    if err != nil {
        return fmt.Errorf("begin migration tx: %w", err)
    }
    defer tx.Rollback()

    for _, stmt := range migrationStatements {
        if _, err := tx.ExecContext(ctx, stmt); err != nil {
            return fmt.Errorf("execute migration: %w", err)
        }
    }
    return tx.Commit()
}
```

### Insert dengan ON CONFLICT

```go
const insertTransactionSQL = `
INSERT INTO telkomsel_transaction
    (msg_id, our_trx_id, msisdn, mid, product_type, product_id, amount, stock_type,
     queue_name, mq_transaction, status, first_requested_at)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, 'PROCESSING', $11)
ON CONFLICT (msg_id) DO NOTHING`
```

### Update Status

```go
// UpdateTransactionStatus memperbarui status, timestamp status, last_response_at, dan updated_at.
// Untuk SUCCESS: mengisi success_at = NOW()
// Untuk FAILED:  mengisi failed_at = NOW()
const updateStatusSuccessSQL = `
UPDATE telkomsel_transaction
SET status = 'SUCCESS', success_at = NOW(), last_response_at = NOW(), updated_at = NOW()
WHERE msg_id = $1`

const updateStatusFailedSQL = `
UPDATE telkomsel_transaction
SET status = 'FAILED', failed_at = NOW(), last_response_at = NOW(), updated_at = NOW()
WHERE msg_id = $1`
```

### Insert Response

```go
const insertResponseSQL = `
INSERT INTO telkomsel_transaction_response
    (msg_id, our_trx_id, telkomsel_trx_id, response_type, status_code, status_desc,
     request_payload, raw_payload, requested_at, response_latency_ms)
VALUES
    ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)`
```

### Query SLA Monitoring

```sql
-- Durasi PROCESSING → SUCCESS/FAILED per transaksi
SELECT msg_id, product_type, status,
  EXTRACT(EPOCH FROM (COALESCE(success_at, failed_at) - processing_at)) AS duration_seconds
FROM telkomsel_transaction
WHERE success_at IS NOT NULL OR failed_at IS NOT NULL
ORDER BY processing_at DESC;

-- Transaksi stuck di PROCESSING > 5 menit
SELECT msg_id, msisdn, product_type, processing_at
FROM telkomsel_transaction
WHERE status = 'PROCESSING'
AND processing_at < NOW() - INTERVAL '5 minutes';

-- Latency SYNC per API call
SELECT msg_id, response_type, response_latency_ms
FROM telkomsel_transaction_response
WHERE response_type = 'SYNC'
ORDER BY received_at DESC;

-- Durasi end-to-end async paket data (dari request sampai callback)
SELECT msg_id,
  EXTRACT(EPOCH FROM (last_response_at - first_requested_at)) * 1000 AS async_duration_ms
FROM telkomsel_transaction
WHERE product_type = 'paket data'
AND last_response_at IS NOT NULL;
```

### Deteksi Duplikat (Requirement 7.2)

Setelah `InsertSyncResponse` untuk transaksi pulsa, `ConsumerServiceImpl` memanggil `GetResponsesByMsgID` dan log warning jika count > 1:

```go
responses, err := s.transactionLogger.GetResponsesByMsgID(dbCtx, msgID)
if err == nil {
    syncCount := 0
    for _, r := range responses {
        if r.ResponseType == "SYNC" {
            syncCount++
        }
    }
    if syncCount > 1 {
        s.logger.Warn("duplicate SYNC responses detected", "msg_id", msgID, "sync_count", syncCount)
    }
}
```

---

## Migration

### Cara Menjalankan DDL saat Startup

Migration dijalankan di `main.go` setelah `NewTransactionLogger` berhasil, sebelum `consumer.Start()`:

```go
// main.go (tambahan)
if cfg.PostgresDSN != "" {
    txLogger, err := postgres.NewTransactionLogger(cfg.PostgresDSN, logger)
    if err != nil {
        logger.Error("failed to initialize transaction logger", "error", err)
        os.Exit(1)
    }
    defer txLogger.Close()

    migCtx, migCancel := context.WithTimeout(ctx, 30*time.Second)
    defer migCancel()
    if err := txLogger.RunMigration(migCtx); err != nil {
        logger.Error("failed to run database migration", "error", err)
        os.Exit(1)
    }
    logger.Info("database migration completed")
    consumer.SetTransactionLogger(txLogger)
}
```

Migration menggunakan `CREATE TABLE IF NOT EXISTS` dan `CREATE INDEX IF NOT EXISTS` sehingga aman dijalankan berulang kali (idempoten). Tidak ada dependency ke tool migration eksternal seperti `golang-migrate` — cukup dengan `database/sql` standar.

---

## Dependency

### Tambah ke `go.mod`

```
github.com/jackc/pgx/v5 v5.x.x
```

Driver digunakan via stdlib interface:

```go
import (
    "database/sql"
    _ "github.com/jackc/pgx/v5/stdlib"
)
```

Menggunakan `database/sql` (bukan pgx native) karena:
1. Interface standar memudahkan mocking di unit test.
2. Konsisten dengan pola yang sudah ada di codebase (HTTP client menggunakan `net/http` standar).
3. Connection pooling `database/sql` sudah cukup untuk kebutuhan service ini.

---

## Config

`POSTGRES_DSN` sudah ditambahkan ke `Config` struct dan dibaca di `config.Load()`:

```go
// internal/config/config.go — sudah ada
type Config struct {
    // ...
    PostgresDSN string
}

// Load() — sudah membaca:
PostgresDSN: strings.TrimSpace(os.Getenv("POSTGRES_DSN")),
```

Tidak ada perubahan yang diperlukan pada `config.go` karena field `PostgresDSN` sudah ada.

Contoh nilai DSN:
```
POSTGRES_DSN=postgres://user:password@localhost:5432/telkomsel_db?sslmode=disable
```

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Migration Idempoten

*For any* jumlah pemanggilan `RunMigration` yang berurutan pada database yang sama, semua pemanggilan harus berhasil tanpa error dan struktur tabel harus tetap identik setelah setiap pemanggilan.

**Validates: Requirements 2.4**

---

### Property 2: Insert Transaction Round-Trip

*For any* `TransactionRecord` yang valid (msg_id non-kosong, msisdn valid, amount > 0), setelah `InsertTransaction` dipanggil, query `SELECT` berdasarkan `msg_id` harus mengembalikan tepat satu baris dengan semua field yang identik dengan input dan `status = 'PROCESSING'`.

**Validates: Requirements 3.1, 3.2**

---

### Property 3: Insert Transaction Idempoten (ON CONFLICT DO NOTHING)

*For any* `TransactionRecord` yang valid, memanggil `InsertTransaction` dua kali atau lebih dengan `msg_id` yang sama tidak boleh mengembalikan error dan harus menghasilkan tepat satu baris di `telkomsel_transaction`.

**Validates: Requirements 3.3**

---

### Property 4: Update Status Round-Trip

*For any* `msg_id` yang sudah ada di `telkomsel_transaction` dan sembarang nilai status (`'SUCCESS'` atau `'FAILED'`), setelah `UpdateTransactionStatus` dipanggil, query `SELECT` berdasarkan `msg_id` harus mengembalikan baris dengan `status` yang identik dengan nilai yang di-update dan `updated_at` yang lebih baru atau sama dengan `created_at`.

**Validates: Requirements 4.1, 4.2, 4.3**

---

### Property 5: Insert Response Round-Trip

*For any* `ResponseRecord` yang valid, setelah `InsertSyncResponse` dipanggil, query `SELECT` berdasarkan `id` yang dikembalikan harus mengembalikan baris dengan semua field yang identik dengan input dan `response_type = 'SYNC'`.

**Validates: Requirements 5.1, 5.2**

---

### Property 6: Multiple Responses per msg_id

*For any* `msg_id` dan sembarang jumlah N ≥ 2 pemanggilan `InsertSyncResponse` dengan `msg_id` yang sama, `GetResponsesByMsgID` harus mengembalikan tepat N baris — tidak ada dedup, tidak ada error.

**Validates: Requirements 7.1, 7.3**

---

### Property 7: Callback Response Type

*For any* `ResponseRecord` yang valid, setelah `InsertCallbackResponse` dipanggil, query `SELECT` berdasarkan `msg_id` harus mengembalikan setidaknya satu baris dengan `response_type = 'CALLBACK'`.

**Validates: Requirements 8.1, 8.2**

---

### Property 8: Context Cancellation Propagation

*For any* operasi database (`InsertTransaction`, `UpdateTransactionStatus`, `InsertSyncResponse`), jika context yang diberikan sudah di-cancel sebelum operasi dimulai, operasi harus segera mengembalikan error context (bukan hang atau timeout lama).

**Validates: Requirements 9.3**

---

## Error Handling

### Prinsip Non-Blocking

Semua pemanggilan `TransactionLogger` di `ConsumerServiceImpl` dibungkus dalam helper method yang:
1. Memeriksa apakah `transactionLogger` nil (DSN tidak dikonfigurasi) → skip.
2. Membuat context dengan timeout 5 detik.
3. Memanggil method TransactionLogger.
4. Jika error → log error, **tidak** return error ke caller.
5. Consumer melanjutkan alur normal (ack pesan, forward callback).

### Tabel Error Handling

| Kondisi | Behavior TransactionLogger | Behavior Consumer |
|---|---|---|
| `POSTGRES_DSN` kosong | Tidak diinisialisasi (nil) | Berjalan normal, skip semua DB ops |
| Koneksi Postgres terputus | Return error dari operasi | Log error, lanjut |
| Insert gagal (constraint, dll) | Return error | Log error, lanjut |
| Update gagal | Return error | Log error, lanjut |
| Migration gagal | Return error | `os.Exit(1)` di main.go |
| Context timeout (5s) | Return context.DeadlineExceeded | Log error, lanjut |
| Context canceled (shutdown) | Return context.Canceled | Shutdown normal |

### Startup vs Runtime

- **Startup** (migration): kegagalan bersifat fatal → `os.Exit(1)`. Lebih baik gagal cepat daripada berjalan tanpa skema yang benar.
- **Runtime** (insert/update): kegagalan bersifat non-fatal → log error, lanjut. Transaksi tetap diproses dan callback tetap diteruskan.

---

## Testing Strategy

### Pendekatan Dual Testing

Fitur ini menggunakan dua lapisan testing yang saling melengkapi:

1. **Unit test** — menggunakan mock `TransactionLogger` untuk memverifikasi bahwa `ConsumerServiceImpl` memanggil method yang benar pada waktu yang tepat, dan bahwa kegagalan DB tidak memblokir alur utama.

2. **Property-based test** — menggunakan database Postgres nyata (via Docker atau test container) untuk memverifikasi correctness properties di atas dengan input yang di-generate secara acak.

### Library Property-Based Testing

Gunakan **`pgx/v5`** untuk koneksi dan **`github.com/leanovate/gopter`** atau **`testing/quick`** (stdlib) sebagai PBT library. Rekomendasi: `testing/quick` untuk kesederhanaan, atau `gopter` untuk generator yang lebih ekspresif.

Setiap property test dikonfigurasi minimum **100 iterasi**.

### Tag Format

Setiap property test diberi komentar:
```go
// Feature: telkomsel-transaction-log, Property 2: Insert Transaction Round-Trip
```

### Unit Test (Mock-Based)

```
internal/infrastructure/postgres/transaction_logger_test.go
  - TestInsertTransaction_RoundTrip          (Property 2)
  - TestInsertTransaction_Idempotent         (Property 3)
  - TestUpdateTransactionStatus_RoundTrip    (Property 4)
  - TestInsertSyncResponse_RoundTrip         (Property 5)
  - TestMultipleResponsesPerMsgID            (Property 6)
  - TestInsertCallbackResponse_Type          (Property 7)
  - TestContextCancellation                  (Property 8)
  - TestRunMigration_Idempotent              (Property 1)

internal/infrastructure/rabbitmq/consumer_service_test.go
  - TestConsumer_DBFailure_NonBlocking       (Req 3.4, 4.4, 5.4)
  - TestConsumer_NilLogger_NoOp              (Req 1.2)
  - TestConsumer_BrowseOfferFail_StatusFailed (Req 6.4)
```

### Integration Test (Postgres Nyata)

Dijalankan dengan tag build `//go:build integration` dan membutuhkan `POSTGRES_DSN` di environment:

```
internal/infrastructure/postgres/transaction_logger_integration_test.go
  - TestMigration_Idempotent_Integration
  - TestFullFlow_Pulsa_Integration
  - TestFullFlow_PaketData_Integration
  - TestDuplicateMsgID_Integration
```

### Smoke Test

Dijalankan sebagai bagian dari deployment verification:
- Verifikasi tabel `telkomsel_transaction` dan `telkomsel_transaction_response` ada.
- Verifikasi index `idx_telkomsel_transaction_response_msg_id` ada.
- Verifikasi koneksi Postgres berhasil dengan DSN yang dikonfigurasi.
