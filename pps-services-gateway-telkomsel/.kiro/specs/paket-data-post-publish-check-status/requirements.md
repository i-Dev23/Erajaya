# Requirements Document

## Introduction

Fitur ini mengimplementasikan goroutine check status untuk flow "paket data" setelah publish downstream berhasil (rcPPS == 0) maupun saat status masih pending (rcPPS == 9). Saat ini, setelah OrderDealer berhasil dan status di-publish ke downstream dengan `StatusToBeProcess`, tidak ada mekanisme untuk memverifikasi apakah transaksi benar-benar selesai di sisi Telkomsel. Selain itu, untuk rcPPS == 9 (pending), retry saat ini menggunakan `retryCheckStatus` yang langsung memanggil API tanpa mengecek status di database terlebih dahulu. Fitur ini menambahkan goroutine baru yang secara periodik mengecek status transaksi di database terlebih dahulu sebelum memanggil Telkomsel Check Order Status API, dan digunakan untuk kedua kondisi rcPPS (0 dan 9), sehingga menghindari API call yang tidak perlu jika callback sudah menyelesaikan transaksi.

## Glossary

- **Consumer_Service**: Komponen `ConsumerServiceImpl` di `internal/infrastructure/rabbitmq/consumer_service.go` yang mengkonsumsi pesan dari RabbitMQ dan memproses transaksi Telkomsel.
- **Transaction_Table**: Tabel `transaction.telkomsel_transaction` di PostgreSQL yang menyimpan lifecycle transaksi dengan kolom `status` (PROCESSING | SUCCESS | FAILED).
- **Transaction_Logger**: Interface `TransactionLogger` di `internal/domain/contract/service/transaction_logger.go` yang mendefinisikan kontrak akses data transaksi ke PostgreSQL.
- **Check_Order_Status_API**: Fungsi `CheckOrderStatusOnConsume` di `pkg/telkomsel/check_order_status_consume.go` yang memanggil Telkomsel ESB Check Order Status endpoint.
- **Retry_Config**: Struct `RetryConfig` di `internal/config/config.go` yang menyimpan `MaxAttempts` dan `WaitDuration` untuk konfigurasi retry.
- **RC_PPS**: Response Code PPS, hasil resolusi dari error mapping. Nilai 0 = sukses, 1 = gagal, 9 = pending/unknown.
- **Downstream_MQ**: RabbitMQ tujuan untuk mempublikasikan status transaksi ke sistem upstream (PPS).
- **Pending_Status**: Status transaksi di Transaction_Table yang bernilai "PROCESSING", menandakan transaksi belum mendapat resolusi final dari callback maupun check status.
- **Resolved_Status**: Status transaksi di Transaction_Table yang bernilai "SUCCESS" atau "FAILED", menandakan transaksi sudah mendapat resolusi final.
- **Msg_ID**: Primary key di Transaction_Table, identifier unik untuk setiap transaksi.

## Requirements

### Requirement 1: Lookup Status Transaksi berdasarkan Msg_ID

**User Story:** Sebagai developer, saya ingin bisa mengambil status transaksi dari Transaction_Table berdasarkan Msg_ID, sehingga goroutine check status bisa menentukan apakah perlu memanggil Check_Order_Status_API atau tidak.

#### Acceptance Criteria

1. THE Transaction_Logger SHALL menyediakan method `GetTransactionStatusByMsgID` yang menerima `context.Context` dan `msgID string` sebagai parameter dan mengembalikan `(string, error)`.
2. WHEN `GetTransactionStatusByMsgID` dipanggil dengan Msg_ID yang valid dan ada di Transaction_Table, THE Transaction_Logger SHALL mengembalikan nilai kolom `status` dari baris yang sesuai.
3. WHEN `GetTransactionStatusByMsgID` dipanggil dengan Msg_ID yang tidak ditemukan di Transaction_Table, THE Transaction_Logger SHALL mengembalikan error yang mengindikasikan data tidak ditemukan.
4. IF terjadi error koneksi database saat eksekusi `GetTransactionStatusByMsgID`, THEN THE Transaction_Logger SHALL mengembalikan error yang membungkus error asli dari database.

### Requirement 2: Goroutine Check Status pada Paket Data (rcPPS == 0 dan rcPPS == 9)

**User Story:** Sebagai developer, saya ingin Consumer_Service meluncurkan goroutine check status yang sama (dengan pengecekan DB sebelum API call) untuk flow paket data baik saat rcPPS == 0 maupun rcPPS == 9, sehingga kedua kondisi menggunakan logic yang konsisten dan menghindari API call yang tidak perlu.

#### Acceptance Criteria

1. WHEN flow paket data OrderDealer menghasilkan rcPPS == 0 dan publish downstream dengan `StatusToBeProcess` berhasil, THE Consumer_Service SHALL meluncurkan goroutine check status baru (dengan pengecekan status di Transaction_Table sebelum memanggil Check_Order_Status_API) secara asinkron.
2. WHEN flow paket data OrderDealer menghasilkan rcPPS == 9 (pending), THE Consumer_Service SHALL meluncurkan goroutine check status baru yang sama (dengan pengecekan status di Transaction_Table sebelum memanggil Check_Order_Status_API) secara asinkron, menggantikan pemanggilan `retryCheckStatus` yang sebelumnya digunakan.
3. THE Consumer_Service SHALL meluncurkan goroutine tersebut secara non-blocking sehingga consumer tetap bisa memproses pesan berikutnya.
4. THE Consumer_Service SHALL menggunakan `context.Background()` sebagai context untuk goroutine, sehingga goroutine tidak dibatalkan saat parent context selesai.
5. THE Consumer_Service SHALL menggunakan function goroutine check status yang sama untuk rcPPS == 0 dan rcPPS == 9, sehingga logic pengecekan DB dan retry konsisten di kedua kondisi.

### Requirement 3: Sleep sebelum Setiap Iterasi Check Status

**User Story:** Sebagai developer, saya ingin goroutine menunggu selama `WaitDuration` sebelum setiap iterasi pengecekan, sehingga ada jeda waktu yang cukup bagi callback untuk menyelesaikan transaksi.

#### Acceptance Criteria

1. WHILE goroutine check status berjalan, THE Consumer_Service SHALL menunggu selama `WaitDuration` dari Retry_Config sebelum setiap iterasi pengecekan status.
2. IF Retry_Config bernilai nil, THEN THE Consumer_Service SHALL memperlakukan transaksi sebagai FAILED tanpa melakukan retry dan mempublikasikan status `StatusToBeCancel` ke Downstream_MQ.

### Requirement 4: Pengecekan Status di Database sebelum API Call

**User Story:** Sebagai developer, saya ingin goroutine mengecek status transaksi di Transaction_Table terlebih dahulu sebelum memanggil Check_Order_Status_API, sehingga API call yang tidak perlu bisa dihindari jika callback sudah menyelesaikan transaksi.

#### Acceptance Criteria

1. WHEN goroutine check status memulai iterasi pengecekan, THE Consumer_Service SHALL memanggil `GetTransactionStatusByMsgID` pada Transaction_Logger dengan Msg_ID transaksi yang sedang diproses.
2. WHEN status transaksi di Transaction_Table bernilai Resolved_Status ("SUCCESS" atau "FAILED"), THE Consumer_Service SHALL menghentikan goroutine tanpa memanggil Check_Order_Status_API dan tanpa mempublikasikan pesan tambahan ke Downstream_MQ.
3. WHEN status transaksi di Transaction_Table bernilai Pending_Status ("PROCESSING"), THE Consumer_Service SHALL melanjutkan untuk memanggil Check_Order_Status_API.
4. IF terjadi error saat memanggil `GetTransactionStatusByMsgID`, THEN THE Consumer_Service SHALL mencatat error ke log dan melanjutkan untuk memanggil Check_Order_Status_API sebagai fallback.

### Requirement 5: Pemanggilan Check Order Status API dan Resolusi Hasil

**User Story:** Sebagai developer, saya ingin goroutine memanggil Check_Order_Status_API dan memproses hasilnya berdasarkan RC_PPS, sehingga status transaksi bisa diperbarui dan dipublikasikan ke downstream sesuai hasil pengecekan.

#### Acceptance Criteria

1. WHEN status di Transaction_Table masih Pending_Status, THE Consumer_Service SHALL memanggil `CheckOrderStatusOnConsume` dengan parameter msisdn, mid, queueName, msgID, originalTransactionID, dan serialNumber.
2. WHEN Check_Order_Status_API mengembalikan RC_PPS == 0 (sukses), THE Consumer_Service SHALL memperbarui status transaksi di Transaction_Table menjadi "SUCCESS" dan mempublikasikan status `StatusToBeFinish` ke Downstream_MQ.
3. WHEN Check_Order_Status_API mengembalikan RC_PPS == 1 (gagal), THE Consumer_Service SHALL memperbarui status transaksi di Transaction_Table menjadi "FAILED" dan mempublikasikan status `StatusToBeCancel` ke Downstream_MQ.
4. WHEN Check_Order_Status_API mengembalikan RC_PPS == 9 (pending), THE Consumer_Service SHALL melanjutkan ke iterasi berikutnya.
5. IF Check_Order_Status_API mengembalikan error tanpa response yang bisa di-parse, THEN THE Consumer_Service SHALL mencatat error ke log dan melanjutkan ke iterasi berikutnya.

### Requirement 6: Batas Maksimum Retry dan Penanganan Timeout

**User Story:** Sebagai developer, saya ingin goroutine memiliki batas maksimum percobaan, sehingga transaksi yang tidak pernah mendapat resolusi final tidak menggantung selamanya.

#### Acceptance Criteria

1. THE Consumer_Service SHALL melakukan iterasi check status maksimal sebanyak `MaxAttempts` dari Retry_Config.
2. WHEN jumlah iterasi mencapai `MaxAttempts` dan status transaksi masih Pending_Status, THE Consumer_Service SHALL memperbarui status transaksi di Transaction_Table menjadi "FAILED".
3. WHEN jumlah iterasi mencapai `MaxAttempts` dan status transaksi masih Pending_Status, THE Consumer_Service SHALL mempublikasikan status `StatusToBeCancel` ke Downstream_MQ dengan pesan tambahan "pending: max retry reached".

### Requirement 7: Logging pada Setiap Tahap Goroutine

**User Story:** Sebagai developer, saya ingin setiap tahap penting dalam goroutine check status dicatat ke log, sehingga proses bisa di-debug dan dimonitor dengan mudah.

#### Acceptance Criteria

1. WHEN goroutine check status dimulai, THE Consumer_Service SHALL mencatat log level Info dengan informasi queue, msisdn, mid, dan msgid.
2. WHEN goroutine mendeteksi status Resolved_Status di Transaction_Table, THE Consumer_Service SHALL mencatat log level Info yang menyebutkan status yang ditemukan dan bahwa goroutine dihentikan.
3. WHEN goroutine memanggil Check_Order_Status_API, THE Consumer_Service SHALL mencatat log level Info dengan nomor attempt dan max_attempts.
4. WHEN goroutine mencapai MaxAttempts, THE Consumer_Service SHALL mencatat log level Warn yang menyebutkan bahwa max attempts tercapai dan transaksi diperlakukan sebagai FAILED.
5. IF terjadi error pada `GetTransactionStatusByMsgID` atau Check_Order_Status_API, THEN THE Consumer_Service SHALL mencatat log level Error dengan detail error.

### Requirement 8: Dukungan Status "PROCESSING" pada UpdateTransactionStatus

**User Story:** Sebagai developer, saya ingin `UpdateTransactionStatus` mendukung status "PROCESSING" selain "SUCCESS" dan "FAILED", sehingga status transaksi bisa diperbarui ke PROCESSING jika diperlukan oleh flow paket data.

#### Acceptance Criteria

1. WHEN `UpdateTransactionStatus` dipanggil dengan status "PROCESSING", THE Transaction_Logger SHALL memperbarui kolom `status` menjadi "PROCESSING" dan kolom `updated_at` menjadi waktu saat ini di Transaction_Table.
2. THE Transaction_Logger SHALL tetap mendukung status "SUCCESS" dan "FAILED" seperti sebelumnya tanpa perubahan perilaku.
3. IF `UpdateTransactionStatus` dipanggil dengan status selain "PROCESSING", "SUCCESS", atau "FAILED", THEN THE Transaction_Logger SHALL mengembalikan error yang menyebutkan status tidak valid.
