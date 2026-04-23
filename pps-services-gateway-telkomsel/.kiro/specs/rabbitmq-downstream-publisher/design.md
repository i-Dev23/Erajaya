# Dokumen Desain: RabbitMQ Downstream Publisher

## Overview

Fitur ini menggantikan `DownstreamClient` (HTTP POST ke `pps-services-publisher-database`) dengan komponen `MQPublisher` yang mempublikasikan status akhir transaksi langsung ke RabbitMQ. Setiap pesan transaksi dari `pps-services-publisher-provider` sudah membawa field `MQTransaction` (URL RabbitMQ tujuan) dan `QueueName` (nama queue tujuan), sehingga gateway dapat mempublikasikan langsung tanpa perantara HTTP.

Perubahan utama:
- **Komponen baru**: `MQPublisher` — interface + implementasi menggunakan `amqp091-go`
- **Struct baru**: `ProviderPublishMessage` — wrapper `{"source": "PROVIDER", "data": {...}}`
- **Modifikasi**: `ConsumerServiceImpl` — ganti semua `DownstreamClient.ForwardToPublisher()` dengan `MQPublisher.Publish()`
- **Modifikasi**: `CallbackHandler` — ganti `DownstreamClient` dengan `MQPublisher`
- **Modifikasi**: `main.go` — inisialisasi `MQPublisher`, hapus `DownstreamClient`
- **Hapus**: `internal/infrastructure/downstream/client.go`, `DownstreamConfig`, `LoadDownstream()`

Prinsip desain utama: **kegagalan publish ke RabbitMQ tujuan tidak boleh mengganggu alur utama**. Jika RabbitMQ tujuan tidak tersedia, Consumer tetap memproses pesan, memanggil Telkomsel API, dan mencatat log ke database secara normal.

Koneksi RabbitMQ dibuka baru untuk setiap operasi publish karena setiap transaksi dapat memiliki `MQTransaction` URL yang berbeda — tidak ada connection pooling.

---

## Architecture

### Alur Publish (Menggantikan DownstreamClient)

```
ConsumerServiceImpl / CallbackHandler
   │
   ├─► Proses transaksi (Telkomsel API / callback)
   │
   ├─► Bangun ProviderPublishMessage
   │       {"source": "PROVIDER", "data": {...}}
   │
   ├─► json.Marshal(message) → []byte
   │
   └─► MQPublisher.Publish(ctx, mqTransactionURL, queueName, body)
           │
           ├─► Validasi: mqTransactionURL dan queueName tidak kosong
           ├─► amqp.Dial(mqTransactionURL) → conn (baru setiap publish)
           ├─► conn.Channel() → ch
           ├─► ch.QueueDeclare(queueName, ...)
           ├─► ch.PublishWithContext(ctx, ..., body)
           └─► defer conn.Close(), ch.Close()
```

### Perbandingan Sebelum vs Sesudah

```
SEBELUM:
  Consumer → DownstreamClient.ForwardToPublisher()
               → HTTP POST /api/callback → pps-services-publisher-database
               → (circuit breaker + exponential backoff)

SESUDAH:
  Consumer → MQPublisher.Publish(ctx, mqTransactionURL, queueName, body)
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
      publisher.go                   ← BARU: implementasi MQPublisher
      publisher_test.go              ← BARU: unit test
      message.go                     ← BARU: struct ProviderPublishMessage
    rabbitmq/
      consumer_service.go            ← MODIFIKASI: ganti DownstreamClient → MQPublisher
    downstream/
      client.go                      ← HAPUS
  handler/
    callback_handler.go              ← MODIFIKASI: ganti DownstreamClient → MQPublisher
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
    contractsvc "pps-services-gateway-telkomsel/internal/domain/contract/service"
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

### Modifikasi `ConsumerServiceImpl`

Ganti field `downstreamClient` dengan `mqPublisher`:

```go
type ConsumerServiceImpl struct {
    cfg               *config.Config
    logger            contractsvc.Logger
    mqPublisher       contractsvc.MQPublisher       // menggantikan downstreamClient
    transactionLogger contractsvc.TransactionLogger
}

// SetMQPublisher menyuntikkan MQ publisher untuk forwarding status transaksi.
func (s *ConsumerServiceImpl) SetMQPublisher(pub contractsvc.MQPublisher) {
    s.mqPublisher = pub
}
```

Hapus:
- Field `downstreamClient *downstream.DownstreamClient`
- Method `SetDownstreamClient`
- Fungsi `NewDownstreamClient`
- Semua import `downstream` package

### Helper Method `publishToDownstream`

Tambahkan helper method di `ConsumerServiceImpl` untuk menggantikan semua blok `if s.downstreamClient != nil`:

```go
// publishToDownstream mempublikasikan status transaksi ke RabbitMQ tujuan.
// Non-blocking: error di-log saja, tidak menghentikan alur utama.
func (s *ConsumerServiceImpl) publishToDownstream(ctx context.Context, mqTransactionURL, queueName string, data mqpublisher.ProviderPublishData) {
    if s.mqPublisher == nil {
        s.logger.Warn("mq publisher not initialized, skipping publish",
            "msg_id", data.MsgID, "queue_name", queueName)
        return
    }

    msg := mqpublisher.NewProviderPublishMessage(data)
    body, err := json.Marshal(msg)
    if err != nil {
        s.logger.Error("failed to marshal publish message",
            "msg_id", data.MsgID, "error", err)
        return
    }

    if err := s.mqPublisher.Publish(ctx, mqTransactionURL, queueName, body); err != nil {
        s.logger.Error("failed to publish to downstream rabbitmq",
            "msg_id", data.MsgID, "queue_name", queueName,
            "mq_transaction", mqTransactionURL, "error", err)
        return
    }

    s.logger.Info("published to downstream rabbitmq",
        "msg_id", data.MsgID, "queue_name", queueName,
        "mq_transaction", mqTransactionURL)
}
```

### Contoh Penggunaan di Alur Pulsa (Menggantikan DownstreamClient)

Sebelum (DownstreamClient):
```go
if s.downstreamClient != nil {
    msgIDInt := 0
    if n, err := strconv.Atoi(msgID); err == nil {
        msgIDInt = n
    }
    callbackReq := &downstream.CallbackRequest{
        MsgId:            msgIDInt,
        StatusToBe:       "SUCCESS",
        SerialNumber:     resp.Transaction.TransactionID,
        // ...
    }
    if fwdErr := s.downstreamClient.ForwardToPublisher(ctx, callbackReq); fwdErr != nil {
        s.logger.Error("failed to forward callback to publisher-database", ...)
    }
}
```

Sesudah (MQPublisher):
```go
msgIDInt := 0
if n, err := strconv.Atoi(msgID); err == nil {
    msgIDInt = n
}
s.publishToDownstream(ctx, payload.MQTransaction, queueName, mqpublisher.ProviderPublishData{
    MsgID:            msgIDInt,
    StatusToBe:       "SUCCESS",
    SerialNumber:     resp.Transaction.TransactionID,
    ClientNumber:     payload.MSISDN,
    Nominal:          fmt.Sprintf("%d", payload.Amount),
    ConversationID:   resp.Transaction.TransactionID,
    MessageToCustomer: resp.Transaction.StatusDesc,
    QueueName:        queueName,
})
```

### Modifikasi `CallbackHandler`

Ganti field `downstreamClient` dengan `mqPublisher`:

```go
type CallbackHandler struct {
    logger            contractsvc.Logger
    transactionLogger contractsvc.TransactionLogger
    mqPublisher       contractsvc.MQPublisher   // menggantikan downstreamClient
    queueName         string
}

func NewCallbackHandler(
    logger contractsvc.Logger,
    transactionLogger contractsvc.TransactionLogger,
    mqPublisher contractsvc.MQPublisher,
    queueName string,
) *CallbackHandler
```

Di dalam `Handle()`, setelah lookup `GetTransactionByOurTrxID`:

```go
// Lookup transaksi untuk mendapatkan msg_id, MQTransaction, dan QueueName
txRec, err := h.transactionLogger.GetTransactionByOurTrxID(dbCtx, query.TransactionID)
if err != nil {
    h.logger.Warn("failed to lookup transaction for callback publish, skipping MQ publish",
        "transaction_id", query.TransactionID, "error", err)
    // Skip publish — tanpa MQTransaction dan QueueName, tidak bisa publish
} else if h.mqPublisher != nil {
    msgIDInt := 0
    if n, err := strconv.Atoi(txRec.MsgID); err == nil {
        msgIDInt = n
    }
    publishQueueName := txRec.QueueName
    if publishQueueName == "" {
        publishQueueName = h.queueName
    }

    msg := mqpublisher.NewProviderPublishMessage(mqpublisher.ProviderPublishData{
        MsgID:             msgIDInt,
        StatusToBe:        callbackStatus,
        SerialNumber:      query.SerialNumber,
        ClientNumber:      query.ServiceID,
        ConversationID:    query.TransactionID,
        MessageToCustomer: decodedMessage,
        QueueName:         publishQueueName,
    })
    body, _ := json.Marshal(msg)

    if pubErr := h.mqPublisher.Publish(ctx, txRec.MQTransaction, publishQueueName, body); pubErr != nil {
        h.logger.Error("failed to publish callback to downstream rabbitmq",
            "transaction_id", query.TransactionID, "queue_name", publishQueueName,
            "mq_transaction", txRec.MQTransaction, "error", pubErr)
    } else {
        h.logger.Info("published callback to downstream rabbitmq",
            "transaction_id", query.TransactionID, "queue_name", publishQueueName,
            "mq_transaction", txRec.MQTransaction)
    }
} else {
    h.logger.Warn("mq publisher not initialized, skipping callback publish",
        "transaction_id", query.TransactionID)
}
```

### Modifikasi `main.go`

```go
// HAPUS:
// downstreamCfg, err := config.LoadDownstream()
// downstreamClient := rabbitmq.NewDownstreamClient(downstreamCfg, logger)
// consumer.SetDownstreamClient(downstreamClient)

// TAMBAH:
mqPublisher := mqpublisher.NewAMQPPublisher(logger)
consumer.SetMQPublisher(mqPublisher)
logger.Info("mq publisher initialized")

// Untuk CallbackHandler (jika callback endpoint sudah diimplementasikan):
callbackHandler := handler.NewCallbackHandler(logger, txLogger, mqPublisher, cfg.QueueName)
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
    "serial_number": "TRX-20250101-001",
    "client_number": "08123456789",
    "nominal": "15000",
    "original_conversation_id": "",
    "conversation_id": "TRX-20250101-001",
    "message_to_customer": "Transaksi berhasil",
    "additional_message": "",
    "queue_name": "pps.gateway.queue.staging.telkomsel"
  }
}
```

**Catatan penting:**
- `msg_id` adalah integer. Jika `msgID` string tidak bisa di-parse ke integer, gunakan `0`.
- `source` selalu `"PROVIDER"`.
- `original_conversation_id` selalu kosong string (tidak digunakan di alur ini).

### Mapping: Alur Pulsa → ProviderPublishData

| Kondisi | `StatusToBe` | `SerialNumber` | `ConversationID` | `MessageToCustomer` | `AdditionalMessage` |
|---|---|---|---|---|---|
| Sukses (`StatusCode = "0"`) | `"SUCCESS"` | `resp.TransactionID` | `resp.TransactionID` | `resp.StatusDesc` | `""` |
| Gagal (`StatusCode != "0"`) | `"FAILED"` | `resp.TransactionID` | `resp.TransactionID` | `resp.StatusDesc` | `""` |
| Error teknis | `"FAILED"` | `""` | `msgID` | `"Transaksi gagal"` | `error.Error()` |

### Mapping: Alur Paket Data → ProviderPublishData

| Kondisi | `StatusToBe` | `SerialNumber` | `ConversationID` | `MessageToCustomer` | `AdditionalMessage` |
|---|---|---|---|---|---|
| OrderDealer sukses (`StatusCode = "0"`) | `"SUCCESS"` | `resp.TransactionID` | `resp.TransactionID` | `resp.StatusDesc` | `""` |
| OrderDealer gagal (`StatusCode != "0"`) | `"FAILED"` | `resp.TransactionID` | `resp.TransactionID` | `resp.StatusDesc` | `""` |
| OrderDealer error teknis | `"FAILED"` | `""` | `msgID` | `"Transaksi gagal"` | `error.Error()` |

### Mapping: Callback → ProviderPublishData

| Field `ProviderPublishData` | Sumber |
|---|---|
| `MsgID` | `strconv.Atoi(txRec.MsgID)` — dari lookup `GetTransactionByOurTrxID` |
| `StatusToBe` | `status` dari callback query parameter (`"SUCCESS"` / `"FAILED"`) |
| `SerialNumber` | `serial_number` dari callback query parameter |
| `ClientNumber` | `service_id` dari callback query parameter |
| `ConversationID` | `transaction_id` dari callback query parameter |
| `MessageToCustomer` | `message` dari callback query parameter (URL-decoded) |
| `QueueName` | `txRec.QueueName` dari lookup — fallback ke `h.queueName` |

### File yang Dihapus

| File/Komponen | Alasan |
|---|---|
| `internal/infrastructure/downstream/client.go` | Seluruh file — `DownstreamClient`, `CallbackRequest`, `NewDownstreamClient` |
| `DownstreamConfig` di `config.go` | Struct dan `LoadDownstream()` tidak lagi digunakan |
| `SetDownstreamClient` di `consumer_service.go` | Digantikan oleh `SetMQPublisher` |
| `NewDownstreamClient` di `consumer_service.go` | Wrapper function tidak lagi digunakan |
| Env vars `PUBLISHER_DATABASE_*` di `.env.example` | Konfigurasi downstream HTTP tidak lagi digunakan |

---

## Correctness Properties

*A property is a characteristic or behavior that should hold true across all valid executions of a system — essentially, a formal statement about what the system should do. Properties serve as the bridge between human-readable specifications and machine-verifiable correctness guarantees.*

### Property 1: Validasi Input Menolak Parameter Tidak Valid

*For any* string `mqTransactionURL` yang kosong atau hanya berisi whitespace, dan *for any* string `queueName` yang kosong atau hanya berisi whitespace, `MQPublisher.Publish()` harus mengembalikan error yang deskriptif tanpa mencoba membuka koneksi RabbitMQ.

**Validates: Requirements 1.2, 1.3**

### Property 2: Serialisasi Round-Trip ProviderPublishMessage

*For any* `ProviderPublishData` yang valid (dengan field string dan integer sembarang), setelah dibungkus dengan `NewProviderPublishMessage()` dan di-`json.Marshal`, hasil deserialisasi (`json.Unmarshal`) harus menghasilkan objek yang identik dengan input, dan field `source` harus selalu bernilai `"PROVIDER"`.

**Validates: Requirements 2.1, 2.2**

### Property 3: Konversi msg_id String ke Integer

*For any* string `msgID`, jika string tersebut merepresentasikan angka integer yang valid maka konversi `strconv.Atoi(msgID)` harus menghasilkan integer yang benar. Jika string tersebut bukan angka valid (termasuk string kosong, string dengan huruf, float), maka `msg_id` di `ProviderPublishData` harus bernilai `0`.

**Validates: Requirements 2.3**

### Property 4: Status Mapping Berdasarkan StatusCode

*For any* respons Telkomsel dengan `StatusCode` sembarang, jika `StatusCode = "0"` maka `status_to_be` di pesan publish harus `"SUCCESS"`, dan jika `StatusCode` bernilai selain `"0"` (termasuk string kosong) maka `status_to_be` harus `"FAILED"`. Property ini berlaku untuk alur pulsa (`InitiateRegularRecharge`) maupun alur paket data (`OrderDealer`).

**Validates: Requirements 3.1, 3.2, 4.2, 4.3**

### Property 5: Routing Menggunakan MQTransaction dan QueueName dari Payload

*For any* payload transaksi dengan field `MQTransaction` dan `QueueName` sembarang, `MQPublisher.Publish()` harus dipanggil dengan `mqTransactionURL` = `payload.MQTransaction` dan `queueName` = `payload.QueueName` (setelah fallback logic). Tidak boleh ada hardcoded URL atau queue name.

**Validates: Requirements 3.4, 4.4**

### Property 6: Callback Handler Mapping dari DB Lookup ke Publish Message

*For any* callback query parameter yang valid dan *for any* `TransactionRecord` hasil lookup `GetTransactionByOurTrxID`, pesan yang dipublikasikan harus memenuhi: `msg_id` = integer dari `txRec.MsgID`, `status_to_be` = `status` callback, `serial_number` = `serial_number` callback, `client_number` = `service_id` callback, `conversation_id` = `transaction_id` callback, `queue_name` = `txRec.QueueName`, dan publish target = `txRec.MQTransaction`.

**Validates: Requirements 5.1, 5.2**

### Property 7: Context Cancellation Propagation

*For any* operasi `Publish` yang menerima context yang sudah di-cancel, operasi harus segera mengembalikan error context (bukan hang atau timeout lama), tanpa meninggalkan koneksi atau channel yang terbuka.

**Validates: Requirements 8.4**

---

## Error Handling

### Prinsip Non-Blocking

Semua pemanggilan `MQPublisher.Publish()` di `ConsumerServiceImpl` dan `CallbackHandler` bersifat non-blocking terhadap alur utama:
1. Jika `mqPublisher` nil → log warning, skip publish.
2. Jika `Publish()` return error → log error, lanjut.
3. Consumer tetap ack pesan RabbitMQ sumber.
4. CallbackHandler tetap return HTTP 200 OK ke Telkomsel.

### Tabel Error Handling

| Kondisi | Behavior MQPublisher | Behavior Consumer/Handler |
|---|---|---|
| `mqTransactionURL` kosong | Return error | Log error, lanjut |
| `queueName` kosong | Return error | Log error, lanjut |
| Koneksi RabbitMQ gagal (`amqp.Dial` error) | Return error | Log error, lanjut |
| Channel gagal dibuka | Return error | Log error, lanjut |
| Queue declare gagal | Return error | Log error, lanjut |
| Publish gagal | Return error | Log error, lanjut |
| `mqPublisher` nil | N/A | Log warning, skip publish |
| Context canceled | Return context error | Log error, lanjut |
| `json.Marshal` gagal | N/A (di helper) | Log error, skip publish |
| Lookup `GetTransactionByOurTrxID` gagal | N/A | Log warning, skip publish (callback handler) |

### Resource Cleanup

`AMQPPublisher.Publish()` menggunakan `defer conn.Close()` dan `defer ch.Close()` untuk memastikan koneksi dan channel selalu ditutup, bahkan jika terjadi error di tengah proses. Ini mencegah kebocoran resource.

---

## Testing Strategy

### Pendekatan Dual Testing

Fitur ini menggunakan dua lapisan testing:

1. **Unit test** — menggunakan mock `MQPublisher` untuk memverifikasi bahwa `ConsumerServiceImpl` dan `CallbackHandler` memanggil `Publish()` dengan parameter yang benar, dan bahwa kegagalan publish tidak memblokir alur utama.

2. **Property-based test** — menggunakan generated input untuk memverifikasi correctness properties: validasi input, serialisasi round-trip, konversi msg_id, dan status mapping.

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
  - TestPublish_ResourceCleanup                            (Req 1.5 — example)
  - TestPublish_ContentTypeJSON                            (Req 2.4 — example)

internal/infrastructure/mqpublisher/message_test.go
  - TestProviderPublishMessage_RoundTrip                   (Property 2 — PBT)
  - TestProviderPublishMessage_SourceAlwaysProvider         (Property 2 — PBT)
  - TestMsgID_IntegerConversion                            (Property 3 — PBT)

internal/infrastructure/rabbitmq/consumer_service_test.go
  - TestConsumer_Pulsa_StatusMapping                        (Property 4 — PBT)
  - TestConsumer_PaketData_StatusMapping                    (Property 4 — PBT)
  - TestConsumer_RoutingFromPayload                         (Property 5 — PBT)
  - TestConsumer_PublishFailure_NonBlocking                 (Req 3.5, 4.5 — example)
  - TestConsumer_NilPublisher_SkipsPublish                  (Req 7.3 — example)
  - TestConsumer_Pulsa_ErrorTeknis_PublishFailed            (Req 3.3 — example)
  - TestConsumer_PaketData_OrderDealerError_PublishFailed   (Req 4.1 — example)

internal/handler/callback_handler_test.go
  - TestCallback_MappingFromDBLookup                       (Property 6 — PBT)
  - TestCallback_LookupFails_SkipPublish                   (Req 5.3 — example)
  - TestCallback_NilPublisher_Returns200                   (Req 5.4 — example)
  - TestCallback_PublishFails_Returns200                   (Req 5.4 — example)
```

### Integration Test

Dijalankan dengan tag build `//go:build integration` dan membutuhkan RabbitMQ test instance:

```
internal/infrastructure/mqpublisher/publisher_integration_test.go
  - TestPublish_RealRabbitMQ_Integration
  - TestPublish_InvalidURL_Integration
  - TestPublish_ContextCanceled_Integration                (Property 7)
```

### Smoke Test

Verifikasi setelah deployment:
- File `internal/infrastructure/downstream/client.go` tidak ada.
- Tidak ada import `downstream` package di codebase.
- Tidak ada environment variable `PUBLISHER_DATABASE_*` di `.env.example`.
- `DownstreamConfig` dan `LoadDownstream()` tidak ada di `config.go`.
