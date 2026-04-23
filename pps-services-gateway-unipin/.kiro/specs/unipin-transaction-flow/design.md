# Dokumen Desain: UniPin Transaction Flow

## Overview

Fitur ini mengimplementasikan alur transaksi lengkap untuk dua product type di `pps-services-gateway-unipin`:

1. **unipin-game** (baru) — Method `processGame` yang menangani alur in-game topup: parsing `Command` → parsing `MSISDN` sebagai JSON → ValidateUser API → CreateOrder API → fallback OrderInquiry saat timeout.
2. **unipin-voucher** (update) — Mengubah sumber `denominationCode` dari field `ProductCode` menjadi parsing dari field `Command` (format: `voucher_code*denomination_code`).

Perubahan utama:
- **Field baru**: `Command` pada struct `consumePayload` — di-parse dari JSON payload RabbitMQ
- **Method baru**: `processGame` pada `ConsumerServiceImpl` — menangani seluruh alur unipin-game
- **Modifikasi**: `processMessage` switch — tambah case `"unipin-game"` yang memanggil `processGame`
- **Modifikasi**: `processVoucher` — ganti sumber `denominationCode` dari `ProductCode` ke parsing `Command`

Prinsip desain utama: **kegagalan pada satu tahap alur tidak boleh menghentikan consumer**. Setiap error di-forward sebagai status `FAILED` ke downstream via `forwardCallback`, pesan RabbitMQ tetap di-ack, dan consumer melanjutkan pemrosesan pesan berikutnya.

Tidak ada perubahan pada `forwardCallback`, `MQPublisher`, `ProviderPublishMessage`, atau `main.go` — semua infrastruktur downstream sudah tersedia dan digunakan apa adanya.

---

## Architecture

### Alur unipin-game (Baru)

```
RabbitMQ Message (productType = "unipin-game")
   │
   ├─► processMessage() → switch "unipin-game" → processGame()
   │
   ├─► 1. Parse Command: "gamecode*denomination_id"
   │       ├─► Split by "*" → gameCode, denominationID
   │       └─► Validasi: Command tidak kosong, ada delimiter, kedua bagian tidak kosong
   │
   ├─► 2. Parse MSISDN: JSON string → map[string]any
   │       ├─► json.Unmarshal(MSISDN) → fields map
   │       └─► Validasi: MSISDN tidak kosong, JSON valid, map tidak kosong
   │
   ├─► 3. ValidateUser API
   │       ├─► Request: GameCode + Fields (dari MSISDN)
   │       ├─► Sukses (Status=1): simpan ValidationToken
   │       └─► Gagal/Error: forwardCallback FAILED, return
   │
   ├─► 4. CreateOrder API
   │       ├─► Request: GameCode + ValidationToken + ReferenceNo(msgID) + DenominationID
   │       ├─► Sukses (Status=1): forwardCallback SUCCESS
   │       ├─► Gagal (Status≠1): forwardCallback FAILED
   │       ├─► Timeout (DeadlineExceeded): fallback ke OrderInquiry
   │       └─► Error teknis lain: forwardCallback FAILED
   │
   └─► 5. OrderInquiry API (fallback)
           ├─► Request: ReferenceNo(msgID)
           ├─► Sukses (Status=1): forwardCallback SUCCESS
           ├─► Gagal (Status≠1): forwardCallback FAILED
           └─► Error: forwardCallback FAILED
```

### Alur unipin-voucher (Update)

```
RabbitMQ Message (productType = "unipin-voucher")
   │
   ├─► processMessage() → switch "unipin-voucher" → processVoucher()
   │
   ├─► 1. Parse Command: "voucher_code*denomination_code"  ← BERUBAH (sebelumnya: ProductCode langsung)
   │       ├─► Split by "*" → voucherCode, denominationCode
   │       └─► Validasi: Command tidak kosong, ada delimiter, denominationCode tidak kosong
   │
   ├─► 2. VoucherRequest API (TIDAK BERUBAH)
   │       ├─► Request: DenominationCode + Quantity(1) + ReferenceNo(msgID)
   │       ├─► Sukses: forwardCallback SUCCESS
   │       ├─► Timeout: fallback ke VoucherInquiry
   │       └─► Error: forwardCallback FAILED
   │
   └─► 3. VoucherInquiry API - fallback (TIDAK BERUBAH)
```

### Posisi Komponen dalam Codebase

```
internal/
  infrastructure/
    rabbitmq/
      consumer_service.go            ← MODIFIKASI:
                                        - Tambah field Command di consumePayload
                                        - Tambah case "unipin-game" di processMessage
                                        - Tambah method processGame (baru)
                                        - Tambah method processOrderInquiry (baru)
                                        - Update processVoucher (parsing Command)
```

---

## Components and Interfaces

### Perubahan Struct `consumePayload`

Tambah field `Command` pada struct `consumePayload`:

```go
type consumePayload struct {
    Amount        int
    StockType     string
    ProductCode   string
    ProductID     string
    ProductType   string
    MID           string
    StoreID       string
    QueueName     string
    MSISDN        string
    MsgID         string
    CallbackURL   string
    MQTransaction string
    Command       string    // BARU
}
```

Pada method `UnmarshalJSON`, tambahkan parsing `Command`:

```go
p.Command = parseString(getAny(raw, "command"))
```

Mengikuti pola field lain: case-insensitive lookup via `getAny`, lalu `parseString` yang sudah melakukan `strings.TrimSpace`.

### Perubahan `processMessage` Switch

```go
func (s *ConsumerServiceImpl) processMessage(ctx context.Context, delivery *amqp.Delivery) {
    // ... (parsing payload, queueName, msgID — tidak berubah)

    productType := strings.ToLower(strings.TrimSpace(payload.ProductType))
    switch productType {
    case "unipin-voucher":
        s.processVoucher(ctx, &payload, queueName, msgID)
    case "unipin-game":                                    // BARU
        s.processGame(ctx, &payload, queueName, msgID)     // BARU
    default:
        s.logger.Warn("unsupported product type", ...)
    }
}
```

### Method Baru: `processGame`

```go
func (s *ConsumerServiceImpl) processGame(ctx context.Context, payload *consumePayload, queueName, msgID string) {
    // 1. Parse Command → gameCode, denominationID
    // 2. Parse MSISDN → fields map[string]any
    // 3. ValidateUser(gameCode, fields) → validationToken
    // 4. CreateOrder(gameCode, validationToken, msgID, denominationID)
    //    - Sukses → forwardCallback SUCCESS
    //    - Gagal → forwardCallback FAILED
    //    - Timeout → processOrderInquiry(msgID)
    //    - Error lain → forwardCallback FAILED
}
```

### Method Baru: `processOrderInquiry`

```go
func (s *ConsumerServiceImpl) processOrderInquiry(ctx context.Context, payload *consumePayload, queueName, msgID, referenceNo string) {
    // OrderInquiry(referenceNo)
    // - Sukses → forwardCallback SUCCESS
    // - Gagal → forwardCallback FAILED
    // - Error → forwardCallback FAILED
}
```

Pola ini mengikuti `processVoucherInquiry` yang sudah ada — method terpisah untuk fallback inquiry.

### Update `processVoucher`

Perubahan pada bagian awal method — ganti sumber `denominationCode`:

```go
func (s *ConsumerServiceImpl) processVoucher(ctx context.Context, payload *consumePayload, queueName, msgID string) {
    // SEBELUM:
    // denominationCode := strings.TrimSpace(payload.ProductCode)

    // SESUDAH:
    // Parse Command: "voucher_code*denomination_code"
    parts := strings.SplitN(payload.Command, "*", 2)
    if len(parts) < 2 {
        // log error, forwardCallback FAILED, return
    }
    denominationCode := strings.TrimSpace(parts[1])
    if denominationCode == "" {
        // log error, forwardCallback FAILED, return
    }

    // ... sisa alur tidak berubah (VoucherRequest → timeout fallback → VoucherInquiry)
}
```

### Helper: Parsing Command

Kedua alur (game dan voucher) menggunakan pola parsing yang sama: `strings.SplitN(command, "*", 2)`. Tidak perlu helper function terpisah karena validasi setelah split berbeda untuk masing-masing alur (game memvalidasi kedua bagian, voucher hanya memvalidasi bagian kedua).

### Helper: Parsing MSISDN sebagai JSON

Khusus untuk alur unipin-game, MSISDN berisi JSON string yang perlu di-unmarshal:

```go
var fields map[string]any
if err := json.Unmarshal([]byte(payload.MSISDN), &fields); err != nil {
    // log error, forwardCallback FAILED, return
}
if len(fields) == 0 {
    // log error, forwardCallback FAILED, return
}
```

### `forwardCallback` — Tidak Berubah

Method `forwardCallback` yang sudah ada digunakan apa adanya oleh `processGame` dan `processOrderInquiry`. Tidak ada perubahan pada signature atau behavior-nya.

---

## Data Models

### Format Command

| Product Type | Format Command | Contoh | Bagian 1 | Bagian 2 |
|---|---|---|---|---|
| `unipin-game` | `gamecode*denomination_id` | `MLBB*123` | `gameCode` = `MLBB` | `denominationID` = `123` |
| `unipin-voucher` | `voucher_code*denomination_code` | `STEAM*STEAM-50K` | `voucherCode` = `STEAM` | `denominationCode` = `STEAM-50K` |

### Format MSISDN untuk unipin-game

MSISDN berisi JSON string (bukan nomor telepon):

```json
"{\"userid\":\"123456\",\"zone\":\"ID\"}"
```

Setelah `json.Unmarshal`:

```go
map[string]any{
    "userid": "123456",
    "zone":   "ID",
}
```

Map ini langsung digunakan sebagai `Fields` pada `ValidateUserRequest`.

### Mapping: processGame → forwardCallback

| Tahap | Kondisi | `statusToBe` | `serialNumber` | `message` |
|---|---|---|---|---|
| Parse Command gagal | Command kosong / tidak ada `*` / bagian kosong | `FAILED` | `""` | Deskripsi error parsing |
| Parse MSISDN gagal | MSISDN kosong / JSON invalid / map kosong | `FAILED` | `""` | Deskripsi error parsing |
| ValidateUser sukses | `Status = 1` | — (lanjut ke CreateOrder) | — | — |
| ValidateUser gagal | `Status ≠ 1` | `FAILED` | `""` | `resp.Reason` |
| ValidateUser error teknis | `TechnicalError` | `FAILED` | `""` | `err.Error()` |
| CreateOrder sukses | `Status = 1` | `SUCCESS` | `resp.ReferenceNo` | `resp.Reason` |
| CreateOrder gagal | `Status ≠ 1` | `FAILED` | `resp.ReferenceNo` | `resp.Reason` |
| CreateOrder timeout | `TechnicalError` + `DeadlineExceeded` | — (fallback ke OrderInquiry) | — | — |
| CreateOrder error teknis | `TechnicalError` (bukan timeout) | `FAILED` | `""` | `err.Error()` |
| OrderInquiry sukses | `Status = 1` | `SUCCESS` | `resp.ReferenceNo` | `resp.Reason` |
| OrderInquiry gagal | `Status ≠ 1` | `FAILED` | `resp.ReferenceNo` | `resp.Reason` |
| OrderInquiry error | Any error | `FAILED` | `referenceNo` | `err.Error()` |

### Mapping: processVoucher (Update)

Perubahan hanya pada sumber `denominationCode`:

| Sebelum | Sesudah |
|---|---|
| `denominationCode = payload.ProductCode` | `denominationCode = parts[1]` dari `strings.SplitN(payload.Command, "*", 2)` |

Sisa mapping (VoucherRequest → VoucherInquiry → forwardCallback) tidak berubah.

### Contoh Payload RabbitMQ (unipin-game)

```json
{
  "amount": 50000,
  "product_type": "unipin-game",
  "command": "MLBB*123",
  "msisdn": "{\"userid\":\"789012\",\"zone\":\"ID\"}",
  "msgid": "TRX-20250101-001",
  "mq_transaction": "amqp://guest:guest@localhost:5672/",
  "queue_name": "pps.gateway.queue.staging.unipin",
  "mid": "MID001",
  "store_id": "STORE001"
}
```

### Contoh Payload RabbitMQ (unipin-voucher)

```json
{
  "amount": 100000,
  "product_type": "unipin-voucher",
  "command": "STEAM*STEAM-100K",
  "msisdn": "08123456789",
  "msgid": "TRX-20250101-002",
  "mq_transaction": "amqp://guest:guest@localhost:5672/",
  "queue_name": "pps.gateway.queue.staging.unipin",
  "mid": "MID002",
  "store_id": "STORE002"
}
```


---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Parsing Command dengan Delimiter Mengekstrak Bagian yang Benar

*For any* string `command` yang mengandung tepat satu karakter `*`, `strings.SplitN(command, "*", 2)` harus menghasilkan dua bagian di mana bagian pertama sama dengan substring sebelum `*` pertama dan bagian kedua sama dengan substring setelah `*` pertama (termasuk jika ada `*` tambahan di bagian kedua). Setelah `strings.TrimSpace` pada masing-masing bagian, hasilnya harus konsisten — yaitu `TrimSpace(part1) + "*" + TrimSpace(part2)` harus merekonstruksi command yang di-trim dengan benar.

**Validates: Requirements 1.2, 1.3, 2.1, 8.1, 8.2**

### Property 2: MSISDN JSON Round-Trip Mempertahankan Fields

*For any* `map[string]any` yang valid (dengan key string dan value string), setelah di-`json.Marshal` menjadi JSON string dan kemudian di-`json.Unmarshal` kembali ke `map[string]any`, hasilnya harus identik dengan input asli. Map ini harus sampai ke `ValidateUserRequest.Fields` tanpa modifikasi.

**Validates: Requirements 3.1, 3.4, 4.1**

### Property 3: Parameter CreateOrder Terpetakan dengan Benar dari Input

*For any* kombinasi `gameCode`, `denominationID` (dari parsing Command), `validationToken` (dari respons ValidateUser), dan `msgID` (dari payload), `CreateOrderRequest` yang dikirim ke UniPin API harus memiliki: `GameCode` = `gameCode`, `ValidationToken` = `validationToken`, `ReferenceNo` = `msgID`, dan `DenominationID` = `denominationID`.

**Validates: Requirements 5.1**

### Property 4: Setiap Error Menghasilkan Forward FAILED tanpa Menghentikan Consumer

*For any* error yang terjadi pada tahap apapun dalam alur unipin-game (parsing Command gagal, parsing MSISDN gagal, ValidateUser error, CreateOrder error non-timeout, OrderInquiry error), consumer harus selalu memanggil `forwardCallback` dengan `statusToBe = "FAILED"` dan tidak boleh panic atau menghentikan pemrosesan pesan berikutnya.

**Validates: Requirements 10.1**

---

## Error Handling

### Prinsip Non-Blocking

Semua error pada alur unipin-game dan unipin-voucher bersifat non-blocking terhadap consumer:
1. Error parsing (Command, MSISDN) → log error, `forwardCallback` FAILED, return dari method.
2. Error API (ValidateUser, CreateOrder, OrderInquiry, VoucherRequest) → log error, `forwardCallback` FAILED, return.
3. Timeout CreateOrder → log warn, fallback ke OrderInquiry (bukan langsung FAILED).
4. `forwardCallback` gagal publish → log error, lanjut (sudah ditangani oleh `forwardCallback` existing).
5. Consumer tetap ack pesan RabbitMQ dan melanjutkan ke pesan berikutnya.

### Tabel Error Handling — processGame

| Tahap | Kondisi Error | Behavior |
|---|---|---|
| Parse Command | Command kosong | Log error, forwardCallback FAILED, return |
| Parse Command | Tidak ada delimiter `*` | Log error, forwardCallback FAILED, return |
| Parse Command | gameCode kosong setelah trim | Log error, forwardCallback FAILED, return |
| Parse Command | denominationID kosong setelah trim | Log error, forwardCallback FAILED, return |
| Parse MSISDN | MSISDN kosong | Log error, forwardCallback FAILED, return |
| Parse MSISDN | JSON invalid | Log error, forwardCallback FAILED, return |
| Parse MSISDN | Map kosong (`len(fields) == 0`) | Log error, forwardCallback FAILED, return |
| ValidateUser | `Status ≠ 1` (BusinessError) | Log error, forwardCallback FAILED (message = Reason), return |
| ValidateUser | TechnicalError | Log error, forwardCallback FAILED (message = err.Error()), return |
| CreateOrder | `Status ≠ 1` (BusinessError) | forwardCallback FAILED (serialNumber = ReferenceNo, message = Reason) |
| CreateOrder | TechnicalError + DeadlineExceeded | Log warn, fallback ke processOrderInquiry |
| CreateOrder | TechnicalError (bukan timeout) | forwardCallback FAILED (message = err.Error()) |
| OrderInquiry | `Status ≠ 1` (BusinessError) | forwardCallback FAILED (serialNumber = ReferenceNo, message = Reason) |
| OrderInquiry | Any error | forwardCallback FAILED (message = err.Error()) |

### Tabel Error Handling — processVoucher (Update)

| Tahap | Kondisi Error | Behavior |
|---|---|---|
| Parse Command | Command kosong | Log error, forwardCallback FAILED, return |
| Parse Command | Tidak ada delimiter `*` | Log error, forwardCallback FAILED, return |
| Parse Command | denominationCode kosong setelah trim | Log error, forwardCallback FAILED, return |
| VoucherRequest | (tidak berubah dari existing) | — |
| VoucherInquiry | (tidak berubah dari existing) | — |

### Timeout Handling Pattern

Pola deteksi timeout mengikuti pattern yang sudah ada di `processVoucher`:

```go
var techErr *unipin.TechnicalError
if errors.As(err, &techErr) && errors.Is(techErr.Cause, context.DeadlineExceeded) {
    // fallback ke inquiry
}
```

Pattern ini digunakan di:
- `processGame` → CreateOrder timeout → fallback ke `processOrderInquiry`
- `processVoucher` → VoucherRequest timeout → fallback ke `processVoucherInquiry` (existing, tidak berubah)

---

## Testing Strategy

### Pendekatan Dual Testing

Fitur ini menggunakan dua lapisan testing:

1. **Unit test** — menggunakan mock `unipin.Client` dan mock `MQPublisher` untuk memverifikasi alur `processGame`, `processVoucher` (update), dan parsing logic.
2. **Property-based test** — menggunakan generated input untuk memverifikasi correctness properties: parsing Command, MSISDN round-trip, parameter mapping, dan fault tolerance.

### Library Property-Based Testing

Gunakan **`testing/quick`** (stdlib Go) untuk property-based testing. Setiap property test dikonfigurasi minimum **100 iterasi**.

### Tag Format

Setiap property test diberi komentar:
```go
// Feature: unipin-transaction-flow, Property 1: Parsing Command dengan Delimiter Mengekstrak Bagian yang Benar
```

### Unit Test

```
internal/infrastructure/rabbitmq/consumer_service_test.go

  // --- Parsing ---
  - TestConsumePayload_CommandField_Parsed                  (Req 1.1, 1.2 — example)
  - TestParseCommand_Delimiter_ExtractsCorrectParts         (Property 1 — PBT)
  - TestParseMSISDN_JSON_RoundTrip                          (Property 2 — PBT)

  // --- processGame: happy path ---
  - TestProcessGame_HappyPath_Success                       (Req 4.1, 5.1, 5.2 — example)

  // --- processGame: parsing errors ---
  - TestProcessGame_EmptyCommand_ForwardsFailed             (Req 2.2 — example)
  - TestProcessGame_NoDelimiter_ForwardsFailed              (Req 2.2 — example)
  - TestProcessGame_EmptyGameCode_ForwardsFailed            (Req 2.3 — example)
  - TestProcessGame_EmptyDenominationID_ForwardsFailed      (Req 2.4 — example)
  - TestProcessGame_EmptyMSISDN_ForwardsFailed              (Req 3.2 — example)
  - TestProcessGame_InvalidMSISDNJSON_ForwardsFailed        (Req 3.2 — example)
  - TestProcessGame_EmptyMSISDNMap_ForwardsFailed           (Req 3.3 — example)

  // --- processGame: API errors ---
  - TestProcessGame_ValidateUserFailed_ForwardsFailed       (Req 4.3 — example)
  - TestProcessGame_ValidateUserTechError_ForwardsFailed    (Req 4.4 — example)
  - TestProcessGame_CreateOrderFailed_ForwardsFailed        (Req 5.3 — example)
  - TestProcessGame_CreateOrderTimeout_FallbackInquiry      (Req 5.4 — example)
  - TestProcessGame_CreateOrderTechError_ForwardsFailed     (Req 5.5 — example)
  - TestProcessGame_OrderInquirySuccess_ForwardsSuccess     (Req 6.2 — example)
  - TestProcessGame_OrderInquiryFailed_ForwardsFailed       (Req 6.3 — example)
  - TestProcessGame_OrderInquiryError_ForwardsFailed        (Req 6.4 — example)

  // --- processGame: parameter mapping ---
  - TestProcessGame_CreateOrderParams_MappedCorrectly       (Property 3 — PBT)

  // --- processGame: fault tolerance ---
  - TestProcessGame_AnyError_AlwaysForwardsFailed           (Property 4 — PBT)

  // --- processVoucher: update ---
  - TestProcessVoucher_ParsesCommandForDenominationCode     (Req 8.1, 8.2 — example)
  - TestProcessVoucher_EmptyCommand_ForwardsFailed          (Req 8.3 — example)
  - TestProcessVoucher_EmptyDenominationCode_ForwardsFailed (Req 8.4 — example)
  - TestProcessVoucher_FullFlow_Preserved                   (Req 8.5 — example)

  // --- processMessage: routing ---
  - TestProcessMessage_UnipinGame_RoutesToProcessGame       (Req 7.1, 7.2 — example)

  // --- logging ---
  - TestProcessGame_LogsAtEachStage                         (Req 9.1-9.7 — example)
```

### Catatan Testing

- UniPin client perlu di-mock via interface atau dependency injection. Saat ini `ConsumerServiceImpl` menyimpan `*unipin.Client` secara langsung. Untuk testability, perlu dipertimbangkan apakah akan menggunakan interface atau test helper yang mengganti method pada client.
- `forwardCallback` sudah teruji melalui alur voucher yang ada. Test baru fokus pada alur `processGame` dan perubahan parsing di `processVoucher`.
- Property test untuk fault tolerance (Property 4) menggunakan table-driven test dengan generated error conditions.
