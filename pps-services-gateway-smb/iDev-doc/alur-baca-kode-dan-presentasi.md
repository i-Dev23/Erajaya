# Panduan Belajar & Presentasi: pps-services-gateway-smb (PLN Token)

> Dokumen ini menjelaskan SEMUA alur kode dari awal sampai akhir,
> ditulis untuk orang yang masih belajar Go dan arsitektur microservice.
> Bisa langsung dipakai buat presentasi.

---

## BAGIAN 0: FLOW AWAL — DARI TOKOPEDIA SAMPAI GATEWAY SMB

> **"Pesan dari user masuk lewat mana? Lewat file apa aja sampai nyampe ke gateway SMB?"**

### Peta Lengkap: File Mana di Repo Mana

```mermaid
flowchart TD
    subgraph TOKOPEDIA ["📦 Repo: pps-services-tokopedia"]
        T1["🌐 POST /api/v1/payment\n📄 delivery/http/payment_handler.go\nPaymentHandler.Payment()"]
        T2["🧠 Business Logic\n📄 usecase/payment_usecase.go\npaymentUsecaseImpl.Payment()"]
        T3["🗄️ Oracle Preorder\n📄 repository/preorder_oracle_repository.go\nPreorder() + UpdatePreorderStatus()"]
        T4["📬 Publish ke RabbitMQ\n📄 service/rabbit_mq_service.go\nrabbitMQService.Publish()"]
    end

    subgraph CONSUMER ["📦 Repo: pps-services-consumer"]
        C1["📨 Consume dari RabbitMQ\n📄 repository/consumerTunning.go\nConsumerFIFO() → ProsesDataFIFO()"]
        C2["🔀 Route: PRE-ORDER\n📄 repository/consumerTunning.go\nprocessPreOrder()"]
        C3["🗄️ Oracle: UpdPreOrderConsume\n+ SellWithId\n📄 repository/oracle.go"]
        C4["📤 HTTP POST ke Publisher-Provider\n📄 repository/publisherClient.go\nCallPublisherProvider()"]
    end

    subgraph PUBPROV ["📦 Service: publisher-provider"]
        PP1["🔀 Route berdasarkan remarkImsi\n→ Publish ke queue provider\n(queue SMB / queue Telkomsel / dll)"]
    end

    subgraph GATEWAY ["📦 Repo: pps-services-gateway-smb"]
        G1["📨 Consume dari RabbitMQ\n📄 infrastructure/rabbitmq/\nconsume_session.go"]
        G2["🧠 Logic PLN Token\n📄 usecase/plntoken/\npln_token_usecase.go"]
    end

    T1 -->|"1. User kirim\npayment request"| T2
    T2 -->|"2. Validasi +\nOracle preorder"| T3
    T3 -->|"3. Dapat server_id"| T2
    T2 -->|"4. Publish pesan\nformat: PRE_ORDER||IP||MDN||..."| T4
    T4 -->|"5. Masuk queue\nRabbitMQ"| C1
    C1 -->|"6. Detect source:\nPRE-ORDER"| C2
    C2 -->|"7. Oracle SP:\nprocess preorder"| C3
    C3 -->|"8. Dapat remarkImsi,\nqueueName, command"| C4
    C4 -->|"9. HTTP POST\n/api/v1/publish"| PP1
    PP1 -->|"10. Publish ke queue\nSMB provider"| G1
    G1 -->|"11. Parse payload,\nroute by product_type"| G2

    style TOKOPEDIA fill:#e8f5e9,stroke:#2e7d32
    style CONSUMER fill:#e3f2fd,stroke:#1565c0
    style PUBPROV fill:#fff3e0,stroke:#ef6c00
    style GATEWAY fill:#fce4ec,stroke:#c62828
```

### Step-by-Step: Cara Baca Kode dari Awal

**Step 1 — User kirim payment request ke Tokopedia**
```
Repo:  pps-services-tokopedia
File:  internal/delivery/http/payment_handler.go
Func:  PaymentHandler.Payment()
```
- Terima HTTP POST `/api/v1/payment`
- Parse request body (encrypted → decrypted oleh middleware)
- Panggil `paymentUsecase.Payment()`

**Step 2 — Payment usecase: validasi + Oracle preorder**
```
Repo:  pps-services-tokopedia
File:  internal/usecase/payment_usecase.go
Func:  paymentUsecaseImpl.Payment()
```
- Validasi: cut-off, mandatory params, duplicate ref_id, inquiry exists, amount match
- Call Oracle `Preorder()` → dapat `server_id`
- Call Oracle `UpdatePreorderStatus()` → set status "10" (success publish)

**Step 3 — Publish ke RabbitMQ (format legacy pipe-delimited)**
```
Repo:  pps-services-tokopedia
File:  internal/service/rabbit_mq_service.go
Func:  rabbitMQService.Publish()
```
- Format pesan: `PRE_ORDER||IP||MDN||NoTrx||Product||Signature||ServerID||User`
- Publish ke queue RabbitMQ (default exchange, routing key dari env `RABBITMQ_QUEUE_NAME`)

**Step 4 — Consumer ambil pesan dari RabbitMQ**
```
Repo:  pps-services-consumer
File:  repository/consumerTunning.go
Func:  ConsumerFIFO() → ProsesDataFIFO()
```
- Consume pesan dari queue (QoS=1, manual ack)
- `extractSource()` → detect apakah JSON dengan field `source`
- Kalau bukan JSON (pipe-delimited) → `processLegacy()`
- Kalau `source: "PRE-ORDER"` → `processPreOrder()`

**Step 5 — processPreOrder: Oracle SP + call Publisher-Provider**
```
Repo:  pps-services-consumer
File:  repository/consumerTunning.go
Func:  processPreOrder()
```
- `UpdPreOrderConsume()` → Oracle SP, dapat `remarkImsi`, `queueName`, `command`, `mid`
- `SellWithId()` → Oracle SP, proses jual
- Mapping data ke `PublishProviderRequest`
- `CallPublisherProvider()` → HTTP POST ke publisher-provider

**Step 6 — CallPublisherProvider: HTTP POST ke publisher-provider**
```
Repo:  pps-services-consumer
File:  repository/publisherClient.go
Func:  CallPublisherProvider()
```
- HTTP POST ke `{PUBLISHER_PROVIDER_BASE_URL}/api/v1/publish`
- Body: JSON `PublishProviderRequest` (berisi msgID, clientNumber, remarkIMSI, queueName, command, dll)
- Retry: exponential backoff, max 3x

**Step 7 — Publisher-Provider route ke queue provider**
```
Service: pps-services-publisher-provider (repo terpisah)
```
- Terima HTTP POST `/api/v1/publish`
- Baca `remarkIMSI` → tentukan queue tujuan (misal: queue SMB, queue Telkomsel, queue Unipin)
- Publish pesan ke queue provider yang sesuai

**Step 8 — Gateway SMB consume dari queue provider**
```
Repo:  pps-services-gateway-smb
File:  internal/infrastructure/rabbitmq/consume_session.go
Func:  consumeSession() → processDelivery() → handlePLNToken()
```
- Consume pesan dari queue SMB
- Parse JSON payload → `consumePayload`
- Route by `product_type` → `handlePLNToken()`
- Panggil usecase `ProcessTransaction()` (inquiry → payment → advice)

### Ringkasan: Pesan Lewat 4 Service

| # | Service | File Utama | Apa yang dilakukan |
|---|---------|------------|--------------------|
| 1 | **pps-services-tokopedia** | `usecase/payment_usecase.go` | Validasi + Oracle preorder + publish ke RabbitMQ |
| 2 | **pps-services-consumer** | `repository/consumerTunning.go` | Consume PRE-ORDER + Oracle SP + call publisher-provider |
| 3 | **publisher-provider** | (repo terpisah) | Route ke queue provider berdasarkan remarkImsi |
| 4 | **pps-services-gateway-smb** | `usecase/plntoken/pln_token_usecase.go` | Inquiry + Payment + Advice ke SMB API |

### Bisa Dipakai Selain Tokopedia? — YA!

Gateway SMB **gak peduli** siapa yang kirim pesan. Dia cuma baca dari RabbitMQ queue.
Selama pesannya masuk ke queue dengan format JSON yang benar, gateway SMB tetap jalan.

```mermaid
flowchart LR
    subgraph "Siapapun bisa jadi pengirim"
        A["🟢 Tokopedia"]
        B["🔵 Partner Baru X"]
        C["🟡 Internal System"]
        D["🟣 Partner Lain Y"]
    end

    subgraph "Titik masuk yang sama"
        RMQ["📬 RabbitMQ\nQueue SMB"]
    end

    subgraph "Gateway SMB"
        GW["🏪 Gateway SMB\n(consume + proses)"]
    end

    A -->|"publish ke queue"| RMQ
    B -->|"publish ke queue"| RMQ
    C -->|"publish ke queue"| RMQ
    D -->|"publish ke queue"| RMQ
    RMQ --> GW
```

**Kenapa bisa?** Karena arsitektur **event-driven** pakai RabbitMQ:
- Gateway SMB cuma tahu: "ada pesan di queue, gw proses"
- Dia **gak tahu dan gak perlu tahu** siapa yang publish pesan itu
- Yang penting: format JSON-nya benar

### Kalau Flow Pertama Bukan Tokopedia, Gimana?

Ada **2 cara** tergantung seberapa custom partner barunya:

#### Cara 1: Lewat Jalur yang Sudah Ada (Consumer + Publisher-Provider)

Kalau partner baru juga pakai Oracle preorder (sistem PPS existing), alurnya sama:

```mermaid
flowchart TD
    subgraph "Partner Baru (misal: Shopee)"
        P1["🌐 Service Gateway Shopee\n(mirip pps-services-tokopedia)"]
    end

    subgraph "Infrastruktur PPS yang Sudah Ada"
        RMQ1["📬 RabbitMQ\n(queue transaksi)"]
        CON["🧑‍💼 Consumer\n(PRE-ORDER flow)"]
        PP["🚚 Publisher-Provider\n(routing)"]
        RMQ2["📬 RabbitMQ\n(queue SMB)"]
    end

    subgraph "Gateway SMB"
        GW["🏪 Gateway SMB"]
    end

    P1 -->|"1. Publish PRE-ORDER\nke RabbitMQ"| RMQ1
    RMQ1 -->|"2. Consumer proses\n(Oracle SP)"| CON
    CON -->|"3. Call publisher-provider"| PP
    PP -->|"4. Route ke queue SMB"| RMQ2
    RMQ2 -->|"5. Gateway SMB\nproses PLN Token"| GW

    style P1 fill:#e8f5e9,stroke:#2e7d32
```

Yang perlu dibuat: **Service gateway baru untuk partner** (mirip pps-services-tokopedia).
Gateway SMB **gak perlu diubah sama sekali**.

#### Cara 2: Publish Langsung ke Queue SMB (Shortcut)

Kalau partner baru **gak pakai** Oracle preorder, bisa publish langsung ke queue SMB:

```mermaid
flowchart TD
    subgraph "Partner Baru"
        P1["🌐 Service Partner Baru"]
    end

    subgraph "Langsung ke Queue"
        RMQ["📬 RabbitMQ\n(queue SMB)"]
    end

    subgraph "Gateway SMB"
        GW["🏪 Gateway SMB"]
    end

    P1 -->|"Publish JSON langsung\nke queue SMB"| RMQ
    RMQ -->|"Consume + proses"| GW

    style P1 fill:#fff3e0,stroke:#ef6c00
```

Yang perlu dilakukan partner: publish JSON ke queue SMB dengan format ini:

```json
{
  "msg_id": "12345",
  "client_number": "12345678901",
  "product_code": "PLN50",
  "product_type": "pln_token",
  "amount": 50000,
  "mid": "PARTNER_X",
  "queue_name": "queue_smb_pln_token",
  "MQTransaction": "amqp://user:pass@rabbitmq-host:5672/"
}
```

| Field | Wajib? | Penjelasan |
|-------|--------|------------|
| `msg_id` | ✅ | ID unik transaksi (untuk tracking) |
| `client_number` | ✅ | Nomor meter PLN pelanggan |
| `product_code` | ✅ | Kode produk PLN Token (misal: PLN20, PLN50, PLN100) |
| `product_type` | ✅ | Harus `"pln_token"` supaya di-route ke flow PLN Token |
| `amount` | ✅ | Nominal dalam rupiah |
| `mid` | ❌ | ID merchant (untuk tracking) |
| `queue_name` | ❌ | Override queue name (default dari config) |
| `MQTransaction` | ✅ | URL RabbitMQ tujuan untuk publish hasil balik |

> **`MQTransaction` itu penting!** Ini URL RabbitMQ tempat gateway SMB akan publish hasil
> (sukses/gagal) dalam format `{"source":"PROVIDER","data":{...}}`.
> Tanpa ini, gateway SMB gak bisa kirim hasil balik.

### Kesimpulan

```
Gateway SMB = "Loket PLN yang melayani siapa saja"

Mau Tokopedia, Shopee, atau sistem internal —
selama kirim pesan ke queue dengan format yang benar,
gateway SMB akan proses dan kirim hasil balik.

Yang berubah: SIAPA yang publish ke queue
Yang TIDAK berubah: Gateway SMB itu sendiri
```

### Validasi & Identifikasi Partner — Di Mana?

> **"Taunya itu transaksi dari Tokopedia atau dari partner lain gimana?"**

Jawabannya: validasi partner **BUKAN di gateway SMB**, tapi di **layer sebelumnya**.
Setiap layer punya validasi sendiri-sendiri:

```mermaid
flowchart TD
    subgraph LAYER1 ["🔒 Layer 1: Service Gateway Partner\n(pps-services-tokopedia)"]
        V1["✅ IP Whitelist\n📄 middleware/ip_whitelist_middleware.go\nCek IP request vs daftar IP di Redis/Oracle"]
        V2["✅ Bearer Token (JWT)\n📄 middleware/crypto_middleware.go\nValidasi JWT dari client_id + client_secret"]
        V3["✅ Encrypt/Decrypt\n📄 middleware/crypto_middleware.go\nRequest harus di-encrypt pakai RSA"]
        V4["✅ Rate Limit\n📄 middleware/rate_limit_middleware.go\nBatasi jumlah request per detik"]
        V5["✅ Client ID\n📄 usecase/payment_usecase.go\nEnv TP_CLIENT_ID → jadi field 'User' di Oracle"]
    end

    subgraph LAYER2 ["🔒 Layer 2: Oracle Database\n(Stored Procedure)"]
        V6["✅ User/Partner Identity\n📄 Oracle SP: Preorder()\nField 'User' = client_id partner\n→ Oracle tahu ini transaksi dari siapa"]
        V7["✅ Product Mapping\n📄 Oracle SP: updPreOrderConsume()\nOracle return remarkImsi, queueName,\nprovider berdasarkan User + Product"]
    end

    subgraph LAYER3 ["🔒 Layer 3: Publisher-Provider"]
        V8["✅ Routing by remarkImsi\nremarkImsi menentukan\nke queue provider mana"]
    end

    subgraph LAYER4 ["📨 Layer 4: Gateway SMB"]
        V9["❌ Tidak ada validasi partner\nGateway SMB cuma proses pesan\nyang sudah masuk ke queue-nya"]
    end

    V1 --> V2 --> V3 --> V4 --> V5
    V5 -->|"User = TP_CLIENT_ID"| V6
    V6 -->|"remarkImsi, queueName"| V7
    V7 -->|"remarkImsi"| V8
    V8 -->|"pesan masuk queue"| V9

    style LAYER1 fill:#e8f5e9,stroke:#2e7d32
    style LAYER2 fill:#fff3e0,stroke:#ef6c00
    style LAYER3 fill:#e3f2fd,stroke:#1565c0
    style LAYER4 fill:#fce4ec,stroke:#c62828
```

**Detail per layer:**

**Layer 1 — Service Gateway Partner (Tokopedia)**
| Validasi | File | Cara Kerja |
|----------|------|------------|
| IP Whitelist | `middleware/ip_whitelist_middleware.go` | Cek IP request ada di daftar IP yang terdaftar di Redis/Oracle. Kalau IP gak terdaftar → reject |
| JWT Token | `middleware/crypto_middleware.go` | Partner harus minta token dulu (`POST /auth/token` pakai `client_id` + `client_secret`). Setiap request harus bawa token ini di header |
| Encrypt/Decrypt | `middleware/crypto_middleware.go` | Request body harus di-encrypt pakai RSA public key. Response juga di-encrypt balik |
| Rate Limit | `middleware/rate_limit_middleware.go` | Batasi jumlah request per detik per endpoint |
| Client ID | `usecase/payment_usecase.go` | Env `TP_CLIENT_ID` dipakai sebagai field `User` saat call Oracle Preorder. **Ini yang bikin Oracle tahu transaksi dari partner mana** |

**Layer 2 — Oracle Database**
| Validasi | Stored Procedure | Cara Kerja |
|----------|-----------------|------------|
| Partner Identity | `Preorder(User, MDN, Product, ...)` | Field `User` = client_id partner. Oracle simpan ini di tabel transaksi |
| Product Routing | `updPreOrderConsume(InMsgId, ...)` | Oracle return `OutRemarkImsi`, `OutQueueName`, `OutProvider` — ini yang menentukan pesan dikirim ke gateway mana |

**Layer 3 — Publisher-Provider**
| Validasi | Cara Kerja |
|----------|------------|
| Routing | `remarkImsi` dari Oracle menentukan queue tujuan. Misal: remarkImsi "TELKOMSEL" → queue Telkomsel, remarkImsi "SMB" → queue SMB |

**Layer 4 — Gateway SMB**
| Validasi | Cara Kerja |
|----------|------------|
| **Tidak ada** | Gateway SMB **tidak validasi** siapa pengirimnya. Dia cuma proses pesan yang sudah masuk ke queue-nya. Validasi sudah dilakukan di layer 1-3 |

**Jadi kesimpulannya:**

```
Pertanyaan: "Taunya ini dari Tokopedia gimana?"
Jawaban:    Oracle yang tahu.

Field 'User' di Oracle Preorder = client_id partner.
- Tokopedia → User = "TOKOPEDIA" (dari env TP_CLIENT_ID)
- Partner X → User = "PARTNER_X" (dari env di service partner X)

Oracle juga yang tentukan routing:
- updPreOrderConsume() return remarkImsi + queueName
- remarkImsi inilah yang menentukan pesan dikirim ke gateway mana
```

---

## BAGIAN 1: GAMBARAN BESAR — "Ini Service Apa Sih?"

### Satu Kalimat
Gateway SMB adalah **microservice Go** yang tugasnya:
1. **Dengarkan** pesan dari RabbitMQ (antrian pesan)
2. **Panggil** API SMB/Loket Bayar untuk beli token PLN
3. **Kirim balik** hasilnya (sukses/gagal) ke RabbitMQ supaya service lain tahu

### Analogi Sederhana

Bayangkan kamu kerja di loket pembayaran:


#### Contoh 1
```mermaid
flowchart LR
    A["🧑 Pelanggan\n(Tokopedia)"] -->|"minta beli\ntoken PLN"| B["🧑‍💼 Kasir\n(Consumer)"]
    B -->|"kirim pesanan\nke kurir"| C["🚚 Kurir\n(Publisher-Provider)"]
    C -->|"antar pesanan\nke loket PLN"| D["🏪 Loket PLN\n(Gateway SMB)"]
    D -->|"beli token\nke PLN"| E["⚡ PLN\n(SMB API)"]
    E -->|"kasih struk\n+ token"| D
    D -->|"kirim struk\nbalik ke kasir"| B
```
#### Contoh 2
```
Pelanggan (Tokopedia) → Kasir (Consumer) → Kurir (Publisher-Provider) → Loket PLN (Gateway SMB) → PLN (SMB API)
                                                                              ↓
                                                                     Kasir terima hasil ← Loket kirim struk
```

> Pelanggan gak langsung ke PLN. Dia lewat kasir, kurir, baru sampai loket.
> Loket yang ngurus beli token, terus kirim struk balik ke kasir.

### Posisi di Arsitektur Keseluruhan

#### Contoh 1
```mermaid
flowchart TD
    subgraph "Fase 1: Pelanggan Minta Beli"
        TOK["🧑 Tokopedia\n(payment request)"]
        CON1["🧑‍💼 Consumer\n(proses PRE-ORDER)"]
        PP["🚚 Publisher-Provider\n(routing ke provider)"]
        RMQ1["📬 RabbitMQ\n(queue SMB)"]
    end

    subgraph "Fase 2: Gateway SMB Proses"
        GW["🏪 Gateway SMB\n(service ini!)"]
        SMB["⚡ SMB API\n(PLN Token)"]
        PG["🗄️ PostgreSQL\n(transaction log)"]
    end

    subgraph "Fase 3: Hasil Dikirim Balik"
        RMQ2["📬 RabbitMQ\n(result queue)"]
        CON2["🧑‍💼 Consumer\n(proses PROVIDER)"]
        ORA["🗄️ Oracle DB\n(update status final)"]
    end

    TOK -->|"POST /payment"| CON1
    CON1 -->|"HTTP POST"| PP
    PP -->|"publish berdasarkan\nremarkImsi"| RMQ1
    RMQ1 -->|"consume pesan"| GW
    GW -->|"inquiry + payment"| SMB
    SMB -->|"response\n(token/gagal)"| GW
    GW -.->|"log transaksi"| PG
    GW -->|"publish\nsource=PROVIDER"| RMQ2
    RMQ2 -->|"consume pesan"| CON2
    CON2 -->|"SP_SetTransaction\nStatus"| ORA
```
#### Contoh 2
```
┌─────────────┐     ┌──────────────┐     ┌───────────────────┐     ┌──────────────┐
│  Tokopedia  │────▶│   Consumer   │────▶│ Publisher-Provider│────▶│  RabbitMQ    │
│  (payment)  │     │  (PRE-ORDER) │     │  (routing)        │     │  (queue SMB) │
└─────────────┘     └──────────────┘     └───────────────────┘     └──────┬───────┘
                                                                          │
                                                                          ▼
                    ┌──────────────┐     ┌───────────────────┐     ┌──────────────┐
                    │   Consumer   │◀────│    RabbitMQ       │◀────│ Gateway SMB  │
                    │  (PROVIDER)  │     │  (result queue)   │     │ (service ini)│
                    └──────┬───────┘     └───────────────────┘     └──────┬───────┘
                           │                                              │
                           ▼                                              ▼
                    ┌──────────────┐                               ┌──────────────┐
                    │  Oracle DB   │                               │  SMB API     │
                    │  (update     │                               │  (PLN Token) │
                    │   status)    │                               └──────────────┘
                    └──────────────┘
```

> **Kunci:** Service ini BUKAN HTTP server biasa. Dia **consumer** — dia dengarkan queue, bukan terima HTTP request dari user.

---

## BAGIAN 2: STRUKTUR FOLDER — "File Apa di Mana?"

```
pps-services-gateway-smb/
│
├── cmd/app/
│   └── main.go                          ← 🟢 MULAI BACA DARI SINI
│                                           Entry point, tempat semua komponen di-wire
│
├── internal/
│   ├── config/
│   │   └── config.go                    ← 🔵 Baca environment variable
│   │                                       (RABBITMQ_URL, SMB_BASE_URL, dll)
│   │
│   ├── domain/contract/service/         ← 🟡 Interface / Kontrak
│   │   ├── logger.go                       Interface Logger
│   │   ├── mq_publisher.go                 Interface MQPublisher
│   │   ├── smb_client.go                   Interface SMBClient (inquiry/payment/advice)
│   │   └── transaction_logger.go           Interface TransactionLogger (Postgres)
│   │
│   ├── usecase/plntoken/                ← 🔴 LOGIC BISNIS ADA DI SINI
│   │   └── pln_token_usecase.go            ProcessTransaction() = inquiry → payment
│   │                                       RetryAdvice() = retry advice kalau pending
│   │
│   ├── infrastructure/
│   │   ├── rabbitmq/
│   │   │   ├── consumer_service.go      ← 🔴 INTI SERVICE — struct + Start()
│   │   │   └── consume_session.go       ← 🔴 INTI SERVICE — logic consume + PLN Token flow
│   │   │
│   │   ├── mqpublisher/
│   │   │   ├── publisher.go             ← Implementasi publish ke RabbitMQ
│   │   │   └── message.go               ← Format pesan {"source":"PROVIDER","data":{...}}
│   │   │
│   │   ├── postgres/
│   │   │   └── transaction_logger.go    ← Implementasi logging ke PostgreSQL
│   │   │
│   │   └── smbclient/
│   │       └── adapter.go               ← Adapter: pkg/smb → domain interface
│   │
│   └── util/
│       ├── rc.go                        ← Mapping response code SMB → RC PPS
│       └── trxid.go                     ← Generate transaction ID
│
├── pkg/smb/
│   ├── client.go                        ← HTTP client ke SMB API (struct + constructor)
│   └── pln_token.go                     ← 3 method: InquiryPLNToken, PaymentPLNToken, AdvicePLNToken
│
├── .env.example                         ← Contoh environment variable
├── Dockerfile                           ← Build Docker image
├── Makefile                             ← Command: build, run, test, lint
└── go.mod                               ← Dependency management
```

### Urutan Baca yang Disarankan (untuk belajar):

```mermaid
flowchart TD
    A["1️⃣ .env.example\nApa aja yang perlu di-set?"] --> B["2️⃣ internal/config/\nGimana cara baca env var?"]
    B --> C["3️⃣ domain/contract/service/\nInterface apa aja yang ada?"]
    C --> D["4️⃣ pkg/smb/\nGimana cara panggil API SMB?"]
    D --> E["5️⃣ infrastructure/smbclient/\nAdapter: pkg/smb → interface"]
    E --> F["6️⃣ usecase/plntoken/ 🔴\nLogic bisnis: inquiry → payment → advice"]
    F --> G["7️⃣ infrastructure/rabbitmq/\nConsumer: consume → panggil usecase → publish"]
    G --> H["8️⃣ cmd/app/main.go\nWiring semua komponen jadi satu"]

    style F fill:#ff6b6b,color:#fff
```

---

## BAGIAN 3: ALUR KODE DARI AWAL SAMPAI AKHIR

### Fase 1: Startup (main.go)

Saat service dijalankan (`go run cmd/app/main.go`), ini yang terjadi step by step:

```go
// 1. Setup logger (JSON format, timezone Jakarta)
slogLogger := slog.New(slog.NewJSONHandler(os.Stdout, ...))

// 2. Baca semua konfigurasi dari environment variable
cfg, _ := config.Load()           // RABBITMQ_URL, QUEUE_NAME, POSTGRES_DSN
smbCfg, _ := config.LoadSMB()     // SMB_BASE_URL, SMB_PARTNER_ID, SMB_SECRET_KEY
retryConfig, _ := config.LoadRetryConfig()  // RETRY_MAX_ATTEMPTS, RETRY_WAIT_SECONDS
httpCfg, _ := config.LoadCallbackServer()   // HTTP_PORT

// 3. Buat context yang bisa di-cancel pakai Ctrl+C
ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)

// 4. Inisialisasi SMB HTTP client (untuk panggil API PLN Token)
smbHTTPClient := smb.NewClient(smbCfg.BaseURL, smbCfg.PartnerID, smbCfg.SecretKey, ...)
smbAdapter := smbclient.NewAdapter(smbHTTPClient, logger)

// 5. Inisialisasi RabbitMQ consumer
consumer := rabbitmq.NewConsumerServiceImpl(cfg, logger)
consumer.SetSMBClient(smbAdapter)       // inject SMB client
consumer.SetRetryConfig(retryConfig)    // inject retry config

// 6. (Opsional) Inisialisasi Postgres untuk logging transaksi
if cfg.PostgresDSN != "" {
    pgLogger, _ := postgres.NewTransactionLogger(cfg.PostgresDSN, logger)
    pgLogger.RunMigration(ctx)          // auto-create tabel kalau belum ada
    consumer.SetTransactionLogger(pgLogger)
}

// 7. Inisialisasi MQ Publisher (untuk kirim hasil ke downstream)
mqPub := mqpublisher.NewAMQPPublisher(logger)
consumer.SetMQPublisher(mqPub)

// 8. Jalankan HTTP server + RabbitMQ consumer secara paralel
g.Go(func() { app.Listen(":8080") })           // HTTP health check
g.Go(func() { consumer.Start(ctx) })           // RabbitMQ consumer loop
```

**Poin penting:**
- Semua dependency di-inject manual (bukan pakai framework DI)
- `errgroup` dipakai supaya HTTP server dan consumer jalan bareng
- Kalau salah satu mati, semuanya ikut mati (graceful shutdown)

### Fase 2: Consumer Loop (consumer_service.go → Start)

```go
func (s *ConsumerServiceImpl) Start(ctx context.Context) error {
    backoff := time.Second        // mulai dari 1 detik
    maxBackoff := 30 * time.Second

    for {
        // Cek apakah service diminta berhenti
        if ctx.Done() → return

        // Coba consume dari RabbitMQ
        err := s.consumeSession(ctx)

        // Kalau error (koneksi putus), tunggu lalu coba lagi
        // Backoff: 1s → 2s → 4s → 8s → 16s → 30s (max)
        time.Sleep(backoff)
        backoff = backoff * 2  // exponential backoff
    }
}
```

**Kenapa pakai loop + backoff?**
Karena koneksi RabbitMQ bisa putus kapan saja (network issue, server restart).
Service harus bisa reconnect otomatis tanpa perlu di-restart manual.

### Fase 3: Consume Session (consume_session.go → consumeSession)

```go
func (s *ConsumerServiceImpl) consumeSession(ctx context.Context) error {
    // 1. Buka koneksi ke RabbitMQ
    conn, _ := amqp.Dial(s.cfg.RabbitMQURL)

    // 2. Buka channel
    ch, _ := conn.Channel()

    // 3. Declare queue (idempotent — aman dipanggil berulang)
    ch.QueueDeclare(s.cfg.QueueName, true, false, false, false, nil)
    //                                ^^^^
    //                                durable = true → queue survive restart

    // 4. Set QoS = 1 (FIFO — proses satu pesan dulu, baru ambil berikutnya)
    ch.Qos(1, 0, false)

    // 5. Mulai consume
    deliveries, _ := ch.Consume(queue, consumerTag, false, ...)
    //                                               ^^^^^
    //                                               auto-ack = false → manual ack

    // 6. Loop: ambil pesan satu per satu
    for delivery := range deliveries {
        s.processDelivery(ctx, delivery, &wg)
    }
}
```

**Kenapa QoS = 1?**
Supaya pesan diproses satu-satu (FIFO). Kalau QoS = 10, bisa ada 10 pesan diproses paralel — bisa race condition.

**Kenapa manual ack?**
Supaya kalau proses gagal, pesan bisa di-requeue. Kalau auto-ack, pesan langsung hilang begitu diambil.

### Fase 4: Process Delivery (consume_session.go → processDelivery)

```go
func (s *ConsumerServiceImpl) processDelivery(ctx, delivery, wg) {
    defer delivery.Ack(false)  // SELALU ack setelah selesai

    // 1. Parse JSON payload
    var payload consumePayload
    json.Unmarshal(delivery.Body, &payload)
    // payload berisi: client_number, product_code, product_type, amount, MQTransaction, dll

    // 2. Tentukan msgID (bisa dari payload, atau dari header RabbitMQ)
    msgID := payload.MsgID || delivery.MessageId || delivery.CorrelationId

    // 3. Route berdasarkan product_type
    switch productType {
    case "pln_token":
        s.processPLNToken(ctx, payload, msgID, queueName, wg)
    default:
        log.Warn("unsupported product type")
    }
}
```

**Kenapa `defer delivery.Ack(false)`?**
Pesan SELALU di-ack setelah diproses, apapun hasilnya (sukses/gagal).
Kalau gagal, kita publish status "C" (cancel) ke downstream — bukan requeue pesan.

### Fase 5: PLN Token Flow — LOGIC BISNIS

> **File:** `internal/usecase/plntoken/pln_token_usecase.go`
> Ini tempat semua logic bisnis PLN Token. Consumer cuma panggil usecase ini.

```mermaid
flowchart TD
    START(["📨 Pesan masuk dari RabbitMQ"]) --> GEN["Generate transaction ID\n(ourTrxID)"]
    GEN --> LOG_PROC["📝 Log ke Postgres\nstatus = PROCESSING"]

    LOG_PROC --> INQ_START

    subgraph INQUIRY ["🔍 STEP 1: INQUIRY — Cek Data Pelanggan PLN"]
        INQ_START["Call SMB API\nPOST /pln-prepaid/inquiry"] --> INQ_CHECK{Response code?}
        INQ_CHECK -->|"❌ Error\n(timeout/network)"| INQ_FAIL["GAGAL"]
        INQ_CHECK -->|"❌ != 00\n(data salah, dll)"| INQ_FAIL
        INQ_CHECK -->|"✅ 00\n(sukses)"| INQ_OK["Dapat:\n• ref_id\n• nama pelanggan\n• tarif/daya\n• total harga"]
    end

    INQ_FAIL --> PUB_C1["Publish 'C' (cancel)\nke downstream"]
    PUB_C1 --> DONE1(["🔴 SELESAI — GAGAL"])

    INQ_OK --> PAY_START

    subgraph PAYMENT ["💳 STEP 2: PAYMENT — Beli Token PLN"]
        PAY_START["Call SMB API\nPOST /pln-prepaid/payment\n(pakai ref_id dari inquiry)"] --> PAY_CHECK{Response code?}
        PAY_CHECK -->|"✅ 00"| PAY_OK["SUKSES\nDapat token PLN!"]
        PAY_CHECK -->|"❌ 93/97/dll"| PAY_FAIL["GAGAL"]
        PAY_CHECK -->|"⏳ 28/68\n(timeout/pending)"| PAY_PENDING["PENDING"]
        PAY_CHECK -->|"❌ Error\n(network)"| PAY_PENDING
    end

    PAY_OK --> LOG_SUC["📝 Log: status = SUCCESS"]
    LOG_SUC --> PUB_F["Publish 'F' (final)\n+ token PLN ke downstream"]
    PUB_F --> DONE2(["🟢 SELESAI — SUKSES"])

    PAY_FAIL --> LOG_FAIL["📝 Log: status = FAILED"]
    LOG_FAIL --> PUB_C2["Publish 'C' (cancel)\nke downstream"]
    PUB_C2 --> DONE3(["🔴 SELESAI — GAGAL"])

    PAY_PENDING --> RETRY_START

    subgraph ADVICE ["🔄 STEP 3: RETRY ADVICE — Cek Status (async goroutine)"]
        RETRY_START["Loop: attempt 1..4\n(interval 10 detik)"] --> ADV_CALL["Call SMB API\nPOST /pln-prepaid/advice"]
        ADV_CALL --> ADV_CHECK{Response code?}
        ADV_CHECK -->|"✅ 00"| ADV_OK["SUKSES\nDapat token!"]
        ADV_CHECK -->|"❌ gagal"| ADV_FAIL["GAGAL"]
        ADV_CHECK -->|"⏳ 28/68\nmasih pending"| ADV_RETRY{"Masih ada\nattempt?"}
        ADV_RETRY -->|"Ya"| ADV_CALL
        ADV_RETRY -->|"Tidak\n(4x habis)"| ADV_EXHAUST["GAGAL\n(retry habis)"]
    end

    ADV_OK --> LOG_SUC2["📝 Log: status = SUCCESS"]
    LOG_SUC2 --> PUB_F2["Publish 'F' + token"]
    PUB_F2 --> DONE4(["🟢 SELESAI — SUKSES"])

    ADV_FAIL --> PUB_C3["Publish 'C' (cancel)"]
    PUB_C3 --> DONE5(["🔴 SELESAI — GAGAL"])

    ADV_EXHAUST --> PUB_C4["Publish 'C' (cancel)"]
    PUB_C4 --> DONE6(["🔴 SELESAI — GAGAL"])

    style INQUIRY fill:#e3f2fd,stroke:#1976d2
    style PAYMENT fill:#fff3e0,stroke:#f57c00
    style ADVICE fill:#fce4ec,stroke:#c62828
```

**Penjelasan per step:**

| Step | File | Method | Apa yang dilakukan |
|------|------|--------|--------------------|
| 1. Inquiry | `usecase/plntoken/pln_token_usecase.go` | `ProcessTransaction()` | Panggil SMB API inquiry, cek data pelanggan PLN |
| 2. Payment | `usecase/plntoken/pln_token_usecase.go` | `ProcessTransaction()` | Panggil SMB API payment, beli token PLN |
| 3. Advice | `usecase/plntoken/pln_token_usecase.go` | `RetryAdvice()` | Retry cek status kalau payment pending |
| Orchestrator | `infrastructure/rabbitmq/consume_session.go` | `handlePLNToken()` | Panggil usecase, log ke Postgres, publish ke downstream |

### Fase 6: Publish ke Downstream (consume_session.go → publishToDownstream)

Setelah dapat hasil (sukses/gagal), service publish ke RabbitMQ dalam format:

```json
{
  "source": "PROVIDER",
  "data": {
    "msg_id": 12345,
    "status_to_be": "F",
    "serial_number": "TOKEN_PLN_xxxx",
    "client_number": "12345678901",
    "nominal": "50000",
    "conversation_id": "SMB-MID-123-20260422120000",
    "message_to_customer": "Token PLN: TOKEN_PLN_xxxx",
    "queue_name": "queue_smb_pln_token"
  }
}
```

**status_to_be:**
- `"F"` = Final (sukses) — token PLN berhasil didapat
- `"C"` = Cancel (gagal) — transaksi gagal
- `"S"` = Still processing — masih diproses (jarang dipakai)

**Siapa yang consume pesan ini?**
`pps-services-consumer` — dia akan baca `source: "PROVIDER"`, lalu panggil stored procedure Oracle `SP_SetTransactionStatus` untuk update status final di database.

---

## BAGIAN 4: KONSEP-KONSEP PENTING

### 4.1 Clean Architecture
```
Domain (interface)  ← paling dalam, tidak tergantung siapapun
    ↑
Infrastructure      ← implementasi interface (RabbitMQ, Postgres, HTTP client)
    ↑
cmd/app             ← wiring semua komponen
```

**Kenapa pakai interface?**
- Supaya bisa di-mock untuk testing
- Supaya bisa ganti implementasi tanpa ubah business logic
- Contoh: `SMBClient` interface bisa diimplementasi pakai HTTP, gRPC, atau mock

### 4.2 Event-Driven Architecture
```
Service A → [RabbitMQ Queue] → Service B → [RabbitMQ Queue] → Service C
```
- Service tidak saling panggil langsung (decoupled)
- Kalau satu service mati, pesan tetap aman di queue
- Bisa scale horizontal (tambah consumer)

### 4.3 MQPublisher — Kenapa Buka Koneksi Baru Tiap Publish?
```go
func (p *AMQPPublisher) Publish(ctx, mqTransactionURL, queueName, body) error {
    conn, _ := amqp.Dial(mqTransactionURL)  // koneksi baru!
    defer conn.Close()
    // ...publish...
}
```
Karena setiap transaksi bisa punya **URL RabbitMQ yang berbeda** (field `MQTransaction` di payload).
Jadi tidak bisa pakai satu koneksi shared.

### 4.4 Response Code Mapping
```
SMB Response Code → RC PPS → status_to_be
     "00"         →   0    →    "F" (sukses)
     "28"         →   9    →    "S" (pending, retry)
     "68"         →   9    →    "S" (pending, retry)
     ""           →   9    →    "S" (pending, retry)
     lainnya      →   1    →    "C" (gagal)
```

### 4.5 Exponential Backoff
Kalau koneksi RabbitMQ putus:
```
Retry 1: tunggu 1 detik
Retry 2: tunggu 2 detik
Retry 3: tunggu 4 detik
Retry 4: tunggu 8 detik
Retry 5: tunggu 16 detik
Retry 6: tunggu 30 detik (max)
Retry 7: tunggu 30 detik (max)
...
```
Supaya tidak membanjiri server dengan retry terus-menerus.

---

## BAGIAN 5: MATERI PRESENTASI

### Slide 1: Judul
```
pps-services-gateway-smb
Gateway PLN Token via SMB/Loket Bayar API
Event-Driven Architecture dengan RabbitMQ
```

### Slide 2: Masalah yang Diselesaikan
```
SEBELUM:
- Belum ada integrasi PLN Token via SMB
- Sistem hanya support Telkomsel dan Unipin

SESUDAH:
- Gateway SMB menambah provider PLN Token baru
- Mengikuti arsitektur event-driven yang sudah ada
- Plug-and-play: tidak perlu ubah service lain
```

### Slide 3: Arsitektur
```
[Gambar diagram dari BAGIAN 1 di atas]

Poin:
- Service ini adalah RabbitMQ consumer, bukan HTTP server
- Consume pesan → Panggil SMB API → Publish hasil
- Sama polanya dengan gateway-telkomsel dan gateway-unipin
```

### Slide 4: Tech Stack
```
| Komponen        | Teknologi                    |
|-----------------|------------------------------|
| Bahasa          | Go 1.24                      |
| Message Queue   | RabbitMQ (amqp091-go)        |
| HTTP Client     | net/http (standard library)  |
| HTTP Server     | Go Fiber (health check only) |
| Database        | PostgreSQL (transaction log) |
| Logging         | slog (structured JSON)       |
| Concurrency     | errgroup + goroutine         |
```

### Slide 5: Flow PLN Token
```
[Gambar flowchart dari BAGIAN 3 Fase 5]

3 langkah:
1. INQUIRY  → cek data pelanggan PLN
2. PAYMENT  → eksekusi pembelian token
3. ADVICE   → retry kalau payment pending (timeout)
```

### Slide 6: Handling Error & Retry
```
| Skenario              | Response Code | Aksi                    |
|-----------------------|---------------|-------------------------|
| Inquiry sukses        | 00            | Lanjut ke payment       |
| Inquiry gagal         | != 00         | Publish "C" (cancel)    |
| Payment sukses        | 00            | Publish "F" + token     |
| Payment gagal         | 93, 97, dll   | Publish "C" (cancel)    |
| Payment pending       | 28, 68        | Retry advice max 4x     |
| Advice sukses         | 00            | Publish "F" + token     |
| Advice masih pending  | 28, 68        | Retry lagi              |
| Retry habis           | -             | Publish "C" (cancel)    |
```

### Slide 7: Format Pesan Downstream
```json
{
  "source": "PROVIDER",
  "data": {
    "msg_id": 12345,
    "status_to_be": "F",
    "serial_number": "TOKEN_PLN_xxxx",
    "client_number": "12345678901",
    "nominal": "50000",
    "message_to_customer": "Token PLN: TOKEN_PLN_xxxx"
  }
}
```
```
Pesan ini di-consume oleh pps-services-consumer
→ Update status final di Oracle via SP_SetTransactionStatus
```

### Slide 8: Impact ke Service Lain
```
| Service               | Perubahan?  | Keterangan                          |
|-----------------------|-------------|-------------------------------------|
| pps-services-consumer | TIDAK       | Sudah support source "PROVIDER"     |
| pps-services-tokopedia| TIDAK       | Publish ke consumer seperti biasa   |
| publisher-provider    | KONFIGURASI | Tambah routing rule ke queue SMB    |
| Gateway SMB           | BARU        | Service baru yang kita develop      |
```

### Slide 9: Struktur Kode
```
[Gambar tree folder dari BAGIAN 2]

Poin:
- Clean Architecture: domain → infrastructure → cmd
- Interface di domain, implementasi di infrastructure
- pkg/smb = HTTP client murni, bisa di-reuse
```

### Slide 10: Apa yang Bisa Dikembangkan
```
- [ ] Tambah produk lain (PLN Pascabayar, BPJS, dll)
- [ ] Tambah error mapping ke Postgres (seperti gateway-telkomsel)
- [ ] Tambah unit test untuk consumer dan SMB client
- [ ] Tambah metrics/monitoring (Prometheus)
- [ ] Tambah circuit breaker kalau SMB API sering down
```

---

## BAGIAN 6: GLOSSARY (Istilah-Istilah)

| Istilah | Artinya |
|---------|---------|
| **Consumer** | Service yang membaca/mengambil pesan dari queue RabbitMQ |
| **Publisher** | Service yang mengirim pesan ke queue RabbitMQ |
| **Queue** | Antrian pesan di RabbitMQ (seperti antrian di loket) |
| **Ack** | Acknowledgement — konfirmasi bahwa pesan sudah diproses |
| **QoS** | Quality of Service — berapa pesan yang boleh diambil sekaligus |
| **FIFO** | First In First Out — pesan diproses sesuai urutan masuk |
| **Downstream** | Service yang menerima hasil dari service ini |
| **Upstream** | Service yang mengirim pesan ke service ini |
| **MQTransaction** | URL RabbitMQ tujuan untuk publish hasil (bisa beda tiap transaksi) |
| **RC PPS** | Response Code internal PPS: 0=sukses, 1=gagal, 9=pending |
| **status_to_be** | Status yang dikirim ke consumer: F=final, C=cancel, S=still processing |
| **Inquiry** | Cek data pelanggan PLN (nama, tarif, daya) |
| **Payment** | Eksekusi pembelian token PLN |
| **Advice** | Cek status transaksi yang pending (retry mechanism) |
| **Exponential Backoff** | Strategi retry: tunggu makin lama tiap kali gagal |
| **Graceful Shutdown** | Matikan service dengan bersih (selesaikan proses dulu, baru stop) |
| **Interface** | Kontrak/blueprint yang mendefinisikan method tanpa implementasi |
| **Adapter** | Kelas yang mengubah satu interface ke interface lain |
| **errgroup** | Library Go untuk menjalankan beberapa goroutine dan tunggu semuanya selesai |

---

## BAGIAN 7: FAQ — Pertanyaan yang Mungkin Ditanya Saat Presentasi

**Q: Kenapa bikin service baru, bukan extend pps-services-tokopedia?**
A: Karena arsitektur event-driven. Setiap provider punya gateway sendiri (Telkomsel, Unipin, SMB). Ini supaya:
- Kalau SMB down, gateway lain tidak terpengaruh
- Bisa scale independent
- Kode lebih fokus dan mudah di-maintain

**Q: Kalau SMB API timeout, apa yang terjadi?**
A: Payment akan return response code "28" atau "68" (pending). Service akan retry advice sampai 4x dengan interval 10 detik. Kalau masih pending, transaksi di-mark FAILED dan publish "C" ke downstream.

**Q: Kenapa pakai RabbitMQ, bukan langsung HTTP call?**
A: Supaya decoupled. Kalau pakai HTTP langsung:
- Kalau consumer mati, pesan hilang
- Kalau traffic tinggi, bisa overload
- Tidak bisa retry otomatis
Dengan RabbitMQ, pesan aman di queue sampai diproses.

**Q: Apa bedanya service ini dengan gateway-telkomsel?**
A: Polanya sama (consume → call API → publish), tapi:
- Telkomsel: produk pulsa + paket data, API ESB REST
- SMB: produk PLN Token, API H2H Loket Bayar
- Telkomsel punya callback endpoint (async paket data)
- SMB pakai advice/check status (sync retry)

**Q: Gimana kalau mau tambah produk baru (misal BPJS)?**
A: Tinggal:
1. Tambah method di `SMBClient` interface
2. Tambah implementasi di `pkg/smb/`
3. Tambah case di `processDelivery()` switch
4. Buat function `processBPJS()` mirip `processPLNToken()`

**Q: Service ini perlu database?**
A: Opsional. Postgres dipakai untuk logging transaksi (audit trail). Kalau `POSTGRES_DSN` tidak di-set, service tetap jalan — cuma tidak ada log ke database.
