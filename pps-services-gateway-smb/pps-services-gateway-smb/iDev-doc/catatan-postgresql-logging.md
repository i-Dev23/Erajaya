# Catatan Pembelajaran: PostgreSQL Logging Service — Gateway SMB

> Dokumen ini menjelaskan SEMUA tentang fitur logging ke PostgreSQL di service `pps-services-gateway-smb`.
> Ditulis untuk orang yang masih belajar Go + PostgreSQL.
> Referensi pola: `pps-services-gateway-telkomsel`.

---

## 1. Gambaran Umum — "Logging Ini Buat Apa?"

### Masalah
Service gateway SMB proses transaksi PLN Token secara async (dari RabbitMQ).
Kalau ada masalah (gagal, pending, timeout), kita perlu tahu:
- Transaksi mana yang gagal?
- Response dari SMB API apa?
- Berapa lama response time-nya?
- Kapan terakhir retry?

Tanpa logging ke database, kita cuma bisa lihat dari log stdout (hilang kalau container restart).

### Solusi
Simpan semua data transaksi dan response ke **PostgreSQL** dalam 2 skema terpisah:
- `transaction_smb` → lifecycle transaksi (PROCESSING → SUCCESS/FAILED)
- `log_smb` → raw HTTP request/response ke SMB API

### Kenapa 2 Skema?
| Aspek | `transaction_smb` | `log_smb` |
|-------|-------------------|-----------|
| Fungsi | Tracking status transaksi end-to-end | Log detail setiap HTTP call ke SMB API |
| Granularity | 1 row per transaksi (msg_id) | 1 row per HTTP request |
| Siapa yang nulis | Consumer (RabbitMQ handler) | SMB HTTP Client (pkg/smb) |
| Kapan ditulis | Saat consume message + setiap response | Otomatis setiap kali panggil API SMB |
| Contoh query | "Transaksi X statusnya apa?" | "Inquiry ke SMB response-nya apa?" |

---

## 2. Arsitektur — Siapa Nulis ke Mana?

```mermaid
flowchart TD
    subgraph "Gateway SMB Service"
        CONSUMER["📨 RabbitMQ Consumer\nconsume_session.go"]
        USECASE["🧠 PLN Token Usecase\npln_token_usecase.go"]
        SMBCLIENT["🌐 SMB HTTP Client\npkg/smb/pln_token.go"]
    end

    subgraph "Logging Layer"
        TXLOGGER["📝 TransactionLogger\ntransaction_logger.go"]
        APILOGGER["📝 APILoggerAdapter\napi_logger_adapter.go"]
        APIREPO["🗄️ APILogRepository\napi_log_repository.go"]
    end

    subgraph "PostgreSQL Database"
        TBL1["📋 transaction_smb.smb_transaction\n(lifecycle transaksi)"]
        TBL2["📋 transaction_smb.smb_transaction_response\n(setiap response dari SMB)"]
        TBL3["📋 log_smb.smb_api_logs\n(raw HTTP request/response)"]
    end

    CONSUMER -->|"1. InsertTransaction\n(status=PROCESSING)"| TXLOGGER
    CONSUMER -->|"2. InsertSyncResponse"| TXLOGGER
    CONSUMER -->|"3. UpdateTransactionStatus\n(SUCCESS/FAILED)"| TXLOGGER
    CONSUMER -->|"4. InsertCallbackResponse"| TXLOGGER
    SMBCLIENT -->|"Auto-log setiap\nHTTP call"| APILOGGER
    APILOGGER -->|"Insert"| APIREPO
    TXLOGGER --> TBL1
    TXLOGGER --> TBL2
    APIREPO --> TBL3

    style TXLOGGER fill:#e3f2fd,stroke:#1976d2
    style APILOGGER fill:#fff3e0,stroke:#f57c00
    style APIREPO fill:#fff3e0,stroke:#f57c00
```

---

## 3. Database Schema — Tabel Apa Aja?

### 3.1 Skema `transaction_smb` — Tracking Lifecycle Transaksi

#### Tabel: `transaction_smb.smb_transaction`
Satu baris per transaksi (per pesan RabbitMQ yang masuk).

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `msg_id` | VARCHAR (PK) | ID unik pesan dari RabbitMQ |
| `our_trx_id` | VARCHAR | Transaction ID yang kita generate |
| `client_number` | VARCHAR | Nomor meter PLN pelanggan |
| `mid` | VARCHAR | Merchant ID |
| `product_type` | VARCHAR | Tipe produk (misal: PLN_TOKEN) |
| `product_code` | VARCHAR | Kode produk (misal: PLN50, PLN100) |
| `amount` | INTEGER | Nominal dalam rupiah |
| `queue_name` | VARCHAR | Nama queue RabbitMQ asal |
| `mq_transaction` | VARCHAR | URL RabbitMQ untuk publish hasil |
| `status` | VARCHAR | **PROCESSING** / **SUCCESS** / **FAILED** |
| `processing_at` | TIMESTAMPTZ | Kapan mulai diproses |
| `success_at` | TIMESTAMPTZ | Kapan sukses (NULL kalau belum) |
| `failed_at` | TIMESTAMPTZ | Kapan gagal (NULL kalau belum) |
| `first_requested_at` | TIMESTAMPTZ | Kapan pertama kali request ke SMB |
| `last_response_at` | TIMESTAMPTZ | Kapan terakhir dapat response |
| `created_at` | TIMESTAMPTZ | Kapan row dibuat |
| `updated_at` | TIMESTAMPTZ | Kapan row terakhir diupdate |

**Lifecycle status:**
```
PROCESSING ──→ SUCCESS   (payment berhasil, dapat token PLN)
PROCESSING ──→ FAILED    (inquiry/payment gagal, atau retry habis)
```

#### Tabel: `transaction_smb.smb_transaction_response`
Satu baris per response dari SMB API (bisa banyak per transaksi).

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `id` | BIGSERIAL (PK) | Auto-increment ID |
| `msg_id` | VARCHAR | FK ke smb_transaction |
| `our_trx_id` | VARCHAR | Transaction ID kita |
| `smb_trx_id` | VARCHAR | Transaction ID dari SMB (ref_id) |
| `response_type` | VARCHAR | **SYNC** atau **CALLBACK** |
| `status_code` | VARCHAR | Response code dari SMB (00, 28, 93, dll) |
| `status_desc` | VARCHAR | Deskripsi status |
| `request_payload` | JSONB | Body request yang dikirim |
| `raw_payload` | JSONB | Raw response dari SMB |
| `requested_at` | TIMESTAMPTZ | Kapan request dikirim |
| `response_latency_ms` | INTEGER | Berapa ms response time |
| `received_at` | TIMESTAMPTZ | Kapan response diterima |

**Contoh data per 1 transaksi PLN Token:**
```
smb_transaction_response:
  #1: SYNC  - Inquiry   - status_code=00 - latency=180ms
  #2: SYNC  - Payment   - status_code=00 - latency=250ms
  #3: SYNC  - Advice    - status_code=00 - latency=120ms  (kalau ada retry)
```

### 3.2 Skema `log_smb` — Raw HTTP Request/Response Log

#### Tabel: `log_smb.smb_api_logs`
Satu baris per HTTP call ke SMB API. Otomatis dicatat oleh SMB HTTP Client.

| Kolom | Tipe | Keterangan |
|-------|------|------------|
| `id` | BIGSERIAL (PK) | Auto-increment ID |
| `endpoint` | VARCHAR(255) | Path API (misal: /api/v1/pln-prepaid/inquiry) |
| `method` | VARCHAR(10) | HTTP method (POST) |
| `client_number` | VARCHAR(50) | Nomor meter PLN |
| `mid` | VARCHAR(50) | Merchant ID |
| `queue_name` | VARCHAR(100) | Queue asal |
| `msg_id` | VARCHAR(100) | Message ID |
| `request_url` | TEXT | Full URL yang dipanggil |
| `request_headers` | JSONB | HTTP headers request |
| `request_body` | JSONB | Body request (JSON) |
| `response_status_code` | INT | HTTP status code (200, 500, dll) |
| `response_body` | JSONB | Body response dari SMB |
| `response_duration_ms` | INT | Berapa ms total request |
| `status_code` | VARCHAR(100) | SMB response code (00, 28, 93) |
| `status_desc` | TEXT | Deskripsi dari SMB |
| `error_message` | TEXT | Error message kalau gagal |
| `error_type` | VARCHAR(20) | Tipe error: NETWORK, PARSE, dll |
| `created_at` | TIMESTAMPTZ | Kapan log dibuat |

**Bedanya dengan `smb_transaction_response`:**
| Aspek | `smb_transaction_response` | `smb_api_logs` |
|-------|---------------------------|----------------|
| Level | Business (transaksi) | Technical (HTTP) |
| Isi | Status code + payload ringkas | Full request + response + headers |
| Siapa nulis | Consumer via TransactionLogger | SMB Client via APILogger |
| Ada error info? | Tidak | Ya (error_message, error_type) |
| Ada HTTP status? | Tidak | Ya (response_status_code) |

---

## 4. Pola Arsitektur — "Kenapa Ribet Gini?"

### 4.1 Clean Architecture: Interface → Implementasi → Adapter

Kita pakai pola **Contract (Interface) → Implementation → Adapter** supaya:
- Kode bisnis (consumer) gak tahu detail PostgreSQL
- Bisa ganti database tanpa ubah kode bisnis
- Bisa di-mock untuk unit test

```mermaid
flowchart LR
    subgraph "Domain Layer (Interface)"
        I1["TransactionLogger\n(interface)"]
        I2["APILogRepository\n(interface)"]
        I3["APILogger\n(interface)"]
    end

    subgraph "Infrastructure Layer (Implementasi)"
        P1["PostgresTransactionLogger\n(implements TransactionLogger)"]
        P2["APILogRepositoryImpl\n(implements APILogRepository)"]
        P3["APILoggerAdapter\n(implements APILogger)"]
    end

    subgraph "Pengguna"
        C["Consumer\n(pakai TransactionLogger)"]
        S["SMB Client\n(pakai APILogger)"]
    end

    C -->|"pakai interface"| I1
    S -->|"pakai interface"| I3
    I1 -.->|"diimplementasi oleh"| P1
    I2 -.->|"diimplementasi oleh"| P2
    I3 -.->|"diimplementasi oleh"| P3
    P3 -->|"pakai"| I2
```

### 4.2 Kenapa Ada Adapter?

**Masalah:** SMB Client (`pkg/smb`) ada di package `smb`, tapi `APILogRepository` ada di package `service` (domain).
Package `smb` gak boleh import package `service` (dependency rule: outer → inner, bukan sebaliknya).

**Solusi:** Buat interface `APILogger` di package `smb`, lalu buat `APILoggerAdapter` yang "menjembatani":

```
pkg/smb/api_logger.go          → interface APILogger (di package smb)
    ↑ implements
infrastructure/postgres/
  api_logger_adapter.go         → APILoggerAdapter (convert smb.APICallLog → service.APILogEntry)
    ↓ pakai
domain/contract/service/
  api_log_repository.go         → interface APILogRepository (di package service)
    ↑ implements
infrastructure/postgres/
  api_log_repository.go         → APILogRepositoryImpl (SQL INSERT ke PostgreSQL)
```

**Analogi:** Adapter itu kayak **colokan listrik universal**. Laptop kamu (SMB Client) punya colokan tipe A, tapi stop kontak (PostgreSQL) tipe B. Adapter yang convert.

---

## 5. File-File Penting — Urutan Baca

### Transaction Logging (skema `transaction_smb`)

| # | File | Fungsi | Baca Untuk |
|---|------|--------|------------|
| 1 | `domain/contract/service/transaction_logger.go` | Interface + struct `TransactionRecord`, `ResponseRecord` | Pahami kontrak: method apa aja yang tersedia |
| 2 | `infrastructure/postgres/transaction_logger.go` | Implementasi PostgreSQL | Pahami SQL query + cara koneksi DB |
| 3 | `infrastructure/rabbitmq/consume_session.go` | Consumer yang panggil TransactionLogger | Pahami kapan logging dipanggil |
| 4 | `cmd/app/main.go` | Wiring: buat instance + inject ke consumer | Pahami cara komponen di-connect |

### API Logging (skema `log_smb`)

| # | File | Fungsi | Baca Untuk |
|---|------|--------|------------|
| 1 | `pkg/smb/api_logger.go` | Interface `APILogger` + struct `APICallLog` | Pahami kontrak di level HTTP client |
| 2 | `domain/contract/service/api_log_repository.go` | Interface `APILogRepository` + struct `APILogEntry` | Pahami kontrak di level domain |
| 3 | `infrastructure/postgres/api_log_repository.go` | Implementasi SQL INSERT | Pahami cara data disimpan |
| 4 | `infrastructure/postgres/api_logger_adapter.go` | Adapter: convert `APICallLog` → `APILogEntry` | Pahami cara bridging antar layer |
| 5 | `pkg/smb/pln_token.go` | SMB Client yang panggil `apiLogger.Log()` | Pahami kapan logging dipanggil |
| 6 | `cmd/app/main.go` | Wiring semua komponen | Pahami cara komponen di-connect |

---

## 6. Alur Kode Step-by-Step

### 6.1 Transaction Logging — Dari Consume Sampai Insert

```mermaid
sequenceDiagram
    participant RMQ as RabbitMQ
    participant CON as Consumer<br>(consume_session.go)
    participant UC as Usecase<br>(pln_token_usecase.go)
    participant SMB as SMB API
    participant TXL as TransactionLogger<br>(transaction_logger.go)
    participant PG as PostgreSQL<br>(transaction_smb)

    RMQ->>CON: 1. Deliver message (JSON payload)
    CON->>CON: 2. Parse payload, extract msgID
    CON->>TXL: 3. InsertTransaction(rec) → status=PROCESSING
    TXL->>PG: INSERT INTO transaction_smb.smb_transaction
    
    CON->>UC: 4. ProcessTransaction()
    UC->>SMB: 5. Inquiry PLN Token
    SMB-->>UC: 6. Response (ref_id, nama, harga)
    CON->>TXL: 7. InsertSyncResponse(inquiry response)
    TXL->>PG: INSERT INTO transaction_smb.smb_transaction_response (SYNC)
    
    UC->>SMB: 8. Payment PLN Token
    SMB-->>UC: 9. Response (token PLN)
    CON->>TXL: 10. InsertSyncResponse(payment response)
    TXL->>PG: INSERT INTO transaction_smb.smb_transaction_response (SYNC)
    
    CON->>TXL: 11. UpdateTransactionStatus("SUCCESS")
    TXL->>PG: UPDATE smb_transaction SET status='SUCCESS', success_at=NOW()
```

### 6.2 API Logging — Otomatis di HTTP Client

```mermaid
sequenceDiagram
    participant UC as Usecase
    participant CLI as SMB Client<br>(pln_token.go)
    participant SMB as SMB API
    participant ADA as APILoggerAdapter<br>(api_logger_adapter.go)
    participant REPO as APILogRepository<br>(api_log_repository.go)
    participant PG as PostgreSQL<br>(log_smb)

    UC->>CLI: 1. InquiryPLNToken(clientNumber, productCode)
    CLI->>CLI: 2. start = time.Now()
    CLI->>SMB: 3. HTTP POST /api/v1/pln-prepaid/inquiry
    SMB-->>CLI: 4. Response (200 OK + JSON body)
    CLI->>CLI: 5. durationMs = time.Since(start)
    CLI->>ADA: 6. logAPICall(endpoint, method, body, response, duration)
    ADA->>REPO: 7. Insert(APILogEntry{...})
    REPO->>PG: 8. INSERT INTO log_smb.smb_api_logs
    Note over ADA: Error? Log ke stdout, JANGAN propagate
    CLI-->>UC: 9. Return response
```

**Poin penting:**
- API logging **non-blocking** — kalau insert gagal, error cuma di-log ke stdout, gak bikin transaksi gagal
- Duration dihitung dari sebelum `client.Do(req)` sampai setelah response dibaca
- Kalau network error (timeout), tetap di-log dengan `error_type = "NETWORK"`

---

## 7. Cara Wiring di main.go — "Gimana Cara Connect Semua?"

```go
// 1. Buat TransactionLogger (koneksi ke PostgreSQL)
pgLogger, _ := postgres.NewTransactionLogger(cfg.PostgresDSN, logger)
defer pgLogger.Close()

// 2. Jalankan migration untuk skema transaction_smb
pgLogger.RunMigration(ctx)
// → CREATE SCHEMA IF NOT EXISTS transaction_smb
// → CREATE TABLE IF NOT EXISTS transaction_smb.smb_transaction ...
// → CREATE TABLE IF NOT EXISTS transaction_smb.smb_transaction_response ...

// 3. Inject TransactionLogger ke Consumer
consumer.SetTransactionLogger(pgLogger)

// 4. Buat APILogRepository (sharing koneksi DB yang sama!)
apiLogRepo := postgres.NewAPILogRepositoryImpl(pgLogger.DB(), logger)
//                                             ^^^^^^^^^^^
//                                             Pakai DB() dari pgLogger
//                                             Gak buka koneksi baru!

// 5. Jalankan migration untuk skema log_smb
apiLogRepo.RunMigration(ctx)
// → CREATE SCHEMA IF NOT EXISTS log_smb
// → CREATE TABLE IF NOT EXISTS log_smb.smb_api_logs ...

// 6. Buat APILoggerAdapter (bridge antara smb.APILogger dan APILogRepository)
apiLoggerAdapter := postgres.NewAPILoggerAdapter(apiLogRepo, logger)

// 7. Inject APILogger ke SMB HTTP Client
smbHTTPClient.SetAPILogger(apiLoggerAdapter)
```

**Poin penting:**
- `pgLogger.DB()` → sharing connection pool yang sama. Gak buka koneksi baru ke PostgreSQL.
- Semua migration bersifat **idempoten** — aman dipanggil berulang kali (`CREATE IF NOT EXISTS`).
- Kalau `POSTGRES_DSN` kosong → semua logging di-skip, service tetap jalan normal.

---

## 8. Koneksi Database — Detail Teknis

### Connection String (DSN)
```
POSTGRES_DSN=postgres://pps_telkomsel:pps_1233%23@192.168.3.247:5434/gateway_telkomsel?sslmode=disable
```

| Bagian | Nilai | Keterangan |
|--------|-------|------------|
| User | `pps_telkomsel` | Username PostgreSQL |
| Password | `pps_1233#` | Password (`%23` = URL-encoded `#`) |
| Host | `192.168.3.247` | IP server PostgreSQL |
| Port | `5434` | Port PostgreSQL |
| Database | `gateway_telkomsel` | Nama database |
| SSL | `disable` | Tanpa SSL (internal network) |

### Connection Pool Settings
```go
db.SetMaxOpenConns(5)              // Max 5 koneksi aktif bersamaan
db.SetMaxIdleConns(2)              // Max 2 koneksi idle (standby)
db.SetConnMaxLifetime(5 * time.Minute)  // Koneksi di-recycle setiap 5 menit
db.SetConnMaxIdleTime(2 * time.Minute)  // Koneksi idle ditutup setelah 2 menit
```

**Kenapa settingan segitu?**
- Service ini bukan high-throughput (proses 1 message at a time dari RabbitMQ)
- 5 koneksi cukup: 1 untuk insert transaction, 1 untuk insert response, 1 untuk update status, 2 cadangan
- Lifetime 5 menit → mencegah koneksi stale (PostgreSQL punya timeout sendiri)

### Library: pgx/v5 via database/sql
```go
import _ "github.com/jackc/pgx/v5/stdlib"

db, err := sql.Open("pgx", dsn)
```

**Kenapa pakai `database/sql` wrapper, bukan pgx langsung?**
- `database/sql` = standard library Go, semua orang familiar
- Lebih mudah di-mock untuk testing
- Connection pooling sudah built-in
- pgx/v5 tetap jadi driver di belakang layar (performa tetap bagus)

---

## 9. Design Pattern yang Dipakai

### 9.1 Compile-Time Interface Check
```go
var _ contractsvc.TransactionLogger = (*PostgresTransactionLogger)(nil)
```
**Apa ini?** Memastikan `PostgresTransactionLogger` mengimplementasikan semua method di interface `TransactionLogger` **saat compile**, bukan saat runtime.

**Kenapa penting?** Kalau kamu lupa implement satu method, compiler langsung error. Gak perlu tunggu sampai runtime baru ketahuan.

### 9.2 Idempotent Migration
```go
`CREATE SCHEMA IF NOT EXISTS transaction_smb`
`CREATE TABLE IF NOT EXISTS transaction_smb.smb_transaction (...)`
`CREATE INDEX IF NOT EXISTS idx_smb_transaction_our_trx_id ...`
```
**Apa ini?** DDL yang aman dipanggil berulang kali. Kalau tabel sudah ada, gak error.

**Kenapa penting?** Migration dijalankan setiap kali service start. Kalau gak idempoten, service gagal start setelah pertama kali.

### 9.3 ON CONFLICT DO NOTHING (Idempotent Insert)
```sql
INSERT INTO transaction_smb.smb_transaction (msg_id, ...)
VALUES ($1, ...)
ON CONFLICT (msg_id) DO NOTHING
```
**Apa ini?** Kalau `msg_id` sudah ada, insert di-skip tanpa error.

**Kenapa penting?** RabbitMQ bisa deliver message lebih dari sekali (at-least-once delivery). Tanpa ini, duplicate message bikin error.

### 9.4 Non-Blocking Logging
```go
func (a *APILoggerAdapter) Log(ctx context.Context, entry smb.APICallLog) {
    err := a.repo.Insert(ctx, ...)
    if err != nil {
        a.logger.Error("failed to insert smb api log", "error", err)
        // TIDAK return error ke caller!
    }
}
```
**Apa ini?** Kalau logging gagal (DB down, timeout), error cuma di-log. Transaksi utama tetap jalan.

**Kenapa penting?** Logging itu "nice to have". Gak boleh bikin transaksi gagal cuma karena logging error.

### 9.5 Nullable Columns dengan sql.NullString
```go
var productCode, mqTransaction sql.NullString

err := db.QueryRowContext(ctx, query, ourTrxID).Scan(
    &rec.MsgID, ..., &productCode, ..., &mqTransaction,
)

rec.ProductCode = productCode.String    // "" kalau NULL
rec.MQTransaction = mqTransaction.String
```
**Apa ini?** Kolom yang bisa NULL di PostgreSQL harus di-scan pakai `sql.NullString` (bukan `string` biasa).

**Kenapa?** Kalau kolom NULL di-scan ke `string`, Go panic. `sql.NullString` handle NULL dengan aman.

---

## 10. Helper Functions — Utility Kecil tapi Penting

### nullIfEmpty — Convert empty string ke SQL NULL
```go
func nullIfEmpty(s string) any {
    if s == "" { return nil }  // nil = SQL NULL
    return s
}
```
**Kapan dipakai?** Saat INSERT. Kolom opsional (MID, QueueName) yang kosong disimpan sebagai NULL, bukan empty string.

### toJSONB — Convert Go struct ke PostgreSQL JSONB
```go
func toJSONB(v any) ([]byte, error) {
    if v == nil { return nil, nil }
    return json.Marshal(v)
}
```
**Kapan dipakai?** Saat INSERT request_headers (map[string]string → JSONB).

### toRawJSONB — Handle raw bytes yang mungkin bukan valid JSON
```go
func toRawJSONB(b []byte) []byte {
    if len(b) == 0 { return nil }
    if !json.Valid(b) {
        wrapped, _ := json.Marshal(string(b))  // wrap sebagai JSON string
        return wrapped
    }
    return b
}
```
**Kapan dipakai?** Saat INSERT request_body dan response_body. Kadang body bukan valid JSON (misal: HTML error page). Fungsi ini wrap jadi JSON string supaya tetap bisa disimpan di kolom JSONB.

---

## 11. Query Berguna — Buat Debugging di Production

### Cek status transaksi
```sql
SELECT msg_id, our_trx_id, client_number, status, 
       processing_at, success_at, failed_at, last_response_at
FROM transaction_smb.smb_transaction
WHERE client_number = '12345678901'
ORDER BY created_at DESC
LIMIT 10;
```

### Lihat semua response per transaksi
```sql
SELECT r.response_type, r.status_code, r.status_desc, 
       r.response_latency_ms, r.received_at
FROM transaction_smb.smb_transaction_response r
WHERE r.msg_id = 'xxx'
ORDER BY r.received_at ASC;
```

### Cek transaksi yang masih PROCESSING (kemungkinan stuck)
```sql
SELECT msg_id, our_trx_id, client_number, processing_at,
       NOW() - processing_at AS duration
FROM transaction_smb.smb_transaction
WHERE status = 'PROCESSING'
  AND processing_at < NOW() - INTERVAL '5 minutes'
ORDER BY processing_at ASC;
```

### Lihat API call yang error
```sql
SELECT endpoint, client_number, error_message, error_type,
       response_duration_ms, created_at
FROM log_smb.smb_api_logs
WHERE error_message IS NOT NULL
ORDER BY created_at DESC
LIMIT 20;
```

### Rata-rata response time per endpoint
```sql
SELECT endpoint,
       COUNT(*) AS total_calls,
       AVG(response_duration_ms) AS avg_ms,
       MAX(response_duration_ms) AS max_ms,
       MIN(response_duration_ms) AS min_ms
FROM log_smb.smb_api_logs
WHERE created_at > NOW() - INTERVAL '1 hour'
GROUP BY endpoint;
```

### Cek success rate per jam
```sql
SELECT DATE_TRUNC('hour', created_at) AS hour,
       COUNT(*) FILTER (WHERE status = 'SUCCESS') AS success,
       COUNT(*) FILTER (WHERE status = 'FAILED') AS failed,
       COUNT(*) FILTER (WHERE status = 'PROCESSING') AS processing,
       COUNT(*) AS total
FROM transaction_smb.smb_transaction
WHERE created_at > NOW() - INTERVAL '24 hours'
GROUP BY hour
ORDER BY hour DESC;
```

---

## 12. Cara Test — Integration Test

### Jalankan test API logging (skema log_smb)
```bash
cd pps-services-gateway-smb
go test -v -tags=integration -run TestIntegration_FullAPILogFlow -count=1 -timeout 30s ./internal/infrastructure/postgres/
```

### Jalankan test transaction logging (skema transaction_smb)
```bash
cd pps-services-gateway-smb
go test -v -tags=integration -run TestIntegration_FullTransactionFlow -count=1 -timeout 60s ./internal/infrastructure/postgres/
```

### Apa yang di-test?
| Step | Test API Log | Test Transaction |
|------|-------------|-----------------|
| 1 | Koneksi DB | Koneksi DB |
| 2 | Migration skema log_smb | Migration skema transaction_smb |
| 3 | Verifikasi tabel ada | Verifikasi tabel ada |
| 4 | Insert via repository | InsertTransaction (PROCESSING) |
| 5 | Insert via adapter | GetTransactionStatusByMsgID |
| 6 | Insert error log | GetTransactionByOurTrxID |
| 7 | Verifikasi jumlah row | InsertSyncResponse (Inquiry) |
| 8 | Read back + verifikasi field | InsertSyncResponse (Payment) |
| 9 | - | UpdateTransactionStatus → SUCCESS |
| 10 | - | InsertCallbackResponse (SUCCESS) |
| 11 | - | InsertCallbackResponse (FAILED) |
| 12 | - | GetResponsesByMsgID |
| 13 | - | Direct DB verification + timestamps |
| 14 | - | Idempotence test (duplicate insert) |

---

## 13. Ringkasan File & Lokasi

```
pps-services-gateway-smb/
│
├── database/migrations/
│   ├── 001_create_smb_api_logs.up.sql           ← DDL skema log_smb
│   ├── 001_create_smb_api_logs.down.sql         ← Rollback log_smb
│   ├── 002_create_smb_transaction.up.sql        ← DDL skema transaction_smb
│   └── 002_create_smb_transaction.down.sql      ← Rollback transaction_smb
│
├── internal/domain/contract/service/
│   ├── api_log_repository.go                    ← Interface APILogRepository + APILogEntry
│   └── transaction_logger.go                    ← Interface TransactionLogger + TransactionRecord + ResponseRecord
│
├── internal/infrastructure/postgres/
│   ├── api_log_repository.go                    ← Implementasi INSERT ke log_smb.smb_api_logs
│   ├── api_logger_adapter.go                    ← Adapter: smb.APILogger → APILogRepository
│   ├── transaction_logger.go                    ← Implementasi CRUD ke transaction_smb.*
│   ├── api_log_integration_test.go              ← Integration test API logging
│   └── transaction_integration_test.go          ← Integration test transaction logging
│
├── pkg/smb/
│   ├── api_logger.go                            ← Interface APILogger + APICallLog (di package smb)
│   ├── client.go                                ← HTTP client + SetAPILogger()
│   └── pln_token.go                             ← Inquiry/Payment/Advice + auto-log API calls
│
└── cmd/app/main.go                              ← Wiring: buat semua instance + inject
```

---

## 14. FAQ — Pertanyaan yang Sering Muncul

**Q: Kalau PostgreSQL down, service ikut mati?**
A: Tergantung kapan down-nya:
- Saat startup → Ya, service gagal start (karena `NewTransactionLogger` return error)
- Saat runtime → Tidak. Logging error cuma di-log ke stdout, transaksi tetap jalan

**Q: Kenapa gak pakai ORM (GORM, Ent)?**
A: Karena query-nya simpel (INSERT, UPDATE, SELECT). ORM overkill untuk kasus ini. Pakai `database/sql` langsung lebih ringan dan transparan.

**Q: Kenapa migration di kode Go, bukan pakai tool (golang-migrate, Flyway)?**
A: Supaya self-contained. Service bisa auto-create tabel tanpa perlu jalankan tool terpisah. Cocok untuk deployment di Docker/Kubernetes.

**Q: Bisa gak logging ke database lain (MySQL, MongoDB)?**
A: Bisa! Tinggal buat implementasi baru yang implement interface `TransactionLogger` dan `APILogRepository`. Kode bisnis (consumer, usecase) gak perlu diubah.

**Q: Kenapa `smb_transaction` pakai `msg_id` sebagai primary key?**
A: Karena `msg_id` unik per pesan RabbitMQ. Ini juga yang bikin `ON CONFLICT DO NOTHING` bisa kerja — kalau message di-deliver ulang, insert di-skip.

**Q: Apa bedanya `response_type` SYNC vs CALLBACK?**
A: 
- **SYNC** = response langsung dari API call (inquiry, payment, advice)
- **CALLBACK** = response async yang datang belakangan (misal: SMB kirim notifikasi setelah payment selesai diproses)

**Q: Kenapa `InsertCallbackResponse` otomatis update status transaksi?**
A: Karena callback biasanya adalah "jawaban final" dari SMB. Kalau status_code `00` → SUCCESS, selain itu → FAILED. Ini mengikuti pola dari `pps-services-gateway-telkomsel`.
