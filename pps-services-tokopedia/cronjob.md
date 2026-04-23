# Cron Jobs (Scheduler) — PPS Services Tokopedia

Dokumen ini merangkum semua scheduled job (cron) yang didaftarkan oleh service ini.

## Format cron
Scheduler menggunakan `robfig/cron` dengan `WithSeconds()`, jadi format cron harus **6 field**:

`second minute hour day-of-month month day-of-week`

Contoh:
- Setiap hari jam 14:35: `0 35 14 * * *`
- Setiap 5 menit: `0 */5 * * * *`

## Global switch
Semua job di bawah hanya akan di-setup bila:
- `RUN_JOBS_LOG_RETENTION=Y`

Jika tidak, service akan log: *Log retention jobs are disabled by env*.

## Daftar cron job

### 1) HTTP logs cleanup
- ENV schedule: `HTTP_LOG_RETENTION_CRON`
- Default schedule: `0 0 1 * * *` (setiap hari 01:00)
- Retention (days): `HTTP_LOG_RETENTION_DAYS` (default `31`)
- Aksi: hapus HTTP logs yang lebih lama dari N hari.

### 2) Callback logs cleanup
- ENV schedule: `CALLBACK_LOG_RETENTION_CRON`
- Default schedule: `0 10 1 * * *` (setiap hari 01:10)
- Retention (days): `CALLBACK_LOG_RETENTION_DAYS` (default `31`)
- Aksi: hapus callback logs yang lebih lama dari N hari.

### 3) Inquiry logs cleanup
- ENV schedule: `INQUIRY_LOG_RETENTION_CRON`
- Default schedule: `0 20 1 * * *` (setiap hari 01:20)
- Retention (days, shared): `INQUIRY_AND_PAYMENT_LOG_RETENTION_DAYS` (default `31`)
- Aksi: hapus inquiry logs yang lebih lama dari N hari.

### 4) Payment logs cleanup
- ENV schedule: `PAYMENT_LOG_RETENTION_CRON`
- Default schedule: `0 30 1 * * *` (setiap hari 01:30)
- Retention (days, shared): `INQUIRY_AND_PAYMENT_LOG_RETENTION_DAYS` (default `31`)
- Aksi: hapus payment logs yang lebih lama dari N hari.

### 5) File logs cleanup
- ENV schedule: `FILE_LOG_RETENTION_CRON`
- Default schedule: `0 40 1 * * *` (setiap hari 01:40)
- Retention (days): `FILE_LOG_RETENTION_DAYS` (default `50`)
- Aksi: hapus file log lama di filesystem.

### 6) Partition maintenance
- ENV schedule: `PARTITION_MAINTENANCE_CRON`
- Default schedule: `0 0 2 * * *` (setiap hari 02:00)
- Aksi: menjalankan query:
  - `SELECT maintenance.ensure_monthly_partitions(2)`
- Catatan:
  - Jika Postgres tidak tersedia dan service sedang pakai mock Postgres, job akan **skip** (log level `WARN`).

### 7) Reconciliation report export
- ENV schedule: `RECONCILIATION_EXPORT_CRON`
- Default schedule: `0 0 1 * * *` (setiap hari 01:00)
- ENV tambahan:
  - `RECONCILIATION_REPORT_DATE` (opsional) — default: kemarin (`YYYY-MM-DD`)
  - `RECONCILIATION_OUTPUT_DIR` (opsional) — default: `./reports`
- Aksi:
  - Query: `SELECT * FROM payment.get_daily_reconciliation_report($1)`
  - Output: file CSV `reconcile_pps_YYYYMMDD.csv`
- Catatan:
  - Jika Postgres tidak tersedia dan service sedang pakai mock Postgres, job akan **skip** (log level `WARN`).

## Contoh set ENV (PowerShell)
Jalankan partition maintenance tiap hari jam 14:35:

```powershell
$env:RUN_JOBS_LOG_RETENTION = "Y"
$env:PARTITION_MAINTENANCE_CRON = "0 35 14 * * *"
```

> Setelah set ENV, restart service supaya scheduler membaca nilai terbaru.
