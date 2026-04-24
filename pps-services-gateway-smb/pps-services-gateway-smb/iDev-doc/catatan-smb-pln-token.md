# Catatan Pembelajaran SMB H2H Loket Bayar – PLN Token (Prepaid)

## Sistem Mitra Bayar
## Switching Multi Biller

## Sumber Dokumentasi
- Postman: https://documenter.getpostman.com/view/5797953/2sA35Bd5T3#b670db23-ea2d-4a22-a020-a2f0ce0a54d5
- Fokus: **PLN Prepaid/Token** saja

---

## 1. Gambaran Umum SMB

SMB (H2H Loket Bayar) adalah API Host-to-Host yang menyediakan berbagai produk pembayaran:
- PLN Pascabayar
- **PLN Prepaid/Token** ← fokus
- PLN Non-Taglis
- BPJS Kesehatan
- PBB, TELKOM, PDAM/PGN/PERTAGAS/FINANCE/TV
- TELCO, TRANSFER, EWALLET, PULSA

Setiap produk punya 2 endpoint utama:
- **Info** (Inquiry) → cek data pelanggan & harga
- **Advice** (Payment) → eksekusi pembayaran

---

## 2. Error Code SMB

|-----------------------|-----------------------------------------------|
| Code                  | Keterangan                                    |
|-----------------------|-----------------------------------------------|
| 00                    | Success                                       |
| 28                    | Timeout atau Pending Transaksi                |
| 68                    | Timeout atau Pending Transaksi                |
| Null/Empty/Unknown    | Timeout atau Pending Transaksi saat Payment   |
| 99                    | Error Credential or IP                        |
| 98                    | Error Parameter Request                       |
| 97                    | Error Client Data                             |
| 96                    | Error Id Request                              |
| 95                    | Error Price Setting                           |
| 94                    | Error Inquiry Data                            |
| 93                    | Error Payment                                 |
| 400                   | Invalid Parameter                             |
| 401                   | Partner Tidak Aktif                           |
| 405                   | Access Not Allowed                            |
| 4000                  | Produk/Data Pelanggan Tidak Ditemukan         |
| 4005                  | Not Allowed                                   |
| 07                    | Layanan Sedang Gangguan                       |
| 1090                  | Transaksi Cut-off                             |
| 88                    | Tagihan Sudah Terbayar                        |
| 10                    | Tagihan Sudah Terbayar                        |
|-----------------------|-----------------------------------------------|

---

## 3. Service Existing: `pps-services-tokopedia`

### Tech Stack

|-------------------|---------------------------------------------------|
| Komponen          | Teknologi                                         |
|-------------------|---------------------------------------------------|
| Framework HTTP    | Go Fiber                                          |
| Database Utama    | PostgreSQL (log inquiry/payment)                  |
| Database Legacy   | Oracle (preorder, product price, cut-off)         |
| Cache             | Redis                                             |
| Message Queue     | RabbitMQ                                          |
| External Service  | Ultima (PLN inquiry)                              |
| DI                | Google Wire                                       |
| Security          | RSA encryption + Digital Signature + JWT Token    |
|-------------------|---------------------------------------------------|

### Arsitektur (Clean Architecture)
```
cmd/app/          → Entry point + Wire DI
internal/
├── config/       → Konfigurasi environment
├── domain/       → Entity & Interface (core business)
├── dto/          → Data Transfer Object (request/response JSON)
├── usecase/      → Business logic
├── repository/   → Data access (PostgreSQL, Oracle)
├── service/      → External service (Redis, RabbitMQ, Ultima, Crypto)
├── delivery/http/→ HTTP Handler (Fiber)
├── middleware/   → IP Whitelist, Rate Limit, Crypto, Logging
└── utils/        → Helper functions
```

---

## 4. Flow Transaksi PLN Token di Service Existing

### 4.1 Generate Token
```
POST /auth/token
```
- Input: `client_id`, `client_secret`, `timestamp`
- Validasi credential vs env `TP_CLIENT_ID` / `TP_CLIENT_SECRET`
- Generate JWT (HS256), expiry default 3600s
- Output: token untuk request selanjutnya

### 4.2 Inquiry
```
POST /api/v1/inquiry
```
Flow:
1. Validasi cut-off (Redis + Oracle)
2. Validasi mandatory params: `ref_id`, `client_number`, `category`, `rsid`, `product_code`, `timestamp`
3. Cek duplicate `ref_id`
4. Insert inquiry request ke PostgreSQL
5. Get product price (Redis cache → Oracle fallback)
6. Validasi status produk (harus "00")
7. Call **Ultima API** (`checkIdPlnUltima`) → data PLN: Nama, No Meter, ID Pelanggan, Tarif/Daya
8. Cache PLN data ke Redis (24 jam)
9. Insert inquiry response + bill details ke PostgreSQL
10. Return response dengan `partner_inquiry_id`

Response fields:
- `partner_inquiry_id` → ID unik dari PPS
- `bill_details` → Nama, Nomor Meter, ID Pelanggan, Tarif/Daya, Harga
- `total_amount` → harga produk

### 4.3 Payment
```
POST /api/v1/payment
```
Flow:
1. Validasi cut-off (Redis + Oracle)
2. Validasi mandatory params
3. Cek duplicate `ref_id`
4. Validasi `partner_inquiry_id` exists
5. Validasi data match (client_number, product_code, amount = inquiry)
6. Validasi inquiry belum dibayar
7. Insert payment request ke PostgreSQL
8. Validasi amount == product price
9. Call **Oracle Preorder** → generate `server_id`
10. Update preorder status
11. **Publish ke RabbitMQ** format: `PRE_ORDER||IP||MDN||NoTrx||Product||Signature||ServerID||User`
12. Return response code `01` (Pending)
13. Copy bill details dari inquiry

### 4.4 Callback (Async via RabbitMQ)
- Listen queue `RABBITMQ_CALLBACK_QUEUE_NAME`
- Get payment status dari DB
- Format response berdasarkan `response_code`
- Sign → Encrypt → Kirim ke `CALLBACK_URL` Tokopedia
- Log ke database

### 4.5 Check Status
```
POST /api/v1/check-status
```
- Input: `ref_id`, `timestamp`, `category`
- Query payment status dari PostgreSQL
- Return status terkini

---

## 5. Middleware Stack (urutan eksekusi)

1. DatabaseErrorHandlingMiddleware
2. IPWhitelistMiddleware → validasi IP
3. RateLimitMiddleware → rate limiting per endpoint
4. CheckBearerTokenMiddleware → validasi JWT
5. DecryptRequestMiddleware → decrypt body (RSA + AES)
6. HTTPLoggingMiddleware → log ke DB
7. EncryptResponseMiddleware → encrypt response
8. ResponseFormatterMiddleware → format berdasarkan response_code

---

## 6. Response Code Mapping (Tokopedia)

|-------|-------------------------------|-------------------|
| Code  | Message                       | Behavior          |
|-------|-------------------------------|-------------------|
| 00    | Success                       | Success           |
| 01    | On process                    | Pending           |
| 12    | Transaction not found         | Failed and Retry  |
| 13    | Duplicate transaction         | Failed and Retry  |
| 14    | Ineligible Product            | Failed and Refund |
| 20    | Unregistered number           | Failed and Refund |
| 21    | Number blocked                | Failed and Refund |
| 30    | Invalid token                 | Failed and Refund |
| 32    | Invalid signature             | Failed and Refund |
| 42    | Invalid parameter             | Failed and Retry  |
| 43    | Insufficient balance          | Failed and Retry  |
| 44    | Invalid transaction amount    | Failed and Refund |
| 61    | Server maintenance            | Failed and Retry  |
| 62    | Server error                  | Failed and Retry  |
| 63    | Biller maintenance            | Failed and Retry  |
| 64    | Biller error                  | Failed and Retry  |
|-------|-------------------------------|-------------------|

---

## 7. Impact Analysis: Integrasi SMB PLN Token

### Yang perlu diperhatikan:

1. **Ultima Service** → Saat ini dipakai untuk inquiry PLN (check ID pelanggan). Kalau SMB punya endpoint inquiry sendiri, perlu adapter baru atau modifikasi.

2. **Oracle Preorder** → Payment flow pakai Oracle stored procedure. SMB mungkin punya flow berbeda untuk payment/advice.

3. **RabbitMQ** → Payment di-publish ke queue, callback dikirim balik ke Tokopedia. Perlu disesuaikan jika SMB punya mekanisme callback sendiri.

4. **Response Code Mapping** → SMB punya error code sendiri (28, 68, 93, 94, 95, dll) yang BERBEDA dari mapping Tokopedia. Perlu mapping baru SMB → Internal → Tokopedia.

5. **Encryption/Signature** → Komunikasi dengan Tokopedia pakai RSA + Digital Signature. Komunikasi dengan SMB kemungkinan pakai mekanisme auth berbeda (credential/IP based).

6. **Product Price** → Saat ini dari Oracle. Perlu cek apakah SMB return harga di response inquiry-nya.

---

## 8. Service Baru: pps-services-gateway-smb ✅ SUDAH DIBUAT

### Keputusan: Bikin Service Baru (bukan extend Tokopedia)
Alasan:
- Arsitektur event-driven → setiap provider punya gateway sendiri
- Sama polanya dengan gateway-telkomsel dan gateway-unipin
- Kalau SMB down, service lain tidak terpengaruh
- Bisa scale independent

### Arsitektur End-to-End
```
Tokopedia (payment)
    │
    ▼
Consumer (PRE-ORDER) → Oracle preorder → Publisher-Provider
    │                                          │
    │                                          ▼
    │                                    RabbitMQ (queue SMB)
    │                                          │
    │                                          ▼
    │                                  ┌── Gateway SMB ──┐
    │                                  │                  │
    │                                  │  1. Inquiry API  │
    │                                  │  2. Payment API  │
    │                                  │  3. Advice (retry)│
    │                                  │                  │
    │                                  └────────┬─────────┘
    │                                           │
    │                                           ▼
    │                                    RabbitMQ (result)
    │                                    {"source":"PROVIDER"}
    │                                           │
    ▼                                           │
Consumer (PROVIDER) ◀───────────────────────────┘
    │
    ▼
Oracle SP_SetTransactionStatus (update status final)
```

### Alur PLN Token di Gateway SMB (3 Step)
```
Consume dari RabbitMQ
        │
        ▼
  ┌─ STEP 1: INQUIRY ─────────────────────────┐
  │  POST /api/v1/pln-prepaid/inquiry          │
  │  → cek data pelanggan PLN                  │
  │  ← dapat: ref_id, nama, tarif/daya, harga  │
  │                                             │
  │  Gagal? → publish "C" (cancel) → SELESAI   │
  └─────────────────────────────────────────────┘
        │ Sukses (response_code "00")
        ▼
  ┌─ STEP 2: PAYMENT ─────────────────────────┐
  │  POST /api/v1/pln-prepaid/payment          │
  │  → kirim: ref_id dari inquiry, nominal     │
  │  ← dapat: token PLN, serial number         │
  │                                             │
  │  Sukses "00"? → publish "F" + token → DONE │
  │  Gagal?       → publish "C" → DONE         │
  │  Pending "28"/"68"? → lanjut ke step 3     │
  └─────────────────────────────────────────────┘
        │ Pending (timeout/belum selesai)
        ▼
  ┌─ STEP 3: RETRY ADVICE (async) ────────────┐
  │  Loop max 4x, interval 10 detik:          │
  │    POST /api/v1/pln-prepaid/advice         │
  │    → cek status transaksi                  │
  │                                             │
  │    Sukses? → publish "F" + token → STOP    │
  │    Gagal?  → publish "C" → STOP            │
  │    Masih pending? → retry lagi             │
  │                                             │
  │  4x retry habis? → publish "C" → STOP     │
  └─────────────────────────────────────────────┘
```

### File-File Penting (urutan baca)
```
1. .env.example                              → env var apa aja yang perlu di-set
2. internal/config/config.go                 → cara baca env var
3. internal/domain/contract/service/         → semua interface (kontrak)
4. pkg/smb/client.go + pln_token.go          → HTTP client ke SMB API
5. internal/infrastructure/smbclient/        → adapter pkg/smb → interface
6. internal/infrastructure/rabbitmq/         → INTI: consumer + PLN Token flow
7. internal/infrastructure/mqpublisher/      → publish hasil ke downstream
8. cmd/app/main.go                           → wiring semua komponen
```

### Impact ke Service Lain
| Service | Perubahan? | Keterangan |
|---------|-----------|------------|
| pps-services-consumer | TIDAK | Sudah support source "PROVIDER" |
| pps-services-tokopedia | TIDAK | Publish ke consumer seperti biasa |
| publisher-provider | KONFIGURASI | Tambah routing rule ke queue SMB |
| Gateway SMB | BARU | Service baru yang kita develop |

### Dokumen Lengkap
Lihat file: `iDev-doc/alur-baca-kode-dan-presentasi.md`
→ Penjelasan lengkap alur kode + materi presentasi + FAQ

---

## 9. Koneksi & Infrastruktur yang Dibutuhkan

Service ini connect ke beberapa external service/infra. Semua konfigurasi via environment variable (lihat file `.env` atau `.env.example`).

### 9.1 RabbitMQ — Consumer (WAJIB)
- **Fungsi:** Consume message transaksi PLN Token dari queue.
- **Protocol:** AMQP
- **Library:** `github.com/rabbitmq/amqp091-go`
- **Env:**
  | Variable | Keterangan | Contoh | Wajib? |
  |----------|-----------|--------|--------|
  | `RABBITMQ_URL` | URL koneksi RabbitMQ | `amqp://guest:guest@localhost:5672/` | ✅ Ya |
  | `QUEUE_NAME_PROVIDER` | Nama queue yang di-consume | `queue_smb_pln_token` | ✅ Ya |
  | `CONSUMER_TAG` | Tag consumer (identifier) | `pps-services-gateway-smb-consumer` | Opsional (ada default) |
- **Behavior:** Auto-reconnect dengan exponential backoff (1s → max 30s) kalau koneksi putus.

### 9.2 RabbitMQ — Publisher (WAJIB)
- **Fungsi:** Publish hasil transaksi (success/cancel) ke queue downstream (consumer-database).
- **Library:** Sama, `amqp091-go`
- **Koneksi:** Dinamis per-message, URL diambil dari field `MQTransaction` di payload message yang di-consume. Bukan dari env variable.
- **Publish mode:** `mandatory=true` dengan return listener (detect unroutable message).

### 9.3 SMB / Loket Bayar API — HTTP Client (WAJIB)
- **Fungsi:** Panggil API SMB untuk transaksi PLN Token (inquiry, payment, advice).
- **Protocol:** HTTPS (REST API)
- **Library:** Standard `net/http` + custom client di `pkg/smb/`
- **Auth:** MD5 signature dari `partner_id + secret_key + ref_id`
- **Env:**
  | Variable | Keterangan | Contoh | Wajib? |
  |----------|-----------|--------|--------|
  | `SMB_BASE_URL` | Base URL API SMB | `https://api.example.com` | ✅ Ya |
  | `SMB_PARTNER_ID` | Partner ID dari SMB | `your_partner_id` | ✅ Ya |
  | `SMB_SECRET_KEY` | Secret key untuk signature | `your_secret_key` | ✅ Ya |
  | `SMB_TIMEOUT_SEC` | Timeout per-request (detik) | `30` | Opsional (default 30) |
- **Retry Config:**
  | Variable | Keterangan | Default |
  |----------|-----------|---------|
  | `RETRY_MAX_ATTEMPTS` | Max retry advice saat pending | `4` |
  | `RETRY_WAIT_SECONDS` | Jeda antar retry (detik) | `10` |

### 9.4 PostgreSQL — Transaction Logger (OPSIONAL)
- **Fungsi:** Simpan log transaksi dan response ke database untuk tracking/audit.
- **Library:** `github.com/jackc/pgx/v5` (driver PostgreSQL via `database/sql`)
- **Env:**
  | Variable | Keterangan | Contoh | Wajib? |
  |----------|-----------|--------|--------|
  | `POSTGRES_DSN` | Connection string PostgreSQL | `postgres://user:pass@localhost:5432/pps_smb?sslmode=disable` | Opsional |
- **Behavior:**
  - Kalau `POSTGRES_DSN` kosong/tidak diset → transaction logging di-skip, service tetap jalan.
  - Auto-migration: create schema `transaction` + tabel `smb_transaction` dan `smb_transaction_response` kalau belum ada.
  - Connection pool: max 5 open, 2 idle, lifetime 5 menit.

### 9.5 HTTP Server — Fiber (WAJIB)
- **Fungsi:** Expose health check endpoint dan (future) callback endpoint.
- **Library:** `github.com/gofiber/fiber/v2`
- **Endpoint:** `GET /health` → `{"status":"ok","service":"pps-services-gateway-smb"}`
- **Env:**
  | Variable | Keterangan | Default |
  |----------|-----------|---------|
  | `HTTP_PORT` | Port HTTP server | `8080` |
  | `READ_TIMEOUT_SEC` | Read timeout (detik) | `30` |

### Ringkasan Koneksi
```
┌─────────────────────────────────────────────────┐
│           pps-services-gateway-smb              │
│                                                 │
│  ┌──────────┐   consume    ┌──────────────┐    │
│  │ RabbitMQ │ ◀──────────  │  Consumer    │    │
│  │ (queue)  │              │  Service     │    │
│  └──────────┘              └──────┬───────┘    │
│                                   │             │
│  ┌──────────┐   HTTP POST  ┌─────▼───────┐    │
│  │ SMB API  │ ◀──────────  │  SMB Client │    │
│  │ (H2H)   │              │  (pkg/smb)  │    │
│  └──────────┘              └──────┬───────┘    │
│                                   │             │
│  ┌──────────┐   publish    ┌─────▼───────┐    │
│  │ RabbitMQ │ ◀──────────  │ MQPublisher │    │
│  │ (result) │              │             │    │
│  └──────────┘              └─────────────┘    │
│                                                 │
│  ┌──────────┐   SQL        ┌─────────────┐    │
│  │ Postgres │ ◀──────────  │  Tx Logger  │    │
│  │ (log)   │  (opsional)  │             │    │
│  └──────────┘              └─────────────┘    │
│                                                 │
│  ┌──────────┐              ┌─────────────┐    │
│  │ Client   │ ──────────▶  │ Fiber HTTP  │    │
│  │ (health) │   GET /health│ Server      │    │
│  └──────────┘              └─────────────┘    │
└─────────────────────────────────────────────────┘
```

---

## 10. Daftar Credential / Environment Variable

Ringkasan cepat semua env var yang perlu di-set. Referensi: `.env.example` dan `internal/config/config.go`.

### Wajib (service gagal start kalau kosong)
| Variable | Fungsi | Contoh |
|----------|--------|--------|
| `RABBITMQ_URL` | Koneksi ke RabbitMQ | `amqp://guest:guest@localhost:5672/` |
| `QUEUE_NAME_PROVIDER` | Nama queue yang di-consume | `queue_smb_pln_token` |
| `SMB_BASE_URL` | Base URL API SMB (H2H Loket Bayar) | `https://api.example.com` |
| `SMB_PARTNER_ID` | Partner ID untuk auth ke SMB | `your_partner_id` |
| `SMB_SECRET_KEY` | Secret key untuk generate MD5 signature | `your_secret_key` |

### Opsional (ada default value)
| Variable | Fungsi | Default |
|----------|--------|---------|
| `CONSUMER_TAG` | Identifier consumer RabbitMQ | `pps-services-gateway-smb-consumer` |
| `SMB_TIMEOUT_SEC` | Timeout per-request ke SMB API | `30` |
| `RETRY_MAX_ATTEMPTS` | Max retry advice saat pending | `4` |
| `RETRY_WAIT_SECONDS` | Jeda antar retry (detik) | `10` |
| `POSTGRES_DSN` | Connection string PostgreSQL (kalau kosong, logging di-skip) | - |
| `HTTP_PORT` | Port HTTP server (health check) | `8080` |
| `READ_TIMEOUT_SEC` | Read timeout HTTP server | `30` |

### Catatan Penting
- **RabbitMQ Publisher** tidak pakai env var — URL diambil dari field `MQTransaction` di payload message yang di-consume.
- **SMB Auth** pakai MD5 signature: `md5(partner_id + secret_key + ref_id)`, bukan token/JWT.
- **PostgreSQL** opsional — kalau `POSTGRES_DSN` tidak diset, service tetap jalan tanpa transaction logging.

---

## 11. TODO / Next Steps

- [x] Design arsitektur gateway SMB
- [x] Develop service pps-services-gateway-smb
- [x] Implementasi PLN Token flow (inquiry → payment → advice)
- [x] Implementasi MQPublisher (publish ke downstream)
- [x] Implementasi Postgres transaction logging
- [x] Dokumentasi alur kode + materi presentasi
- [ ] Konfigurasi Publisher-Provider routing ke queue SMB
- [ ] Unit test untuk consumer dan SMB client
- [ ] Integration test dengan SMB API sandbox
- [ ] Deploy ke staging environment
- [ ] Monitoring & alerting setup