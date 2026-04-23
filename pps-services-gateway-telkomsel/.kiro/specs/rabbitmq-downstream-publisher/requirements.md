# Dokumen Requirements

## Pendahuluan

Fitur ini menggantikan mekanisme forwarding status transaksi dari HTTP-based `DownstreamClient` (yang melakukan POST ke `pps-services-publisher-database`) menjadi RabbitMQ publisher. Saat ini, setelah mendapatkan status akhir dari Telkomsel (sync untuk pulsa, async callback untuk paket data), gateway meneruskan hasilnya via HTTP POST menggunakan `DownstreamClient`. Pendekatan ini memiliki keterbatasan: tight coupling ke satu endpoint HTTP, konfigurasi circuit breaker dan retry yang kompleks, serta single point of failure jika `pps-services-publisher-database` down.

Pendekatan baru: gateway mempublikasikan status akhir transaksi ke RabbitMQ menggunakan URL (`MQTransaction`) dan nama queue (`QueueName`) yang sudah dibawa oleh setiap pesan transaksi dari `pps-services-publisher-provider`. Pesan yang dipublikasikan menggunakan format wrapper `{"source": "PROVIDER", "data": {...}}` yang akan dikonsumsi oleh `pps-services-consumer`.

Perubahan ini berlaku untuk:
- **Alur pulsa** — respons sync dari `InitiateRegularRecharge`
- **Alur paket data** — respons sync dari `OrderDealer` yang gagal, dan async callback dari Telkomsel
- **Callback endpoint** — endpoint `GET /callback/ext` yang menerima notifikasi asinkron dari Telkomsel

Komponen `DownstreamClient` (HTTP POST) beserta seluruh konfigurasinya akan dihapus sepenuhnya.

## Glosarium

- **MQ_Publisher**: Komponen baru yang bertanggung jawab mempublikasikan pesan status transaksi ke RabbitMQ. Menggantikan `DownstreamClient`.
- **Consumer**: Komponen `ConsumerServiceImpl` yang mengonsumsi pesan dari RabbitMQ dan memproses transaksi pulsa/paket data.
- **Callback_Handler**: Komponen HTTP handler yang menerima async callback dari Telkomsel di endpoint `/callback/ext`.
- **DownstreamClient**: Komponen HTTP client yang saat ini meneruskan status ke `pps-services-publisher-database` via HTTP POST. Akan dihapus oleh fitur ini.
- **MQTransaction**: URL koneksi RabbitMQ tujuan untuk mempublikasikan status transaksi. Setiap pesan transaksi dari RabbitMQ sudah membawa field ini. URL ini unik per queue — setiap transaksi mengetahui RabbitMQ mana yang harus dituju.
- **QueueName**: Nama queue RabbitMQ tujuan untuk mempublikasikan status transaksi. Dibawa oleh setiap pesan transaksi dari RabbitMQ.
- **Publish_Message**: Struktur pesan wrapper yang dipublikasikan ke RabbitMQ, berformat `{"source": "PROVIDER", "data": {...}}`.
- **pps-services-consumer**: Service downstream yang mengonsumsi pesan dari RabbitMQ dengan source `"PROVIDER"`.
- **msg_id**: Identifier pesan dari sistem kita, diambil dari field `msgid` payload RabbitMQ.
- **status_to_be**: Status akhir transaksi yang dikirim ke downstream, bernilai `"SUCCESS"` atau `"FAILED"`.

---

## Requirements

### Requirement 1: Komponen MQ Publisher

**User Story:** Sebagai developer, saya ingin komponen publisher RabbitMQ yang dapat mempublikasikan status transaksi ke queue tujuan, sehingga gateway tidak lagi bergantung pada HTTP POST ke `pps-services-publisher-database`.

#### Acceptance Criteria

1. THE MQ_Publisher SHALL menyediakan method untuk mempublikasikan pesan ke RabbitMQ dengan parameter: URL koneksi RabbitMQ (`mqTransactionURL`), nama queue tujuan (`queueName`), dan body pesan dalam format byte array.
2. WHEN `mqTransactionURL` kosong atau tidak valid, THE MQ_Publisher SHALL mengembalikan error yang menjelaskan bahwa URL RabbitMQ tidak tersedia.
3. WHEN `queueName` kosong, THE MQ_Publisher SHALL mengembalikan error yang menjelaskan bahwa nama queue tidak tersedia.
4. THE MQ_Publisher SHALL membuka koneksi baru ke RabbitMQ menggunakan `mqTransactionURL` untuk setiap operasi publish, karena setiap transaksi dapat memiliki URL RabbitMQ yang berbeda.
5. THE MQ_Publisher SHALL menutup koneksi dan channel RabbitMQ setelah setiap operasi publish selesai, untuk menghindari kebocoran resource.
6. THE MQ_Publisher SHALL menggunakan library `github.com/rabbitmq/amqp091-go` yang sudah ada di project untuk koneksi dan publish ke RabbitMQ.

---

### Requirement 2: Format Pesan Publish

**User Story:** Sebagai developer, saya ingin pesan yang dipublikasikan ke RabbitMQ menggunakan format wrapper yang sesuai dengan ekspektasi `pps-services-consumer`, sehingga pesan dapat dikonsumsi dengan benar.

#### Acceptance Criteria

1. THE MQ_Publisher SHALL mempublikasikan pesan dalam format JSON wrapper berikut:
   ```json
   {
     "source": "PROVIDER",
     "data": {
       "msg_id": <integer>,
       "status_to_be": "<SUCCESS|FAILED>",
       "serial_number": "<string>",
       "client_number": "<string>",
       "nominal": "<string>",
       "original_conversation_id": "",
       "conversation_id": "<string>",
       "message_to_customer": "<string>",
       "additional_message": "<string>",
       "queue_name": "<string>"
     }
   }
   ```
2. THE MQ_Publisher SHALL selalu mengisi field `source` dengan nilai `"PROVIDER"`.
3. THE MQ_Publisher SHALL mengisi field `msg_id` di dalam `data` sebagai integer. WHEN nilai `msg_id` tidak dapat dikonversi ke integer, THE MQ_Publisher SHALL menggunakan nilai `0`.
4. THE MQ_Publisher SHALL mempublikasikan pesan dengan `ContentType` bernilai `"application/json"`.

---

### Requirement 3: Integrasi dengan Alur Pulsa (Consumer)

**User Story:** Sebagai operator, saya ingin status akhir transaksi pulsa dipublikasikan ke RabbitMQ setelah respons dari Telkomsel diterima, sehingga `pps-services-consumer` dapat memproses status tersebut.

#### Acceptance Criteria

1. WHEN `InitiateRegularRecharge` mengembalikan respons sukses (`status_code = "0"`), THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "SUCCESS"`, `serial_number` diisi dari `TransactionID` respons Telkomsel, dan `conversation_id` diisi dari `TransactionID` respons Telkomsel.
2. WHEN `InitiateRegularRecharge` mengembalikan respons gagal (`status_code` bukan `"0"`), THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "FAILED"`, `serial_number` diisi dari `TransactionID` respons Telkomsel, dan `conversation_id` diisi dari `TransactionID` respons Telkomsel.
3. WHEN `InitiateRegularRecharge` mengalami error teknis (network error, timeout), THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "FAILED"`, `serial_number` kosong, dan `conversation_id` diisi dari `msgID`.
4. THE Consumer SHALL menggunakan `MQTransaction` dari payload sebagai URL koneksi RabbitMQ dan `QueueName` dari payload sebagai nama queue tujuan saat mempublikasikan pesan.
5. IF publish ke RabbitMQ gagal, THEN THE Consumer SHALL mencatat error ke log dan tetap melanjutkan pemrosesan (ack pesan RabbitMQ).

---

### Requirement 4: Integrasi dengan Alur Paket Data (Consumer)

**User Story:** Sebagai operator, saya ingin status akhir transaksi paket data dipublikasikan ke RabbitMQ saat terjadi kegagalan pada tahap BrowseOffer atau OrderDealer, sehingga `pps-services-consumer` dapat memproses status tersebut.

#### Acceptance Criteria

1. WHEN `OrderDealer` mengalami error teknis, THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "FAILED"`, `serial_number` kosong, `conversation_id` diisi dari `msgID`, dan `message_to_customer` berisi `"Transaksi gagal"`.
2. WHEN `OrderDealer` mengembalikan respons sukses (`status_code = "0"`), THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "SUCCESS"`, `serial_number` diisi dari `TransactionID` respons Telkomsel, dan `conversation_id` diisi dari `TransactionID` respons Telkomsel.
3. WHEN `OrderDealer` mengembalikan respons gagal (`status_code` bukan `"0"`), THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "FAILED"`, `serial_number` diisi dari `TransactionID` respons Telkomsel, dan `conversation_id` diisi dari `TransactionID` respons Telkomsel.
4. THE Consumer SHALL menggunakan `MQTransaction` dari payload sebagai URL koneksi RabbitMQ dan `QueueName` dari payload sebagai nama queue tujuan saat mempublikasikan pesan paket data.
5. IF publish ke RabbitMQ gagal, THEN THE Consumer SHALL mencatat error ke log dan tetap melanjutkan pemrosesan (ack pesan RabbitMQ).

---

### Requirement 5: Integrasi dengan Callback Endpoint

**User Story:** Sebagai operator, saya ingin callback endpoint juga menggunakan RabbitMQ publisher untuk meneruskan status akhir fulfillment paket data, sehingga seluruh alur forwarding konsisten menggunakan RabbitMQ.

#### Acceptance Criteria

1. WHEN callback diterima dengan parameter yang valid, THE Callback_Handler SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher menggunakan `MQTransaction` dan `QueueName` yang diambil dari record transaksi di database (hasil lookup `GetTransactionByOurTrxID`).
2. THE Callback_Handler SHALL memetakan data callback ke format Publish_Message sebagai berikut:
   - `msg_id` diisi dari `msg_id` hasil lookup (dikonversi ke integer)
   - `status_to_be` diisi dari `status` callback (`"SUCCESS"` atau `"FAILED"`)
   - `serial_number` diisi dari `serial_number` callback
   - `client_number` diisi dari `service_id` callback
   - `conversation_id` diisi dari `transaction_id` callback
   - `message_to_customer` diisi dari `message` callback (URL-decoded)
   - `queue_name` diisi dari `QueueName` hasil lookup
3. IF lookup `GetTransactionByOurTrxID` gagal (transaksi tidak ditemukan atau database error), THEN THE Callback_Handler SHALL mencatat log warning dan melewati publish ke RabbitMQ, karena `MQTransaction` dan `QueueName` tidak tersedia tanpa data transaksi.
4. IF MQ_Publisher nil atau publish gagal, THEN THE Callback_Handler SHALL mencatat error ke log dan tetap mengembalikan HTTP 200 OK ke Telkomsel.

---

### Requirement 6: Penghapusan DownstreamClient

**User Story:** Sebagai developer, saya ingin komponen `DownstreamClient` dan seluruh konfigurasi HTTP downstream dihapus, sehingga codebase bersih dari kode yang tidak lagi digunakan.

#### Acceptance Criteria

1. THE Consumer SHALL menghapus dependency terhadap `DownstreamClient` dan menggantikannya dengan MQ_Publisher.
2. THE Callback_Handler SHALL menghapus dependency terhadap `DownstreamClient` dan menggantikannya dengan MQ_Publisher.
3. THE service SHALL menghapus file `internal/infrastructure/downstream/client.go` beserta seluruh isinya.
4. THE service SHALL menghapus struct `DownstreamConfig` dari `internal/config/config.go` dan fungsi `LoadDownstream()`.
5. THE service SHALL menghapus inisialisasi `DownstreamClient` dari `cmd/app/main.go`, termasuk pemanggilan `config.LoadDownstream()` dan `consumer.SetDownstreamClient()`.
6. THE service SHALL menghapus method `SetDownstreamClient` dari `ConsumerServiceImpl`.
7. THE service SHALL menghapus environment variable terkait downstream dari `.env.example`: `PUBLISHER_DATABASE_BASE_URL`, `PUBLISHER_DATABASE_API_KEY`, dan seluruh konfigurasi `PUBLISHER_DATABASE_*`.

---

### Requirement 7: Inisialisasi MQ Publisher di Main

**User Story:** Sebagai developer, saya ingin MQ Publisher diinisialisasi di `main.go` dan di-inject ke komponen yang membutuhkan, sehingga dependency management tetap terpusat.

#### Acceptance Criteria

1. THE service SHALL membuat instance MQ_Publisher di `main.go` dan menyuntikkannya ke Consumer via setter method.
2. THE service SHALL menyuntikkan MQ_Publisher ke Callback_Handler saat inisialisasi.
3. IF MQ_Publisher nil (tidak diinisialisasi), THEN THE Consumer SHALL mencatat log warning saat publish diperlukan dan tetap melanjutkan pemrosesan tanpa mempublikasikan pesan.
4. IF MQ_Publisher nil, THEN THE Callback_Handler SHALL mencatat log warning dan melewati publish ke RabbitMQ, tetap mengembalikan HTTP 200 OK ke Telkomsel.

---

### Requirement 8: Toleransi Kegagalan Publish

**User Story:** Sebagai operator, saya ingin kegagalan publish ke RabbitMQ tidak mengganggu alur utama pemrosesan transaksi, sehingga service tetap dapat memproses pesan meskipun RabbitMQ tujuan sedang tidak tersedia.

#### Acceptance Criteria

1. IF koneksi ke RabbitMQ tujuan (`MQTransaction` URL) gagal, THEN THE MQ_Publisher SHALL mengembalikan error dan THE Consumer SHALL mencatat error ke log tanpa menghentikan pemrosesan pesan.
2. IF publish pesan ke queue gagal, THEN THE MQ_Publisher SHALL mengembalikan error dan THE Consumer SHALL mencatat error ke log tanpa menghentikan pemrosesan pesan.
3. WHILE RabbitMQ tujuan tidak tersedia, THE Consumer SHALL tetap mengonsumsi pesan dari RabbitMQ sumber, memproses transaksi via Telkomsel API, dan mencatat log transaksi ke database secara normal.
4. THE MQ_Publisher SHALL menggunakan context untuk setiap operasi publish, sehingga pembatalan context Consumer juga membatalkan operasi publish yang sedang berjalan.

---

### Requirement 9: Logging Publish

**User Story:** Sebagai operator, saya ingin setiap operasi publish ke RabbitMQ dicatat di log, sehingga saya dapat memantau dan men-debug alur forwarding status transaksi.

#### Acceptance Criteria

1. WHEN pesan berhasil dipublikasikan ke RabbitMQ, THE Consumer SHALL mencatat log info yang menyertakan `msg_id`, `queue_name`, dan `mq_transaction` URL.
2. WHEN publish ke RabbitMQ gagal, THE Consumer SHALL mencatat log error yang menyertakan `msg_id`, `queue_name`, `mq_transaction` URL, dan detail error.
3. WHEN Callback_Handler berhasil mempublikasikan pesan ke RabbitMQ, THE Callback_Handler SHALL mencatat log info yang menyertakan `transaction_id`, `queue_name`, dan `mq_transaction` URL.
4. WHEN Callback_Handler gagal mempublikasikan pesan ke RabbitMQ, THE Callback_Handler SHALL mencatat log error yang menyertakan `transaction_id`, `queue_name`, `mq_transaction` URL, dan detail error.
