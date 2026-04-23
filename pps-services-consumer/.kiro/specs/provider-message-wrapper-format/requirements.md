# Dokumen Persyaratan (Requirements)

## Pendahuluan

Fitur ini mengubah cara `pps-services-consumer` mem-parsing pesan PROVIDER dari RabbitMQ. Saat ini, pesan PROVIDER dikirim dalam format JSON flat (semua field di level root). Format baru menggunakan struktur wrapper `{"source": "PROVIDER", "data": {...}}` yang konsisten dengan pola yang sudah digunakan untuk pesan PREORDER. Selama masa transisi, consumer harus mendukung kedua format (flat dan wrapper) agar tidak terjadi gangguan layanan.

## Glosarium

- **Consumer**: Aplikasi `pps-services-consumer` yang mengkonsumsi pesan dari RabbitMQ dan memprosesnya
- **RabbitMQ**: Message broker yang digunakan untuk komunikasi antar layanan
- **ProviderMessage**: Struct Go yang merepresentasikan data pesan PROVIDER, berisi field seperti `msg_id`, `status_to_be`, `serial_number`, dll.
- **Wrapper_Format**: Struktur JSON envelope `{"source": "...", "data": {...}}` yang membungkus payload pesan di dalam field `data`
- **Flat_Format**: Struktur JSON lama di mana semua field pesan PROVIDER berada di level root tanpa envelope
- **processProvider**: Fungsi di `consumerTunning.go` yang menangani parsing dan pemrosesan pesan PROVIDER
- **ProviderWrapperMessage**: Struct Go baru yang merepresentasikan format wrapper untuk pesan PROVIDER, berisi field `source` dan `data`
- **SetTransactionStatus**: Stored procedure Oracle (`MSG.SETTRANSACTIONSTATUS`) yang dipanggil untuk mengupdate status transaksi berdasarkan data ProviderMessage
- **extractSource**: Fungsi yang mengekstrak field `source` dari JSON message untuk routing

## Persyaratan

### Persyaratan 1: Definisi Struct Wrapper untuk Pesan PROVIDER

**User Story:** Sebagai developer, saya ingin memiliki struct Go yang merepresentasikan format wrapper pesan PROVIDER, sehingga parsing JSON wrapper dapat dilakukan secara type-safe.

#### Acceptance Criteria

1. THE Consumer SHALL menyediakan struct `ProviderWrapperMessage` di package `model` dengan field `Source` (string, json tag `source`) dan `Data` (ProviderMessage, json tag `data`)
2. WHEN JSON wrapper PROVIDER yang valid di-unmarshal ke `ProviderWrapperMessage`, THE Consumer SHALL menghasilkan struct dengan field `Source` berisi `"PROVIDER"` dan field `Data` berisi ProviderMessage yang lengkap
3. WHEN `ProviderWrapperMessage` di-marshal ke JSON lalu di-unmarshal kembali, THE Consumer SHALL menghasilkan struct yang identik dengan struct asli (round-trip property)

### Persyaratan 2: Parsing Pesan PROVIDER Format Wrapper

**User Story:** Sebagai sistem, saya ingin consumer dapat mem-parsing pesan PROVIDER yang menggunakan format wrapper baru, sehingga consumer kompatibel dengan publisher yang sudah diupdate.

#### Acceptance Criteria

1. WHEN pesan JSON diterima dengan `"source": "PROVIDER"` dan field `data` berisi objek JSON yang valid, THE processProvider SHALL mengekstrak isi field `data` ke dalam struct ProviderMessage
2. WHEN pesan PROVIDER format wrapper berhasil di-parse, THE processProvider SHALL meneruskan ProviderMessage ke SetTransactionStatus dengan semua field yang lengkap dan benar
3. WHEN pesan PROVIDER format wrapper memiliki field `data` yang kosong atau tidak valid, THE processProvider SHALL mengembalikan `resultNackDiscard` dan mencatat log error

### Persyaratan 3: Backward Compatibility dengan Format Flat

**User Story:** Sebagai operator sistem, saya ingin consumer tetap dapat memproses pesan PROVIDER format flat selama masa transisi, sehingga tidak terjadi gangguan layanan saat migrasi publisher.

#### Acceptance Criteria

1. WHEN pesan JSON diterima dengan `"source": "PROVIDER"` dan tanpa field `data` (format flat), THE processProvider SHALL mem-parse pesan tersebut langsung ke struct ProviderMessage
2. WHEN pesan PROVIDER format flat berhasil di-parse, THE processProvider SHALL meneruskan ProviderMessage ke SetTransactionStatus dengan semua field yang lengkap dan benar
3. WHEN pesan PROVIDER format flat memiliki JSON yang tidak valid, THE processProvider SHALL mengembalikan `resultNackDiscard` dan mencatat log error

### Persyaratan 4: Deteksi Otomatis Format Pesan PROVIDER

**User Story:** Sebagai developer, saya ingin consumer secara otomatis mendeteksi apakah pesan PROVIDER menggunakan format wrapper atau flat, sehingga tidak diperlukan konfigurasi manual untuk beralih antar format.

#### Acceptance Criteria

1. WHEN pesan PROVIDER diterima, THE processProvider SHALL mendeteksi format pesan berdasarkan keberadaan field `data` yang berisi objek JSON
2. WHEN field `data` ada dan berisi objek JSON, THE processProvider SHALL memperlakukan pesan sebagai format wrapper
3. WHEN field `data` tidak ada atau bukan objek JSON, THE processProvider SHALL memperlakukan pesan sebagai format flat
4. THE processProvider SHALL memproses kedua format tanpa memerlukan konfigurasi eksternal atau flag toggle

### Persyaratan 5: Logging untuk Traceability

**User Story:** Sebagai operator sistem, saya ingin consumer mencatat log yang jelas tentang format pesan yang diterima, sehingga saya dapat memantau proses migrasi dari format flat ke wrapper.

#### Acceptance Criteria

1. WHEN pesan PROVIDER format wrapper berhasil di-parse, THE Consumer SHALL mencatat log yang menyebutkan format wrapper dan `msg_id` dari pesan tersebut
2. WHEN pesan PROVIDER format flat berhasil di-parse, THE Consumer SHALL mencatat log yang menyebutkan format flat dan `msg_id` dari pesan tersebut
3. IF parsing pesan PROVIDER gagal untuk kedua format, THEN THE Consumer SHALL mencatat log error yang menyertakan data pesan asli dan pesan error yang deskriptif

### Persyaratan 6: Konsistensi Field Setelah Parsing

**User Story:** Sebagai developer, saya ingin memastikan bahwa semua field ProviderMessage terisi dengan benar terlepas dari format pesan yang diterima, sehingga SetTransactionStatus menerima data yang konsisten.

#### Acceptance Criteria

1. WHEN pesan PROVIDER format wrapper di-parse, THE Consumer SHALL menghasilkan ProviderMessage dengan semua 10 field terisi sesuai data di dalam blok `data` (msg_id, status_to_be, serial_number, client_number, nominal, original_conversation_id, conversation_id, message_to_customer, additional_message, queue_name)
2. WHEN pesan PROVIDER format flat di-parse, THE Consumer SHALL menghasilkan ProviderMessage dengan semua 10 field terisi sesuai data di level root JSON
3. FOR ALL pesan PROVIDER yang valid, parsing format wrapper dan parsing format flat dengan data yang sama SHALL menghasilkan ProviderMessage yang identik (kecuali field `source` yang hanya ada di format flat)
