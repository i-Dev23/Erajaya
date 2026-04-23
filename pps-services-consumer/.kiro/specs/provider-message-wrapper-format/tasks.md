# Rencana Implementasi: Provider Message Wrapper Format

## Overview

Implementasi dukungan format wrapper untuk pesan PROVIDER di `pps-services-consumer`. Perubahan minimal: tambah struct `ProviderWrapperMessage` di `model/model.go`, tambah fungsi helper `parseProviderMessage()` di `repository/consumerTunning.go`, dan update `processProvider()` untuk auto-deteksi format wrapper vs flat. Backward compatibility dengan format flat tetap terjaga.

## Tasks

- [x] 1. Tambahkan struct `ProviderWrapperMessage` di `model/model.go`
  - Tambahkan struct `ProviderWrapperMessage` dengan field `Source` (string, json tag `source`) dan `Data` (ProviderMessage, json tag `data`)
  - Tambahkan komentar dokumentasi yang menjelaskan bahwa struct ini adalah envelope untuk format wrapper pesan PROVIDER
  - _Requirements: 1.1, 1.2_

  - [ ]* 1.1 Tulis unit test untuk `ProviderWrapperMessage` di `model/model_test.go`
    - `TestProviderWrapperMessageJSONUnmarshal` — unmarshal JSON wrapper valid, verifikasi field `Source` dan semua field di `Data` terisi benar
    - `TestProviderWrapperMessageJSONMarshal` — marshal struct ke JSON, verifikasi key `source` dan `data` ada dengan nilai benar
    - _Requirements: 1.1, 1.2_

  - [ ]* 1.2 Tulis property test untuk round-trip serialization `ProviderWrapperMessage`
    - **Property 1: Round-Trip Serialization ProviderWrapperMessage**
    - **Validates: Requirements 1.3**
    - Untuk sembarang `ProviderWrapperMessage` yang valid, marshal ke JSON lalu unmarshal kembali harus menghasilkan struct identik
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: provider-message-wrapper-format, Property 1: Round-Trip Serialization ProviderWrapperMessage`

- [x] 2. Implementasikan `parseProviderMessage()` dan update `processProvider()` di `repository/consumerTunning.go`
  - [x] 2.1 Tambahkan fungsi `parseProviderMessage(data string) (model.ProviderMessage, string, error)`
    - Coba unmarshal ke `ProviderWrapperMessage` terlebih dahulu
    - Jika `Data.MsgId != 0` → return `Data`, `"wrapper"`, `nil`
    - Fallback: unmarshal langsung ke `ProviderMessage` → return result, `"flat"`, `nil`
    - Jika keduanya gagal → return zero-value, `""`, error
    - _Requirements: 2.1, 3.1, 4.1, 4.2, 4.3_

  - [x] 2.2 Update fungsi `processProvider()` untuk menggunakan `parseProviderMessage()`
    - Ganti unmarshal langsung dengan panggilan `parseProviderMessage(data)`
    - Tambahkan log format yang terdeteksi (`wrapper` atau `flat`) beserta `msg_id`
    - Pertahankan behavior Ack/Nack yang sama: error → `resultNackDiscard`, sukses → `resultOK`
    - _Requirements: 2.2, 3.2, 5.1, 5.2, 5.3_

- [x] 3. Checkpoint — Verifikasi kompilasi dan fungsi dasar
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Pastikan semua test lulus, tanyakan ke user jika ada pertanyaan.

- [ ] 4. Tulis test untuk `parseProviderMessage()` dan `processProvider()` di `repository/consumerTunning_test.go`
  - [ ] 4.1 Buat file `repository/consumerTunning_test.go` dengan unit test
    - `TestParseProviderMessage_WrapperFormat` — input JSON wrapper valid, verifikasi return ProviderMessage lengkap dan format `"wrapper"` (Req 2.1)
    - `TestParseProviderMessage_FlatFormat` — input JSON flat valid, verifikasi return ProviderMessage lengkap dan format `"flat"` (Req 3.1)
    - `TestParseProviderMessage_InvalidJSON_ReturnsError` — input JSON invalid, verifikasi return error (Req 3.3)
    - `TestParseProviderMessage_EmptyDataField_FallbackFlat` — wrapper dengan `data` kosong, verifikasi fallback ke flat (Req 4.3)
    - _Requirements: 2.1, 2.3, 3.1, 3.3, 4.3_

  - [ ]* 4.2 Tulis property test untuk ekuivalensi format (Property 2)
    - **Property 2: Ekuivalensi Format — Wrapper dan Flat Menghasilkan ProviderMessage Identik**
    - **Validates: Requirements 6.3, 6.1, 6.2, 2.1, 3.1**
    - Untuk sembarang data ProviderMessage valid, encode sebagai wrapper dan flat, lalu `parseProviderMessage` harus menghasilkan ProviderMessage identik (kecuali field `Source`)
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: provider-message-wrapper-format, Property 2: Ekuivalensi Format`

  - [ ]* 4.3 Tulis property test untuk auto-deteksi format (Property 3)
    - **Property 3: Auto-Deteksi Format yang Benar**
    - **Validates: Requirements 4.1, 4.2, 4.3**
    - Untuk sembarang data ProviderMessage valid, encode sebagai wrapper → format harus `"wrapper"`, encode sebagai flat → format harus `"flat"`
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: provider-message-wrapper-format, Property 3: Auto-Deteksi Format yang Benar`

  - [ ]* 4.4 Tulis property test untuk data wrapper invalid (Property 4)
    - **Property 4: Data Wrapper Invalid Menghasilkan Error**
    - **Validates: Requirements 2.3, 3.3**
    - Untuk sembarang JSON dengan `"source": "PROVIDER"` tetapi `data` berisi non-objek (string, number, array, null) atau JSON sepenuhnya invalid, `parseProviderMessage` harus return error atau fallback flat yang juga gagal
    - Gunakan `testing/quick` dengan minimum 100 iterasi
    - Tag: `// Feature: provider-message-wrapper-format, Property 4: Data Wrapper Invalid Menghasilkan Error`

- [x] 5. Checkpoint akhir — Pastikan semua test lulus
  - Jalankan `go build ./...` untuk memastikan tidak ada compile error
  - Jalankan `go vet ./...` untuk memastikan tidak ada issue
  - Pastikan semua test lulus dengan `go test ./...`
  - Tanyakan ke user jika ada pertanyaan sebelum selesai.

## Catatan

- Task bertanda `*` bersifat opsional dan dapat dilewati untuk MVP yang lebih cepat
- Setiap task mereferensikan requirements spesifik untuk traceability
- Checkpoint memastikan validasi inkremental di setiap tahap
- Property test memvalidasi correctness properties universal dari design document
- Unit test memvalidasi contoh spesifik dan edge case
- Perubahan sangat minimal: hanya 2 file dimodifikasi (`model.go`, `consumerTunning.go`) dan 1 file test baru (`consumerTunning_test.go`)
