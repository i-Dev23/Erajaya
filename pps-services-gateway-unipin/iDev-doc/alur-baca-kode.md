# Panduan Alur Baca Kode — pps-services-gateway-unipin

Dokumen ini menjelaskan cara membaca kode project dari awal (main.go) sampai data masuk/keluar ke database dan API eksternal. Cocok untuk yang baru belajar Go.

---

## Gambaran Besar

```
┌─────────────────────────────────────────────────────────────────────┐
│                          main.go (titik awal)                       │
│                                                                     │
│  Baca .env → Buat koneksi DB → Buat client API → Jalankan service   │
└──────────┬──────────────┬──────────────┬───────────────┬────────────┘
           │              │              │               │
           ▼              ▼              ▼               ▼
    ┌──────────┐   ┌──────────┐   ┌──────────┐   ┌──────────┐
    │ HTTP     │   │ RabbitMQ │   │ Cron     │   │ Shutdown │
    │ Server   │   │ Consumer │   │ Scheduler│   │ Handler  │
    │ (Fiber)  │   │          │   │          │   │          │
    └────┬─────┘   └────┬─────┘   └────┬─────┘   └──────────┘
         │              │              │
         ▼              ▼              ▼
    ┌──────────┐   ┌──────────┐   ┌──────────┐
    │ Handler  │   │ Consumer │   │ SyncSvc  │
    │          │   │ Service  │   │          │
    └────┬─────┘   └────┬─────┘   └────┬─────┘
         │              │              │
         ▼              ▼              ▼
    ┌───────────────────────────────────────────┐
    │          pkg/unipin (UniPin Client)       │
    │     + Oracle DB  + Postgres  + RabbitMQ   │
    └───────────────────────────────────────────┘
```

---

## Langkah 1: Mulai dari main.go

File: `cmd/app/main.go`

Ini titik awal program. Urutan yang terjadi:

```
1. Load .env file (baca environment variables)
2. Buat logger (slog JSON)
3. Load config dari env: Config, UnipinConfig, OracleConfig, PostgresConfig
4. Buat UniPin HTTP client (pkg/unipin/client.go)
5. Koneksi ke Postgres (optional, untuk API logging)
6. Koneksi ke Oracle DB
7. Buat repository (akses database)
8. Buat service (business logic)
9. Jalankan 4 goroutine secara paralel:
   a. HTTP Server (Fiber)
   b. RabbitMQ Consumer
   c. Cron Scheduler
   d. Shutdown handler
10. Tunggu semua selesai
```

Cara baca: buka `main.go`, baca dari atas ke bawah. Setiap `NewXxx()` itu membuat komponen baru.

---

## Langkah 2: Pahami Struktur Folder

```
cmd/app/main.go                    ← Titik awal, wiring semua komponen
│
├── internal/config/               ← Baca environment variables
│   └── config.go                     Load(), LoadUnipin(), LoadOracle(), LoadPostgres()
│
├── internal/delivery/http/        ← HTTP endpoint (terima request dari luar)
│   └── handler/
│       └── gamelist_api_handler.go   POST /api/v1/game-list
│
├── internal/domain/contract/      ← Interface (kontrak/janji)
│   ├── repository/                   Interface untuk akses database
│   │   ├── game_list_repository.go      UpsertGameList()
│   │   ├── game_api_repository.go       ValidateSignature(), GetGameList()
│   │   └── api_log_repository.go        Insert() — log API
│   └── service/                      Interface untuk business logic
│       ├── consumer_service.go          Start()
│       ├── game_sync_service.go         SyncGameList(), SyncVoucherList()
│       ├── mq_publisher.go             Publish()
│       └── logger.go                    Info(), Warn(), Error()
│
├── internal/infrastructure/       ← Implementasi nyata (kodingan asli)
│   ├── oracle/                       Koneksi + query ke Oracle DB
│   │   ├── db.go                        NewDB() — buka koneksi
│   │   ├── game_list_repository.go      UpsertGameList() via SP
│   │   ├── game_api_repository.go       ValidateSignature(), GetGameList() via SP
│   │   └── voucher_list_repository.go   UpsertVoucherList() via SP
│   ├── postgres/                     Koneksi + query ke Postgres
│   │   ├── db.go                        NewDB() — buka koneksi
│   │   └── api_log_repository.go        Insert() — simpan log API
│   ├── rabbitmq/                     Consumer RabbitMQ
│   │   └── consumer_service.go          Start(), processMessage(), processGame(), processVoucher()
│   ├── mqpublisher/                  Publisher ke RabbitMQ downstream
│   │   ├── publisher.go                 Publish() — kirim pesan
│   │   └── message.go                   Format pesan JSON
│   └── scheduler/                    Cron job
│       └── cron.go                      Jadwal sync otomatis
│
├── internal/usecase/gamesync/     ← Business logic sync game/voucher
│   └── sync_service.go              SyncGameList(), SyncVoucherList(), SyncSingleGame()
│
├── internal/util/                 ← Helper/utility
│   └── generate_message_util.go     GenerateMessage() — format pesan customer
│
└── pkg/unipin/                    ← HTTP client ke UniPin API (reusable)
    ├── client.go                     NewClient(), generateAuth()
    ├── types.go                      BusinessError, TechnicalError
    ├── ingame_types.go               Request/Response struct
    ├── ingame_game_list.go           GameList() — GET daftar game
    ├── ingame_game_detail.go         GameDetail() — GET detail game
    ├── ingame_validate_user.go       ValidateUser() — validasi user game
    ├── ingame_create_order.go        CreateOrder() — buat order topup
    ├── ingame_order_inquiry.go       OrderInquiry() — cek status order
    ├── voucher_types.go              Request/Response struct voucher
    ├── voucher_list.go               VoucherList() — GET daftar voucher
    ├── voucher_detail.go             VoucherDetail() — GET detail voucher
    ├── voucher_request.go            VoucherRequest() — beli voucher
    ├── voucher_inquiry.go            VoucherInquiry() — cek status voucher
    └── logging_transport.go          LoggingTransport — catat semua request ke Postgres
```

---

## Langkah 3: Alur Data — HTTP Request (Game List API)

```
Client (Postman/Frontend)
    │
    │  POST /api/v1/game-list
    │  Body: {"user":"xxx","signature":"yyy"}
    │
    ▼
┌───────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  gamelist_api_handler.go                                                                                  │
│  HandleGameList()                                                                                         │
│                                                                                                           │
│  1. Parse request body (BodyParser)                                                                       │
│  2. Validasi: user & signature tidak kosong                                                               │
│  3. Panggil repo.ValidateSignature()    ────────► Oracle SP: MSG.PKG_UNIPIN.validateSignatureGameList     |
│  4. Jika signature invalid → 401                                                                          │
│  5. Panggil repo.GetGameList()          ────────► Oracle SP: MSG.PKG_UNIPIN.getGameList()                 |
│  6. Group hasil per product_code                                                                          │
│  7. Return JSON response                                                                                  │
└───────────────────────────────────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
Client terima response:
{"status":"0","message":"Successfully","data":[...]}
```

File yang perlu dibaca (urut):
1. `cmd/app/main.go` — cari `api.Post("/game-list", ...)`
2. `internal/delivery/http/handler/gamelist_api_handler.go` — `HandleGameList()`
3. `internal/domain/contract/repository/game_api_repository.go` — interface
4. `internal/infrastructure/oracle/game_api_repository.go` — implementasi SP call

---

## Langkah 4: Alur Data — RabbitMQ Consumer (Game Direct Topup)

```
RabbitMQ Queue
    │
    │  Pesan JSON masuk:
    │  {"product_type":"unipin-game","command":"MLBB*123",
    │   "msisdn":"{\"userid\":\"789\"}","msgid":"TRX-001",...}
    │
    ▼
┌───────────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  consumer_service.go                                                                                      │
│                                                                                                           │
│  Start()                                                                                                  │
│    └─► consumeSession()                                                                                   │
│          └─► Konek ke RabbitMQ                                                                            │
│              └─► Terima pesan dari queue                                                                  │
│                  └─► processMessage()                                                                     │
│                        │                                                                                  │
│                        ▼                                                                                  │
│                  resolveTxType()                                                                          │
│                  "unipin-game" → "GAME-DIRECT-TOP-UP"                                                     |
│                        │                                                                                  │
│                        ▼                                                                                  │
│                  processGame()                                                                            │
│                  1. Parse Command "MLBB*123"                                                              │
│                     → gameCode = "MLBB"                                                                   │
│                     → denominationID = "123"                                                              │
│                  2. Parse MSISDN JSON                                                                     │
│                     → fields = {"userid":"789"}                                                           │
│                  3. ValidateUser() ──────────────► UniPin API: POST /in-game-topup/user/validate          |
│                     ← validationToken                                                                     │
│                  4. CreateOrder() ───────────────► UniPin API: POST /in-game-topup/order/create           |
│                     ← referenceNo, status                                                                 │
│                  5. Jika timeout:                                                                         │
│                     OrderInquiry() ─────────────► UniPin API: POST /in-game-topup/order/inquiry           |
│                  6. forwardCallback() ──────────► RabbitMQ downstream (publish status)                    |
└───────────────────────────────────────────────────────────────────────────────────────────────────────────┘
    │
    ▼
Downstream consumer terima:
{"source":"PROVIDER","data":{"status_to_be":"F","serial_number":"REF-001",...}}
```

File yang perlu dibaca (urut):
1. `cmd/app/main.go` — cari `consumer.Start(ctx)`
2. `internal/infrastructure/rabbitmq/consumer_service.go` — `Start()` → `consumeSession()` → `processMessage()` → `processGame()`
3. `pkg/unipin/ingame_validate_user.go` — `ValidateUser()`
4. `pkg/unipin/ingame_create_order.go` — `CreateOrder()`
5. `internal/infrastructure/mqpublisher/publisher.go` — `Publish()` (kirim ke RabbitMQ downstream)

---

## Langkah 5: Alur Data — RabbitMQ Consumer (Voucher)

```
RabbitMQ Queue
    │
    │  {"product_type":"unipin-voucher","command":"STEAM*STEAM-100K",
    │   "msisdn":"08123456789","msgid":"TRX-002",...}
    │
    ▼
processMessage() → resolveTxType() → "GAME-VOUCHER" → processVoucher()
    │
    │  1. Parse Command "STEAM*STEAM-100K" → denominationCode = "STEAM-100K"
    │  2. VoucherRequest() ──────────────────► UniPin API: POST /voucher/request
    │  3. Jika timeout:
    │     VoucherInquiry() ──────────────────► UniPin API: POST /voucher/inquiry
    │  4. forwardCallback() ─────────────────► RabbitMQ downstream
    │
    ▼
Downstream consumer terima status
```

---

## Langkah 6: Alur Data — Cron Sync Game List ke Oracle

```
Cron Scheduler (setiap hari jam 01:00)
    │
    ▼
┌───────────────────────────────────────────────────────────────────────────────────────────────────────┐
│  sync_service.go — SyncGameList()                                                                     │
│                                                                                                       │
│  1. GameList() ─────────────────────────────────► UniPin API: POST /in-game-topup/list                |
│     ← daftar 100+ game                                                                                │
│  2. Untuk setiap game:                                                                                │
│     GameDetail(gameCode) ───────────────────────► UniPin API: POST /in-game-topup/detail              |
│     ← denominations + fields                                                                          │
│  3. Untuk setiap denomination:                                                                        │
│     repo.UpsertGameList(row) ───────────────────► Oracle SP: MSG.PKG_UNIPIN.INSUPDGAMELIST            |
│                                                                                                       │
│  SyncVoucherList() — sama tapi untuk voucher                                                          │
│  1. VoucherList() ──────────────────────────────► UniPin API: POST /voucher/list                      |
│  2. VoucherDetail(code) ────────────────────────► UniPin API: POST /voucher/details                   |
│  3. repo.UpsertVoucherList(row) ────────────────► Oracle SP: MSG.PKG_UNIPIN.INSUPDVOUCHERLIST         |
└───────────────────────────────────────────────────────────────────────────────────────────────────────┘
```

File yang perlu dibaca (urut):
1. `internal/infrastructure/scheduler/cron.go` — `AddGameSync()`
2. `internal/usecase/gamesync/sync_service.go` — `SyncGameList()`, `SyncVoucherList()`
3. `pkg/unipin/ingame_game_list.go` — `GameList()`
4. `internal/infrastructure/oracle/game_list_repository.go` — `UpsertGameList()`

---

## Langkah 7: Alur Data — API Logging ke Postgres

```
Setiap request ke UniPin API
    │
    ▼
┌───────────────────────────────────────────────────────────────────────────────────┐
│  logging_transport.go — RoundTrip()                                               │
│                                                                                   │
│  1. Catat request: endpoint, method, body                                         │
│  2. Kirim request ke UniPin (http.Do)                                             │
│  3. Catat response: status_code, body, durasi                                     │
│  4. Simpan ke Postgres (async) ─────────────────► Postgres: log_unipin.api_log    |
└───────────────────────────────────────────────────────────────────────────────────┘
```

---

## Konsep Penting untuk Pemula Go

### Interface (Kontrak)
```
domain/contract/repository/game_list_repository.go  ← "Janji" — apa yang bisa dilakukan
infrastructure/oracle/game_list_repository.go        ← "Implementasi" — cara melakukannya
```
Interface = kontrak. Implementasi = kode asli. Ini memudahkan testing (bisa diganti mock).

### Dependency Injection
Di `main.go`, semua komponen dibuat dan "disuntikkan" ke komponen lain:
```go
repo := oracle.NewGameListRepository(db)          // buat repo
syncSvc := gamesync.NewSyncService(client, repo)  // suntikkan repo ke service
```

### Goroutine
Program ini menjalankan 4 hal secara paralel (bersamaan):
```go
go func() { app.Listen(":8080") }()     // HTTP server
go func() { consumer.Start(ctx) }()     // RabbitMQ consumer
go func() { cronScheduler.Start() }()   // Cron scheduler
go func() { <-ctx.Done(); app.Shutdown() }() // Shutdown handler
```

### Context
`ctx` digunakan untuk membatalkan operasi. Kalau program mau shutdown, `ctx` di-cancel dan semua goroutine berhenti.

---

## Tips Membaca Kode

1. Selalu mulai dari `main.go`
2. Ikuti alur: main → handler/consumer → usecase → repository → database/API
3. Kalau ketemu interface, cari implementasinya di `infrastructure/`
4. Kalau ketemu `pkg/unipin/`, itu HTTP client ke API luar
5. Kalau bingung, cari test file (`_test.go`) — test menunjukkan cara pakai fungsi tersebut
