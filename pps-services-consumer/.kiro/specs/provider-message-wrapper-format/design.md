# Dokumen Desain: Provider Message Wrapper Format

## Overview

Fitur ini mengubah fungsi `processProvider()` di `consumerTunning.go` agar dapat mem-parsing pesan PROVIDER dalam dua format:

1. **Format Wrapper (baru)**: `{"source": "PROVIDER", "data": {"msg_id": ..., "status_to_be": ..., ...}}`
2. **Format Flat (lama)**: `{"source": "PROVIDER", "msg_id": ..., "status_to_be": ..., ...}`

Komponen yang berubah:
- **Struct `ProviderWrapperMessage`** (baru) di `model/model.go` — representasi format wrapper
- **Fungsi `processProvider()`** di `repository/consumerTunning.go` — auto-deteksi format dan parsing
- **Fungsi helper `parseProviderMessage()`** (baru) — logika parsing yang bisa di-unit-test secara terpisah

Alur deteksi: coba unmarshal ke `ProviderWrapperMessage` terlebih dahulu. Jika field `Data` valid (cek `MsgId != 0` atau field lain terisi), gunakan `Data` sebagai `ProviderMessage`. Jika tidak, fallback ke unmarshal langsung sebagai `ProviderMessage` (format flat).

Prinsip desain: **backward compatibility wajib** — selama masa transisi, kedua format harus berjalan tanpa konfigurasi manual atau feature flag.

---

## Architecture

### Alur Parsing Pesan PROVIDER

```
RabbitMQ Message
   |
   v
ConsumerFIFO → ProsesDataFIFO(data)
   |
   |-- extractSource(data) → "PROVIDER"
   |
   v
processProvider(data)
   |
   v
parseProviderMessage(data)
   |
   |-- 1. Coba unmarshal ke ProviderWrapperMessage
   |       Jika Data.MsgId != 0 → format wrapper terdeteksi
   |       Return Data (ProviderMessage), "wrapper", nil
   |
   |-- 2. Fallback: unmarshal ke ProviderMessage langsung
   |       Jika berhasil → format flat terdeteksi
   |       Return ProviderMessage, "flat", nil
   |
   |-- 3. Kedua gagal → return error
   |
   v
processProvider lanjut:
   |-- Log format yang terdeteksi + msg_id
   |-- SetTransactionStatus(provMsg)
   |-- Return resultOK atau resultNackDiscard
```

### Posisi Komponen dalam Codebase

```
model/
  model.go                    ← MODIFIKASI: tambah struct ProviderWrapperMessage
  model_test.go               ← MODIFIKASI: tambah test untuk ProviderWrapperMessage
repository/
  consumerTunning.go          ← MODIFIKASI: update processProvider(), tambah parseProviderMessage()
  consumerTunning_test.go     ← BARU: unit test + property test untuk parseProviderMessage
```

Perubahan sangat minimal — hanya 2 file yang dimodifikasi, 1 file test baru.

---

## Components and Interfaces

### Struct `ProviderWrapperMessage` (Baru)

Ditambahkan di `model/model.go`:

```go
// ProviderWrapperMessage adalah format wrapper untuk pesan PROVIDER dari RabbitMQ.
// Format: {"source": "PROVIDER", "data": {"msg_id": ..., ...}}
type ProviderWrapperMessage struct {
    Source string          `json:"source"`
    Data   ProviderMessage `json:"data"`
}
```

### Fungsi `parseProviderMessage` (Baru)

Ditambahkan di `repository/consumerTunning.go`:

```go
// parseProviderMessage mem-parsing JSON pesan PROVIDER dengan auto-deteksi format.
// Mengembalikan ProviderMessage, format yang terdeteksi ("wrapper" atau "flat"), dan error.
//
// Logika deteksi:
//   1. Coba unmarshal ke ProviderWrapperMessage
//   2. Jika Data.MsgId != 0 → wrapper format, return Data
//   3. Jika tidak → coba unmarshal langsung ke ProviderMessage (flat format)
//   4. Jika keduanya gagal → return error
func parseProviderMessage(data string) (model.ProviderMessage, string, error)
```

### Fungsi `processProvider` (Dimodifikasi)

Perubahan pada `repository/consumerTunning.go`:

```go
// processProvider menangani pesan PROVIDER: parse ProviderMessage + SetTransactionStatus.
// Mendukung dua format: wrapper {"source":"PROVIDER","data":{...}} dan flat {...}.
func processProvider(data string) sourceResult {
    provMsg, format, err := parseProviderMessage(data)
    if err != nil {
        log.Println("Error parse ProviderMessage => " + err.Error() + " data: " + data)
        return resultNackDiscard
    }

    log.Printf("Processing PROVIDER message (%s format), msg_id: %d", format, provMsg.MsgId)

    outError, outMessage := SetTransactionStatus(provMsg)
    if outError != 0 {
        log.Printf("SetTransactionStatus warning, msg_id: %d, outError: %d, outMessage: %s",
            provMsg.MsgId, outError, outMessage)
    }

    return resultOK
}
```

---

## Data Models

### Struct yang Sudah Ada (Tidak Berubah)

`ProviderMessage` di `model/model.go` tetap sama — 11 field dengan JSON tag yang sudah ada:

| Field | JSON Tag | Tipe |
|---|---|---|
| MsgId | `msg_id` | int |
| StatusToBe | `status_to_be` | string |
| SerialNumber | `serial_number` | string |
| ClientNumber | `client_number` | string |
| Nominal | `nominal` | string |
| OriginalConversationID | `original_conversation_id` | string |
| ConversationID | `conversation_id` | string |
| MessageToCustomer | `message_to_customer` | string |
| AdditionalMessage | `additional_message` | string |
| QueueName | `queue_name` | string |
| Source | `source` | string |

### Struct Baru

`ProviderWrapperMessage` — envelope untuk format wrapper:

| Field | JSON Tag | Tipe | Keterangan |
|---|---|---|---|
| Source | `source` | string | Selalu `"PROVIDER"` |
| Data | `data` | ProviderMessage | Payload pesan PROVIDER (tanpa field `source`) |

### Contoh JSON: Format Wrapper vs Flat

**Format Wrapper (baru):**
```json
{
  "source": "PROVIDER",
  "data": {
    "msg_id": 12345,
    "status_to_be": "SUCCESS",
    "serial_number": "SN123",
    "client_number": "08123456789",
    "nominal": "50000",
    "original_conversation_id": "ORIG001",
    "conversation_id": "CONV001",
    "message_to_customer": "Transaksi berhasil",
    "additional_message": "Pulsa telah dikirim",
    "queue_name": "telkomsel-queue"
  }
}
```

**Format Flat (lama):**
```json
{
  "source": "PROVIDER",
  "msg_id": 12345,
  "status_to_be": "SUCCESS",
  "serial_number": "SN123",
  "client_number": "08123456789",
  "nominal": "50000",
  "original_conversation_id": "ORIG001",
  "conversation_id": "CONV001",
  "message_to_customer": "Transaksi berhasil",
  "additional_message": "Pulsa telah dikirim",
  "queue_name": "telkomsel-queue"
}
```

### Catatan: Field `source` pada Format Wrapper

Pada format wrapper, field `source` ada di level root (dibaca oleh `extractSource()`), sedangkan field-field ProviderMessage ada di dalam `data`. Field `source` di dalam `ProviderMessage.Source` akan kosong pada format wrapper — ini tidak masalah karena `SetTransactionStatus` tidak menggunakan field `Source` dari `ProviderMessage`.

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Round-Trip Serialization ProviderWrapperMessage

*For any* `ProviderWrapperMessage` yang valid, marshal ke JSON lalu unmarshal kembali harus menghasilkan struct yang identik dengan struct asli.

**Validates: Requirements 1.3**

### Property 2: Ekuivalensi Format — Wrapper dan Flat Menghasilkan ProviderMessage Identik

*For any* data ProviderMessage yang valid (10 field: msg_id, status_to_be, serial_number, client_number, nominal, original_conversation_id, conversation_id, message_to_customer, additional_message, queue_name), jika data tersebut di-encode sebagai format wrapper dan juga sebagai format flat, maka `parseProviderMessage` harus menghasilkan `ProviderMessage` yang identik untuk kedua format (kecuali field `Source` yang hanya terisi pada format flat).

**Validates: Requirements 6.3, 6.1, 6.2, 2.1, 3.1**

### Property 3: Auto-Deteksi Format yang Benar

*For any* data ProviderMessage yang valid, jika di-encode sebagai format wrapper maka `parseProviderMessage` harus mengembalikan format `"wrapper"`, dan jika di-encode sebagai format flat maka harus mengembalikan format `"flat"`.

**Validates: Requirements 4.1, 4.2, 4.3**

### Property 4: Data Wrapper Invalid Menghasilkan Error

*For any* JSON string yang memiliki `"source": "PROVIDER"` tetapi field `data` berisi nilai non-objek (string, number, array, null) atau JSON yang sepenuhnya invalid, `parseProviderMessage` harus mengembalikan error atau fallback ke flat parsing yang juga gagal.

**Validates: Requirements 2.3, 3.3**

---

## Error Handling

### Tabel Error Handling

| Kondisi | Behavior | Return Value |
|---|---|---|
| JSON wrapper valid, Data lengkap | Parse sebagai wrapper, lanjut ke SetTransactionStatus | `resultOK` |
| JSON flat valid, field lengkap | Parse sebagai flat, lanjut ke SetTransactionStatus | `resultOK` |
| JSON valid tapi `data` kosong/invalid, flat juga gagal | Log error + data asli | `resultNackDiscard` |
| JSON tidak valid sama sekali | Log error + data asli | `resultNackDiscard` |
| SetTransactionStatus return outError != 0 | Log warning, tetap Ack | `resultOK` |

### Strategi Deteksi Format

Deteksi menggunakan pendekatan "try wrapper first, fallback to flat":

1. **Unmarshal ke `ProviderWrapperMessage`** — jika berhasil DAN `Data.MsgId != 0`, ini format wrapper
2. **Fallback unmarshal ke `ProviderMessage`** — jika berhasil, ini format flat
3. **Keduanya gagal** — return error, pesan di-nack tanpa requeue

Alasan menggunakan `MsgId != 0` sebagai indikator: pada format flat, unmarshal ke `ProviderWrapperMessage` akan berhasil secara teknis (JSON valid), tetapi field `Data` akan berisi zero-value struct karena tidak ada key `data` di JSON. Dengan mengecek `MsgId != 0`, kita bisa membedakan wrapper yang benar dari flat yang "kebetulan" berhasil di-unmarshal.

---

## Testing Strategy

### Pendekatan Dual Testing

Fitur ini menggunakan dua lapisan testing:

1. **Unit test** — memverifikasi behavior spesifik dengan contoh konkret (happy path, edge case, error handling)
2. **Property-based test** — memverifikasi correctness properties di atas dengan generated input untuk cakupan yang lebih luas

### Library Property-Based Testing

Gunakan **`testing/quick`** (stdlib Go) untuk property-based testing. Setiap property test dikonfigurasi minimum **100 iterasi**.

### Tag Format

Setiap property test diberi komentar:
```go
// Feature: provider-message-wrapper-format, Property N: [judul property]
```

### Test Plan

```
model/model_test.go (tambahan)
  - TestProviderWrapperMessageJSONUnmarshal                (Req 1.1, 1.2 — example)
  - TestProviderWrapperMessageJSONMarshal                  (Req 1.1 — example)
  - TestProviderWrapperMessageRoundTrip                    (Property 1 — PBT)

repository/consumerTunning_test.go (baru)
  - TestParseProviderMessage_WrapperFormat                 (Req 2.1 — example)
  - TestParseProviderMessage_FlatFormat                    (Req 3.1 — example)
  - TestParseProviderMessage_InvalidJSON_ReturnsError      (Req 3.3 — example)
  - TestParseProviderMessage_EmptyDataField_FallbackFlat   (Req 4.3 — example)
  - TestParseProviderMessage_FormatEquivalence             (Property 2 — PBT)
  - TestParseProviderMessage_AutoDetectFormat              (Property 3 — PBT)
  - TestParseProviderMessage_InvalidWrapperData            (Property 4 — PBT)
  - TestProcessProvider_WrapperFormat_LogsWrapper          (Req 5.1 — example)
  - TestProcessProvider_FlatFormat_LogsFlat                (Req 5.2 — example)
  - TestProcessProvider_InvalidJSON_LogsError              (Req 5.3 — example)
```

### Dependency

Tidak ada dependency baru. Menggunakan:
- `encoding/json` (stdlib)
- `testing/quick` (stdlib)
- `github.com/stretchr/testify` (sudah ada di go.mod)
