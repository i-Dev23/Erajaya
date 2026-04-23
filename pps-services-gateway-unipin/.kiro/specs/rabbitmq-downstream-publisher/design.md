# Dokumen Desain: RabbitMQ Downstream Publisher (Gateway Unipin)

## Overview

Fitur ini menggantikan `DownstreamClient` (HTTP POST ke `pps-services-publisher-database`) dengan komponen `MQPublisher` yang mempublikasikan status akhir transaksi langsung ke RabbitMQ. Setiap pesan transaksi dari `pps-services-publisher-provider` sudah membawa field `MQTransaction` (URL RabbitMQ tujuan) dan `QueueName` (nama queue tujuan), sehingga gateway dapat mempublikasikan langsung tanpa perantara HTTP.

Perubahan utama:
- **Komponen baru**: `MQPublisher` — interface + implementasi menggunakan `amqp091-go`
- **Struct baru**: `ProviderPublishMessage` — wrapper `{"source": "PROVIDER", "data": {...}}`
- **Modifikasi**: `consumePayload` — tambah field `MQTransaction`
- **Modifikasi**: `forwardCallback` — ganti `DownstreamClient.ForwardToPublisher()` dengan `MQPublisher.Publish()`
- **Modifikasi**: `main.go` — inisialisasi `MQPublisher`, hapus `DownstreamClient`
- **Hapus**: `internal/infrastructure/downstream/client.go`, `DownstreamConfig`, `LoadDownstream()`

Prinsip desain utama: **kegagalan publish ke RabbitMQ tujuan tidak boleh mengganggu alur utama**. Jika RabbitMQ tujuan tidak tersedia, Consumer tetap memproses pesan, memanggil Unipin API, dan melanjutkan pemrosesan secara normal.

Koneksi RabbitMQ dibuka baru untuk setiap operasi publish karena setiap transaksi dapat memiliki `MQTransaction` URL yang berbeda — tidak ada connection pooling.

Catatan: gateway-unipin lebih sederhana dari gateway-telkomsel karena tidak memiliki `CallbackHandler` — semua downstream call sudah terpusat di helper method `forwardCallback`.

---

## Architecture

### Alur Publish (Menggantikan DownstreamClient)

```
ConsumerServiceImpl
   │
   ├─► Proses transaksi (Unipin VoucherRequest / VoucherInquiry)
   │
   ├─► forwardCallback(ctx, payload, queueName, msgID, statusToBe, serialNumber, message)
   │       │
   │       ├─► Bangun ProviderPublishMessage
   │       │       {"source": "PROVIDER", "data": {...}}
   │       │
   │       ├─► json.Marshal(message) → []byte
   │       │
   │       └─► MQPublisher.Publish(ctx, payload.MQTransaction, queueName, body)
   │               │
   │               ├─► Validasi: mqTransactionURL dan queueName tidak kosong
   │               ├─► amqp.Dial(mqTransactionURL) → conn (baru setiap publish)
   │               ├─► conn.Channel() → ch
   │               ├─► ch.QueueDeclare(queueName, ...)
   │               ├─► ch.PublishWithContext(ctx, ..., body)
   │               └─► defer conn.Close(), ch.Close()
   │
   └─► Ack pesan RabbitMQ sumber (terlepas dari hasil publish)
```

### Perbandingan Sebelum vs Sesudah

```
SEBELUM:
  Consumer → forwardCallback()
               → DownstreamClient.ForwardToPublisher()
               → HTTP POST /api/callback → pps-services-publisher-database
               → (circuit breaker + exponential backoff)

SESUDAH:
  Consumer → forwardCallback()
               → MQPublisher.Publish(ctx, mqTransactionURL, queueName, body)
               → amqp.Dial(mqTransactionURL)
               → ch.PublishWithContext(ctx, "", queueName, body)
               → (fire-and-forget, error di-log saja)
```

### Posisi Komponen dalam Codebase

```
internal/
  domain/
    contract/
      service/
        mq_publisher.go              ← BARU: interface MQPublisher
  infrastructure/
    mqpublisher/
      publisher.go                   ← BARU: implementasi AMQPPublisher
      publisher_test.go              ← BARU: unit test
      message.go                     ← BARU: struct ProviderPublishMessage
    rabbitmq/
      consumer_service.go            ← MODIFIKASI: tambah MQTransaction di consumePayload,
                                                    ganti DownstreamClient → MQPublisher di forwardCallback
    downstream/
      client.go                      ← HAPUS
  config/
    config.go                        ← MODIFIKASI: hapus DownstreamConfig + LoadDownstream()
cmd/
  app/
    main.go                          ← MODIFIKASI: inisialisasi MQPublisher, hapus DownstreamClient
```

---

## Components and Interfaces

### Interface `MQPublisher`

Ditempatkan di `internal/domain/contract/service/mq_publisher.go`:

```go
package service

import "context"

// MQPublisher mendefinisikan kontrak untuk mempublikasikan pesan ke RabbitMQ.
// Setiap pemanggilan Publish membuka koneksi baru ke mqTransactionURL
// karena setiap transaksi dapat memiliki URL RabbitMQ yang berbeda.
type MQPublisher interface {
    // Publish mempublikasikan body ke queue queueName pada RabbitMQ di mqTransactionURL.
    // Mengembalikan error jika parameter tidak valid, koneksi gagal, atau publish gagal.
    Publish(ctx context.Context, mqTransactionURL string, queueName string, body []byte) error
}
```

### Implementasi `AMQPPublisher`

Ditempatkan di `internal/infrastructure/mqpublisher/publisher.go`:

```go
package mqpublisher

import (
    "context"
    "fmt"
    "strings"

    amqp "github.com/rabbitmq/amqp091-go"
    contractsvc "pps-services-gateway-unipin/internal/domain/contract/service"
)

// AMQPPublisher mengimplementasikan MQPublisher menggunakan amqp091-go.
type AMQPPublisher struct {
    logger contractsvc.Logger
}

// NewAMQPPublisher membuat instance baru AMQPPublisher.
func NewAMQPPublisher(logger contractsvc.Logger) *AMQPPublisher {
    return &AMQPPublisher{logger: logger}
}

// Publish membuka koneksi baru ke mqTransactionURL, declare queue, dan publish body.
// Koneksi dan channel ditutup setelah publish selesai.
func (p *AMQPPublisher) Publish(ctx context.Context, mqTransactionURL string, queueName string, body []byte) error {
    if strings.TrimSpace(mqTransactionURL) == "" {
        return fmt.Errorf("mq_transaction URL is empty")
    }
    if strings.TrimSpace(queueName) == "" {
        return fmt.Errorf("queue name is empty")
    }

    conn, err := amqp.Dial(mqTransactionURL)
    if err != nil {
        return fmt.Errorf("dial rabbitmq %s: %w", mqTransactionURL, err)
    }
    defer conn.Close()

    ch, err := conn.Channel()
    if err != nil {
        return fmt.Errorf("open channel: %w", err)
    }
    defer ch.Close()

    _, err = ch.QueueDeclare(queueName, false, false, false, false, nil)
    if err != nil {
        return fmt.Errorf("declare queue %s: %w", queueName, err)
    }

    err = ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{
        ContentType: "application/json",
        Body:        body,
    })
    if err != nil {
        return fmt.Errorf("publish to %s: %w", queueName, err)
    }

    return nil
}
```

### Struct `ProviderPublishMessage`

Ditempatkan di `internal/infrastructure/mqpublisher/message.go`:

```go
package mqpublisher

// ProviderPublishData berisi data transaksi yang dipublikasikan ke downstream consumer.
type ProviderPublishData struct {
    MsgID                  int    `json:"msg_id"`
    StatusToBe             string `json:"status_to_be"`
    SerialNumber           string `json:"serial_number"`
    ClientNumber           string `json:"client_number"`
    Nominal                string `json:"nominal"`
    OriginalConversationID string `json:"original_conversation_id"`
    ConversationID         string `json:"conversation_id"`
    MessageToCustomer      string `json:"message_to_customer"`
    AdditionalMessage      string `json:"additional_message"`
    QueueName              string `json:"queue_name"`
}

// ProviderPublishMessage adalah wrapper JSON yang dipublikasikan ke RabbitMQ.
// Format: {"source": "PROVIDER", "data": {...}}
type ProviderPublishMessage struct {
    Source string              `json:"source"`
    Data   ProviderPublishData `json:"data"`
}

// NewProviderPublishMessage membuat ProviderPublishMessage dengan source = "PROVIDER".
func NewProviderPublishMessage(data ProviderPublishData) ProviderPublishMessage {
    return ProviderPublishMessage{
        Source: "PROVIDER",
        Data:   data,
    }
}
```

### Modifikasi `consumePayload`

Tambah field `MQTransaction` di struct `consumePayload`:

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
    MQTransaction string  // ← BARU: URL RabbitMQ tujuan untuk publish status
}
```

Di method `UnmarshalJSON`, tambahkan parsing:

```go
p.MQTransaction = parseString(getAny(raw, "mq_transaction", "mqTransaction", "MQTransaction"))
```

### Modifikasi `ConsumerServiceImpl`

Ganti field `downstreamClient` dengan `mqPublisher`:

```go
type ConsumerServiceImpl struct {
    cfg          *config.Config
    unipinClient *unipin.Client
    logger       contractsvc.Logger
    mqPublisher  contractsvc.MQPublisher  // menggantikan downstreamClient
}

// SetMQPublisher menyuntikkan MQ publisher untuk forwarding status transaksi.
func (s *ConsumerServiceImpl) SetMQPublisher(pub contractsvc.MQPublisher) {
    s.mqPublisher = pub
}
```

Hapus:
- Field `downstreamClient *downstream.DownstreamClient`
- Method `SetDownstreamClient`
- Semua import `downstream` package

### Modifikasi `forwardCallback`

Method `forwardCallback` dimodifikasi untuk menggunakan `MQPublisher` sebagai pengganti `DownstreamClient`:

```go
// forwardCallback mempublikasikan status transaksi ke RabbitMQ tujuan.
// Non-blocking: error di-log saja, tidak menghentikan alur utama.
func (s *ConsumerServiceImpl) forwardCallback(ctx context.Context, payload *consumePayload, queueName, msgID, statusToBe, serialNumber, message string) {
    if s.mqPublisher == nil {
        s.logger.Warn("mq publisher not initialized, skipping publish",
            "msgid", msgID, "queue_name", queueName)
        return
    }

    msgIDInt := 0
    if n, err := strconv.Atoi(msgID); err == nil {
        msgIDInt = n
    }

    msg := mqpublisher.NewProviderPublishMessage(mqpublisher.ProviderPublishData{
        MsgID:             msgIDInt,
        StatusToBe:        statusToBe,
        SerialNumber:      serialNumber,
        ClientNumber:      payload.MSISDN,
        Nominal:           fmt.Sprintf("%d", payload.Amount),
        ConversationID:    serialNumber,
        MessageToCustomer: message,
        QueueName:         queueName,
    })

    body, err := json.Marshal(msg)
    if err != nil {
        s.logger.Error("failed to marshal publish message",
            "msgid", msgID, "error", err)
        return
    }

    if err := s.mqPublisher.Publish(ctx, payload.MQTransaction, queueName, body); err != nil {
        s.logger.Error("failed to publish to downstream rabbitmq",
            "msgid", msgID, "queue_name", queueName,
            "mq_transaction", payload.MQTransaction, "error", err)
        return
    }

    s.logger.Info("published to downstream rabbitmq",
        "msgid", msgID, "queue_name", queueName,
        "mq_transaction", payload.MQTransaction)
}
```

**Catatan penting**: Signature `forwardCallback` tidak berubah — hanya isi method yang berubah. Semua pemanggil (`processVoucher`, `processVoucherInquiry`) tidak perlu dimodifikasi karena `MQTransaction` sudah tersedia di `payload`.

### Modifikasi `main.go`

```go
// HAPUS:
// downstreamCfg, err := config.LoadDownstream()
// downstreamClient := downstream.NewDownstreamClient(downstreamCfg, logger)
// consumer.SetDownstreamClient(downstreamClient)

// TAMBAH:
import "pps-services-gateway-unipin/internal/infrastructure/mqpublisher"

mqPublisher := mqpublisher.NewAMQPPublisher(logger)
consumer.SetMQPublisher(mqPublisher)
logger.Info("mq publisher initialized")
```

---

## Data Models

### Struct `ProviderPublishMessage` (Format Pesan)

Pesan yang dipublikasikan ke RabbitMQ mengikuti format wrapper yang diharapkan oleh `pps-services-consumer`:

```json
{
  "source": "PROVIDER",
  "data": {
    "msg_id": 12345,
    "status_to_be": "SUCCESS",
    "serial_number": "REF-20250101-001",
    "client_number": "08123456789",
    "nominal": "50000",
    "original_conversation_id": "",
    "conversation_id": "REF-20250101-001",
    "message_to_customer": "Transaksi berhasil",
    "additional_message": "",
    "queue_name": "pps.gateway.queue.staging.unipin"
  }
}
```

**Catatan penting:**
- `msg_id` adalah integer. Jika `msgID` string tidak bisa di-parse ke integer, gunakan `0`.
- `source` selalu `"PROVIDER"`.
- `original_conversation_id` selalu kosong string (tidak digunakan di alur ini).

### Mapping: Alur Voucher → ProviderPublishData

| Kondisi | `StatusToBe` | `SerialNumber` | `ConversationID` | `MessageToCustomer` | `AdditionalMessage` |
|---|---|---|---|---|---|
| Sukses (`Status = 0`) | `"SUCCESS"` | `resp.ReferenceNo` | `resp.ReferenceNo` | `resp.Reason` | `""` |
| Gagal (`Status != 0`) | `"FAILED"` | `resp.ReferenceNo` | `resp.ReferenceNo` | `resp.Reason` | `""` |
| Error teknis (bukan timeout) | `"FAILED"` | `""` | `""` | `error.Error()` | `""` |
| Timeout → fallback inquiry | Lihat mapping inquiry di bawah | | | | |

### Mapping: Alur Voucher Inquiry → ProviderPublishData

| Kondisi | `StatusToBe` | `SerialNumber` | `ConversationID` | `MessageToCustomer` | `AdditionalMessage` |
|---|---|---|---|---|---|
| Sukses (`Status = 0`) | `"SUCCESS"` | `resp.ReferenceNo` | `resp.ReferenceNo` | `resp.Reason` | `""` |
| Gagal (`Status != 0`) | `"FAILED"` | `resp.ReferenceNo` | `resp.ReferenceNo` | `resp.Reason` | `""` |
| Error teknis | `"FAILED"` | `referenceNo` | `referenceNo` | `error.Error()` | `""` |

### File yang Dihapus

| File/Komponen | Alasan |
|---|---|
| `internal/infrastructure/downstream/client.go` | Seluruh file — `DownstreamClient`, `CallbackRequest`, `NewDownstreamClient` |
| `DownstreamConfig` di `config.go` | Struct dan `LoadDownstream()` tidak lagi digunakan |
| `SetDownstreamClient` di `consumer_service.go` | Digantikan oleh `SetMQPublisher` |
| Env vars `PUBLISHER_DATABASE_*` di `.env.example` | Konfigurasi downstream HTTP tidak lagi digunakan |

### Dependency yang Dihapus dari `go.mod`

| Dependency | Alasan |
|---|---|
| `github.com/cenkalti/backoff/v5` | Hanya digunakan oleh `DownstreamClient` untuk retry |
| `github.com/sony/gobreaker` | Hanya digunakan oleh `DownstreamClient` untuk circuit breaker |

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Validasi Input Menolak Parameter Tidak Valid

*For any* string `mqTransactionURL` yang kosong atau hanya berisi whitespace, dan *for any* string `queueName` yang kosong atau hanya berisi whitespace, `MQPublisher.Publish()` harus mengembalikan error yang deskriptif tanpa mencoba membuka koneksi RabbitMQ.

**Validates: Requirements 2.2, 2.3**

### Property 2: Serialisasi Round-Trip ProviderPublishMessage

*For any* `ProviderPublishData` yang valid (dengan field string dan integer sembarang), setelah dibungkus dengan `NewProviderPublishMessage()` dan di-`json.Marshal`, hasil deserialisasi (`json.Unmarshal`) harus menghasilkan objek yang identik dengan input, dan field `source` harus selalu bernilai `"PROVIDER"`.

**Validates: Requirements 3.1, 3.2**

### Property 3: Konversi msg_id String ke Integer

*For any* string `msgID`, jika string tersebut merepresentasikan angka integer yang valid maka konversi `strconv.Atoi(msgID)` harus menghasilkan integer yang benar. Jika string tersebut bukan angka valid (termasuk string kosong, string dengan huruf, float), maka `msg_id` di `ProviderPublishData` harus bernilai `0`.

**Validates: Requirements 3.3**

### Property 4: Status Mapping Berdasarkan Status Unipin

*For any* respons Unipin dengan `Status` sembarang, jika `Status = 0` maka `status_to_be` di pesan publish harus `"SUCCESS"`, dan jika `Status` bernilai selain `0` maka `status_to_be` harus `"FAILED"`. Property ini berlaku untuk alur voucher (`VoucherRequest`) maupun alur inquiry (`VoucherInquiry`).

**Validates: Requirements 4.1, 4.2, 5.1, 5.2**

### Property 5: Routing Menggunakan MQTransaction dan QueueName dari Payload

*For any* payload transaksi dengan field `MQTransaction` dan `QueueName` sembarang, `MQPublisher.Publish()` harus dipanggil dengan `mqTransactionURL` = `payload.MQTransaction` dan `queueName` = `payload.QueueName` (setelah fallback logic). Tidak boleh ada hardcoded URL atau queue name.

**Validates: Requirements 4.5, 5.4**

### Property 6: Parsing MQTransaction dari Payload JSON

*For any* string value dan *for any* key variant dari `["mq_transaction", "mqTransaction", "MQTransaction"]`, jika payload JSON mengandung field tersebut, maka `consumePayload.MQTransaction` harus berisi value yang sama setelah di-unmarshal. Jika tidak ada field MQTransaction di payload, maka nilainya harus string kosong.

**Validates: Requirements 1.1, 1.2**

---

## Error Handling

### Prinsip Non-Blocking

Semua pemanggilan `MQPublisher.Publish()` di `forwardCallback` bersifat non-blocking terhadap alur utama:
1. Jika `mqPublisher` nil → log warning, skip publish.
2. Jika `json.Marshal` gagal → log error, skip publish.
3. Jika `Publish()` return error → log error, lanjut.
4. Consumer tetap ack pesan RabbitMQ sumber.

### Tabel Error Handling

| Kondisi | Behavior MQPublisher | Behavior Consumer |
|---|---|---|
| `mqTransactionURL` kosong | Return error | Log error, lanjut |
| `queueName` kosong | Return error | Log error, lanjut |
| Koneksi RabbitMQ gagal (`amqp.Dial` error) | Return error | Log error, lanjut |
| Channel gagal dibuka | Return error | Log error, lanjut |
| Queue declare gagal | Return error | Log error, lanjut |
| Publish gagal | Return error | Log error, lanjut |
| `mqPublisher` nil | N/A | Log warning, skip publish |
| Context canceled | Return context error | Log error, lanjut |
| `json.Marshal` gagal | N/A (di forwardCallback) | Log error, skip publish |

### Resource Cleanup

`AMQPPublisher.Publish()` menggunakan `defer conn.Close()` dan `defer ch.Close()` untuk memastikan koneksi dan channel selalu ditutup, bahkan jika terjadi error di tengah proses. Ini mencegah kebocoran resource.

---

## Testing Strategy

### Pendekatan Dual Testing

Fitur ini menggunakan dua lapisan testing:

1. **Unit test** — menggunakan mock `MQPublisher` untuk memverifikasi bahwa `forwardCallback` memanggil `Publish()` dengan parameter yang benar, dan bahwa kegagalan publish tidak memblokir alur utama.

2. **Property-based test** — menggunakan generated input untuk memverifikasi correctness properties: validasi input, serialisasi round-trip, konversi msg_id, status mapping, dan parsing MQTransaction.

### Library Property-Based Testing

Gunakan **`testing/quick`** (stdlib Go) untuk property-based testing. Setiap property test dikonfigurasi minimum **100 iterasi**.

### Tag Format

Setiap property test diberi komentar:
```go
// Feature: rabbitmq-downstream-publisher, Property 1: Validasi Input Menolak Parameter Tidak Valid
```

### Unit Test

```
internal/infrastructure/mqpublisher/publisher_test.go
  - TestPublish_EmptyURL_ReturnsError                      (Property 1 — PBT)
  - TestPublish_EmptyQueueName_ReturnsError                (Property 1 — PBT)
  - TestPublish_ResourceCleanup                            (Req 2.5 — example)
  - TestPublish_ContentTypeJSON                            (Req 3.4 — example)

internal/infrastructure/mqpublisher/message_test.go
  - TestProviderPublishMessage_RoundTrip                   (Property 2 — PBT)
  - TestProviderPublishMessage_SourceAlwaysProvider         (Property 2 — PBT)
  - TestMsgID_IntegerConversion                            (Property 3 — PBT)

internal/infrastructure/rabbitmq/consumer_service_test.go
  - TestConsumePayload_MQTransactionParsing                (Property 6 — PBT)
  - TestConsumer_Voucher_StatusMapping                     (Property 4 — PBT)
  - TestConsumer_Inquiry_StatusMapping                     (Property 4 — PBT)
  - TestConsumer_RoutingFromPayload                        (Property 5 — PBT)
  - TestConsumer_PublishFailure_NonBlocking                 (Req 4.6, 5.5 — example)
  - TestConsumer_NilPublisher_SkipsPublish                  (Req 6.4 — example)
  - TestConsumer_VoucherTimeout_FallbackInquiry             (Req 4.3 — example)
  - TestConsumer_VoucherError_PublishFailed                 (Req 4.4 — example)
  - TestConsumer_InquiryError_PublishFailed                 (Req 5.3 — example)
```

### Integration Test

Dijalankan dengan tag build `//go:build integration` dan membutuhkan RabbitMQ test instance:

```
internal/infrastructure/mqpublisher/publisher_integration_test.go
  - TestPublish_RealRabbitMQ_Integration
  - TestPublish_InvalidURL_Integration
  - TestPublish_ContextCanceled_Integration                (Req 9.4)
```

### Smoke Test

Verifikasi setelah deployment:
- File `internal/infrastructure/downstream/client.go` tidak ada.
- Tidak ada import `downstream` package di codebase.
- Tidak ada environment variable `PUBLISHER_DATABASE_*` di `.env.example`.
- `DownstreamConfig` dan `LoadDownstream()` tidak ada di `config.go`.
- Dependency `cenkalti/backoff` dan `sony/gobreaker` tidak ada di `go.mod`.
