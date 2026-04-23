# Unit Test Tasks (5 repos)

Dokumen ini rangkum gap unit test berdasarkan:
- Inventaris `*_test.go`
- Output `go test ./...` per repo (packages yang masih `[no test files]`)

Scope dokumen ini hanya **unit test**. Live/integration test yang sudah ada di repo **diabaikan** (tidak dilanjutkan).

---

## pps-services-gateway-telkomsel

**Status singkat**
- `go test ./...` ✅
- Package tanpa test: `internal/util`, `cmd/app`, `internal/domain/contract/service`

**Tasks**
- [ ] Tambah unit test untuk generator pesan di `internal/util/generate_message_util.go`:
  - status `F` menghasilkan string sukses dan include serial number.
  - status `C` menghasilkan string gagal dan include `Status Code`.
  - status `S` memakai `PROCESSING_MESSAGE` (env) dan fallback ke default bila env kosong.
- [ ] Tambah test callback handler untuk case `FAILED`:
  - pastikan mapping downstream `status_to_be = "C"` (normalisasi dari `FAILED`).
  - pastikan `StatusCode` yang disimpan/pakai untuk pesan adalah `"1"`.
  - (opsional) assert `message_to_customer` mengandung kata `GAGAL`.

---

## pps-services-gateway-unipin

**Status singkat**
- `go test ./...` ✅
- Package tanpa test: `internal/util`, `internal/config`, `internal/delivery/http/handler`, `internal/infrastructure/postgres`, `internal/infrastructure/scheduler`, `cmd/app`, `internal/domain/contract/*`

**Tasks**
- [ ] Tambah unit test untuk `internal/util/generate_message_util.go` (paritas dengan Telkomsel): `F/C/S`, env `PROCESSING_MESSAGE`, dan formatting ref/status code.
- [ ] Tambah unit test untuk HTTP handler di `internal/delivery/http/handler`:
  - validasi request payload (missing/invalid fields).
  - response code & body untuk sukses / gagal.
- [ ] Tambah unit test untuk scheduler (jika ada job/trigger logic) di `internal/infrastructure/scheduler`:
  - memastikan job tidak jalan ketika context canceled.
  - memastikan error path di-log dan tidak panic.

---

## pps-services-consumer

**Status singkat**
- `go test ./...` ✅
- Test yang ada: `model`, `repository`.
- Package tanpa test: root package, `constanta`, `database`, `util`.

**Tasks**
- [ ] Tambah unit test untuk util Telegram di `util/telegram.go`:
  - `ComposeMessageTelegramNotification` membentuk message sesuai format (cek prefix environment + error message).
  - `CharLimiter` untuk input < limit dan > limit.
    - Catatan: implementasi saat ini memakai `io.ReadAtLeast` dan mengembalikan `string(buff)` yang bisa mengandung `\x00` bila string lebih pendek; test akan meng-expose ini (bisa jadi follow-up fix).
- [ ] Tambah unit test untuk logger helper di `util/logger.go` (minimal: tidak panic; format output konsisten jika ada).
- [ ] Tambah unit test untuk `database/dbpool.go` (jika memungkinkan):
  - minimal test bahwa fungsi init/close tidak panic.
  - idealnya refactor kecil agar dependency DB bisa di-mock (sqlmock) sehingga error-path bisa dites.

---

## pps-services-publisher

**Status singkat**
- `go test ./...` ❌ (FAIL)
- Package tanpa test: root, `constanta`, `database`, `model`, `util`.

**Tasks**
- [ ] Tambah unit test untuk logika repository yang tidak harus konek DB:
  - `isConnectionError(err)` table-driven test (berbagai error message).
- [ ] Tambah unit test untuk `util/telegram.go` terutama `CharLimiter` (lihat catatan `\x00` yang sama seperti consumer).
- [ ] Jika memungkinkan, refactor repository agar DB access bisa di-mock (`sqlmock`) untuk menguji:
  - error-path (Exec gagal, connection error → ResetPool dipanggil).
  - mapping output parameter.

---

## pps-services-publisher-provider

**Status singkat**
- `go test ./...` ✅
- Package tanpa test: `internal/config`, `internal/delivery/http/router`, `internal/domain/contract/*`, `internal/domain/entity`, `internal/dto/request`, `internal/dto/response`, `internal/mocks`.

**Tasks**
- [ ] Tambah unit test untuk `internal/config` (parsing env, default value, validation required fields).
- [ ] Tambah unit test untuk router wiring `internal/delivery/http/router`:
  - route ter-register sesuai yang diharapkan.
  - handler dipanggil untuk endpoint utama (bisa pakai httptest + in-memory server).
- [ ] (Opsional) Tambah test ringan untuk DTO request/response (json marshal/unmarshal) jika ada field/format kritikal.
