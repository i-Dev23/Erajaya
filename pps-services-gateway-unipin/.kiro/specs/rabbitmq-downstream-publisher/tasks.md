# Rencana Implementasi: RabbitMQ Downstream Publisher (Gateway Unipin)

## Overview

Menggantikan `DownstreamClient` (HTTP POST ke `pps-services-publisher-database`) dengan `MQPublisher` yang mempublikasikan status akhir transaksi langsung ke RabbitMQ. Setiap transaksi sudah membawa `MQTransaction` (URL RabbitMQ tujuan) dan `QueueName` dari payload, sehingga gateway publish langsung tanpa perantara HTTP. Gateway Unipin lebih sederhana dari gateway Telkomsel karena tidak memiliki `CallbackHandler` — semua downstream call sudah terpusat di helper method `forwardCallback`.

## Tasks

- [ ] 1. Tambahkan field MQTransaction di consumePayload
  - [x] 1.1 Modifikasi struct `consumePayload` di `internal/infrastructure/rabbitmq/consumer_service.go`
    - Tambahkan field `MQTransaction string` ke struct `consumePayload`
    - Di method `UnmarshalJSON`, tambahkan parsing: `p.MQTransaction = parseString(getAny(raw, "mq_transaction", "mqTransaction", "MQTransaction"))`
    - _Requirements: 1.1, 1.2, 1.3_

  - [ ]* 1.2 Tulis property test untuk parsing MQTransaction (Property 6: Parsing MQTransaction dari Payload JSON)
    - **Property 6: Parsing MQTransaction dari Payload JSON**
    - **Validates: Requirements 1.1, 1.2**
    - Untuk sembarang string value dan sembarang key variant dari `["mq_transaction", "mqTransaction", "MQTransaction"]`, `consumePayload.MQTransaction` harus berisi value yang sama setelah di-unmarshal. Jika tidak ada field, nilainya harus string kosong
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 6: Parsing MQTransaction dari Payload JSON`


- [ ] 2. Buat interface MQPublisher dan struct ProviderPublishMessage
  - [x] 2.1 Buat file `internal/domain/contract/service/mq_publisher.go`
    - Definisikan interface `MQPublisher` dengan method `Publish(ctx context.Context, mqTransactionURL string, queueName string, body []byte) error`
    - _Requirements: 2.1, 2.7_

  - [x] 2.2 Buat file `internal/infrastructure/mqpublisher/message.go`
    - Definisikan struct `ProviderPublishData` dengan field JSON: `msg_id` (int), `status_to_be`, `serial_number`, `client_number`, `nominal`, `original_conversation_id`, `conversation_id`, `message_to_customer`, `additional_message`, `queue_name`
    - Definisikan struct `ProviderPublishMessage` dengan field `Source` (string, json:"source") dan `Data` (ProviderPublishData, json:"data")
    - Implementasikan fungsi `NewProviderPublishMessage(data ProviderPublishData) ProviderPublishMessage` yang selalu mengisi `Source = "PROVIDER"`
    - _Requirements: 3.1, 3.2_

  - [ ]* 2.3 Tulis property test untuk ProviderPublishMessage (Property 2: Serialisasi Round-Trip)
    - **Property 2: Serialisasi Round-Trip ProviderPublishMessage**
    - **Validates: Requirements 3.1, 3.2**
    - Untuk sembarang `ProviderPublishData`, setelah `NewProviderPublishMessage` + `json.Marshal` + `json.Unmarshal`, hasilnya harus identik dan `source` selalu `"PROVIDER"`
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 2: Serialisasi Round-Trip ProviderPublishMessage`

  - [ ]* 2.4 Tulis property test untuk konversi msg_id (Property 3: Konversi msg_id String ke Integer)
    - **Property 3: Konversi msg_id String ke Integer**
    - **Validates: Requirements 3.3**
    - Untuk sembarang string `msgID`, jika valid integer maka `strconv.Atoi` menghasilkan integer yang benar; jika tidak valid maka `msg_id` harus bernilai `0`
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 3: Konversi msg_id String ke Integer`

- [ ] 3. Implementasi AMQPPublisher
  - [x] 3.1 Buat file `internal/infrastructure/mqpublisher/publisher.go`
    - Definisikan struct `AMQPPublisher` dengan field `logger contractsvc.Logger`
    - Implementasikan `NewAMQPPublisher(logger contractsvc.Logger) *AMQPPublisher`
    - Implementasikan method `Publish(ctx, mqTransactionURL, queueName, body)`:
      - Validasi `mqTransactionURL` dan `queueName` tidak kosong (return error jika kosong)
      - `amqp.Dial(mqTransactionURL)` → `conn`
      - `conn.Channel()` → `ch`
      - `ch.QueueDeclare(queueName, false, false, false, false, nil)`
      - `ch.PublishWithContext(ctx, "", queueName, false, false, amqp.Publishing{ContentType: "application/json", Body: body})`
      - `defer conn.Close()` dan `defer ch.Close()` untuk resource cleanup
    - Tambahkan compile-time check: `var _ contractsvc.MQPublisher = (*AMQPPublisher)(nil)`
    - _Requirements: 2.1, 2.2, 2.3, 2.4, 2.5, 2.6, 3.4, 9.4_

  - [ ]* 3.2 Tulis property test untuk validasi input (Property 1: Validasi Input Menolak Parameter Tidak Valid)
    - **Property 1: Validasi Input Menolak Parameter Tidak Valid**
    - **Validates: Requirements 2.2, 2.3**
    - Untuk sembarang string `mqTransactionURL` yang kosong/whitespace-only dan sembarang `queueName` yang kosong/whitespace-only, `Publish()` harus return error tanpa mencoba koneksi
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 1: Validasi Input Menolak Parameter Tidak Valid`

  - [ ]* 3.3 Tulis unit test untuk AMQPPublisher
    - `TestPublish_EmptyURL_ReturnsError` — verifikasi error saat URL kosong
    - `TestPublish_EmptyQueueName_ReturnsError` — verifikasi error saat queue name kosong
    - `TestPublish_ContentTypeJSON` — verifikasi ContentType = "application/json"
    - _Requirements: 2.2, 2.3, 3.4_

- [x] 4. Checkpoint — Verifikasi komponen MQPublisher dan ProviderPublishMessage
  - Pastikan `AMQPPublisher` mengimplementasikan interface `MQPublisher` (compile-time check: `var _ contractsvc.MQPublisher = (*AMQPPublisher)(nil)`)
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

- [ ] 5. Modifikasi ConsumerServiceImpl dan forwardCallback untuk menggunakan MQPublisher
  - [x] 5.1 Modifikasi struct `ConsumerServiceImpl` di `internal/infrastructure/rabbitmq/consumer_service.go`
    - Ganti field `downstreamClient *downstream.DownstreamClient` dengan `mqPublisher contractsvc.MQPublisher`
    - Ganti method `SetDownstreamClient` dengan `SetMQPublisher(pub contractsvc.MQPublisher)`
    - Hapus import package `downstream`
    - _Requirements: 7.1, 7.5, 7.6, 7.7_

  - [x] 5.2 Modifikasi method `forwardCallback` di `ConsumerServiceImpl`
    - Ganti cek `s.downstreamClient == nil` dengan `s.mqPublisher == nil` → log warning dengan `msgid` dan `queue_name`, return
    - Bangun `ProviderPublishMessage` via `mqpublisher.NewProviderPublishMessage(mqpublisher.ProviderPublishData{...})` dengan mapping:
      - `MsgID`: `strconv.Atoi(msgID)` (0 jika gagal)
      - `StatusToBe`: `statusToBe`
      - `SerialNumber`: `serialNumber`
      - `ClientNumber`: `payload.MSISDN`
      - `Nominal`: `fmt.Sprintf("%d", payload.Amount)`
      - `ConversationID`: `serialNumber`
      - `MessageToCustomer`: `message`
      - `QueueName`: `queueName`
    - `json.Marshal(msg)` → jika error, log error, return
    - `s.mqPublisher.Publish(ctx, payload.MQTransaction, queueName, body)` → jika error, log error dengan `msgid`, `queue_name`, `mq_transaction`; jika sukses, log info
    - Tambahkan import `"pps-services-gateway-unipin/internal/infrastructure/mqpublisher"`
    - _Requirements: 6.1, 6.2, 6.3, 6.4, 6.5, 6.6, 6.7, 10.1, 10.2, 10.3_

  - [ ]* 5.3 Tulis property test untuk status mapping (Property 4: Status Mapping Berdasarkan Status Unipin)
    - **Property 4: Status Mapping Berdasarkan Status Unipin**
    - **Validates: Requirements 4.1, 4.2, 5.1, 5.2**
    - Untuk sembarang respons Unipin dengan `Status` sembarang, jika `Status = 0` maka `status_to_be = "SUCCESS"`, selainnya `"FAILED"`
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 4: Status Mapping Berdasarkan Status Unipin`

  - [ ]* 5.4 Tulis property test untuk routing (Property 5: Routing Menggunakan MQTransaction dan QueueName dari Payload)
    - **Property 5: Routing Menggunakan MQTransaction dan QueueName dari Payload**
    - **Validates: Requirements 4.5, 5.4**
    - Untuk sembarang payload dengan `MQTransaction` dan `QueueName`, `Publish` harus dipanggil dengan parameter yang sama — tidak ada hardcoded URL/queue
    - Tag: `// Feature: rabbitmq-downstream-publisher, Property 5: Routing Menggunakan MQTransaction dan QueueName dari Payload`

  - [ ]* 5.5 Tulis unit test untuk ConsumerServiceImpl dengan mock MQPublisher
    - `TestConsumer_NilPublisher_SkipsPublish` — verifikasi Consumer berjalan normal saat `mqPublisher = nil` (Req 6.4)
    - `TestConsumer_PublishFailure_NonBlocking` — verifikasi kegagalan publish tidak memblokir alur utama (Req 4.6, 5.5, 9.1, 9.2)
    - `TestConsumer_VoucherError_PublishFailed` — verifikasi publish FAILED saat error teknis voucher (Req 4.4)
    - `TestConsumer_InquiryError_PublishFailed` — verifikasi publish FAILED saat inquiry error (Req 5.3)
    - `TestConsumer_VoucherTimeout_FallbackInquiry` — verifikasi fallback ke inquiry saat timeout (Req 4.3)
    - _Requirements: 4.3, 4.4, 4.6, 5.3, 5.5, 6.4, 9.1, 9.2_

- [x] 6. Checkpoint — Verifikasi integrasi MQPublisher di ConsumerServiceImpl
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error setelah penghapusan DownstreamClient
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

- [ ] 7. Hapus DownstreamClient dan konfigurasi terkait
  - [x] 7.1 Hapus file `internal/infrastructure/downstream/client.go`
    - Hapus seluruh file beserta struct `DownstreamClient`, `CallbackRequest`, `NewDownstreamClient`, `ForwardToPublisher`
    - _Requirements: 7.2_

  - [x] 7.2 Hapus `DownstreamConfig` dan `LoadDownstream()` dari `internal/config/config.go`
    - Hapus struct `DownstreamConfig`
    - Hapus fungsi `LoadDownstream()`
    - Hapus import yang tidak lagi digunakan (jika ada)
    - _Requirements: 7.3_

  - [x] 7.3 Bersihkan environment variable downstream dari `.env.example`
    - Pastikan tidak ada baris `PUBLISHER_DATABASE_*` di `.env.example` (saat ini sudah bersih, tapi verifikasi)
    - _Requirements: 7.3_

- [x] 8. Update `cmd/app/main.go` — inisialisasi MQPublisher, hapus DownstreamClient
  - Hapus blok `config.LoadDownstream()` dan `downstream.NewDownstreamClient()` dan `consumer.SetDownstreamClient()`
  - Hapus import `"pps-services-gateway-unipin/internal/infrastructure/downstream"`
  - Tambahkan import `"pps-services-gateway-unipin/internal/infrastructure/mqpublisher"`
  - Tambahkan `mqPublisher := mqpublisher.NewAMQPPublisher(logger)`
  - Panggil `consumer.SetMQPublisher(mqPublisher)`
  - Log `"mq publisher initialized"`
  - _Requirements: 7.4, 8.1, 8.2_

- [x] 9. Jalankan `go mod tidy` untuk menghapus dependency yang tidak digunakan
  - Jalankan `go mod tidy` untuk menghapus `github.com/cenkalti/backoff/v5` dan `github.com/sony/gobreaker` dari `go.mod`
  - Verifikasi kedua dependency sudah tidak ada di `go.mod`
  - _Requirements: 7.2_

- [x] 10. Checkpoint akhir — Pastikan semua test lulus dan codebase bersih
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Jalankan `go vet ./...` untuk memastikan tidak ada issue
  - Pastikan tidak ada import `downstream` package tersisa di codebase
  - Pastikan tidak ada referensi `DownstreamClient`, `DownstreamConfig`, `LoadDownstream`, `SetDownstreamClient`, `ForwardToPublisher` tersisa
  - Pastikan `cenkalti/backoff` dan `sony/gobreaker` tidak ada di `go.mod`
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

## Catatan

- Task bertanda `*` bersifat opsional dan dapat dilewati untuk MVP yang lebih cepat
- Setiap task mereferensikan requirements spesifik untuk traceability
- Checkpoint memastikan validasi inkremental di setiap tahap
- Property test menggunakan `testing/quick` (stdlib Go) dengan minimum 100 iterasi
- Prinsip utama: kegagalan publish ke RabbitMQ tujuan tidak boleh mengganggu alur utama pemrosesan transaksi
- Koneksi RabbitMQ dibuka baru setiap publish karena setiap transaksi dapat memiliki `MQTransaction` URL yang berbeda
- Gateway Unipin tidak memiliki `CallbackHandler` — semua downstream call sudah terpusat di `forwardCallback`
