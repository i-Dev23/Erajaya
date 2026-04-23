# Rencana Implementasi: RabbitMQ Downstream Publisher

## Overview

Menggantikan `DownstreamClient` (HTTP POST ke `pps-services-publisher-database`) dengan `MQPublisher` yang mempublikasikan status akhir transaksi langsung ke RabbitMQ. Setiap transaksi sudah membawa `MQTransaction` (URL RabbitMQ tujuan) dan `QueueName` dari payload, sehingga gateway publish langsung tanpa perantara HTTP.

## Tasks

- [x] 1. Buat interface MQPublisher dan struct ProviderPublishMessage
  - [x] 1.1 Buat file `internal/domain/contract/service/mq_publisher.go`
    - Definisikan interface `MQPublisher` dengan method `Publish(ctx context.Context, mqTransactionURL string, queueName string, body []byte) error`
    - _Requirements: 1.1, 1.4, 1.6_

  - [x] 1.2 Buat file `internal/infrastructure/mqpublisher/message.go`
    - Definisikan struct `ProviderPublishData` dengan field JSON: `msg_id` (int), `status_to_be`, `serial_number`, `client_number`, `nominal`, `original_conversation_id`, `conversation_id`, `message_to_customer`, `additional_message`, `queue_name`
    - Definisikan struct `ProviderPublishMessage` dengan field `Source` (string, json:"source") dan `Data` (ProviderPublishData, json:"data")
    - Implementasikan fungsi `NewProviderPublishMessage(data ProviderPublishData) ProviderPublishMessage` yang selalu mengisi `Source = "PROVIDER"`
    - _Requirements: 2.1, 2.2_

  - [ ]* 1.3 Tulis property test untuk ProviderPublishMessage (Property 2: Serialisasi Round-Trip)
    - **Property 2: Serialisasi Round-Trip ProviderPublishMessage**
    - **Validates: Requirements 2.1, 2.2**
    - Untuk sembarang `ProviderPublishData`, setelah `NewProviderPublishMessage` + `json.Marshal` + `json.Unmarshal`, hasilnya harus identik dan `source` selalu `"PROVIDER"`
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 2: Serialisasi Round-Trip ProviderPublishMessage`

  - [ ]* 1.4 Tulis property test untuk konversi msg_id (Property 3: Konversi msg_id String ke Integer)
    - **Property 3: Konversi msg_id String ke Integer**
    - **Validates: Requirements 2.3**
    - Untuk sembarang string `msgID`, jika valid integer maka `strconv.Atoi` menghasilkan integer yang benar; jika tidak valid maka `msg_id` harus bernilai `0`
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 3: Konversi msg_id String ke Integer`

- [x] 2. Implementasi AMQPPublisher
  - [x] 2.1 Buat file `internal/infrastructure/mqpublisher/publisher.go`
    - Definisikan struct `AMQPPublisher` dengan field `logger contractsvc.Logger`
    - Implementasikan `NewAMQPPublisher(logger contractsvc.Logger) *AMQPPublisher`
    - Implementasikan method `Publish(ctx, mqTransactionURL, queueName, body)`:
      - Validasi `mqTransactionURL` dan `queueName` tidak kosong (return error jika kosong)
      - `amqp.Dial(mqTransactionURL)` → `conn`
      - `conn.Channel()` → `ch`
      - `ch.QueueDeclare(queueName, false, false, false, false, nil)`
      - `ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{ContentType: "application/json", Body: body})`
      - `defer conn.Close()` dan `defer ch.Close()` untuk resource cleanup
    - _Requirements: 1.1, 1.2, 1.3, 1.4, 1.5, 1.6, 2.4, 8.4_

  - [ ]* 2.2 Tulis property test untuk validasi input (Property 1: Validasi Input Menolak Parameter Tidak Valid)
    - **Property 1: Validasi Input Menolak Parameter Tidak Valid**
    - **Validates: Requirements 1.2, 1.3**
    - Untuk sembarang string `mqTransactionURL` yang kosong/whitespace-only dan sembarang `queueName` yang kosong/whitespace-only, `Publish()` harus return error tanpa mencoba koneksi
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 1: Validasi Input Menolak Parameter Tidak Valid`

  - [ ]* 2.3 Tulis unit test untuk AMQPPublisher
    - `TestPublish_EmptyURL_ReturnsError` — verifikasi error saat URL kosong
    - `TestPublish_EmptyQueueName_ReturnsError` — verifikasi error saat queue name kosong
    - `TestPublish_ContentTypeJSON` — verifikasi ContentType = "application/json"
    - _Requirements: 1.2, 1.3, 2.4_

- [x] 3. Checkpoint — Verifikasi komponen MQPublisher dan ProviderPublishMessage
  - Pastikan `AMQPPublisher` mengimplementasikan interface `MQPublisher` (compile-time check: `var _ contractsvc.MQPublisher = (*AMQPPublisher)(nil)`)
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

- [x] 4. Tambahkan helper `publishToDownstream` dan ganti semua DownstreamClient di ConsumerServiceImpl
  - [x] 4.1 Modifikasi struct `ConsumerServiceImpl` di `internal/infrastructure/rabbitmq/consumer_service.go`
    - Ganti field `downstreamClient *downstream.DownstreamClient` dengan `mqPublisher contractsvc.MQPublisher`
    - Ganti method `SetDownstreamClient` dengan `SetMQPublisher(pub contractsvc.MQPublisher)`
    - Hapus fungsi `NewDownstreamClient` (wrapper function)
    - Hapus import package `downstream`
    - _Requirements: 6.1, 6.6, 7.1_

  - [x] 4.2 Tambahkan helper method `publishToDownstream` di `ConsumerServiceImpl`
    - Implementasikan `publishToDownstream(ctx, mqTransactionURL, queueName string, data mqpublisher.ProviderPublishData)`
    - Cek `s.mqPublisher == nil` → log warning, return (non-blocking)
    - Bangun `ProviderPublishMessage` via `NewProviderPublishMessage(data)`
    - `json.Marshal(msg)` → jika error, log error, return
    - `s.mqPublisher.Publish(ctx, mqTransactionURL, queueName, body)` → jika error, log error dengan `msg_id`, `queue_name`, `mq_transaction`; jika sukses, log info
    - _Requirements: 7.3, 8.1, 8.2, 9.1, 9.2_

  - [x] 4.3 Ganti semua blok `if s.downstreamClient != nil` di alur pulsa dengan `publishToDownstream`
    - Blok error teknis `InitiateRegularRecharge`: ganti dengan `publishToDownstream(ctx, payload.MQTransaction, queueName, ProviderPublishData{StatusToBe: "FAILED", SerialNumber: "", ConversationID: msgID, MessageToCustomer: "Transaksi gagal", AdditionalMessage: callErr.Error(), ...})`
    - Blok respons sukses/gagal `InitiateRegularRecharge`: ganti dengan `publishToDownstream(ctx, payload.MQTransaction, queueName, ProviderPublishData{StatusToBe: statusToBe, SerialNumber: resp.Transaction.TransactionID, ConversationID: resp.Transaction.TransactionID, ...})`
    - _Requirements: 3.1, 3.2, 3.3, 3.4, 3.5_

  - [x] 4.4 Ganti semua blok `if s.downstreamClient != nil` di alur paket data dengan `publishToDownstream`
    - Blok error teknis `OrderDealer`: ganti dengan `publishToDownstream(ctx, payload.MQTransaction, queueName, ProviderPublishData{StatusToBe: "FAILED", SerialNumber: "", ConversationID: msgID, MessageToCustomer: "Transaksi gagal", AdditionalMessage: orderErr.Error(), ...})`
    - Blok respons sukses/gagal `OrderDealer`: ganti dengan `publishToDownstream(ctx, payload.MQTransaction, queueName, ProviderPublishData{StatusToBe: statusToBe, SerialNumber: orderResp.Transaction.TransactionID, ConversationID: orderResp.Transaction.TransactionID, ...})`
    - _Requirements: 4.1, 4.2, 4.3, 4.4, 4.5_

  - [ ]* 4.5 Tulis property test untuk status mapping (Property 4: Status Mapping Berdasarkan StatusCode)
    - **Property 4: Status Mapping Berdasarkan StatusCode**
    - **Validates: Requirements 3.1, 3.2, 4.2, 4.3**
    - Untuk sembarang `StatusCode`, jika `"0"` maka `status_to_be = "SUCCESS"`, selainnya `"FAILED"`
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 4: Status Mapping Berdasarkan StatusCode`

  - [ ]* 4.6 Tulis property test untuk routing (Property 5: Routing Menggunakan MQTransaction dan QueueName dari Payload)
    - **Property 5: Routing Menggunakan MQTransaction dan QueueName dari Payload**
    - **Validates: Requirements 3.4, 4.4**
    - Untuk sembarang payload dengan `MQTransaction` dan `QueueName`, `Publish` harus dipanggil dengan parameter yang sama — tidak ada hardcoded URL/queue
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 5: Routing Menggunakan MQTransaction dan QueueName dari Payload`

  - [ ]* 4.7 Tulis unit test untuk ConsumerServiceImpl dengan mock MQPublisher
    - `TestConsumer_NilPublisher_SkipsPublish` — verifikasi Consumer berjalan normal saat `mqPublisher = nil` (Req 7.3)
    - `TestConsumer_PublishFailure_NonBlocking` — verifikasi kegagalan publish tidak memblokir alur utama (Req 3.5, 4.5, 8.1, 8.2)
    - `TestConsumer_Pulsa_ErrorTeknis_PublishFailed` — verifikasi publish FAILED saat error teknis pulsa (Req 3.3)
    - `TestConsumer_PaketData_OrderDealerError_PublishFailed` — verifikasi publish FAILED saat OrderDealer error (Req 4.1)
    - _Requirements: 3.3, 3.5, 4.1, 4.5, 7.3, 8.1, 8.2, 8.3_

- [x] 5. Checkpoint — Verifikasi integrasi MQPublisher di ConsumerServiceImpl
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error setelah penghapusan DownstreamClient
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

- [ ] 6. Buat CallbackHandler dengan MQPublisher
  - [ ] 6.1 Buat file `internal/handler/callback_handler.go`
    - Definisikan struct `CallbackHandler` dengan field: `logger`, `transactionLogger`, `mqPublisher contractsvc.MQPublisher`, `queueName string`
    - Implementasikan `NewCallbackHandler(logger, transactionLogger, mqPublisher, queueName)`
    - Implementasikan method `Handle` untuk endpoint `GET /callback/ext`:
      - Parse query parameters: `transaction_id`, `status`, `serial_number`, `service_id`, `message`
      - URL-decode `message`
      - Lookup `GetTransactionByOurTrxID(transaction_id)` untuk mendapatkan `msg_id`, `MQTransaction`, `QueueName`
      - Jika lookup gagal → log warning, skip publish
      - Jika `mqPublisher != nil` → bangun `ProviderPublishMessage`, marshal, publish ke `txRec.MQTransaction` / `txRec.QueueName` (fallback ke `h.queueName`)
      - Jika `mqPublisher == nil` → log warning, skip publish
      - Jika publish gagal → log error
      - Selalu return HTTP 200 OK
    - _Requirements: 5.1, 5.2, 5.3, 5.4, 7.2, 7.4, 9.3, 9.4_

  - [ ]* 6.2 Tulis property test untuk callback mapping (Property 6: Callback Handler Mapping dari DB Lookup ke Publish Message)
    - **Property 6: Callback Handler Mapping dari DB Lookup ke Publish Message**
    - **Validates: Requirements 5.1, 5.2**
    - Untuk sembarang callback query parameter dan sembarang `TransactionRecord` hasil lookup, pesan yang dipublikasikan harus memenuhi mapping yang benar
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 6: Callback Handler Mapping dari DB Lookup ke Publish Message`

  - [ ]* 6.3 Tulis unit test untuk CallbackHandler
    - `TestCallback_LookupFails_SkipPublish` — verifikasi skip publish saat lookup gagal (Req 5.3)
    - `TestCallback_NilPublisher_Returns200` — verifikasi return 200 saat publisher nil (Req 5.4)
    - `TestCallback_PublishFails_Returns200` — verifikasi return 200 saat publish gagal (Req 5.4)
    - _Requirements: 5.3, 5.4, 7.4_

- [x] 7. Hapus DownstreamClient dan konfigurasi terkait
  - [x] 7.1 Hapus file `internal/infrastructure/downstream/client.go`
    - Hapus seluruh file beserta struct `DownstreamClient`, `CallbackRequest`, `NewDownstreamClient`, `ForwardToPublisher`
    - _Requirements: 6.3_

  - [x] 7.2 Hapus `DownstreamConfig` dan `LoadDownstream()` dari `internal/config/config.go`
    - Hapus struct `DownstreamConfig`
    - Hapus fungsi `LoadDownstream()`
    - Hapus import yang tidak lagi digunakan (jika ada)
    - _Requirements: 6.4_

  - [x] 7.3 Bersihkan environment variable downstream dari `.env.example`
    - Hapus semua baris `PUBLISHER_DATABASE_*` (jika ada — saat ini belum ada di `.env.example`, tapi pastikan tidak ada)
    - _Requirements: 6.7_

- [x] 8. Update `cmd/app/main.go` — inisialisasi MQPublisher, hapus DownstreamClient
  - Hapus blok `config.LoadDownstream()` dan `rabbitmq.NewDownstreamClient()` dan `consumer.SetDownstreamClient()`
  - Tambahkan `mqPublisher := mqpublisher.NewAMQPPublisher(logger)`
  - Panggil `consumer.SetMQPublisher(mqPublisher)`
  - Log `"mq publisher initialized"`
  - Tambahkan import `"pps-services-gateway-telkomsel/internal/infrastructure/mqpublisher"`
  - Hapus import `"pps-services-gateway-telkomsel/internal/infrastructure/downstream"` (jika ada)
  - _Requirements: 6.5, 7.1, 7.2_

- [x] 9. Checkpoint akhir — Pastikan semua test lulus dan codebase bersih
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Jalankan `go vet ./...` untuk memastikan tidak ada issue
  - Pastikan tidak ada import `downstream` package tersisa di codebase
  - Pastikan tidak ada referensi `DownstreamClient`, `DownstreamConfig`, `LoadDownstream`, `SetDownstreamClient`, `ForwardToPublisher` tersisa
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

## Catatan

- Task bertanda `*` bersifat opsional dan dapat dilewati untuk MVP yang lebih cepat
- Setiap task mereferensikan requirements spesifik untuk traceability
- Callback handler (`internal/handler/callback_handler.go`) belum ada di codebase — task 6 membuat file baru
- Property test menggunakan `testing/quick` (stdlib Go) dengan minimum 100 iterasi
- Prinsip utama: kegagalan publish ke RabbitMQ tujuan tidak boleh mengganggu alur utama pemrosesan transaksi
- Koneksi RabbitMQ dibuka baru setiap publish karena setiap transaksi dapat memiliki `MQTransaction` URL yang berbeda
