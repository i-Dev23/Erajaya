# Flow Gateway UniPin

Dokumen ini menjelaskan alur utama Gateway UniPin dari konsumsi pesan RabbitMQ hingga publish status ke downstream.

## Ringkasan Alur

```mermaid
flowchart TD
  A[Consume message from RabbitMQ] --> B[Parse payload (alias/casing tolerant)]
  B --> C[Resolve tx type]
  C -->|type_voucher preferred| D{tx type}
  C -->|fallback product_type| D

  D -->|GAME-VOUCHER| E[VoucherRequest]
  D -->|GAME-DIRECT-TOP-UP| F[Game top up flow]
  D -->|Other/unknown| Z[Log warning + stop]

  E --> G[Map result to status_to_be (F/C)]
  F --> H[Order + OrderInquiry]
  H --> I{Inquiry final?}
  I -->|Final| G
  I -->|Pending| J[Publish processing (S)]
  J --> K[Retry inquiry async (configurable)]
  K --> I

  G --> L[Build downstream message]
  L --> M[Set correlation: conversation_id & original_conversation_id = msgid string]
  M --> N[Publish to downstream MQ (no QueueDeclare)]
  N --> O[mandatory publish + NotifyReturn for unroutable]
```

## Aturan Penting

- `type_voucher` adalah sumber kebenaran untuk menentukan jenis transaksi. `product_type` dipakai hanya sebagai fallback untuk backward-compat.
- Standar `status_to_be` untuk downstream:
  - `F` = finish/success
  - `C` = cancel/failed
  - `S` = still processing
- Field korelasi downstream:
  - `conversation_id` dan `original_conversation_id` diisi dari `msgid` (string). Jika `msgid` kosong, fallback ke `delivery.MessageId`.
- MQ publish ke downstream tidak melakukan `QueueDeclare` untuk menghindari mismatch property queue; gunakan publish `mandatory=true` dan cek `NotifyReturn` untuk unroutable.
