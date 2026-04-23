# Source Code Guide — pps-services-publisher-database

Dokumentasi lengkap source code project publisher database service.
Service ini menerima callback HTTP dari biller, lalu mem-publish event ke RabbitMQ
untuk diproses oleh consumer service yang menyimpan data ke Oracle Database.

---

## 1. Arsitektur & Struktur Project

### Arsitektur

Project menggunakan **Clean Architecture** dengan layer separation:

```
HTTP Request
    ↓
[Fiber HTTP Server]
    ↓
[Middleware] → API Key Auth + Request Logger
    ↓
[Controller] → Parse request, return response
    ↓
[UseCase] → Validasi, business logic
    ↓
[Publisher] → Publish event ke RabbitMQ
    ↓
[RabbitMQ] → Message broker (quorum queue + DLX)
```

### Struktur Direktori

```
pps-services-publisher-database/
├── cmd/web/main.go                          # Entry point aplikasi
├── config.json                              # File konfigurasi utama
├── internal/
│   ├── config/
│   │   ├── app.go                           # Bootstrap dependency injection
│   │   ├── fiber.go                         # Konfigurasi Fiber HTTP server
│   │   ├── oracle.go                        # Koneksi Oracle Database
│   │   ├── rabbitmq.go                      # Koneksi RabbitMQ + channel pool + reconnect
│   │   ├── validator.go                     # Konfigurasi go-playground/validator
│   │   ├── viper.go                         # Konfigurasi Viper (config loader)
│   │   └── zerolog.go                       # Konfigurasi Zerolog logger
│   ├── delivery/http/
│   │   ├── transaction_controller.go        # HTTP handler untuk callback & health
│   │   ├── middleware/auth_middleware.go     # API Key auth & request logging
│   │   └── route/route.go                   # Registrasi semua route
│   ├── gateway/messaging/
│   │   ├── interfaces.go                    # Interface RabbitMQProvider & MessageHandler
│   │   └── publisher.go                     # Publisher ke RabbitMQ dengan retry
│   ├── model/model.go                       # DTO: request, response, event
│   ├── pkg/logger/logger.go                 # Logger helper (context logger, PII masking, Telegram)
│   ├── repository/transaction_repository.go # Data access layer (Oracle ping)
│   └── usecase/transaction_usecase.go       # Business logic layer
├── docs/                                    # Swagger docs
├── test/                                    # Unit test & load test
└── logs/                                    # Log file output
```

### Pattern yang Digunakan

| Pattern | Penjelasan |
|---------|-----------|
| Clean Architecture | Separation of concerns: delivery → usecase → gateway/repository |
| Dependency Injection | Manual DI via `Bootstrap()` di `config/app.go` |
| Channel Pool | Pool `*amqp.Channel` untuk menghindari concurrent access |
| Circuit Breaker-like | Exponential backoff pada publish dan reconnect |
| DTO Pattern | `model.CallbackRequest` → `model.TransactionEvent` konversi antar layer |
| PII Masking | Custom `io.Writer` yang mask data sensitif sebelum ditulis ke log |
| Graceful Shutdown | Signal handling (SIGINT/SIGTERM) dengan WaitGroup |

---

## 2. `cmd/web/main.go` — Entry Point

File ini adalah entry point aplikasi. Menginisialisasi semua dependency dan menjalankan HTTP server.

### Flow Inisialisasi

```go
func main() {
    // 1. Load config dari config.json via Viper
    viperConfig := config.NewViper()

    // 2. Setup logger (zerolog) dengan PII masking & file rotation
    log := config.NewLogger(viperConfig)

    // 3. Setup validator untuk validasi request
    validate := config.NewValidator(viperConfig)

    // 4. Buat Fiber HTTP server
    app := config.NewFiber(viperConfig)

    // 5. Koneksi ke Oracle Database dengan connection pool
    oracleDB := config.NewOracleDB(viperConfig, log)

    // 6. Koneksi ke RabbitMQ dengan channel pool & auto-reconnect
    rabbitMQ := config.NewRabbitMQ(viperConfig, log)

    // 7. Bootstrap: wire semua dependency (DI manual)
    config.Bootstrap(&config.BootstrapConfig{...})

    // 8. Start HTTP server di goroutine
    go app.Listen(":3000")

    // 9. Tunggu signal SIGINT/SIGTERM
    // 10. Graceful shutdown: tutup HTTP, RabbitMQ, Oracle secara paralel
}
```

### Graceful Shutdown

- Menangkap `SIGINT` dan `SIGTERM` via channel
- Timeout 30 detik untuk shutdown
- Shutdown paralel menggunakan `sync.WaitGroup`:
  1. `app.ShutdownWithContext()` — stop HTTP server
  2. `rabbitMQ.Close()` — drain channel pool, tutup koneksi AMQP
  3. `config.CloseOracleDB()` — tutup koneksi Oracle

---

## 3. `internal/config/` — Konfigurasi

### `viper.go` — Config Loader

**Fungsi:** `NewViper() *viper.Viper`

- Membaca `config.json` dari current directory atau parent directory
- Mendukung override via environment variable dengan prefix `PPS_`
- Contoh: `PPS_ORACLE_HOST` override `oracle.host`
- Menggunakan `strings.NewReplacer(".", "_")` untuk mapping key → env var
- Panic jika config file tidak ditemukan (config wajib ada)

### `zerolog.go` — Logger

**Struct:** `PIIMaskingWriter`
- Custom `io.Writer` yang mask data PII (telepon, email, NIK, nomor kartu) sebelum log ditulis
- Pattern di-compile saat init, bukan per-write

**Fungsi:** `NewLogger(config *viper.Viper) zerolog.Logger`
- Membuat zerolog logger dengan level dari config (`log.level`: 0=Panic s/d 6=Trace)
- 3 mode output: `"console"` (stdout), `"file"` (rotatelogs), `"both"` (keduanya)
- File rotation menggunakan `lestrrat-go/file-rotatelogs` (rotasi harian)
- Semua output dibungkus `PIIMaskingWriter`

**Fungsi:** `newFileWriter(config *viper.Viper) io.Writer`
- Membuat file writer dengan rotation harian dan max age dari config

**Fungsi:** `SendTelegramLog(config, log, message)`
- Mengirim notifikasi ke Telegram bot via HTTP POST
- Fire-and-forget: error hanya di-log, tidak di-return
- Skip jika `log.telegram.enabled` = false

**Fungsi:** `ContextLogger(log, traceID) zerolog.Logger`
- Menambahkan `trace_id` field ke logger untuk request tracing

**Fungsi:** `MaskPII(s string) string`
- Mask manual pada string: telepon → `***PHONE***`, email → `***EMAIL***`, NIK → `***NIK***`

### `fiber.go` — HTTP Server

**Fungsi:** `NewFiber(config *viper.Viper) *fiber.App`
- Membuat Fiber v3 app dengan nama dari config
- Custom error handler yang return JSON `{"errors": "..."}` dengan status code yang sesuai

**Fungsi:** `NewErrorHandler() fiber.ErrorHandler`
- Jika error adalah `*fiber.Error`, gunakan code-nya
- Selain itu, return 500 Internal Server Error

### `validator.go` — Request Validator

**Fungsi:** `NewValidator(config *viper.Viper) *validator.Validate`
- Membuat instance `go-playground/validator`
- Digunakan untuk validasi struct tag pada `CallbackRequest`

### `oracle.go` — Oracle Database

**Fungsi:** `NewOracleDB(config *viper.Viper, log zerolog.Logger) *sql.DB`
- Membuat koneksi Oracle menggunakan `go-ora` driver (pure Go)
- Connection string: `oracle://user:pass@host:port/service`
- Konfigurasi pool: `MaxOpenConns`, `MaxIdleConns`, `ConnMaxLifetime`
- Ping awal untuk verifikasi koneksi (warn jika gagal, tidak fatal)

**Fungsi:** `CloseOracleDB(db *sql.DB, log zerolog.Logger)`
- Menutup koneksi Oracle dengan aman, dipanggil saat graceful shutdown

### `rabbitmq.go` — RabbitMQ Connection + Channel Pool

**Struct:** `RabbitMQConnection`

| Field | Tipe | Fungsi |
|-------|------|--------|
| `conn` | `*amqp.Connection` | Koneksi AMQP aktif |
| `channels` | `chan *amqp.Channel` | Buffered channel sebagai pool |
| `config` | `*viper.Viper` | Akses konfigurasi |
| `log` | `zerolog.Logger` | Logger |
| `mu` | `sync.RWMutex` | Proteksi akses `conn` |
| `closed` | `int32` | Atomic flag: 0=open, 1=closed |
| `reconnecting` | `int32` | Atomic flag: 0=idle, 1=reconnecting |
| `verifiedQueues` | `sync.Map` | Cache queue yang sudah di-declare |
| `notifyReconn` | `[]chan struct{}` | Subscriber yang menunggu reconnect selesai |

**Fungsi:** `NewRabbitMQ(config, log) *RabbitMQConnection`
- Membuat koneksi, setup exchange, declare startup queues
- Menjalankan `connectionWatcher` goroutine untuk auto-reconnect
- Fatal jika koneksi awal gagal

**Method:** `connect() error`
- Membuka koneksi AMQP dan mengisi channel pool
- Drain pool lama jika reconnect
- Channel di pool tanpa QoS (untuk publisher)

**Method:** `drainPool()`
- Non-blocking drain semua channel lama dari pool

**Method:** `setupExchange() error`
- Declare main exchange (direct) dan dead letter exchange (DLX)

**Method:** `declareStartupQueues()`
- Declare queue saat startup jika `queue_declare_mode` = `"startup"`
- Mode `"lazy"` (default): queue di-declare saat pertama kali publish

**Method:** `connectionWatcher()`
- Goroutine tunggal yang monitor koneksi via `NotifyClose`
- Saat koneksi putus → panggil `reconnectWithBackoff()`
- Tidak spawn goroutine baru saat reconnect

**Method:** `reconnectWithBackoff()`
- Exponential backoff menggunakan `cenkalti/backoff`
- Atomic guard: hanya satu reconnect berjalan pada satu waktu
- Clear queue cache setelah reconnect
- `MaxElapsedTime=0` → retry forever sampai berhasil atau `Close()`
- Kirim notifikasi Telegram saat reconnect berhasil
- Re-declare startup queues setelah reconnect

**Method:** `waitForReconnect()`
- Caller bisa block sampai reconnect selesai via channel

**Method:** `GetChannel() (*amqp.Channel, error)`
- Ambil channel dari pool dengan timeout 10 detik
- Jika sedang reconnect, tunggu dulu
- Jika channel stale (closed), buat replacement dari koneksi aktif

**Method:** `PutChannel(ch *amqp.Channel)`
- Kembalikan channel ke pool
- Discard jika channel sudah closed atau pool penuh

**Method:** `GetConnection() *amqp.Connection`
- Return koneksi AMQP aktif (thread-safe via RWMutex)

**Method:** `EnsureQueue(queueName string) error`
- Declare quorum queue dengan dead letter support
- Bind ke main exchange dengan routing key = queue name
- Declare DLQ (dead letter queue) jika DLX enabled
- Hasil di-cache di `verifiedQueues`, clear saat reconnect

**Method:** `Close()`
- Atomic close: drain pool, tutup semua channel, tutup koneksi
- Idempotent via `CompareAndSwapInt32`

### `app.go` — Bootstrap / Dependency Injection

**Struct:** `BootstrapConfig`
- Container untuk semua dependency: OracleDB, App, Log, Validate, Config, RabbitMQ

**Fungsi:** `Bootstrap(config *BootstrapConfig)`
- Wire semua dependency secara manual (tanpa framework DI):
  1. `TransactionRepository` ← OracleDB, Log
  2. `Publisher` ← RabbitMQ, Log, Config
  3. `TransactionUseCase` ← Repository, Publisher, Validate, Log
  4. `TransactionController` ← UseCase, Log
  5. `RouteConfig` ← App, Controller, Config, Log → `Setup()`

---

## 4. `internal/delivery/http/` — Controller, Middleware, Route

### `transaction_controller.go` — HTTP Handler

**Struct:** `TransactionController`
- Field: `Log` (zerolog.Logger), `UseCase` (*TransactionUseCase)

**Constructor:** `NewTransactionController(useCase, log) *TransactionController`

**Method:** `SubmitCallback(ctx fiber.Ctx) error`
- Parse JSON body ke `CallbackRequest`
- Panggil `UseCase.ProcessCallback()`
- Return `WebResponse[*CallbackResponse]` jika sukses
- Return 400 jika body invalid, 404 jika transaksi tidak ditemukan, 500 untuk error lain

**Method:** `HealthCheck(ctx fiber.Ctx) error`
- Cek Oracle via `UseCase.PingOracle()`
- Return status per service: `"healthy"` atau `"unhealthy"`
- Status keseluruhan: `"healthy"` atau `"degraded"`

### `middleware/auth_middleware.go` — Middleware

**Fungsi:** `NewAPIKeyAuth(config, log) fiber.Handler`
- Validasi header `X-API-Key` terhadap `security.api_key` di config
- Return 401 jika key kosong atau tidak cocok

**Fungsi:** `NewRequestLogger(log) fiber.Handler`
- Log setiap HTTP request: method, path, status, IP
- Dipasang sebagai middleware global

### `route/route.go` — Route Registration

**Struct:** `RouteConfig`
- Field: App, TransactionController, Config, Log

**Method:** `Setup()`
- Pasang `NewRequestLogger` sebagai middleware global
- Public routes (tanpa auth):
  - `GET /health` → `HealthCheck`
  - `GET /swagger/*` → Swagger UI
- Protected routes (dengan API Key):
  - `POST /api/callback` → `SubmitCallback`

---

## 5. `internal/usecase/` — Business Logic

### `transaction_usecase.go`

**Struct:** `TransactionUseCase`
- Field: `repo`, `publisher`, `validate`, `log`

**Constructor:** `NewTransactionUseCase(repo, publisher, validate, log) *TransactionUseCase`

**Method:** `ProcessCallback(ctx, req *CallbackRequest) (*CallbackResponse, error)`
1. Generate `traceID` (UUID) untuk request tracing
2. Buat context logger dengan trace_id
3. Validasi `CallbackRequest` menggunakan validator
4. Konversi `CallbackRequest` → `TransactionEvent` via `req.ToTransactionEvent()`
5. Publish event ke RabbitMQ via `publisher.Publish(event)`
6. Return `CallbackResponse{MsgId, Status: "published"}`

**Method:** `PingOracle(ctx) error`
- Delegasi ke `repo.PingOracle()` untuk health check

---

## 6. `internal/gateway/messaging/` — Publisher & Interfaces

### `interfaces.go`

**Interface:** `RabbitMQProvider`
- `GetChannel() (*amqp.Channel, error)` — ambil channel dari pool
- `PutChannel(*amqp.Channel)` — kembalikan channel ke pool
- `GetConnection() *amqp.Connection` — koneksi AMQP aktif
- `EnsureQueue(queueName string) error` — declare + bind queue
- `IsReconnecting() bool` — status reconnect

**Interface:** `MessageHandler`
- `Handle(ctx, event *TransactionEvent) error` — proses message dari queue

### `publisher.go`

**Struct:** `Publisher`
- Field: `rmq` (RabbitMQProvider), `log`, `config`

**Constructor:** `NewPublisher(rmq, log, config) *Publisher`

**Method:** `Publish(event *TransactionEvent) error`
1. `EnsureQueue(event.QueueName)` — pastikan queue ada
2. Marshal event ke JSON
3. Setup exponential backoff dari config
4. Retry loop: panggil `tryPublish()` sampai berhasil atau timeout

**Method:** `tryPublish(event, body []byte) error`
1. `GetChannel()` dari pool
2. `PublishWithContext()` ke exchange dengan routing key = queue name
3. `PutChannel()` kembalikan channel
4. Message properties: `application/json`, `Persistent`, `MessageId`, `Timestamp`

**Method:** `PublishBatch(events) []error`
- Publish banyak event secara sequential, return error per event

---

## 7. `internal/repository/` — Data Access

### `transaction_repository.go`

**Struct:** `TransactionRepository`
- Field: `oracleDB` (*sql.DB), `log`

**Constructor:** `NewTransactionRepository(oracleDB, log) *TransactionRepository`

**Method:** `PingOracle(ctx) error`
- Panggil `oracleDB.PingContext(ctx)` untuk health check koneksi Oracle

---

## 8. `internal/model/` — DTO / Request / Response

### `model.go`

**Struct:** `WebResponse[T any]`
- Generic response wrapper: `Data T` + `Errors string`

**Struct:** `CallbackRequest`
- Request dari biller dengan validasi struct tag
- Field utama: `MsgId`, `StatusToBe`, `ClientNumber`, `ConversationID`, `QueueName`
- Field opsional: `SerialNumber`, `Nominal`, `OriginalConversationID`, `MessageToCustomer`, `AdditionalMessage`, `Source`

**Method:** `ToTransactionEvent() *TransactionEvent`
- Konversi CallbackRequest → TransactionEvent (copy semua field)

**Struct:** `CallbackResponse`
- Response setelah callback diproses: `MsgId` + `Status`

**Struct:** `TransactionEvent`
- Event yang di-publish ke RabbitMQ, berisi semua data untuk stored procedure di Oracle

**Struct:** `HealthResponse`
- Response health check: `Status` + `Services` (map per service)

---

## 9. `internal/pkg/logger/` — Logger Helper

### `logger.go`

Package terpisah dari `config` untuk menghindari import cycle. Bisa di-import oleh semua layer.

**Fungsi:** `ContextLogger(log, traceID) zerolog.Logger`
- Menambahkan `trace_id` ke logger untuk distributed tracing

**Fungsi:** `SendTelegramLog(config, log, message)`
- Kirim notifikasi ke Telegram bot
- Skip jika disabled atau config kosong

**Variable:** `piiPatterns`
- Pre-compiled regex untuk mask PII: telepon, email, NIK

**Fungsi:** `MaskPII(s string) string`
- Mask data PII dalam string menggunakan pre-compiled patterns

---

## 10. Flow Lengkap: HTTP Request → RabbitMQ

### Alur Normal (Happy Path)

```
1. Client mengirim POST /api/callback dengan header X-API-Key dan body JSON

2. [Middleware: NewRequestLogger]
   → Log: method, path, status, IP

3. [Middleware: NewAPIKeyAuth]
   → Validasi X-API-Key terhadap config security.api_key
   → Jika invalid → 401 Unauthorized

4. [Controller: SubmitCallback]
   → Parse JSON body ke CallbackRequest
   → Jika parse gagal → 400 Bad Request

5. [UseCase: ProcessCallback]
   → Generate trace_id (UUID)
   → Validasi CallbackRequest (struct tag validation)
   → Jika validasi gagal → return error → 500
   → Konversi CallbackRequest → TransactionEvent
   → Panggil Publisher.Publish(event)

6. [Publisher: Publish]
   → EnsureQueue: declare quorum queue + DLQ jika belum ada (cached)
   → Marshal event ke JSON
   → tryPublish dengan exponential backoff:
     a. GetChannel() dari pool
     b. PublishWithContext() ke exchange "pps.transaction"
        routing key = event.QueueName
        content type = application/json
        delivery mode = persistent
     c. PutChannel() kembalikan ke pool
   → Jika gagal, retry dengan backoff sampai timeout

7. [Response]
   → 200 OK: {"data": {"msg_id": 123, "status": "published"}}
```

### Alur Error & Recovery

```
Koneksi RabbitMQ Putus:
1. connectionWatcher() mendeteksi via NotifyClose
2. Kirim notifikasi Telegram: "🔴 RabbitMQ lost"
3. reconnectWithBackoff() dengan exponential backoff (retry forever)
4. Clear queue cache
5. Re-connect, re-setup exchange, re-declare startup queues
6. Kirim notifikasi: "🟢 RabbitMQ reconnected"
7. Publisher yang sedang publish akan retry otomatis

Publish Gagal:
1. tryPublish() gagal (channel stale, koneksi putus)
2. Exponential backoff retry (configurable interval & max elapsed time)
3. Jika semua retry gagal → return error ke UseCase → 500 ke client

Channel Stale:
1. GetChannel() dari pool → channel.IsClosed() = true
2. Buat replacement channel dari koneksi aktif
3. PutChannel() akan discard channel yang closed
```

### Konfigurasi Penting

| Config Key | Default | Fungsi |
|-----------|---------|--------|
| `web.port` | 3000 | Port HTTP server |
| `security.api_key` | - | API key untuk autentikasi |
| `rabbitmq.channel_pool_size` | 10 | Jumlah channel di pool |
| `rabbitmq.exchange.name` | pps.transaction | Nama exchange |
| `rabbitmq.queue_declare_mode` | lazy | `lazy` = declare saat publish, `startup` = declare saat start |
| `rabbitmq.dead_letter.enabled` | true | Enable dead letter queue |
| `rabbitmq.dead_letter.delivery_limit` | 3 | Max retry sebelum masuk DLQ |
| `rabbitmq.publish.backoff_max_elapsed_time` | 60 | Max waktu retry publish (detik) |
| `rabbitmq.reconnect.max_interval` | 60 | Max interval reconnect backoff (detik) |
