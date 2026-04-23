# Source Code Guide — pps-services-consumer-database

Dokumentasi lengkap source code project consumer database.
Service ini consume message dari RabbitMQ dan menyimpan data ke Oracle Database via stored procedure.

---

## 1. Arsitektur & Struktur Project

```
pps-services-consumer-database/
├── cmd/worker/main.go              # Entry point aplikasi (worker process)
├── config.json                     # File konfigurasi utama
├── internal/
│   ├── config/
│   │   ├── viper.go                # Loader konfigurasi (Viper)
│   │   ├── zerolog.go              # Setup logger (zerolog + PII masking)
│   │   ├── validator.go            # Setup validator (go-playground)
│   │   ├── oracle.go               # Koneksi Oracle DB + connection pool
│   │   └── rabbitmq.go             # Koneksi RabbitMQ + auto-reconnect
│   ├── entity/
│   │   └── transaction_entity.go   # Domain entity Transaction
│   ├── model/
│   │   └── model.go                # DTO: event, SP result, response
│   ├── gateway/messaging/
│   │   ├── interfaces.go           # Interface RabbitMQProvider & MessageHandler
│   │   └── consumer.go             # Consumer + circuit breaker + hold mechanism
│   ├── usecase/
│   │   └── transaction_usecase.go  # Business logic layer
│   ├── repository/
│   │   └── transaction_repository.go # Data access (Oracle stored procedure)
│   └── pkg/logger/
│       └── logger.go               # Logger helper (Telegram, PII masking)
├── deployments/oracle/             # Docker setup untuk Oracle dev
├── test/                           # Unit test & load test
├── Dockerfile                      # Container image
├── docker-compose.yml              # Docker compose
├── Makefile                        # Build commands
└── Jenkinsfile                     # CI/CD pipeline
```

### Pattern yang Digunakan

- **Clean Architecture**: Layer terpisah (config → repository → usecase → gateway)
- **Dependency Injection**: Semua dependency di-inject manual di `main.go`
- **Interface Segregation**: `RabbitMQProvider` dan `MessageHandler` di `interfaces.go`
- **Circuit Breaker Pattern**: Proteksi Oracle DB dari cascade failure
- **Hold Message Pattern**: Message di-hold (tidak ack/nack) saat CB open
- **Auto-Reconnect**: RabbitMQ reconnect otomatis dengan exponential backoff
- **Worker Pool**: Multiple goroutine per queue, masing-masing punya AMQP channel sendiri

### Flow Data Antar Layer

```
RabbitMQ Queue
    ↓ (AMQP deliver)
Consumer (gateway/messaging/consumer.go)
    ↓ (processMessage)
Circuit Breaker (sony/gobreaker)
    ↓ (jika CLOSED/HALF-OPEN)
MessageHandler interface
    ↓ (consumerHandler di main.go)
TransactionUseCase (usecase/)
    ↓ (HandleConsumedMessage)
TransactionRepository (repository/)
    ↓ (CallSetTransactionStatus)
Oracle Stored Procedure (MSG.SETTRANSACTIONSTATUS)
```

---

## 2. `cmd/worker/main.go` — Entry Point

### Tujuan
Entry point aplikasi worker. Menginisialisasi semua dependency, memulai consumer, dan menangani graceful shutdown.

### Fungsi `main()`

1. **Inisialisasi dependency** (urutan penting karena ada dependency chain):
   - `config.NewViper()` → load `config.json`
   - `config.NewLogger(viperConfig)` → setup zerolog dengan PII masking
   - `config.NewValidator(viperConfig)` → setup validator
   - `config.NewOracleDB(viperConfig, log)` → buka koneksi Oracle + pool
   - `config.NewRabbitMQ(viperConfig, log)` → koneksi RabbitMQ + auto-reconnect

2. **Wiring layer**:
   - `repository.NewTransactionRepository(oracleDB, log)`
   - `usecase.NewTransactionUseCase(repo, validate, log)`
   - `messaging.NewConsumer(rabbitMQ, handler, viperConfig, log, oracleDB)`

3. **Queue filtering**: Membaca `rabbitmq.consumer.queue_filter` dari config.
   - Jika `"*"` → consume semua queue
   - Jika comma-separated → split dan trim, consume queue yang terdaftar

4. **Start consumer**: `consumer.Start(ctx, queueNames)` — spawn worker goroutine per queue

5. **Graceful shutdown**:
   - Menunggu `SIGINT` atau `SIGTERM`
   - Cancel context → semua worker berhenti
   - `consumer.Wait()` dengan timeout 30 detik
   - Tutup RabbitMQ dan Oracle DB

### Struct `consumerHandler`

Adapter sederhana yang mengimplementasikan `MessageHandler` interface.
Meneruskan event ke `TransactionUseCase.HandleConsumedMessage()`.

```go
type consumerHandler struct {
    useCase *usecase.TransactionUseCase
}
func (h *consumerHandler) Handle(ctx context.Context, event *model.TransactionEvent) error
```

---

## 3. `internal/config/` — Konfigurasi

### 3.1 `viper.go` — Loader Konfigurasi

**Tujuan**: Memuat konfigurasi dari `config.json` menggunakan Viper.

#### `NewViper() *viper.Viper`
- Membuat instance Viper baru
- Mencari file `config.json` di `"./../"` dan `"./"` (relative path)
- Mengaktifkan environment variable override dengan prefix `PPS_`
- Separator `.` diganti `_` untuk env var (contoh: `oracle.host` → `PPS_ORACLE_HOST`)
- Panic jika config file tidak ditemukan (config wajib ada)
- Return: instance `*viper.Viper` yang sudah terisi konfigurasi

### 3.2 `zerolog.go` — Logger Setup

**Tujuan**: Setup zerolog logger dengan PII masking dan multiple output target.

#### Struct `PIIMaskingWriter`
Custom `io.Writer` yang melakukan regex-based masking pada log output sebelum ditulis.
Pattern yang di-mask: nomor telepon Indonesia, email, NIK (16 digit), nomor kartu (13-19 digit).

#### `NewPIIMaskingWriter(writer io.Writer) *PIIMaskingWriter`
- Parameter: `writer` — writer asli yang akan dibungkus
- Return: `*PIIMaskingWriter` dengan pre-compiled regex patterns

#### `NewLogger(config *viper.Viper) zerolog.Logger`
- Membaca `log.level` dari config (0=Panic sampai 6=Trace, default Info)
- Membaca `log.output`: `"console"`, `"file"`, atau `"both"`
- Jika `"file"` atau `"both"`: membuat file writer dengan rotation (per hari, max age configurable)
- Membungkus writer dengan `PIIMaskingWriter`
- Return: `zerolog.Logger` dengan timestamp dan level yang dikonfigurasi

#### `newFileWriter(config *viper.Viper) io.Writer`
- Membuat `rotatelogs` writer untuk log rotation otomatis
- Format file: `{path}-YYYY-MM-DD`
- Rotasi setiap 24 jam, max age dari config `log.file.max_age`
- Fallback ke `os.Stderr` jika gagal

#### `SendTelegramLog(config *viper.Viper, log zerolog.Logger, message string)`
- Mengirim notifikasi ke Telegram Bot API (fire-and-forget)
- Cek `log.telegram.enabled` sebelum kirim
- Validasi `bot_token` dan `chat_id` tidak kosong
- HTTP POST ke `https://api.telegram.org/bot{token}/sendMessage`

#### `ContextLogger(log zerolog.Logger, traceID string) zerolog.Logger`
- Menambahkan field `trace_id` ke logger untuk request tracking
- Return: logger baru dengan trace_id

#### `MaskPII(s string) string`
- Masking manual pada string: phone → `***PHONE***`, email → `***EMAIL***`, NIK → `***NIK***`
- Menggunakan pre-compiled regex patterns (`piiMaskPatterns`)

### 3.3 `validator.go` — Validator Setup

**Tujuan**: Membuat instance `go-playground/validator`.

#### `NewValidator(config *viper.Viper) *validator.Validate`
- Parameter: `config` (belum digunakan, disiapkan untuk custom rules)
- Return: instance `*validator.Validate` baru

### 3.4 `oracle.go` — Koneksi Oracle Database

**Tujuan**: Setup koneksi Oracle DB dengan connection pooling menggunakan go-ora driver (pure Go).

#### `NewOracleDB(config *viper.Viper, log zerolog.Logger) *sql.DB`
- Membaca config: `oracle.host`, `oracle.port`, `oracle.service`, `oracle.username`, `oracle.password`
- Membuat DSN format: `oracle://user:pass@host:port/service`
- `sql.Open("oracle", dsn)` — setup pool (belum connect)
- Konfigurasi pool: `max_open`, `max_idle`, `max_lifetime` dari config
- Ping test — jika gagal hanya warning (akan retry on demand)
- Return: `*sql.DB` dengan connection pool

#### `CloseOracleDB(db *sql.DB, log zerolog.Logger)`
- Menutup koneksi Oracle DB
- Dipanggil saat graceful shutdown
- Nil-safe (cek `db == nil`)

### 3.5 `rabbitmq.go` — Koneksi RabbitMQ + Auto-Reconnect

**Tujuan**: Mengelola koneksi RabbitMQ dengan auto-reconnect, exchange/queue declaration, dan dead letter support.

#### Struct `RabbitMQConnection`
```
conn           *amqp.Connection  — koneksi AMQP aktif
config         *viper.Viper      — konfigurasi
log            zerolog.Logger    — logger
mu             sync.RWMutex      — proteksi akses conn
closed         int32             — atomic flag: 0=open, 1=closed
reconnecting   int32             — atomic flag: 0=idle, 1=reconnecting
verifiedQueues sync.Map          — cache queue yang sudah di-declare
notifyReconn   []chan struct{}   — subscriber yang menunggu reconnect selesai
notifyMu       sync.Mutex        — proteksi notifyReconn slice
```

#### `NewRabbitMQ(config, log) *RabbitMQConnection`
1. Buat struct `RabbitMQConnection`
2. `connect()` — buka koneksi AMQP (fatal jika gagal)
3. `setupExchange()` — declare main exchange + DLX (fatal jika gagal)
4. `declareStartupQueues()` — declare queue jika mode `"startup"`
5. Spawn goroutine `connectionWatcher()` — monitor koneksi
6. Return: `*RabbitMQConnection`

#### `connect() error`
- Membaca config RabbitMQ (host, port, username, password, vhost)
- Format URL: `amqp://user:pass@host:port/vhost`
- `amqp.Dial(url)` — buka koneksi
- Protected oleh `mu.Lock()` (write lock)

#### `setupExchange() error`
- Buka temporary channel
- Declare main exchange (nama dan tipe dari config, durable)
- Jika `dead_letter.enabled`: declare DLX (direct exchange, durable)

#### `declareStartupQueues()`
- Hanya jalan jika `queue_declare_mode == "startup"`
- Iterate `startup_queues` dari config, panggil `EnsureQueue()` untuk masing-masing

#### `connectionWatcher()`
- Goroutine tunggal yang berjalan sepanjang lifetime aplikasi
- Block di `conn.NotifyClose()` — menunggu koneksi putus
- Jika koneksi putus dan bukan graceful shutdown → panggil `reconnectWithBackoff()`
- Loop terus sampai `closed == 1`

#### `reconnectWithBackoff()`
- Guard: hanya satu goroutine yang boleh reconnect (atomic CAS pada `reconnecting`)
- Clear `verifiedQueues` cache (queue mungkin perlu re-declare)
- Exponential backoff via `cenkalti/backoff`:
  - `InitialInterval` dan `MaxInterval` dari config
  - `MaxElapsedTime=0` → retry forever
- Setiap attempt: `connect()` + `setupExchange()`
- Setelah berhasil: re-declare startup queues, notify semua waiter
- Kirim Telegram notifikasi saat reconnect berhasil

#### `waitForReconnect()`
- Membuat channel, masukkan ke `notifyReconn` slice
- Block sampai channel di-close oleh `reconnectWithBackoff()`

#### `IsReconnecting() bool`
- Return `true` jika `reconnecting == 1`

#### `GetConnection() *amqp.Connection`
- Return koneksi AMQP aktif (protected oleh `mu.RLock()`)

#### `EnsureQueue(queueName string) error`
- Cek cache `verifiedQueues` — skip jika sudah di-declare
- Buka temporary channel
- Declare quorum queue dengan args:
  - `x-queue-type: "quorum"` (replicated queue)
  - `x-dead-letter-exchange`: DLX name (jika enabled)
  - `x-dead-letter-routing-key`: DLQ name
  - `x-delivery-limit`: max redelivery sebelum ke DLQ
- Bind queue ke main exchange (routing key = queue name)
- Jika DLX enabled: declare DLQ (classic durable) + bind ke DLX
- Simpan ke cache `verifiedQueues`

#### `Close()`
- Set `closed = 1` (atomic CAS, idempotent)
- Tutup koneksi AMQP

---

## 4. `internal/gateway/messaging/` — Consumer & Interfaces

### 4.1 `interfaces.go`

**Tujuan**: Mendefinisikan interface untuk dependency injection.

#### Interface `RabbitMQProvider`
```go
GetConnection() *amqp.Connection    // Ambil koneksi AMQP aktif
EnsureQueue(queueName string) error // Declare + bind queue (cached)
IsReconnecting() bool               // Cek status reconnect
```
Diimplementasikan oleh `config.RabbitMQConnection`.

#### Interface `MessageHandler`
```go
Handle(ctx context.Context, event *model.TransactionEvent) error
```
Diimplementasikan oleh `consumerHandler` di `main.go`.

### 4.2 `consumer.go`

**Tujuan**: Consumer RabbitMQ dengan circuit breaker dan hold message mechanism.

#### Struct `Consumer`
```
rmq      RabbitMQProvider   — abstraksi koneksi RabbitMQ
handler  MessageHandler     — handler untuk proses message
config   *viper.Viper       — konfigurasi
log      zerolog.Logger     — logger
oracleDB *sql.DB            — koneksi Oracle (untuk health check)
cb       *gobreaker.CircuitBreaker — circuit breaker
wg       sync.WaitGroup     — untuk graceful shutdown
```

#### `NewConsumer(rmq, handler, cfg, log, oracleDB) *Consumer`
- Konfigurasi circuit breaker dari config:
  - `max_requests`: jumlah request yang diizinkan saat half-open
  - `interval`: interval reset counter
  - `timeout`: durasi state open sebelum pindah ke half-open
  - `failure_threshold`: jumlah consecutive failure sebelum open (default 3)
- `OnStateChange`: log state change + kirim Telegram saat OPEN/CLOSED
- Return: `*Consumer` dengan circuit breaker

#### `Start(ctx context.Context, queueNames []string)`
- Membaca `rabbitmq.consumer.workers` dari config (default 3)
- Untuk setiap queue: `EnsureQueue()` + spawn N worker goroutine
- Setiap worker menjalankan `workerLoop()`

#### `workerLoop(ctx, queue, id)`
- Loop luar yang restart worker jika channel mati
- Panggil `consumeOnce()` — jika return error, tunggu `worker_restart_delay` detik lalu restart
- Re-ensure queue setelah kemungkinan reconnect
- Berhenti jika context cancelled

#### `consumeOnce(ctx, queue, id, le) error`
- Ambil koneksi dari `rmq.GetConnection()`
- Buka channel baru (setiap worker punya channel sendiri — sesuai AMQP spec)
- Set QoS/prefetch dari config (default 10)
- Start consume dengan consumer tag `{queue}-w{id}`
- Loop: select antara context done, channel close, atau message delivery
- Return error jika channel/koneksi putus → `workerLoop` akan restart

#### `processMessage(ctx, msg)`
**Ini adalah inti dari mekanisme circuit breaker + hold message.**

1. Unmarshal JSON ke `model.TransactionEvent`
   - Jika gagal: `msg.Reject(false)` — reject tanpa requeue (message corrupt)

2. Loop retry:
   - Eksekusi handler melalui circuit breaker (`cb.Execute()`)
   - **Sukses**: `msg.Ack(false)` → selesai
   - **CB Open/TooManyRequests**: Hold message (tidak ack/nack)
     - Poll setiap `cb_poll_interval` detik sampai CB tidak lagi open
     - Jika context cancelled: `msg.Nack(false, true)` — requeue untuk safety
     - Setelah CB closed/half-open: retry di outer loop
   - **Error lain**: `msg.Nack(false, true)` — requeue

#### `Wait()`
- `wg.Wait()` — block sampai semua worker goroutine selesai

---

## 5. `internal/usecase/transaction_usecase.go` — Business Logic

**Tujuan**: Layer business logic yang mengorkestrasi proses consume message.

#### Struct `TransactionUseCase`
```
repo     *repository.TransactionRepository — data access layer
validate *validator.Validate               — validator (disiapkan untuk validasi)
log      zerolog.Logger                    — logger
```

#### `NewTransactionUseCase(repo, validate, log) *TransactionUseCase`
- Constructor dengan dependency injection

#### `HandleConsumedMessage(ctx, event) error`
- Membuat context logger dengan `trace_id = event.MsgId`
- Memanggil `repo.CallSetTransactionStatus(ctx, event)`
- Log warning jika SP return error (OutError != 0)
- Return error jika SP execution gagal (akan trigger circuit breaker)

#### `PingOracle(ctx) error`
- Proxy ke `repo.PingOracle()` untuk health check

---

## 6. `internal/repository/transaction_repository.go` — Data Access

**Tujuan**: Akses data ke Oracle Database via stored procedure.

#### Struct `transactionRow` (unexported)
Representasi row di database dengan `db` dan `fieldtag` struct tags.
Tidak digunakan langsung untuk SP call, tapi mendokumentasikan schema.

#### Struct `TransactionRepository`
```
oracleDB *sql.DB        — koneksi Oracle
log      zerolog.Logger — logger
```

#### `NewTransactionRepository(oracleDB, log) *TransactionRepository`
- Constructor

#### `CallSetTransactionStatus(ctx, event) (*model.SPResult, error)`
- Memanggil Oracle stored procedure `MSG.SETTRANSACTIONSTATUS`
- 9 input parameter dari `event`:
  - `INMSGID`, `INSTATUSTOBE`, `INSERIALNUMBER`, `INNUMBERBUYER`, `INNOMINAL`
  - `INORIGINALCONVERSATIONID`, `INCONVERSATIONID`, `INMESAGETOCUSTOMER`, `INADDITIONALMESSAGE`
- 3 output parameter via `go_ora.Out`:
  - `OUTID` (int), `OUTERROR` (int), `OUTMESSAGE` (string, max 4000 chars)
- Return: `*model.SPResult` dengan output values
- Log warning jika `OutError != 0`, debug jika sukses

#### `PingOracle(ctx) error`
- `db.PingContext(ctx)` — health check koneksi Oracle

---

## 7. `internal/model/model.go` — DTO/Event/Response

**Tujuan**: Struktur data untuk komunikasi antar layer.

#### `WebResponse[T any]`
Generic response wrapper dengan `Data` dan `Errors`.

#### `TransactionEvent`
Event yang di-consume dari RabbitMQ. Field:
- `MsgId` (int) — ID message/transaksi
- `StatusToBe`, `SerialNumber`, `ClientNumber`, `Nominal` — data transaksi
- `OriginalConversationID`, `ConversationID` — tracking ID
- `MessageToCustomer`, `AdditionalMessage` — pesan
- `QueueName`, `Source` — metadata routing

#### `SPResult`
Hasil dari stored procedure Oracle:
- `OutID` — ID output dari SP
- `OutError` — error code (0 = sukses)
- `OutMessage` — pesan error/sukses

#### `HealthResponse`
Response health check dengan status per service.

---

## 8. `internal/pkg/logger/logger.go` — Logger Helper

**Tujuan**: Helper functions untuk logging yang bisa di-import oleh semua layer tanpa import cycle.
Dipisahkan dari `config` package karena `config` package di-import oleh layer lain yang juga butuh logger helper.

#### `ContextLogger(log, traceID) zerolog.Logger`
- Menambahkan `trace_id` ke logger untuk distributed tracing

#### `SendTelegramLog(config, log, message)`
- Kirim notifikasi ke Telegram Bot API
- Cek `log.telegram.enabled`, validasi token dan chat ID
- Fire-and-forget, error hanya di-log

#### `MaskPII(s string) string`
- Masking PII pada string: phone, email, NIK
- Pre-compiled regex patterns (`piiPatterns`)

---

## 9. `internal/entity/transaction_entity.go` — Domain Entity

**Tujuan**: Domain model yang merepresentasikan data transaksi.

#### Struct `Transaction`
Entity lengkap dengan semua field transaksi + timestamp.
Digunakan sebagai representasi data di database.
Field mencakup: ID, QueueName, RC, Product, ClientNumber, Message, SerialNumber, Nominal,
ConversationID, OriginalConversationID, MessageToCustomer, AdditionalMessage, StatusToBe, Status,
CreatedAt, UpdatedAt.

---

## 10. Flow Lengkap: RabbitMQ → Oracle SP

```
1. Publisher (service lain) publish TransactionEvent ke exchange "pps.transaction"
   dengan routing key = nama queue (misal "biller-telkomsel-1")

2. RabbitMQ route message ke quorum queue "biller-telkomsel-1"
   (queue sudah di-bind ke exchange dengan routing key yang sama)

3. Consumer worker mengambil message dari queue
   - Setiap queue punya N worker (configurable)
   - Setiap worker punya AMQP channel sendiri
   - Prefetch limit mengontrol jumlah unacked message per worker

4. processMessage() unmarshal JSON → TransactionEvent

5. Circuit breaker wrapping:
   - State CLOSED → eksekusi handler
   - State OPEN → hold message (lihat section 11)
   - State HALF-OPEN → izinkan beberapa request untuk test

6. handler.Handle() → TransactionUseCase.HandleConsumedMessage()

7. UseCase memanggil Repository.CallSetTransactionStatus()

8. Repository eksekusi Oracle SP: MSG.SETTRANSACTIONSTATUS
   - 9 input parameter dari event
   - 3 output parameter: OUTID, OUTERROR, OUTMESSAGE

9. Jika sukses (error == nil):
   - msg.Ack(false) → message dihapus dari queue
   - Circuit breaker mencatat success

10. Jika gagal (error != nil, bukan CB error):
    - msg.Nack(false, true) → message di-requeue
    - Circuit breaker mencatat failure
    - Jika consecutive failures >= threshold → CB pindah ke OPEN
```

---

## 11. Mekanisme Hold Message saat Circuit Breaker OPEN

Ini adalah fitur kunci yang mencegah **requeue storm** saat Oracle down.

### Problem tanpa Hold
Jika Oracle down dan message langsung di-nack+requeue:
1. Message balik ke head queue
2. Worker langsung ambil lagi
3. Gagal lagi → nack+requeue
4. Loop tak terbatas → CPU dan network terbuang

### Solusi: Hold Message
Saat circuit breaker OPEN:
1. Message **tidak di-ack dan tidak di-nack** — tetap dalam status "unacked" di RabbitMQ
2. Worker masuk polling loop, cek state CB setiap `cb_poll_interval` detik
3. RabbitMQ **otomatis berhenti mengirim message baru** ke worker ini karena jumlah unacked message sudah mencapai prefetch limit
4. Efek: semua worker ter-"pause" secara natural tanpa message loss

### Saat CB Pindah ke HALF-OPEN/CLOSED
1. Polling loop mendeteksi CB tidak lagi OPEN
2. Message yang di-hold langsung di-retry (tanpa requeue overhead)
3. Jika berhasil → ack, worker kembali normal
4. Jika gagal lagi → CB mungkin kembali ke OPEN, hold lagi

### Saat Graceful Shutdown (context cancelled)
- Message yang di-hold di-nack+requeue → message tidak hilang
- Worker berhenti dengan aman

### Konfigurasi Terkait
```json
{
  "circuit_breaker": {
    "max_requests": 5,        // Request diizinkan saat half-open
    "interval": 60,           // Interval reset counter (detik)
    "timeout": 30,            // Durasi open sebelum half-open (detik)
    "failure_threshold": 3    // Consecutive failures sebelum open
  },
  "rabbitmq.consumer": {
    "prefetch": 10,           // Max unacked message per worker
    "cb_poll_interval": 2     // Interval polling CB state (detik)
  }
}
```

---

## 12. Dead Letter Queue (DLQ) Flow

DLQ menangani **poison message** — message yang gagal diproses berulang kali.

### Setup
1. **Dead Letter Exchange (DLX)**: `pps.transaction.dlx` (direct exchange)
2. **Dead Letter Queue**: `{queue_name}.dead` (classic durable queue)
3. **Delivery Limit**: Configurable via `rabbitmq.dead_letter.delivery_limit`

### Flow
```
1. Message gagal diproses → nack+requeue
2. RabbitMQ increment delivery count (quorum queue feature)
3. Jika delivery count > delivery_limit:
   - RabbitMQ otomatis route message ke DLX
   - DLX route ke DLQ berdasarkan routing key = "{queue_name}.dead"
4. Message tersimpan di DLQ untuk investigasi manual
```

### Konfigurasi
```json
{
  "rabbitmq": {
    "dead_letter": {
      "enabled": true,
      "exchange": "pps.transaction.dlx",
      "suffix": ".dead",
      "delivery_limit": 3
    }
  }
}
```

### Queue Declaration (EnsureQueue)
Saat `EnsureQueue()` dipanggil:
1. Declare main queue sebagai **quorum queue** dengan args:
   - `x-queue-type: "quorum"`
   - `x-dead-letter-exchange: "pps.transaction.dlx"`
   - `x-dead-letter-routing-key: "{queue}.dead"`
   - `x-delivery-limit: 3`
2. Bind main queue ke main exchange
3. Declare DLQ sebagai **classic durable queue** (tanpa special args)
4. Bind DLQ ke DLX

### Kapan Message Masuk DLQ
- Message di-nack+requeue lebih dari `delivery_limit` kali
- Message di-reject tanpa requeue (`msg.Reject(false)`) — langsung ke DLQ
  - Contoh: unmarshal JSON gagal (message corrupt)
