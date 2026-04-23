# Dokumen Requirements

## Pendahuluan

Fitur ini menambahkan Postgres database ke `pps-services-gateway-telkomsel` untuk mencatat setiap transaksi yang diproses oleh service. Service saat ini mengonsumsi pesan dari RabbitMQ, memanggil Telkomsel API (pulsa: `InitiateRegularRecharge`; paket data: `BrowseOffer` + `OrderDealer`), lalu meneruskan callback ke `pps-services-publisher-database`. Seluruh alur tersebut tidak meninggalkan jejak persisten, sehingga debugging dan deteksi duplikat tidak dapat dilakukan.

Fitur ini memperkenalkan dua tabel:
- `telkomsel_transaction` — satu baris per pesan RabbitMQ yang masuk, merekam status pemrosesan.
- `telkomsel_transaction_response` — satu baris per respons yang diterima dari Telkomsel (sync maupun async callback di masa depan).

## Glosarium

- **Transaction_Logger**: Komponen baru yang bertanggung jawab menulis dan memperbarui record di Postgres.
- **Consumer**: Komponen `ConsumerServiceImpl` yang mengonsumsi pesan dari RabbitMQ.
- **Telkomsel_API**: Endpoint eksternal Telkomsel ESB (InitiateRegularRecharge, BrowseOffer, OrderDealer).
- **msg_id**: Identifier pesan dari sistem kita, diambil dari field `msgid` payload RabbitMQ atau `MessageId`/`CorrelationId` dari AMQP delivery.
- **our_trx_id**: Transaction ID yang di-generate oleh service ini dan dikirim ke Telkomsel, dihasilkan oleh fungsi `buildTelkomselTransactionID`.
- **telkomsel_trx_id**: `TransactionID` yang dikembalikan oleh Telkomsel di dalam blok `transaction` pada respons API.
- **PROCESSING**: Status transaksi saat pesan sudah diterima tetapi respons Telkomsel belum diterima.
- **SUCCESS**: Status transaksi saat Telkomsel mengembalikan `status_code` bernilai `"0"`.
- **FAILED**: Status transaksi saat Telkomsel mengembalikan `status_code` bukan `"0"`, atau terjadi error teknis.
- **SYNC**: Tipe respons yang diterima langsung dari HTTP response Telkomsel API.
- **CALLBACK**: Tipe respons yang diterima dari async callback Telkomsel (untuk paket data, akan diimplementasikan di iterasi berikutnya).
- **DSN**: Data Source Name — connection string Postgres, dikonfigurasi via environment variable `POSTGRES_DSN`.
- **Migration**: Proses pembuatan skema tabel di Postgres yang dijalankan saat startup service.

---

## Requirements

### Requirement 1: Koneksi dan Inisialisasi Postgres

**User Story:** Sebagai operator, saya ingin service terhubung ke Postgres saat startup, sehingga pencatatan transaksi dapat berjalan tanpa intervensi manual.

#### Acceptance Criteria

1. WHEN `POSTGRES_DSN` tersedia di environment, THE Transaction_Logger SHALL membuka koneksi ke Postgres menggunakan DSN tersebut saat startup.
2. IF `POSTGRES_DSN` kosong atau tidak tersedia, THEN THE Consumer SHALL tetap berjalan normal dan THE Transaction_Logger SHALL melewati seluruh operasi database tanpa mengembalikan error ke Consumer.
3. WHEN koneksi Postgres berhasil dibuka, THE Transaction_Logger SHALL menjalankan migration untuk memastikan tabel `telkomsel_transaction` dan `telkomsel_transaction_response` sudah ada sebelum Consumer mulai mengonsumsi pesan.
4. IF migration gagal, THEN THE Transaction_Logger SHALL mengembalikan error dan THE Consumer SHALL menghentikan startup dengan log error yang menyertakan pesan kegagalan migration.
5. THE Transaction_Logger SHALL menggunakan `github.com/jackc/pgx/v5` sebagai driver Postgres.

---

### Requirement 2: Skema Database

**User Story:** Sebagai developer, saya ingin skema tabel yang konsisten dan terdefinisi dengan baik, sehingga query operasional dan debugging dapat dilakukan secara andal.

#### Acceptance Criteria

1. THE Transaction_Logger SHALL membuat tabel `telkomsel_transaction` dengan kolom: `msg_id VARCHAR PRIMARY KEY`, `our_trx_id VARCHAR NOT NULL`, `msisdn VARCHAR NOT NULL`, `mid VARCHAR NOT NULL`, `product_type VARCHAR NOT NULL`, `product_id VARCHAR`, `amount INTEGER NOT NULL`, `stock_type VARCHAR`, `queue_name VARCHAR NOT NULL`, `status VARCHAR NOT NULL`, `created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`, `updated_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
2. THE Transaction_Logger SHALL membuat tabel `telkomsel_transaction_response` dengan kolom: `id BIGSERIAL PRIMARY KEY`, `msg_id VARCHAR NOT NULL`, `our_trx_id VARCHAR NOT NULL`, `telkomsel_trx_id VARCHAR`, `response_type VARCHAR NOT NULL`, `status_code VARCHAR`, `status_desc VARCHAR`, `raw_payload JSONB`, `received_at TIMESTAMPTZ NOT NULL DEFAULT NOW()`.
3. THE Transaction_Logger SHALL membuat index `idx_telkomsel_transaction_response_msg_id` pada kolom `msg_id` di tabel `telkomsel_transaction_response`.
4. THE Transaction_Logger SHALL menjalankan DDL migration menggunakan `CREATE TABLE IF NOT EXISTS` sehingga migration bersifat idempoten dan aman dijalankan berulang kali.

---

### Requirement 3: Pencatatan Transaksi Masuk (Insert PROCESSING)

**User Story:** Sebagai operator, saya ingin setiap pesan RabbitMQ yang masuk langsung dicatat di database, sehingga saya dapat mengetahui transaksi mana yang sedang diproses.

#### Acceptance Criteria

1. WHEN Consumer berhasil mem-parse payload RabbitMQ, THE Transaction_Logger SHALL menyisipkan satu baris ke tabel `telkomsel_transaction` dengan `status = 'PROCESSING'` sebelum memanggil Telkomsel API.
2. THE Transaction_Logger SHALL mengisi kolom `msg_id` dengan nilai `msg_id` yang diekstrak dari payload (atau `MessageId`/`CorrelationId` AMQP sebagai fallback), `our_trx_id` dengan `transactionID` yang akan dikirim ke Telkomsel, dan kolom lainnya sesuai field payload.
3. IF `msg_id` sudah ada di tabel `telkomsel_transaction`, THEN THE Transaction_Logger SHALL melakukan `INSERT ... ON CONFLICT (msg_id) DO NOTHING` sehingga tidak terjadi duplikasi baris dan Consumer tetap melanjutkan pemrosesan.
4. IF operasi insert ke database gagal, THEN THE Transaction_Logger SHALL mencatat error ke log dan THE Consumer SHALL tetap melanjutkan pemanggilan Telkomsel API tanpa menghentikan pemrosesan pesan.

---

### Requirement 4: Pembaruan Status Transaksi (Update SUCCESS/FAILED)

**User Story:** Sebagai operator, saya ingin status transaksi diperbarui setelah respons dari Telkomsel diterima, sehingga saya dapat melihat hasil akhir setiap transaksi.

#### Acceptance Criteria

1. WHEN Telkomsel API mengembalikan respons dengan `status_code = "0"`, THE Transaction_Logger SHALL memperbarui kolom `status` menjadi `'SUCCESS'` dan `updated_at` menjadi waktu saat ini pada baris dengan `msg_id` yang sesuai di tabel `telkomsel_transaction`.
2. WHEN Telkomsel API mengembalikan respons dengan `status_code` bukan `"0"`, THE Transaction_Logger SHALL memperbarui kolom `status` menjadi `'FAILED'` dan `updated_at` menjadi waktu saat ini.
3. WHEN terjadi error teknis saat memanggil Telkomsel API (network error, timeout, HTTP 5xx), THE Transaction_Logger SHALL memperbarui kolom `status` menjadi `'FAILED'` dan `updated_at` menjadi waktu saat ini.
4. IF operasi update ke database gagal, THEN THE Transaction_Logger SHALL mencatat error ke log dan THE Consumer SHALL tetap melanjutkan alur pemrosesan (ack pesan, forward callback) tanpa menghentikan pemrosesan.

---

### Requirement 5: Pencatatan Respons Telkomsel (Insert Response)

**User Story:** Sebagai operator, saya ingin setiap respons dari Telkomsel disimpan secara lengkap, sehingga saya dapat melakukan investigasi mendalam terhadap isi respons saat terjadi masalah.

#### Acceptance Criteria

1. WHEN Telkomsel API mengembalikan respons (sukses maupun gagal bisnis), THE Transaction_Logger SHALL menyisipkan satu baris ke tabel `telkomsel_transaction_response` dengan `response_type = 'SYNC'`.
2. THE Transaction_Logger SHALL mengisi kolom `msg_id` dengan `msg_id` transaksi, `our_trx_id` dengan transaction ID yang dikirim ke Telkomsel, `telkomsel_trx_id` dengan `TransactionID` dari blok `transaction` pada respons, `status_code` dan `status_desc` dari blok `transaction` respons, dan `raw_payload` dengan seluruh body respons dalam format JSONB.
3. WHEN terjadi error teknis saat memanggil Telkomsel API, THE Transaction_Logger SHALL menyisipkan satu baris ke tabel `telkomsel_transaction_response` dengan `response_type = 'SYNC'`, `status_code = 'ERROR'`, `status_desc` berisi pesan error, dan `raw_payload = NULL`.
4. IF operasi insert response ke database gagal, THEN THE Transaction_Logger SHALL mencatat error ke log dan THE Consumer SHALL tetap melanjutkan alur pemrosesan tanpa menghentikan pemrosesan.
5. WHERE fitur async callback Telkomsel diaktifkan di masa depan, THE Transaction_Logger SHALL menyisipkan baris ke tabel `telkomsel_transaction_response` dengan `response_type = 'CALLBACK'` menggunakan interface yang sama.

---

### Requirement 6: Alur Khusus Paket Data (BrowseOffer + OrderDealer)

**User Story:** Sebagai operator, saya ingin alur paket data yang terdiri dari dua API call (BrowseOffer lalu OrderDealer) tercatat dengan benar, sehingga saya dapat membedakan kegagalan di tahap BrowseOffer versus OrderDealer.

#### Acceptance Criteria

1. WHEN Consumer memproses pesan dengan `product_type = 'paket data'`, THE Transaction_Logger SHALL menyisipkan satu baris ke `telkomsel_transaction` dengan status `'PROCESSING'` menggunakan `our_trx_id` dari BrowseOffer call.
2. WHEN BrowseOffer berhasil dan OrderDealer dipanggil, THE Transaction_Logger SHALL menyisipkan baris respons BrowseOffer ke `telkomsel_transaction_response` dengan `response_type = 'SYNC'` sebelum memanggil OrderDealer.
3. WHEN OrderDealer mengembalikan respons, THE Transaction_Logger SHALL menyisipkan baris respons OrderDealer ke `telkomsel_transaction_response` dengan `response_type = 'SYNC'` dan memperbarui status di `telkomsel_transaction` berdasarkan `status_code` OrderDealer.
4. IF BrowseOffer gagal, THEN THE Transaction_Logger SHALL menyisipkan baris respons BrowseOffer ke `telkomsel_transaction_response` dan memperbarui status `telkomsel_transaction` menjadi `'FAILED'` tanpa memanggil OrderDealer.

---

### Requirement 7: Deteksi Duplikat Respons

**User Story:** Sebagai operator, saya ingin dapat mendeteksi ketika satu `msg_id` menerima lebih dari satu respons, sehingga saya dapat mengidentifikasi anomali seperti double-processing atau callback ganda.

#### Acceptance Criteria

1. THE Transaction_Logger SHALL mengizinkan lebih dari satu baris di `telkomsel_transaction_response` untuk `msg_id` yang sama, karena paket data menghasilkan dua respons (BrowseOffer + OrderDealer) dan async callback akan menambah baris tambahan.
2. WHEN terdapat lebih dari satu baris di `telkomsel_transaction_response` dengan `msg_id` yang sama dan `response_type = 'SYNC'` untuk transaksi pulsa, THE Transaction_Logger SHALL mencatat log warning yang menyertakan `msg_id` dan jumlah baris yang ditemukan.
3. THE Transaction_Logger SHALL menyediakan query capability sehingga operator dapat mengambil semua baris `telkomsel_transaction_response` berdasarkan `msg_id` untuk investigasi duplikat.

---

### Requirement 8: Kesiapan Async Callback

**User Story:** Sebagai developer, saya ingin skema dan interface Transaction_Logger sudah siap untuk menerima async callback dari Telkomsel, sehingga implementasi callback API di iterasi berikutnya tidak memerlukan perubahan skema database.

#### Acceptance Criteria

1. THE Transaction_Logger SHALL menyediakan method `InsertCallbackResponse` yang menerima `msg_id`, `our_trx_id`, `telkomsel_trx_id`, `status_code`, `status_desc`, dan `raw_payload` untuk digunakan oleh handler async callback di masa depan.
2. WHEN `InsertCallbackResponse` dipanggil, THE Transaction_Logger SHALL menyisipkan baris ke `telkomsel_transaction_response` dengan `response_type = 'CALLBACK'`.
3. WHEN `InsertCallbackResponse` dipanggil, THE Transaction_Logger SHALL memperbarui kolom `status` di `telkomsel_transaction` berdasarkan `status_code` yang diterima, dengan aturan yang sama seperti respons SYNC.

---

### Requirement 9: Non-Blocking dan Toleransi Kegagalan Database

**User Story:** Sebagai operator, saya ingin kegagalan database tidak mengganggu alur utama pemrosesan transaksi, sehingga service tetap dapat melayani transaksi meskipun Postgres sedang tidak tersedia.

#### Acceptance Criteria

1. WHILE Postgres tidak tersedia atau koneksi terputus, THE Consumer SHALL tetap memproses pesan dari RabbitMQ dan meneruskan callback ke `pps-services-publisher-database` secara normal.
2. IF operasi database (insert atau update) melebihi batas waktu 5 detik, THEN THE Transaction_Logger SHALL membatalkan operasi tersebut dan mencatat log error tanpa memblokir goroutine Consumer.
3. THE Transaction_Logger SHALL menggunakan context dengan deadline yang diturunkan dari context Consumer untuk setiap operasi database, sehingga pembatalan context Consumer juga membatalkan operasi database yang sedang berjalan.
