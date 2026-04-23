# Dokumen Requirements (Updated)

## Pendahuluan

Dokumen ini mendefinisikan requirements untuk alur event-driven FIFO pada Payment Processing System (PPS). Arsitektur telah disederhanakan dari desain awal — **Publisher-Database sudah dihapus**, gateway sekarang publish langsung ke RabbitMQ, dan **Consumer-Database sudah di-retire** karena tanggung jawabnya ditangani oleh Consumer.

### Perubahan Arsitektur dari Desain Awal

| Komponen | Desain Awal | Arsitektur Sekarang |
|---|---|---|
| Gateway → Downstream | HTTP POST ke Publisher-Database | RabbitMQ publish langsung via MQPublisher |
| Publisher-Database | Menerima callback, publish ke RabbitMQ | **Dihapus** — tidak ada lagi perantara |
| Consumer-Database | Consume PROVIDER, call SP | **Di-retire** — Consumer handle langsung |
| Callback Telkomsel | Service terpisah | Endpoint `GET /callback/ext` di gateway-telkomsel |
| Format pesan PROVIDER | Flat JSON | Wrapper `{"source":"PROVIDER","data":{...}}` |

## Glossary

- **Consumer**: Service pps-services-consumer yang mengkonsumsi pesan dari RabbitMQ secara FIFO (prefetch=1, manual ack) dan memproses transaksi di Oracle
- **Publisher_Provider**: Service pps-services-publisher-provider yang menerima HTTP request dan mempublish pesan ke queue RabbitMQ spesifik provider berdasarkan remarkImsi
- **Gateway_Telkomsel**: Service pps-services-gateway-telkomsel yang mengkonsumsi pesan dari queue provider Telkomsel, memanggil Telkomsel API, dan publish status akhir langsung ke RabbitMQ
- **Gateway_Unipin**: Service pps-services-gateway-unipin yang mengkonsumsi pesan dari queue provider Unipin, memanggil Unipin API, dan publish status akhir langsung ke RabbitMQ
- **MQPublisher**: Komponen di gateway yang publish pesan langsung ke RabbitMQ menggunakan MQTransaction URL dan QueueName dari payload transaksi
- **PublishMessage**: Struktur JSON pesan RabbitMQ wrapper `{"source":"...", "data":{...}}`
- **Source_Field**: Field `source` dalam PublishMessage: "PRE-ORDER" untuk transaksi baru, "PROVIDER" untuk update status final
- **ProviderPublishMessage**: Format wrapper `{"source":"PROVIDER","data":{...}}` yang dipublish oleh gateway ke RabbitMQ
- **SP_SetTransactionStatus**: Stored procedure Oracle MSG.SETTRANSACTIONSTATUS yang mengupdate status final transaksi

---

## Requirements

### Requirement 1: Source-Based Routing di Consumer ✅ IMPLEMENTED

**Status:** Sudah diimplementasikan di `consumerTunning.go` — `extractSource()`, `ProsesDataFIFO()`, `processPreOrder()`, `processProvider()`, `processLegacy()`.

#### Acceptance Criteria

1. ✅ WHEN Consumer menerima pesan dengan Source_Field "PRE-ORDER", THE Consumer SHALL memanggil SP_UpdPreOrderConsume, SP_SellWithId, lalu HTTP call ke Publisher_Provider
2. ✅ WHEN Consumer menerima pesan dengan Source_Field "PROVIDER", THE Consumer SHALL memanggil SP_SetTransactionStatus
3. ✅ WHEN Consumer menerima pesan dengan Source_Field kosong/tidak dikenali, THE Consumer SHALL memproses menggunakan logik legacy
4. ✅ IF Consumer gagal mem-parse Source_Field, THE Consumer SHALL fallback ke legacy flow
5. ✅ THE Consumer SHALL mempertahankan mekanisme FIFO (prefetch=1, manual ack)

---

### Requirement 2: HTTP Client Consumer ke Publisher_Provider ✅ IMPLEMENTED

**Status:** Sudah diimplementasikan di `publisherClient.go` — `CallPublisherProvider()`.

#### Acceptance Criteria

1. ✅ WHEN Consumer selesai memproses PRE-ORDER, THE Consumer SHALL mengirim HTTP POST ke Publisher_Provider `/api/v1/publish`
2. ✅ THE Consumer SHALL memetakan data ke format PublishProviderRequest
3. ✅ IF HTTP call return non-2xx, THE Consumer SHALL log error
4. ✅ IF timeout/connection error, THE Consumer SHALL retry dengan exponential backoff
5. ✅ THE Consumer SHALL membaca base URL dari environment variable
6. ✅ IF SP_SellWithId error, THE Consumer SHALL skip HTTP call

---

### Requirement 3: Gateway Telkomsel Forward Status ke RabbitMQ ✅ IMPLEMENTED (UPDATED)

**Status:** Sudah diimplementasikan — gateway publish langsung ke RabbitMQ via MQPublisher (bukan HTTP POST ke Publisher-Database).

#### Acceptance Criteria (Updated)

1. ✅ WHEN Gateway_Telkomsel menerima response dari Telkomsel API, THE Gateway SHALL publish status ke RabbitMQ via MQPublisher menggunakan MQTransaction URL dan QueueName dari payload
2. ✅ THE Gateway SHALL publish dalam format wrapper `{"source":"PROVIDER","data":{...}}`
3. ✅ THE Gateway SHALL menyertakan field source="PROVIDER" dan queue_name dalam pesan
4. ✅ IF publish ke RabbitMQ gagal, THE Gateway SHALL log error dan tetap lanjut (non-blocking)
5. ✅ Callback endpoint `GET /callback/ext` sudah diimplementasikan di gateway-telkomsel untuk async callback paket data

---

### Requirement 4: Gateway Unipin Forward Status ke RabbitMQ ✅ IMPLEMENTED (UPDATED)

**Status:** Sudah diimplementasikan — gateway publish langsung ke RabbitMQ via MQPublisher.

#### Acceptance Criteria (Updated)

1. ✅ WHEN Gateway_Unipin menerima response dari Unipin API, THE Gateway SHALL publish status ke RabbitMQ via MQPublisher
2. ✅ THE Gateway SHALL publish dalam format wrapper `{"source":"PROVIDER","data":{...}}`
3. ✅ THE Gateway SHALL menyertakan field source="PROVIDER" dan queue_name
4. ✅ IF publish ke RabbitMQ gagal, THE Gateway SHALL log error dan tetap lanjut (non-blocking)

---

### Requirement 5: SP_SetTransactionStatus di Consumer ✅ IMPLEMENTED

**Status:** Sudah diimplementasikan di `oracle.go` — `SetTransactionStatus()`.

#### Acceptance Criteria

1. ✅ WHEN Consumer menerima pesan PROVIDER, THE Consumer SHALL memanggil SP_SetTransactionStatus
2. ✅ THE Consumer SHALL mem-parse field dari pesan PROVIDER
3. ✅ IF SP return OutError != 0, THE Consumer SHALL log warning
4. ✅ IF connection error, THE Consumer SHALL reset pool dan retry
5. ✅ THE Consumer SHALL Ack pesan setelah SP selesai

---

### Requirement 6: Retire Consumer-Database ✅ IMPLEMENTED

**Status:** Consumer-Database sudah deprecated. Consumer handle semua pemrosesan PROVIDER.

---

### Requirement 7: Model Pesan PROVIDER di Consumer ✅ IMPLEMENTED (UPDATED)

**Status:** Sudah diimplementasikan dengan support format wrapper `{"source":"PROVIDER","data":{...}}` dan backward compatibility format flat.

#### Acceptance Criteria (Updated)

1. ✅ THE Consumer SHALL mem-parse pesan PROVIDER dalam format wrapper `{"source":"PROVIDER","data":{...}}`
2. ✅ THE Consumer SHALL tetap support format flat (backward compatibility) via auto-detection di `parseProviderMessage()`
3. ✅ IF pesan tidak dapat di-parse, THE Consumer SHALL Nack dengan requeue=false
