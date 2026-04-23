# Requirements Document: Error Mapping

## Pendahuluan

Fitur Error Mapping menyediakan mekanisme terpusat untuk menerjemahkan kombinasi HTTP Status Code dan ESB Status Code dari respons Telkomsel API menjadi kode RC PPS (0 = sukses, 1 = gagal, 9 = pending). Data mapping disimpan di tabel PostgreSQL sehingga dapat dikelola tanpa deploy ulang. Fitur ini digunakan oleh flow transaksi pulsa dan paket data untuk menentukan aksi selanjutnya (sukses, log error, atau retry check status).

## Glossary

- **Error_Mapping_Service**: Komponen helper yang melakukan lookup kombinasi HTTP Status Code dan ESB Status Code ke tabel mapping di PostgreSQL, lalu mengembalikan kode RC PPS.
- **RC_PPS**: Kode hasil pemetaan error yang menentukan aksi selanjutnya. Nilai valid: 0 (sukses), 1 (gagal), 9 (pending).
- **HTTP_Status_Code**: Kode status HTTP yang dikembalikan oleh Telkomsel ESB API (contoh: 200, 400, 500, 503).
- **ESB_Status_Code**: Kode status bisnis dari Telkomsel ESB yang terdapat di body respons (contoh: 00000, 20001, 40000).
- **Error_Mapping_Table**: Tabel PostgreSQL `telkomsel_error_mapping` yang menyimpan data pemetaan antara kombinasi HTTP Status Code + ESB Status Code ke RC PPS.
- **Error_Mapping_Repository**: Interface kontrak untuk akses data ke Error_Mapping_Table.
- **Consumer_Flow**: Alur pemrosesan transaksi pulsa atau paket data yang dipicu oleh pesan RabbitMQ.
- **Retry_Max_Attempts**: Jumlah maksimal percobaan retry check status saat RC_PPS bernilai 9. Dikonfigurasi melalui environment variable `RETRY_MAX_ATTEMPTS`, default 4.
- **Retry_Wait_Seconds**: Waktu tunggu (dalam detik) antar percobaan retry check status. Dikonfigurasi melalui environment variable `RETRY_WAIT_SECONDS`, default 10.

## Requirements

### Requirement 1: Tabel Database Error Mapping

**User Story:** Sebagai developer, saya ingin data error mapping tersimpan di tabel database, sehingga mapping dapat diubah tanpa deploy ulang aplikasi.

#### Acceptance Criteria

1. THE Error_Mapping_Table SHALL memiliki kolom: `id` (primary key auto-increment), `http_status_code` (integer, not null), `esb_status_code` (varchar, not null), `rc_pps` (integer, not null), `description` (varchar), `created_at` (timestamptz), dan `updated_at` (timestamptz).
2. THE Error_Mapping_Table SHALL memiliki unique constraint pada kombinasi kolom `http_status_code` dan `esb_status_code`.
3. THE Error_Mapping_Table SHALL hanya menerima nilai `rc_pps` berupa 0, 1, atau 9.
4. WHEN migrasi database dijalankan, THE Error_Mapping_Service SHALL membuat tabel `telkomsel_error_mapping` secara idempoten menggunakan `CREATE TABLE IF NOT EXISTS`.
5. WHEN migrasi database dijalankan, THE Error_Mapping_Service SHALL menyisipkan data seed mapping awal sesuai spesifikasi Telkomsel ESB menggunakan `INSERT ... ON CONFLICT DO NOTHING`.

### Requirement 2: Repository Error Mapping

**User Story:** Sebagai developer, saya ingin interface repository yang terdefinisi dengan jelas untuk akses data error mapping, sehingga implementasi dapat di-mock untuk testing.

#### Acceptance Criteria

1. THE Error_Mapping_Repository SHALL mendefinisikan method `GetResponseCode(ctx context.Context, httpStatusCode int, esbStatusCode string) (int, error)` yang mengembalikan nilai RC PPS.
2. THE Error_Mapping_Repository SHALL mendefinisikan method `RunMigration(ctx context.Context) error` untuk menjalankan DDL dan seed data secara idempoten.
3. THE Error_Mapping_Repository SHALL ditempatkan di package `internal/domain/contract/service` sesuai pola arsitektur yang sudah ada.

### Requirement 3: Implementasi PostgreSQL Error Mapping Repository

**User Story:** Sebagai developer, saya ingin implementasi repository yang mengakses PostgreSQL, sehingga lookup error mapping berjalan efisien.

#### Acceptance Criteria

1. WHEN method `GetResponseCode` dipanggil dengan `http_status_code` dan `esb_status_code` yang ada di tabel, THE Error_Mapping_Service SHALL mengembalikan nilai `rc_pps` yang sesuai dari Error_Mapping_Table.
2. WHEN method `GetResponseCode` dipanggil dengan kombinasi yang tidak ditemukan di tabel, THE Error_Mapping_Service SHALL mengembalikan nilai default 9.
3. IF terjadi error koneksi database saat lookup, THEN THE Error_Mapping_Service SHALL mengembalikan nilai default 9 dan mencatat error ke log.
4. THE Error_Mapping_Service SHALL menggunakan instance `*sql.DB` yang sudah ada dari PostgresTransactionLogger untuk menghindari pembuatan connection pool baru.
5. THE Error_Mapping_Service SHALL ditempatkan di package `internal/infrastructure/postgres` sesuai pola arsitektur yang sudah ada.

### Requirement 4: Helper Function Error Mapping

**User Story:** Sebagai developer, saya ingin helper function yang mudah dipanggil dari flow transaksi pulsa dan paket data, sehingga penentuan aksi berdasarkan respons Telkomsel API menjadi konsisten.

#### Acceptance Criteria

1. THE Error_Mapping_Service SHALL menyediakan function `ResolveRCPPS(ctx context.Context, httpStatusCode int, esbStatusCode string) int` di package `pkg/telkomsel` yang dapat dipanggil dari Consumer_Flow.
2. WHEN `ResolveRCPPS` dipanggil dengan HTTP_Status_Code 200 dan ESB_Status_Code "00000", THE Error_Mapping_Service SHALL mengembalikan RC_PPS 0.
3. WHEN `ResolveRCPPS` dipanggil dengan HTTP_Status_Code 400 dan ESB_Status_Code "20001", THE Error_Mapping_Service SHALL mengembalikan RC_PPS 1.
4. WHEN `ResolveRCPPS` dipanggil dengan HTTP_Status_Code 500 dan ESB_Status_Code "40000", THE Error_Mapping_Service SHALL mengembalikan RC_PPS 9.
5. WHEN `ResolveRCPPS` dipanggil dengan kombinasi yang tidak terdaftar di Error_Mapping_Table, THE Error_Mapping_Service SHALL mengembalikan RC_PPS 9.

### Requirement 5: Migrasi Database

**User Story:** Sebagai developer, saya ingin file migrasi SQL terpisah untuk tabel error mapping, sehingga perubahan schema dapat dilacak dan di-rollback.

#### Acceptance Criteria

1. THE Error_Mapping_Service SHALL menyediakan file migrasi `003_create_telkomsel_error_mapping.up.sql` di folder `database/migrations`.
2. THE Error_Mapping_Service SHALL menyediakan file rollback `003_create_telkomsel_error_mapping.down.sql` di folder `database/migrations`.
3. WHEN migrasi up dijalankan, THE Error_Mapping_Service SHALL membuat tabel dan menyisipkan 15 baris data seed sesuai spesifikasi mapping Telkomsel ESB berikut:

| http_status_code | esb_status_code | rc_pps | description |
|---|---|---|---|
| 200 | 00000 | 0 | Success |
| 400 | 20001 | 1 | No eligible offer / Could not get eligible denomination |
| 400 | 20002 | 1 | Invalid mandatory parameter / Invalid MSISDN |
| 400 | 20003 | 1 | Insufficient Balance |
| 400 | 20004 | 1 | Invalid Organization Code / Invalid Product ID |
| 400 | 20005 | 1 | Invalid stock type / Activated member is not eligible |
| 500 | 40000 | 9 | ESB internal error |
| 503 | 10001 | 9 | Service provider unreachable |
| 503 | 10002 | 9 | System under maintenance |
| 400 | 10003 | 1 | Subscriber location does not match with dealer |
| 400 | 10004 | 1 | Subscriber location not found |

4. WHEN migrasi down dijalankan, THE Error_Mapping_Service SHALL menghapus tabel `telkomsel_error_mapping`.

### Requirement 6: Integrasi dengan Consumer Flow

**User Story:** Sebagai developer, saya ingin flow transaksi pulsa dan paket data menggunakan error mapping untuk menentukan aksi, sehingga penanganan respons Telkomsel API konsisten dan dapat dikonfigurasi.

#### Acceptance Criteria

1. WHEN Consumer_Flow menerima respons dari Telkomsel API, THE Consumer_Flow SHALL memanggil `ResolveRCPPS` dengan HTTP Status Code dan ESB Status Code dari respons tersebut.
2. WHEN RC_PPS bernilai 0, THE Consumer_Flow SHALL memproses transaksi sebagai sukses, mencatat log ke PostgreSQL, dan mempublikasikan hasil ke RabbitMQ downstream.
3. WHEN RC_PPS bernilai 1, THE Consumer_Flow SHALL memproses transaksi sebagai gagal, mencatat log ke PostgreSQL, dan tidak melakukan retry.
4. WHEN RC_PPS bernilai 9, THE Consumer_Flow SHALL memproses transaksi sebagai pending dan melakukan retry check status maksimal sejumlah Retry_Max_Attempts dengan interval Retry_Wait_Seconds detik antar percobaan.
5. WHILE Consumer_Flow melakukan retry check status dan RC_PPS masih bernilai 9 setelah sejumlah Retry_Max_Attempts percobaan, THE Consumer_Flow SHALL memproses transaksi sebagai gagal dan mencatat log ke PostgreSQL.

### Requirement 7: Lookup Error Mapping Round-Trip

**User Story:** Sebagai developer, saya ingin memastikan bahwa data seed yang dimasukkan ke tabel dapat di-lookup kembali dengan benar, sehingga integritas data mapping terjamin.

#### Acceptance Criteria

1. FOR ALL baris data seed di Error_Mapping_Table, lookup menggunakan `GetResponseCode` dengan `http_status_code` dan `esb_status_code` dari baris tersebut SHALL mengembalikan nilai `rc_pps` yang sama dengan data seed (round-trip property).
2. FOR ALL kombinasi `http_status_code` dan `esb_status_code` yang valid, hasil `GetResponseCode` SHALL selalu bernilai 0, 1, atau 9.

### Requirement 8: Konfigurasi Retry via Environment Variable

**User Story:** Sebagai developer, saya ingin max retry dan waiting time untuk pending check status dapat dikonfigurasi melalui environment variable, sehingga parameter retry dapat disesuaikan tanpa mengubah kode.

#### Acceptance Criteria

1. THE Config SHALL membaca environment variable `RETRY_MAX_ATTEMPTS` untuk menentukan jumlah maksimal percobaan retry check status. IF environment variable tidak diset, THEN default value SHALL bernilai 4.
2. THE Config SHALL membaca environment variable `RETRY_WAIT_SECONDS` untuk menentukan waktu tunggu (dalam detik) antar percobaan retry. IF environment variable tidak diset, THEN default value SHALL bernilai 10.
3. IF `RETRY_MAX_ATTEMPTS` diset dengan nilai bukan integer positif, THEN THE Config SHALL mengembalikan error saat loading konfigurasi.
4. IF `RETRY_WAIT_SECONDS` diset dengan nilai bukan integer positif, THEN THE Config SHALL mengembalikan error saat loading konfigurasi.
5. THE Config SHALL menyimpan nilai retry di struct `RetryConfig` dengan field `MaxAttempts int` dan `WaitDuration time.Duration`, dan dimuat melalui function `LoadRetryConfig()` di package `internal/config` sesuai pola yang sudah ada.
6. THE Consumer_Flow SHALL menggunakan nilai dari RetryConfig untuk menentukan jumlah retry dan waktu tunggu, bukan hardcoded value.

## Info Tambahan: File yang Ter-impact

### Fokus Utama (Fase 1)

Dua file utama yang akan diterapkan error mapping logic:

1. **`internal/infrastructure/rabbitmq/consumer_service.go`** — Consumer RabbitMQ yang memproses transaksi pulsa dan paket data. Saat ini logic penentuan sukses/gagal menggunakan hardcoded check `StatusCode == "00000"`. Akan diganti menggunakan `ResolveRCPPS()` berdasarkan HTTP Status Code + ESB Status Code.

2. **`internal/handler/callback_handler.go`** — HTTP callback handler dari Telkomsel. Callback membawa status yang perlu di-map ke RC PPS untuk menentukan aksi downstream.

### Potensi Impact Lainnya (Fase Selanjutnya)

- `internal/config/config.go` — Tambah `LoadRetryConfig()` untuk env `RETRY_MAX_ATTEMPTS` dan `RETRY_WAIT_SECONDS`.
- `cmd/app/main.go` — Inject `ErrorMappingRepository` ke consumer dan callback handler saat startup, load retry config.
- `internal/domain/contract/service/` — File baru untuk interface `ErrorMappingRepository`.
- `internal/infrastructure/postgres/` — File baru untuk implementasi PostgreSQL error mapping repository.
- `database/migrations/` — File migrasi baru `003_create_telkomsel_error_mapping.up.sql` dan `.down.sql`.
- `pkg/telkomsel/client.go` — `TechnicalError.StatusCode` (HTTP) dan `BusinessError.Code` (ESB) sudah tersedia, perlu dipastikan consumer bisa extract kedua value dari response/error.
