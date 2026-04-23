# Rencana Implementasi: Complete Event-Driven Flow (Updated)

## Overview

Semua task utama sudah diimplementasikan. Arsitektur telah disederhanakan — Publisher-Database dihapus, gateway publish langsung ke RabbitMQ via MQPublisher. Task 6 dan 7 (DownstreamClient) sudah diganti dengan implementasi MQPublisher yang lebih sederhana.

## Tasks

- [x] 1. Tambah data model dan konstanta baru di Consumer
  - [x] 1.1 Struct `ProviderMessage`, `PublishProviderRequest`, `ProviderWrapperMessage` di model.go
  - [x] 1.3 Konstanta environment variable di viperConstanta.go

- [x] 2. Implementasi source-based routing di Consumer
  - [x] 2.1 `extractSource()`, `ProsesDataFIFO()` routing, `processPreOrder()`, `processProvider()`, `processLegacy()`

- [x] 3. Implementasi HTTP client Consumer ke Publisher-Provider
  - [x] 3.1 `CallPublisherProvider()` di publisherClient.go
  - [x] 3.2 Integrasi di flow PRE-ORDER

- [x] 4. Implementasi SP_SetTransactionStatus di Consumer
  - [x] 4.1 `SetTransactionStatus()` di oracle.go

- [x] 5. Checkpoint Consumer ✅

- [x] 6. Gateway Telkomsel — Forward Status ke RabbitMQ (UPDATED)
  - [x] 6.1 ~~DownstreamClient HTTP POST~~ → MQPublisher publish langsung ke RabbitMQ
  - [x] 6.2 Format wrapper `{"source":"PROVIDER","data":{...}}`
  - [x] 6.3 Transaction logging ke Postgres (telkomsel_transaction + telkomsel_transaction_response)
  - [x] 6.4 Callback endpoint `GET /callback/ext` untuk async paket data
  - [x] 6.5 HTTP server + RabbitMQ consumer via errgroup

- [x] 7. Gateway Unipin — Forward Status ke RabbitMQ (UPDATED)
  - [x] 7.1 ~~DownstreamClient HTTP POST~~ → MQPublisher publish langsung ke RabbitMQ
  - [x] 7.2 Format wrapper `{"source":"PROVIDER","data":{...}}`
  - [x] 7.3 Voucher list sync ke Oracle via cron
  - [x] 7.4 MQTransaction field ditambahkan ke consumePayload

- [x] 8. Consumer — Support Wrapper Format PROVIDER
  - [x] 8.1 `ProviderWrapperMessage` struct
  - [x] 8.2 `parseProviderMessage()` dengan auto-detection (wrapper vs flat)
  - [x] 8.3 Backward compatibility format flat

- [x] 9. Deprecate Consumer-Database ✅
  - [x] 9.1 Consumer handle semua pemrosesan PROVIDER
  - [x] 9.2 Publisher-Database dihapus dari arsitektur

- [x] 10. Checkpoint final ✅

## Remaining Work

- [ ] Gateway Unipin — unipin-game transaction flow (spec: unipin-transaction-flow)
- [ ] Gateway Unipin — unipin-voucher command parsing update (spec: unipin-transaction-flow)
- [ ] Gateway Unipin — Transaction logging ke Postgres (next spec)

## Catatan

- DownstreamClient (HTTP POST ke Publisher-Database) sudah dihapus dari kedua gateway
- Publisher-Database sudah tidak ada dalam arsitektur
- Consumer-Database sudah di-retire
- Gateway publish langsung ke RabbitMQ menggunakan MQTransaction URL dari payload
- Dependency `cenkalti/backoff` dan `sony/gobreaker` sudah dihapus dari gateway-unipin
- Dependency `golang-migrate` sudah dihapus dari Dockerfile gateway-telkomsel
