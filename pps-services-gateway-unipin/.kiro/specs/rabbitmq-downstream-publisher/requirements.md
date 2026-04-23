# Dokumen Requirements

## Pendahuluan

Fitur ini menggantikan mekanisme forwarding status transaksi dari HTTP-based `DownstreamClient` (yang melakukan POST ke `pps-services-publisher-database`) menjadi RabbitMQ publisher di `pps-services-gateway-unipin`. Saat ini, setelah mendapatkan status akhir dari Unipin (respons sync dari VoucherRequest atau VoucherInquiry), gateway meneruskan hasilnya via HTTP POST menggunakan `DownstreamClient` yang dilengkapi circuit breaker dan exponential backoff. Pendekatan ini memiliki keterbatasan: tight coupling ke satu endpoint HTTP, konfigurasi circuit breaker dan retry yang kompleks, serta single point of failure jika `pps-services-publisher-database` down.

Pendekatan baru: gateway mempublikasikan status akhir transaksi ke RabbitMQ menggunakan URL (`MQTransaction`) dan nama queue (`QueueName`) yang dibawa oleh setiap pesan transaksi dari `pps-services-publisher-provider`. Pesan yang dipublikasikan menggunakan format wrapper `{"source": "PROVIDER", "data": {...}}` yang akan dikonsumsi oleh `pps-services-consumer`.

Catatan konfigurasi penting: perubahan ini hanya mengganti mekanisme **publish downstream**. Gateway tetap berjalan sebagai **RabbitMQ consumer** untuk menerima pesan transaksi dari queue sumber, sehingga konfigurasi RabbitMQ sumber (mis. environment variable `RABBITMQ_URL` dan nama queue yang dikonsumsi) tetap diperlukan untuk menjalankan service.

Perubahan ini berlaku untuk:
- **Alur voucher** — respons sync dari `VoucherRequest` dan fallback `VoucherInquiry`
- **Alur direct top-up** — saat ini belum diimplementasikan, tetapi `forwardCallback` sudah siap digunakan

Catatan penting: field `MQTransaction` belum ada di `consumePayload` saat ini — perlu ditambahkan agar gateway dapat mengetahui URL RabbitMQ tujuan untuk setiap transaksi.

Komponen `DownstreamClient` (HTTP POST) beserta seluruh konfigurasinya akan dihapus sepenuhnya.

Perubahan ini mengikuti pola yang sama dengan yang sudah diterapkan di `pps-services-gateway-telkomsel`.

## Glosarium

- **MQ_Publisher**: Komponen baru yang bertanggung jawab mempublikasikan pesan status transaksi ke RabbitMQ. Menggantikan `DownstreamClient`.
- **Consumer**: Komponen `ConsumerServiceImpl` yang mengonsumsi pesan dari RabbitMQ dan memproses transaksi voucher/direct top-up via Unipin API.
- **DownstreamClient**: Komponen HTTP client yang saat ini meneruskan status ke `pps-services-publisher-database` via HTTP POST. Akan dihapus oleh fitur ini.
- **MQTransaction**: URL koneksi RabbitMQ tujuan untuk mempublikasikan status transaksi. Setiap pesan transaksi dari RabbitMQ membawa field ini. URL ini unik per queue — setiap transaksi mengetahui RabbitMQ mana yang harus dituju.
- **QueueName**: Nama queue RabbitMQ tujuan untuk mempublikasikan status transaksi. Dibawa oleh setiap pesan transaksi dari RabbitMQ.
- **Publish_Message**: Struktur pesan wrapper yang dipublikasikan ke RabbitMQ, berformat `{"source": "PROVIDER", "data": {...}}`.
- **pps-services-consumer**: Service downstream yang mengonsumsi pesan dari RabbitMQ dengan source `"PROVIDER"`.
- **msg_id**: Identifier pesan dari sistem kita, diambil dari field `msgid` payload RabbitMQ.
- **status_to_be**: Status akhir transaksi yang dikirim ke downstream, bernilai `"SUCCESS"` atau `"FAILED"`.
- **forwardCallback**: Helper method di `ConsumerServiceImpl` yang memusatkan semua pemanggilan downstream. Method ini akan dimodifikasi untuk menggunakan MQ_Publisher.

---

## Requirements

### Requirement 1: Penambahan Field MQTransaction pada consumePayload

**User Story:** Sebagai developer, saya ingin payload yang dikonsumsi dari RabbitMQ memiliki field `MQTransaction`, sehingga gateway mengetahui URL RabbitMQ tujuan untuk mempublikasikan status transaksi.

#### Acceptance Criteria

1. THE Consumer SHALL mem-parse field `MQTransaction` dari payload JSON dengan mendukung variasi key: `mq_transaction`, `mqTransaction`, dan `MQTransaction`.
2. WHEN field `MQTransaction` tidak ada di payload JSON, THE Consumer SHALL menggunakan string kosong sebagai nilai default.
3. THE Consumer SHALL menyimpan nilai `MQTransaction` di struct `consumePayload` dan meneruskannya ke `forwardCallback` untuk digunakan sebagai URL koneksi RabbitMQ tujuan.

---

### Requirement 2: Komponen MQ Publisher

**User Story:** Sebagai developer, saya ingin komponen publisher RabbitMQ yang dapat mempublikasikan status transaksi ke queue tujuan, sehingga gateway tidak lagi bergantung pada HTTP POST ke `pps-services-publisher-database`.

#### Acceptance Criteria

1. THE MQ_Publisher SHALL menyediakan method untuk mempublikasikan pesan ke RabbitMQ dengan parameter: URL koneksi RabbitMQ (`mqTransactionURL`), nama queue tujuan (`queueName`), dan body pesan dalam format byte array.
2. WHEN `mqTransactionURL` kosong atau hanya berisi whitespace, THE MQ_Publisher SHALL mengembalikan error yang menjelaskan bahwa URL RabbitMQ tidak tersedia.
3. WHEN `queueName` kosong atau hanya berisi whitespace, THE MQ_Publisher SHALL mengembalikan error yang menjelaskan bahwa nama queue tidak tersedia.
4. THE MQ_Publisher SHALL membuka koneksi baru ke RabbitMQ menggunakan `mqTransactionURL` untuk setiap operasi publish, karena setiap transaksi dapat memiliki URL RabbitMQ yang berbeda.
5. THE MQ_Publisher SHALL menutup koneksi dan channel RabbitMQ setelah setiap operasi publish selesai, untuk menghindari kebocoran resource.
6. THE MQ_Publisher SHALL menggunakan library `github.com/rabbitmq/amqp091-go` yang sudah ada di project untuk koneksi dan publish ke RabbitMQ.
7. THE MQ_Publisher SHALL didefinisikan sebagai interface di package `internal/domain/contract/service`, mengikuti pola yang sudah ada di project untuk pemisahan kontrak dan implementasi.

---

### Requirement 3: Format Pesan Publish

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

### Requirement 4: Integrasi dengan Alur Voucher (Consumer)

**User Story:** Sebagai operator, saya ingin status akhir transaksi voucher dipublikasikan ke RabbitMQ setelah respons dari Unipin diterima, sehingga `pps-services-consumer` dapat memproses status tersebut.

#### Acceptance Criteria

1. WHEN `VoucherRequest` mengembalikan respons sukses (`Status = 0`), THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "SUCCESS"`, `serial_number` diisi dari `ReferenceNo` respons Unipin, dan `conversation_id` diisi dari `ReferenceNo` respons Unipin.
2. WHEN `VoucherRequest` mengembalikan respons gagal (`Status` bukan `0`), THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "FAILED"`, `serial_number` diisi dari `ReferenceNo` respons Unipin, dan `conversation_id` diisi dari `ReferenceNo` respons Unipin.
3. WHEN `VoucherRequest` mengalami timeout, THE Consumer SHALL melakukan fallback ke `VoucherInquiry` dan mempublikasikan status berdasarkan respons inquiry.
4. WHEN `VoucherRequest` mengalami error teknis (bukan timeout), THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "FAILED"`, `serial_number` kosong, dan `message_to_customer` berisi pesan error.
5. THE Consumer SHALL menggunakan `MQTransaction` dari payload sebagai URL koneksi RabbitMQ dan `QueueName` dari payload sebagai nama queue tujuan saat mempublikasikan pesan.
6. IF publish ke RabbitMQ gagal, THEN THE Consumer SHALL mencatat error ke log dan tetap melanjutkan pemrosesan (ack pesan RabbitMQ).

---

### Requirement 5: Integrasi dengan Alur Voucher Inquiry (Consumer)

**User Story:** Sebagai operator, saya ingin status akhir transaksi voucher inquiry (fallback dari timeout) dipublikasikan ke RabbitMQ, sehingga `pps-services-consumer` dapat memproses status tersebut.

#### Acceptance Criteria

1. WHEN `VoucherInquiry` mengembalikan respons sukses (`Status = 0`), THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "SUCCESS"`, `serial_number` diisi dari `ReferenceNo` respons inquiry, dan `conversation_id` diisi dari `ReferenceNo` respons inquiry.
2. WHEN `VoucherInquiry` mengembalikan respons gagal (`Status` bukan `0`), THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "FAILED"`, `serial_number` diisi dari `ReferenceNo` respons inquiry, dan `conversation_id` diisi dari `ReferenceNo` respons inquiry.
3. WHEN `VoucherInquiry` mengalami error teknis, THE Consumer SHALL mempublikasikan pesan ke RabbitMQ via MQ_Publisher dengan `status_to_be = "FAILED"`, `serial_number` diisi dari `referenceNo` yang digunakan untuk inquiry, dan `message_to_customer` berisi pesan error.
4. THE Consumer SHALL menggunakan `MQTransaction` dari payload sebagai URL koneksi RabbitMQ dan `QueueName` dari payload sebagai nama queue tujuan saat mempublikasikan pesan inquiry.
5. IF publish ke RabbitMQ gagal, THEN THE Consumer SHALL mencatat error ke log dan tetap melanjutkan pemrosesan (ack pesan RabbitMQ).

---

### Requirement 6: Modifikasi forwardCallback untuk Menggunakan MQ Publisher

**User Story:** Sebagai developer, saya ingin helper method `forwardCallback` dimodifikasi untuk menggunakan MQ_Publisher sebagai pengganti `DownstreamClient`, sehingga semua downstream call terpusat di satu tempat dan konsisten menggunakan RabbitMQ.

#### Acceptance Criteria

1. THE Consumer SHALL memodifikasi method `forwardCallback` untuk menerima parameter `mqTransactionURL` sebagai URL koneksi RabbitMQ tujuan.
2. THE Consumer SHALL membangun `ProviderPublishMessage` di dalam `forwardCallback` dengan mapping field yang sama seperti `CallbackRequest` saat ini: `msg_id`, `status_to_be`, `serial_number`, `client_number`, `nominal`, `conversation_id`, `message_to_customer`, `queue_name`, dan `source`.
3. THE Consumer SHALL melakukan `json.Marshal` pada `ProviderPublishMessage` dan memanggil `MQ_Publisher.Publish()` dengan `mqTransactionURL`, `queueName`, dan body hasil marshal.
4. IF MQ_Publisher nil (tidak diinisialisasi), THEN THE Consumer SHALL mencatat log warning dan melewati publish tanpa menghentikan pemrosesan.
5. IF `json.Marshal` gagal, THEN THE Consumer SHALL mencatat log error dan melewati publish tanpa menghentikan pemrosesan.
6. IF `MQ_Publisher.Publish()` gagal, THEN THE Consumer SHALL mencatat log error dan tetap melanjutkan pemrosesan.
7. WHEN pesan berhasil dipublikasikan, THE Consumer SHALL mencatat log info yang menyertakan `msg_id`, `queue_name`, dan `mq_transaction` URL.

---

### Requirement 7: Penghapusan DownstreamClient

**User Story:** Sebagai developer, saya ingin komponen `DownstreamClient` dan seluruh konfigurasi HTTP downstream dihapus, sehingga codebase bersih dari kode yang tidak lagi digunakan.

#### Acceptance Criteria

1. THE Consumer SHALL menghapus dependency terhadap `DownstreamClient` dan menggantikannya dengan MQ_Publisher.
2. THE service SHALL menghapus file `internal/infrastructure/downstream/client.go` beserta seluruh isinya.
3. THE service SHALL menghapus struct `DownstreamConfig` dari `internal/config/config.go` dan fungsi `LoadDownstream()`.
4. THE service SHALL menghapus inisialisasi `DownstreamClient` dari `cmd/app/main.go`, termasuk pemanggilan `config.LoadDownstream()` dan `consumer.SetDownstreamClient()`.
5. THE service SHALL menghapus method `SetDownstreamClient` dari `ConsumerServiceImpl`.
6. THE service SHALL menghapus field `downstreamClient` dari struct `ConsumerServiceImpl`.
7. THE service SHALL menghapus import package `pps-services-gateway-unipin/internal/infrastructure/downstream` dari semua file yang menggunakannya.

---

### Requirement 8: Inisialisasi MQ Publisher di Main

**User Story:** Sebagai developer, saya ingin MQ Publisher diinisialisasi di `main.go` dan di-inject ke Consumer, sehingga dependency management tetap terpusat.

#### Acceptance Criteria

1. THE service SHALL membuat instance MQ_Publisher di `main.go` dan menyuntikkannya ke Consumer via setter method `SetMQPublisher`.
2. IF MQ_Publisher nil (tidak diinisialisasi), THEN THE Consumer SHALL mencatat log warning saat publish diperlukan dan tetap melanjutkan pemrosesan tanpa mempublikasikan pesan.

---

### Requirement 9: Toleransi Kegagalan Publish

**User Story:** Sebagai operator, saya ingin kegagalan publish ke RabbitMQ tidak mengganggu alur utama pemrosesan transaksi, sehingga service tetap dapat memproses pesan meskipun RabbitMQ tujuan sedang tidak tersedia.

#### Acceptance Criteria

1. IF koneksi ke RabbitMQ tujuan (`MQTransaction` URL) gagal, THEN THE MQ_Publisher SHALL mengembalikan error dan THE Consumer SHALL mencatat error ke log tanpa menghentikan pemrosesan pesan.
2. IF publish pesan ke queue gagal, THEN THE MQ_Publisher SHALL mengembalikan error dan THE Consumer SHALL mencatat error ke log tanpa menghentikan pemrosesan pesan.
3. WHILE RabbitMQ tujuan tidak tersedia, THE Consumer SHALL tetap mengonsumsi pesan dari RabbitMQ sumber, memproses transaksi via Unipin API, dan melanjutkan pemrosesan secara normal.
4. THE MQ_Publisher SHALL menggunakan context untuk setiap operasi publish, sehingga pembatalan context Consumer juga membatalkan operasi publish yang sedang berjalan.

---

### Requirement 10: Logging Publish

**User Story:** Sebagai operator, saya ingin setiap operasi publish ke RabbitMQ dicatat di log, sehingga saya dapat memantau dan men-debug alur forwarding status transaksi.

#### Acceptance Criteria

1. WHEN pesan berhasil dipublikasikan ke RabbitMQ, THE Consumer SHALL mencatat log info yang menyertakan `msg_id`, `queue_name`, dan `mq_transaction` URL.
2. WHEN publish ke RabbitMQ gagal, THE Consumer SHALL mencatat log error yang menyertakan `msg_id`, `queue_name`, `mq_transaction` URL, dan detail error.
3. WHEN MQ_Publisher nil saat publish diperlukan, THE Consumer SHALL mencatat log warning yang menyertakan `msg_id` dan `queue_name`.
