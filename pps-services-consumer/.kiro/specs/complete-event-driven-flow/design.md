# Dokumen Design: Complete Event-Driven Flow (Updated)

## Overview

Dokumen ini mendeskripsikan arsitektur event-driven FIFO pada Payment Processing System (PPS) setelah semua perubahan diimplementasikan. Arsitektur telah disederhanakan dari desain awal — Publisher-Database dihapus, gateway publish langsung ke RabbitMQ.

### Alur Transaksi End-to-End (Current)

```mermaid
sequenceDiagram
    participant PUB as Publisher
    participant RMQ as RabbitMQ (Transaction)
    participant CON as Consumer
    participant ORA as Oracle DB
    participant PP as Publisher-Provider
    participant RMQ2 as RabbitMQ (Provider)
    participant GW as Gateway (Telkomsel/Unipin)
    participant API as Provider API
    
    Note over PUB,CON: Fase 1: PRE-ORDER
    PUB->>RMQ: Publish {source:"PRE-ORDER", data:{...}}
    RMQ->>CON: Consume (FIFO, prefetch=1)
    CON->>ORA: SP updPreOrderConsume
    CON->>ORA: SP request2JualRandomWithID
    CON->>PP: POST /api/v1/publish (PublishProviderRequest)
    PP->>RMQ2: Publish ke queue provider (berdasarkan remarkImsi)
    CON->>RMQ: Ack
    
    Note over GW,RMQ: Fase 2: PROVIDER (Direct RabbitMQ)
    RMQ2->>GW: Consume pesan provider
    GW->>API: Call Provider API
    API-->>GW: Response (sukses/gagal)
    GW->>RMQ: Publish langsung {source:"PROVIDER", data:{...}}
    Note right of GW: Via MQPublisher<br/>menggunakan MQTransaction URL<br/>dan QueueName dari payload
    
    Note over CON,ORA: Fase 3: Update Status Final
    RMQ->>CON: Consume {source:"PROVIDER", data:{...}}
    CON->>ORA: SP SETTRANSACTIONSTATUS
    CON->>RMQ: Ack
```

### Perubahan dari Desain Awal

```mermaid
graph TB
    subgraph "Desain Awal (Outdated)"
        GW1[Gateway] -->|HTTP POST| PDB[Publisher-Database]
        PDB -->|RabbitMQ| CON1[Consumer]
        CD[Consumer-Database] -->|SP| ORA1[Oracle]
    end
    
    subgraph "Arsitektur Sekarang"
        GW2[Gateway] -->|RabbitMQ langsung| CON2[Consumer]
        CON2 -->|SP SetTransactionStatus| ORA2[Oracle]
        Note1[Publisher-Database: DIHAPUS]
        Note2[Consumer-Database: DI-RETIRE]
    end
```

## Architecture

### Komponen dan Status Implementasi

| Service | Komponen | Status |
|---------|----------|--------|
| pps-services-consumer | Source routing (extractSource, processPreOrder, processProvider) | ✅ Implemented |
| pps-services-consumer | HTTP client ke Publisher-Provider (CallPublisherProvider) | ✅ Implemented |
| pps-services-consumer | SP_SetTransactionStatus | ✅ Implemented |
| pps-services-consumer | Parse wrapper format PROVIDER | ✅ Implemented (parseProviderMessage) |
| pps-services-consumer | Backward compat flat format | ✅ Implemented |
| pps-services-gateway-telkomsel | MQPublisher (replace DownstreamClient) | ✅ Implemented |
| pps-services-gateway-telkomsel | Transaction logging Postgres | ✅ Implemented |
| pps-services-gateway-telkomsel | Callback endpoint GET /callback/ext | ✅ Implemented |
| pps-services-gateway-telkomsel | HTTP server + RabbitMQ consumer via errgroup | ✅ Implemented |
| pps-services-gateway-unipin | MQPublisher (replace DownstreamClient) | ✅ Implemented |
| pps-services-gateway-unipin | Voucher list sync to Oracle | ✅ Implemented |
| pps-services-gateway-unipin | unipin-game transaction flow | 📋 Spec ready, not yet executed |
| pps-services-gateway-unipin | unipin-voucher command parsing update | 📋 Spec ready, not yet executed |
| pps-services-consumer-database | Deprecated | ✅ Retired |
| pps-services-publisher-database | Removed from architecture | ✅ Removed |

### Format Pesan RabbitMQ

Semua pesan menggunakan format wrapper:

**PRE-ORDER (Publisher → Consumer):**
```json
{
  "source": "PRE-ORDER",
  "data": {
    "user": "...",
    "produk": "...",
    "mdn": "...",
    "noTrx": "...",
    "signature": "...",
    "addr": "...",
    "serverId": "..."
  }
}
```

**PROVIDER (Gateway → Consumer):**
```json
{
  "source": "PROVIDER",
  "data": {
    "msg_id": 123,
    "status_to_be": "SUCCESS",
    "serial_number": "SN123",
    "client_number": "6281234567890",
    "nominal": "15000",
    "original_conversation_id": "",
    "conversation_id": "TRX123",
    "message_to_customer": "Success",
    "additional_message": "",
    "queue_name": "..."
  }
}
```

### Gateway Downstream: RabbitMQ Direct (bukan HTTP)

Gateway publish langsung ke RabbitMQ menggunakan:
- `MQTransaction` — URL RabbitMQ tujuan (dibawa oleh setiap transaksi dari publisher-provider)
- `QueueName` — nama queue tujuan (dibawa oleh setiap transaksi)
- `MQPublisher.Publish(ctx, mqTransactionURL, queueName, body)` — buka koneksi baru per publish

Tidak ada lagi:
- ~~DownstreamClient~~ (HTTP POST)
- ~~Publisher-Database~~ (perantara)
- ~~Circuit breaker / exponential backoff ke downstream~~ (fire-and-forget, error di-log)

## Specs Terkait

| Spec | Repo | Status |
|------|------|--------|
| telkomsel-transaction-log | gateway-telkomsel | ✅ Executed |
| telkomsel-callback-endpoint | gateway-telkomsel | ✅ Executed |
| rabbitmq-downstream-publisher | gateway-telkomsel | ✅ Executed |
| rabbitmq-downstream-publisher | gateway-unipin | ✅ Executed |
| provider-message-wrapper-format | consumer | ✅ Executed |
| unipin-transaction-flow | gateway-unipin | 📋 Ready to execute |
