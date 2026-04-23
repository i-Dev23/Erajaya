# Dokumen Requirements

## Pendahuluan

Fitur ini menambahkan HTTP callback endpoint ke `pps-services-gateway-telkomsel` untuk menerima notifikasi asinkron dari Telkomsel terkait fulfillment VAS Recharge (paket data). Saat ini, service hanya memiliki RabbitMQ consumer — belum ada HTTP server. Endpoint callback ini akan berjalan bersamaan (concurrent) dengan RabbitMQ consumer di dalam satu proses.

Alur VAS Recharge paket data bersifat asinkron: setelah `OrderDealer` dipanggil, Telkomsel akan memanggil balik (callback) ke URL yang ditentukan partner untuk memberitahu status akhir transaksi — `SUCCESS` atau `FAILED`. Endpoint callback ini bertugas:
1. Menerima GET request dari Telkomsel di `/callback/ext`
2. Mencatat callback response ke Postgres via `TransactionLogger.InsertCallbackResponse`
3. Meneruskan status akhir ke service `pps-services-publisher-database` via `DownstreamClient`
4. Mengembalikan HTTP response ke Telkomsel

Service menggunakan Go dengan gofiber sebagai HTTP framework untuk callback endpoint.

## Glosarium

- **Callback_Handler**: Komponen baru (HTTP handler) yang menerima dan memproses callback GET request dari Telkomsel di endpoint `/callback/ext`.
- **HTTP_Server**: Instance gofiber yang menjalankan HTTP server untuk menerima callback, berjalan bersamaan dengan RabbitMQ consumer.
- **Transaction_Logger**: Komponen yang sudah ada, bertanggung jawab menulis dan memperbarui record di Postgres. Sudah memiliki method `InsertCallbackResponse`.
- **Downstream_Client**: Komponen yang sudah ada (`DownstreamClient`), bertanggung jawab meneruskan callback ke `pps-services-publisher-database` via HTTP POST.
- **Consumer**: Komponen `ConsumerServiceImpl` yang mengonsumsi pesan dari RabbitMQ.
- **transaction_id**: Transaction ID dari Telkomsel, dikirim sebagai query parameter callback. Digunakan untuk mencocokkan dengan `our_trx_id` di tabel `telkomsel_transaction`.
- **organization_code**: Kode organisasi partner (6-13 karakter), dikirim sebagai query parameter callback.
- **service_id**: MSISDN pelanggan (13 karakter), dikirim sebagai query parameter callback.
- **serial_number**: Serial number yang diberikan Telkomsel hanya saat fulfillment berhasil.
- **status**: Status callback dari Telkomsel, bernilai `"SUCCESS"` atau `"FAILED"`.
- **message**: Deskripsi status dari Telkomsel (contoh: `"Success"`, `"Failed to get subscriber information. Please retry."`).
- **Koneksi_Publik**: Callback dari Telkomsel selalu diterima via URL publik (internet), tanpa Secure Zone atau DMZ/Leased Line.

---

## Requirements

### Requirement 1: HTTP Server untuk Callback

**User Story:** Sebagai operator, saya ingin service menjalankan HTTP server bersamaan dengan RabbitMQ consumer, sehingga Telkomsel dapat mengirim callback ke endpoint yang tersedia.

#### Acceptance Criteria

1. WHEN service di-start, THE HTTP_Server SHALL berjalan bersamaan dengan Consumer dalam satu proses, mendengarkan pada port yang dikonfigurasi via environment variable `CALLBACK_PORT`.
2. IF `CALLBACK_PORT` tidak dikonfigurasi, THEN THE HTTP_Server SHALL menggunakan port default `8080`.
3. THE HTTP_Server SHALL menggunakan framework gofiber (`github.com/gofiber/fiber/v2`) sebagai HTTP framework.
4. WHEN context aplikasi di-cancel (SIGINT/SIGTERM), THE HTTP_Server SHALL melakukan graceful shutdown dan menunggu request yang sedang diproses selesai sebelum berhenti.
5. IF HTTP_Server gagal di-start (misalnya port sudah digunakan), THEN THE HTTP_Server SHALL mengembalikan error dan service SHALL menghentikan startup dengan log error.
6. WHILE HTTP_Server berjalan, THE Consumer SHALL tetap mengonsumsi pesan dari RabbitMQ secara normal tanpa terpengaruh oleh HTTP_Server.

---

### Requirement 2: Callback Endpoint

**User Story:** Sebagai developer, saya ingin endpoint callback sesuai dengan spesifikasi Telkomsel, sehingga Telkomsel dapat mengirim notifikasi fulfillment ke service ini.

#### Acceptance Criteria

1. THE Callback_Handler SHALL menerima request pada endpoint `GET /callback/ext`.
2. THE Callback_Handler SHALL membaca query parameter berikut dari request:
   - `transaction_id` (String, Mandatory)
   - `organization_code` (String, 6-13 karakter, Mandatory)
   - `service_id` (String, 13 karakter, Mandatory)
   - `status` (String, Mandatory — bernilai `"SUCCESS"` atau `"FAILED"`)
   - `message` (String, Mandatory)
   - `serial_number` (String, Optional)
3. WHEN semua parameter mandatory tersedia dan valid, THE Callback_Handler SHALL memproses callback dan mengembalikan HTTP 200 OK.
4. IF satu atau lebih parameter mandatory kosong atau tidak tersedia, THEN THE Callback_Handler SHALL mengembalikan HTTP 400 Bad Request dengan pesan error yang menyebutkan parameter mana yang tidak valid.
5. IF nilai `status` bukan `"SUCCESS"` dan bukan `"FAILED"`, THEN THE Callback_Handler SHALL mengembalikan HTTP 400 Bad Request dengan pesan error yang menjelaskan nilai status tidak valid.

---

### Requirement 3: Validasi Query Parameter

**User Story:** Sebagai developer, saya ingin query parameter divalidasi sesuai spesifikasi Telkomsel, sehingga hanya callback yang valid yang diproses.

#### Acceptance Criteria

1. WHEN `organization_code` memiliki panjang kurang dari 6 karakter atau lebih dari 13 karakter, THE Callback_Handler SHALL mengembalikan HTTP 400 Bad Request dengan pesan error yang menjelaskan panjang `organization_code` tidak valid.
2. WHEN `service_id` memiliki panjang bukan 13 karakter, THE Callback_Handler SHALL mengembalikan HTTP 400 Bad Request dengan pesan error yang menjelaskan panjang `service_id` tidak valid.
3. THE Callback_Handler SHALL melakukan URL-decoding pada parameter `message` sebelum memprosesnya, karena Telkomsel mengirim pesan yang di-encode (contoh: `Failed%20to%20get%20subscriber%20information.%20Please%20retry.`).

---

### Requirement 4: Pencatatan Callback ke Database

**User Story:** Sebagai operator, saya ingin setiap callback dari Telkomsel dicatat di database, sehingga saya dapat melacak dan menginvestigasi status fulfillment paket data.

#### Acceptance Criteria

1. WHEN callback diterima dengan parameter yang valid, THE Callback_Handler SHALL memanggil `TransactionLogger.InsertCallbackResponse` untuk menyisipkan baris ke `telkomsel_transaction_response` dengan `response_type = 'CALLBACK'`.
2. THE Callback_Handler SHALL memetakan query parameter callback ke `ResponseRecord` sebagai berikut:
   - `MsgID` diisi dari `transaction_id` (karena `transaction_id` dari callback adalah `our_trx_id` yang dikirim saat `OrderDealer`)
   - `OurTrxID` diisi dari `transaction_id`
   - `StatusCode` diisi `"0"` jika `status = "SUCCESS"`, atau `"1"` jika `status = "FAILED"`
   - `StatusDesc` diisi dari `message`
   - `RawPayload` diisi dengan seluruh query parameter callback dalam format JSON
3. WHEN `InsertCallbackResponse` dipanggil, THE Transaction_Logger SHALL memperbarui status di `telkomsel_transaction` menjadi `'SUCCESS'` atau `'FAILED'` berdasarkan `StatusCode`.
4. IF operasi database gagal, THEN THE Callback_Handler SHALL mencatat error ke log dan tetap melanjutkan proses forwarding ke downstream tanpa mengembalikan error ke Telkomsel.

---

### Requirement 5: Forwarding Status ke Downstream

**User Story:** Sebagai operator, saya ingin status akhir fulfillment diteruskan ke `pps-services-publisher-database`, sehingga status transaksi di sistem utama diperbarui.

#### Acceptance Criteria

1. WHEN callback berhasil diproses, THE Callback_Handler SHALL meneruskan status ke `pps-services-publisher-database` via Downstream_Client menggunakan format `CallbackRequest` yang sudah ada.
2. THE Callback_Handler SHALL memetakan data callback ke `CallbackRequest` sebagai berikut:
   - `StatusToBe` diisi dari `status` (`"SUCCESS"` atau `"FAILED"`)
   - `SerialNumber` diisi dari `serial_number` (kosong jika tidak ada)
   - `ClientNumber` diisi dari `service_id`
   - `ConversationID` diisi dari `transaction_id`
   - `MessageToCustomer` diisi dari `message`
   - `QueueName` diisi dari konfigurasi queue name service
   - `Source` diisi `"PROVIDER"`
3. IF Downstream_Client tidak tersedia (nil), THEN THE Callback_Handler SHALL mencatat log warning dan tetap mengembalikan HTTP 200 OK ke Telkomsel.
4. IF forwarding ke downstream gagal, THEN THE Callback_Handler SHALL mencatat error ke log dan tetap mengembalikan HTTP 200 OK ke Telkomsel.

---

### Requirement 6: HTTP Response ke Telkomsel

**User Story:** Sebagai developer, saya ingin endpoint mengembalikan response yang tepat ke Telkomsel, sehingga Telkomsel mengetahui bahwa callback telah diterima.

#### Acceptance Criteria

1. WHEN callback berhasil diproses (terlepas dari keberhasilan database atau forwarding), THE Callback_Handler SHALL mengembalikan HTTP 200 OK dengan body JSON yang berisi `{"status": "OK", "message": "Callback received"}`.
2. WHEN validasi parameter gagal, THE Callback_Handler SHALL mengembalikan HTTP 400 Bad Request dengan body JSON yang berisi `{"status": "ERROR", "message": "<deskripsi error>"}`.
3. IF terjadi error internal yang tidak terduga, THEN THE Callback_Handler SHALL mengembalikan HTTP 500 Internal Server Error dengan body JSON yang berisi `{"status": "ERROR", "message": "Internal server error"}`.

---

### Requirement 7: Logging dan Observability

**User Story:** Sebagai operator, saya ingin setiap callback yang masuk dicatat di log, sehingga saya dapat memantau dan men-debug alur callback.

#### Acceptance Criteria

1. WHEN callback diterima, THE Callback_Handler SHALL mencatat log info yang menyertakan `transaction_id`, `organization_code`, `service_id`, `status`, dan `message`.
2. WHEN callback berhasil diproses, THE Callback_Handler SHALL mencatat log info yang menyertakan `transaction_id` dan hasil pemrosesan (database insert berhasil/gagal, forwarding berhasil/gagal).
3. IF terjadi error pada tahap apapun (validasi, database, forwarding), THEN THE Callback_Handler SHALL mencatat log error yang menyertakan `transaction_id` dan detail error.

---

### Requirement 8: Konfigurasi

**User Story:** Sebagai operator, saya ingin konfigurasi HTTP server dapat diatur via environment variable, sehingga deployment dapat disesuaikan dengan kebutuhan infrastruktur.

#### Acceptance Criteria

1. THE HTTP_Server SHALL membaca port dari environment variable `CALLBACK_PORT`.
2. IF `CALLBACK_PORT` tidak tersedia atau kosong, THEN THE HTTP_Server SHALL menggunakan port default `8080`.
3. IF `CALLBACK_PORT` berisi nilai yang bukan angka valid, THEN THE HTTP_Server SHALL mengembalikan error saat startup dengan pesan yang menjelaskan bahwa `CALLBACK_PORT` harus berupa angka.

---

### Requirement 9: Concurrency dan Lifecycle

**User Story:** Sebagai developer, saya ingin HTTP server dan RabbitMQ consumer berjalan bersamaan dengan lifecycle yang terkoordinasi, sehingga shutdown berjalan bersih tanpa kehilangan data.

#### Acceptance Criteria

1. THE HTTP_Server dan Consumer SHALL berjalan dalam goroutine terpisah yang dikoordinasikan oleh context yang sama.
2. WHEN salah satu komponen (HTTP_Server atau Consumer) mengalami fatal error, THE service SHALL menghentikan semua komponen dan melakukan shutdown.
3. WHEN SIGINT atau SIGTERM diterima, THE service SHALL mengirim sinyal cancel ke context, sehingga HTTP_Server dan Consumer melakukan graceful shutdown secara bersamaan.
4. THE HTTP_Server SHALL menyelesaikan request yang sedang diproses sebelum shutdown, dengan batas waktu maksimal yang dikonfigurasi oleh gofiber.

---

### Requirement 10: Toleransi Kegagalan

**User Story:** Sebagai operator, saya ingin kegagalan pada satu komponen tidak mengganggu komponen lain, sehingga service tetap dapat melayani sebagian fungsi meskipun ada masalah.

#### Acceptance Criteria

1. WHILE Postgres tidak tersedia, THE Callback_Handler SHALL tetap menerima callback dari Telkomsel, mencatat error database ke log, dan tetap meneruskan status ke downstream.
2. WHILE Downstream_Client tidak tersedia, THE Callback_Handler SHALL tetap menerima callback dari Telkomsel, mencatat ke database, dan mengembalikan HTTP 200 OK.
3. IF Transaction_Logger nil (karena `POSTGRES_DSN` tidak dikonfigurasi), THEN THE Callback_Handler SHALL melewati operasi database dan tetap memproses callback secara normal.
4. THE Callback_Handler SHALL menggunakan context dengan deadline 5 detik untuk setiap operasi database, sehingga operasi database yang lambat tidak memblokir response ke Telkomsel.
