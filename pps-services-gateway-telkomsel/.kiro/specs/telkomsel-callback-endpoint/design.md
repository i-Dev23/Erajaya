# Dokumen Desain: Telkomsel Callback Endpoint

## Overview

Fitur ini menambahkan HTTP callback endpoint ke `pps-services-gateway-telkomsel` untuk menerima notifikasi asinkron dari Telkomsel terkait fulfillment VAS Recharge (paket data). Saat ini service hanya memiliki RabbitMQ consumer — belum ada HTTP server.

Komponen utama yang ditambahkan:
- **HTTP Server** (gofiber) yang berjalan bersamaan dengan RabbitMQ consumer dalam satu proses
- **Callback Handler** yang menerima `GET /callback/ext`, memvalidasi query parameter, mencatat ke database, dan meneruskan status ke downstream
- **Method baru `GetTransactionByOurTrxID`** pada `TransactionLogger` untuk lookup `msg_id` dari `our_trx_id`

Alur callback: Telkomsel memanggil `GET /callback/ext?transaction_id=...&status=...` via URL publik (internet). Field `transaction_id` dari callback adalah `our_trx_id` yang dikirim saat `OrderDealer` — bukan `msg_id` internal kita. Oleh karena itu, handler perlu melakukan lookup ke tabel `telkomsel_transaction` menggunakan `our_trx_id` untuk mendapatkan `msg_id` asli, agar callback bisa di-link ke transaksi yang benar.

Prinsip desain utama: **kegagalan database atau downstream tidak boleh mengganggu response ke Telkomsel**. Callback selalu di-acknowledge dengan HTTP 200 OK selama parameter valid.

---

## Architecture

### Alur Callback

```
Telkomsel (Internet)
   |
   |  GET /callback/ext?transaction_id=...&organization_code=...
   |      &service_id=...&status=...&message=...&serial_number=...
   |
   v
gofiber HTTP Server (:CALLBACK_PORT)
   |
   v
CallbackHandler.Handle(c *fiber.Ctx)
   |
   |-- 1. Parse & validasi query parameters
   |       Jika invalid -> HTTP 400 Bad Request
   |
   |-- 2. Log info: callback diterima
   |
   |-- 3. TransactionLogger.GetTransactionByOurTrxID(transaction_id)
   |       Lookup msg_id dari tabel telkomsel_transaction berdasarkan our_trx_id
   |       Jika tidak ditemukan -> gunakan transaction_id sebagai fallback msg_id
   |
   |-- 4. TransactionLogger.InsertCallbackResponse(ResponseRecord)
   |       Insert ke telkomsel_transaction_response (response_type = 'CALLBACK')
   |       Update status di telkomsel_transaction (SUCCESS/FAILED)
   |
   |-- 5. DownstreamClient.ForwardToPublisher(CallbackRequest)
   |       Forward status ke pps-services-publisher-database
   |       MsgId diisi dari msg_id hasil lookup (bukan transaction_id)
   |
   +-- 6. HTTP 200 OK {"status": "OK", "message": "Callback received"}
```


### Lifecycle: HTTP Server + RabbitMQ Consumer

```
main.go
   |
   |-- config.Load()
   |-- config.LoadCallbackServer()          <- BARU: baca CALLBACK_PORT
   |-- postgres.NewTransactionLogger()
   |-- txLogger.RunMigration()
   |-- consumer.SetTransactionLogger()
   |-- consumer.SetDownstreamClient()
   |
   |-- errgroup.Group (dengan shared context)
   |     |
   |     |-- goroutine 1: consumer.Start(ctx)
   |     |
   |     +-- goroutine 2: httpServer.Listen(ctx)
   |
   +-- Jika salah satu return error -> cancel context -> semua shutdown
```

Menggunakan `golang.org/x/sync/errgroup` untuk mengoordinasikan lifecycle kedua goroutine. Jika salah satu komponen gagal fatal, context di-cancel dan komponen lain ikut shutdown.

### Posisi Komponen dalam Codebase

```
internal/
  handler/
    callback_handler.go          <- BARU: HTTP handler untuk callback
    callback_handler_test.go     <- BARU: unit test handler
  http/
    server.go                    <- BARU: setup gofiber server + routing
  infrastructure/
    postgres/
      transaction_logger.go      <- MODIFIKASI: tambah GetTransactionByOurTrxID
    downstream/
      client.go                  <- TIDAK BERUBAH
  domain/
    contract/
      service/
        transaction_logger.go    <- MODIFIKASI: tambah GetTransactionByOurTrxID ke interface
  config/
    config.go                    <- MODIFIKASI: tambah LoadCallbackServer()
cmd/
  app/
    main.go                      <- MODIFIKASI: start HTTP server + consumer via errgroup
```

---

## Components and Interfaces

### Struct `CallbackQuery`

Representasi query parameter callback dari Telkomsel:

```go
package handler

// CallbackQuery merepresentasikan query parameter dari callback Telkomsel.
type CallbackQuery struct {
    TransactionID    string `query:"transaction_id"`
    OrganizationCode string `query:"organization_code"`
    ServiceID        string `query:"service_id"`
    Status           string `query:"status"`
    Message          string `query:"message"`
    SerialNumber     string `query:"serial_number"`
}
```

### Struct `CallbackResponse`

Response JSON yang dikembalikan ke Telkomsel:

```go
// CallbackResponse adalah format response JSON untuk callback endpoint.
type CallbackResponse struct {
    Status  string `json:"status"`
    Message string `json:"message"`
}
```

### Interface `CallbackHandler`

```go
package handler

import (
    "github.com/gofiber/fiber/v2"
    "pps-services-gateway-telkomsel/internal/infrastructure/downstream"
    contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
)

// CallbackHandler menangani HTTP callback dari Telkomsel.
type CallbackHandler struct {
    logger           contractsvc.Logger
    transactionLogger contractsvc.TransactionLogger // nil jika POSTGRES_DSN tidak dikonfigurasi
    downstreamClient *downstream.DownstreamClient   // nil jika downstream tidak dikonfigurasi
    queueName        string                          // nama queue untuk field QueueName di CallbackRequest
}

// NewCallbackHandler membuat instance baru CallbackHandler.
func NewCallbackHandler(
    logger contractsvc.Logger,
    transactionLogger contractsvc.TransactionLogger,
    downstreamClient *downstream.DownstreamClient,
    queueName string,
) *CallbackHandler

// Handle memproses callback GET request dari Telkomsel.
// Route: GET /callback/ext
func (h *CallbackHandler) Handle(c *fiber.Ctx) error
```

### Penambahan Method pada Interface `TransactionLogger`

Tambahkan method baru ke interface yang sudah ada di `internal/domain/contract/service/transaction_logger.go`:

```go
// GetTransactionByOurTrxID mengambil satu baris dari telkomsel_transaction berdasarkan our_trx_id.
// Mengembalikan TransactionRecord dan nil error jika ditemukan.
// Mengembalikan error jika tidak ditemukan atau terjadi error database.
GetTransactionByOurTrxID(ctx context.Context, ourTrxID string) (*TransactionRecord, error)
```

**Alasan**: `transaction_id` dari callback Telkomsel adalah `our_trx_id` yang di-generate oleh `buildTelkomselTransactionID()` saat `OrderDealer`. Untuk menghubungkan callback ke transaksi asli, kita perlu lookup `msg_id` dari tabel `telkomsel_transaction` menggunakan `our_trx_id`. Field `msg_id` ini diperlukan untuk:
1. Mengisi `MsgID` pada `ResponseRecord` saat `InsertCallbackResponse`
2. Mengisi `MsgId` (integer) pada `CallbackRequest` saat forward ke downstream

### Implementasi `GetTransactionByOurTrxID`

Ditambahkan di `internal/infrastructure/postgres/transaction_logger.go`:

```go
const getTransactionByOurTrxIDSQL = `
SELECT msg_id, our_trx_id, msisdn, mid, product_type, product_id, amount, stock_type,
       queue_name, mq_transaction
FROM telkomsel_transaction
WHERE our_trx_id = $1
LIMIT 1`

func (l *PostgresTransactionLogger) GetTransactionByOurTrxID(ctx context.Context, ourTrxID string) (*contractsvc.TransactionRecord, error) {
    var rec contractsvc.TransactionRecord
    var productID, stockType, mqTransaction sql.NullString

    err := l.db.QueryRowContext(ctx, getTransactionByOurTrxIDSQL, ourTrxID).Scan(
        &rec.MsgID, &rec.OurTrxID, &rec.MSISDN, &rec.MID,
        &rec.ProductType, &productID, &rec.Amount, &stockType,
        &rec.QueueName, &mqTransaction,
    )
    if err != nil {
        return nil, fmt.Errorf("get transaction by our_trx_id: %w", err)
    }

    rec.ProductID = productID.String
    rec.StockType = stockType.String
    rec.MQTransaction = mqTransaction.String

    return &rec, nil
}
```

**Catatan**: Perlu menambahkan index pada kolom `our_trx_id` untuk performa lookup:

```sql
CREATE INDEX IF NOT EXISTS idx_telkomsel_transaction_our_trx_id
    ON telkomsel_transaction (our_trx_id);
```

Index ini ditambahkan ke `migrationStatements` di `transaction_logger.go`.

### Setup HTTP Server

Ditempatkan di `internal/http/server.go`:

```go
package http

import (
    "context"
    "fmt"

    "github.com/gofiber/fiber/v2"
    "github.com/gofiber/fiber/v2/middleware/recover"

    "pps-services-gateway-telkomsel/internal/handler"
    contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
)

// ServerConfig berisi konfigurasi HTTP server.
type ServerConfig struct {
    Port int
}

// Server mengelola gofiber HTTP server.
type Server struct {
    app    *fiber.App
    config ServerConfig
    logger contractsvc.Logger
}

// NewServer membuat instance Server baru dengan routing yang sudah dikonfigurasi.
func NewServer(cfg ServerConfig, callbackHandler *handler.CallbackHandler, logger contractsvc.Logger) *Server

// Listen memulai HTTP server dan menunggu context di-cancel untuk graceful shutdown.
func (s *Server) Listen(ctx context.Context) error
```

### Konfigurasi `CallbackServerConfig`

Ditambahkan di `internal/config/config.go`:

```go
// CallbackServerConfig stores HTTP callback server configuration.
type CallbackServerConfig struct {
    Port int
}

// LoadCallbackServer reads callback server configuration from environment variables.
func LoadCallbackServer() (*CallbackServerConfig, error)
```

### Modifikasi `main.go`

```go
// main.go — perubahan utama:
// 1. Import errgroup
// 2. Inisialisasi CallbackHandler dan HTTP Server
// 3. Jalankan consumer dan HTTP server dalam errgroup

import "golang.org/x/sync/errgroup"

// ... setelah inisialisasi consumer, transactionLogger, downstreamClient ...

callbackHandler := handler.NewCallbackHandler(logger, txLogger, downstreamClient, cfg.QueueName)
httpServer := http.NewServer(callbackCfg, callbackHandler, logger)

g, gCtx := errgroup.WithContext(ctx)

g.Go(func() error {
    return consumer.Start(gCtx)
})

g.Go(func() error {
    return httpServer.Listen(gCtx)
})

if err := g.Wait(); err != nil {
    logger.Error("service stopped with error", "error", err)
    os.Exit(1)
}
```

---

## Data Models

### Tabel yang Sudah Ada (Tidak Berubah)

Fitur ini menggunakan tabel `telkomsel_transaction` dan `telkomsel_transaction_response` yang sudah dibuat oleh fitur Transaction Log. Tidak ada perubahan DDL pada struktur tabel.

### Index Baru

Satu index baru ditambahkan untuk mendukung lookup callback:

```sql
CREATE INDEX IF NOT EXISTS idx_telkomsel_transaction_our_trx_id
    ON telkomsel_transaction (our_trx_id);
```

Index ini diperlukan karena callback dari Telkomsel mengirim `transaction_id` yang merupakan `our_trx_id` — bukan `msg_id` (primary key). Tanpa index, query `WHERE our_trx_id = $1` akan melakukan full table scan.

### Mapping: Callback Query Parameter ke ResponseRecord

| Field `ResponseRecord` | Sumber |
|---|---|
| `MsgID` | Hasil lookup `GetTransactionByOurTrxID(transaction_id).MsgID` — fallback ke `transaction_id` jika lookup gagal |
| `OurTrxID` | `transaction_id` dari query parameter |
| `TelkomselTrxID` | Kosong (Telkomsel tidak mengirim TrxID di callback) |
| `StatusCode` | `"0"` jika `status = "SUCCESS"`, `"1"` jika `status = "FAILED"` |
| `StatusDesc` | `message` dari query parameter (setelah URL-decode) |
| `RequestPayload` | `nil` (callback adalah GET request, tidak ada request body) |
| `RawPayload` | Seluruh query parameter callback dalam format JSON |
| `RequestedAt` | `time.Time{}` (zero value — tidak relevan untuk callback) |
| `ResponseLatencyMs` | `0` (tidak relevan untuk callback) |

### Mapping: Callback ke CallbackRequest (Downstream)

| Field `CallbackRequest` | Sumber |
|---|---|
| `MsgId` | `strconv.Atoi(msgID)` — dari hasil lookup `GetTransactionByOurTrxID` |
| `StatusToBe` | `status` dari query parameter (`"SUCCESS"` atau `"FAILED"`) |
| `SerialNumber` | `serial_number` dari query parameter (kosong jika tidak ada) |
| `ClientNumber` | `service_id` dari query parameter |
| `Nominal` | Kosong (tidak tersedia di callback) |
| `OriginalConversationID` | Kosong |
| `ConversationID` | `transaction_id` dari query parameter |
| `MessageToCustomer` | `message` dari query parameter |
| `AdditionalMessage` | Kosong |
| `QueueName` | Dari konfigurasi queue name service |
| `Source` | `"PROVIDER"` |

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Valid Callback Menghasilkan HTTP 200

*For any* kombinasi query parameter yang valid (transaction_id non-kosong, organization_code 6-13 karakter, service_id 13 karakter, status "SUCCESS" atau "FAILED", message non-kosong), handler harus mengembalikan HTTP 200 OK dengan body `{"status": "OK", "message": "Callback received"}`.

**Validates: Requirements 2.3, 6.1**

### Property 2: Invalid Callback Menghasilkan HTTP 400

*For any* kombinasi query parameter yang tidak valid — yaitu salah satu dari: parameter mandatory kosong/tidak ada, status bukan "SUCCESS"/"FAILED", organization_code panjang < 6 atau > 13, atau service_id panjang bukan 13 — handler harus mengembalikan HTTP 400 Bad Request.

**Validates: Requirements 2.4, 2.5, 3.1, 3.2**

### Property 3: URL-Decoding Message Round-Trip

*For any* string message yang di-URL-encode, setelah handler menerima dan memproses callback, nilai message yang disimpan ke database (via `InsertCallbackResponse`) dan diteruskan ke downstream (via `ForwardToPublisher`) harus sama dengan string asli sebelum encoding.

**Validates: Requirements 3.3**

### Property 4: Mapping Callback ke ResponseRecord

*For any* callback yang valid, handler harus memanggil `InsertCallbackResponse` dengan `ResponseRecord` yang memenuhi:
- `MsgID` = hasil lookup `GetTransactionByOurTrxID(transaction_id).MsgID` (atau `transaction_id` jika lookup gagal)
- `OurTrxID` = `transaction_id`
- `StatusCode` = `"0"` jika `status = "SUCCESS"`, `"1"` jika `status = "FAILED"`
- `StatusDesc` = `message` (URL-decoded)
- `RawPayload` = JSON dari seluruh query parameter

**Validates: Requirements 4.1, 4.2**

### Property 5: Mapping Callback ke CallbackRequest (Downstream)

*For any* callback yang valid, handler harus memanggil `ForwardToPublisher` dengan `CallbackRequest` yang memenuhi:
- `MsgId` = integer dari `msg_id` hasil lookup
- `StatusToBe` = `status` dari query parameter
- `SerialNumber` = `serial_number` dari query parameter
- `ClientNumber` = `service_id` dari query parameter
- `ConversationID` = `transaction_id` dari query parameter
- `MessageToCustomer` = `message` (URL-decoded)
- `QueueName` = queue name dari konfigurasi
- `Source` = `"PROVIDER"`

**Validates: Requirements 5.1, 5.2**

### Property 6: CALLBACK_PORT Non-Numerik Menghasilkan Error

*For any* string yang bukan angka valid (termasuk string kosong dikecualikan karena fallback ke default), `LoadCallbackServer` harus mengembalikan error yang menjelaskan bahwa `CALLBACK_PORT` harus berupa angka.

**Validates: Requirements 8.3**

---

## Error Handling

### Prinsip: Selalu Acknowledge Callback

Telkomsel mengharapkan HTTP 200 OK sebagai tanda callback diterima. Jika kita mengembalikan error (4xx/5xx), Telkomsel mungkin akan retry — yang bisa menyebabkan duplikasi. Oleh karena itu:

- **Validasi gagal** → HTTP 400 (callback memang invalid, retry tidak akan membantu)
- **Database gagal** → Log error, tetap return HTTP 200
- **Downstream gagal** → Log error, tetap return HTTP 200
- **Panic/unexpected** → gofiber recover middleware → HTTP 500

### Tabel Error Handling

| Kondisi | Behavior Handler | HTTP Response |
|---|---|---|
| Parameter valid, semua berhasil | Proses normal | 200 OK |
| Parameter mandatory kosong | Reject | 400 Bad Request |
| Status bukan SUCCESS/FAILED | Reject | 400 Bad Request |
| organization_code panjang invalid | Reject | 400 Bad Request |
| service_id panjang invalid | Reject | 400 Bad Request |
| `GetTransactionByOurTrxID` gagal | Log warning, gunakan transaction_id sebagai fallback msg_id | 200 OK |
| `InsertCallbackResponse` gagal | Log error, lanjut ke forwarding | 200 OK |
| `ForwardToPublisher` gagal | Log error | 200 OK |
| TransactionLogger nil | Skip semua DB ops | 200 OK |
| DownstreamClient nil | Skip forwarding, log warning | 200 OK |
| Panic/unexpected error | gofiber recover middleware | 500 Internal Server Error |

### Fallback MsgID

Jika `GetTransactionByOurTrxID` gagal (transaksi tidak ditemukan di database, atau database error), handler menggunakan `transaction_id` dari callback sebagai fallback `msg_id`. Ini memastikan callback tetap dicatat dan diteruskan meskipun lookup gagal. Konsekuensinya:
- `MsgId` di `CallbackRequest` downstream mungkin bukan integer yang valid → `strconv.Atoi` akan return 0
- Record di `telkomsel_transaction_response` tetap tercatat dengan `msg_id = transaction_id` (our_trx_id)

---

## Testing Strategy

### Pendekatan Dual Testing

Fitur ini menggunakan dua lapisan testing:

1. **Unit test** — menggunakan mock `TransactionLogger` dan mock `DownstreamClient` untuk memverifikasi behavior handler secara terisolasi. Menggunakan `httptest` atau gofiber test utilities.

2. **Property-based test** — menggunakan generated input untuk memverifikasi correctness properties di atas. Fokus pada validasi parameter dan mapping field.

### Library Property-Based Testing

Gunakan **`testing/quick`** (stdlib Go) untuk property-based testing. Setiap property test dikonfigurasi minimum **100 iterasi**.

### Tag Format

Setiap property test diberi komentar:
```go
// Feature: telkomsel-callback-endpoint, Property 1: Valid Callback Menghasilkan HTTP 200
```

### Unit Test

```
internal/handler/callback_handler_test.go
  - TestHandle_ValidCallback_Returns200                    (Property 1 — PBT)
  - TestHandle_InvalidParams_Returns400                    (Property 2 — PBT)
  - TestHandle_URLDecodedMessage_RoundTrip                 (Property 3 — PBT)
  - TestHandle_ResponseRecordMapping                       (Property 4 — PBT)
  - TestHandle_CallbackRequestMapping                      (Property 5 — PBT)
  - TestHandle_DBFailure_StillReturns200                   (Req 4.4 — example)
  - TestHandle_DownstreamFailure_StillReturns200           (Req 5.4 — example)
  - TestHandle_NilTransactionLogger_StillReturns200        (Req 10.3 — example)
  - TestHandle_NilDownstreamClient_StillReturns200         (Req 5.3 — example)
  - TestHandle_LookupFails_FallbackMsgID                   (example)
  - TestHandle_ExactResponseBody                           (Req 6.1, 6.2 — example)

internal/config/config_test.go
  - TestLoadCallbackServer_NonNumericPort_ReturnsError     (Property 6 — PBT)
  - TestLoadCallbackServer_DefaultPort                     (Req 1.2, 8.2 — example)
  - TestLoadCallbackServer_CustomPort                      (example)

internal/infrastructure/postgres/transaction_logger_test.go
  - TestGetTransactionByOurTrxID_RoundTrip                 (example)
  - TestGetTransactionByOurTrxID_NotFound                  (example)
```

### Integration Test

Dijalankan dengan tag build `//go:build integration`:

```
internal/handler/callback_handler_integration_test.go
  - TestCallbackEndpoint_FullFlow_Integration
  - TestCallbackEndpoint_GracefulShutdown_Integration
  - TestCallbackEndpoint_ConcurrentWithConsumer_Integration
```

### Dependency Baru

```
github.com/gofiber/fiber/v2
golang.org/x/sync/errgroup  (sudah ada di go.sum via indirect)
```
