# Flowchart Gateway SMB — PLN Token

Dokumen ini menjelaskan alur `pps-services-gateway-smb` untuk produk PLN Token menggunakan SMB/Loket Bayar API.

## 1) Startup & Komponen

```mermaid
flowchart TD
  A([Process start]) --> B[Load config: RABBITMQ_URL, QUEUE_NAME_PROVIDER, CONSUMER_TAG]
  B --> C[Load SMB config: SMB_BASE_URL, SMB_PARTNER_ID, SMB_SECRET_KEY]
  C --> D[Load retry config: RETRY_MAX_ATTEMPTS, RETRY_WAIT_SECONDS]
  D --> E[Create context with SIGINT/SIGTERM]

  E --> F[Init SMB HTTP Client]
  F --> G[Init RabbitMQ ConsumerService]

  E --> H{POSTGRES_DSN configured?}
  H -- No --> I[Skip transaction logging]
  H -- Yes --> J[Init Postgres TransactionLogger]
  J --> K[Run migration smb_transaction tables]
  K --> L[Inject TransactionLogger into ConsumerService]

  G --> M[Inject RetryConfig into ConsumerService]
  M --> N[Init MQ Publisher AMQPPublisher]
  N --> O[Inject MQ Publisher into ConsumerService]

  O --> P[Start HTTP server: GET /health]
  O --> Q[Start RabbitMQ consumer loop]

  P --> R[errgroup: run HTTP + consumer]
  Q --> R
  R --> S([Exit on fatal error / context cancel])
```

## 2) RabbitMQ Consumer — Top-level Flow

```mermaid
flowchart TD
  A[Start] --> B{ctx canceled?}
  B -- Yes --> Z([Stop])
  B -- No --> C[consumeSession: Dial RabbitMQ, Channel, QueueDeclare, QoS 1, Consume]

  C --> D[for delivery in deliveries]
  D --> E[Unmarshal JSON -> consumePayload]
  E -->|fail| E1[Log error parse payload]
  E1 --> ACK

  E -->|ok| F[Derive queueName override]
  F --> G[Derive msgID: payload.msgid else delivery.MessageId else CorrelationId]
  G --> H{product_type?}

  H -- pln_token --> P[PLN Token flow]
  H -- other/empty --> W[Warn unsupported product type]

  P --> ACK
  W --> ACK

  ACK[delivery.Ack false] --> D
```

## 3) PLN Token Flow (Inquiry → Payment → Publish Downstream)

```mermaid
flowchart TD
  A[PLN Token branch] --> B[requestedAt = now]
  B --> C[Generate ourTrxID = SMB-mid-msgID-timestamp]
  C --> D[DB: InsertTransaction PROCESSING]

  D --> E[Step 1: Call SMB API Inquiry PLN Token]
  E --> F{inquiry error?}
  F -- Yes --> G[DB: InsertSyncResponse ERROR + UpdateStatus FAILED]
  G --> G2[Publish downstream: status_to_be = C]

  F -- No --> H[DB: InsertSyncResponse SYNC]
  H --> I{inquiry response_code == 00?}
  I -- No --> J[DB: UpdateStatus FAILED]
  J --> J2[Publish downstream: status_to_be = C]

  I -- Yes --> K[Step 2: Call SMB API Payment PLN Token]
  K --> L{payment error?}
  L -- Yes --> M[DB: InsertSyncResponse ERROR]
  M --> M2[Async retryAdvice]

  L -- No --> N[DB: InsertSyncResponse SYNC]
  N --> O{rcPPS from response_code}

  O -- 0 success --> P[DB: UpdateStatus SUCCESS]
  P --> P2[Publish downstream: status_to_be = F, token in serial_number]

  O -- 1 failed --> Q[DB: UpdateStatus FAILED]
  Q --> Q2[Publish downstream: status_to_be = C]

  O -- 9 pending --> R[Async retryAdvice]
```

## 4) Async Retry — Advice/Check Status

```mermaid
flowchart TD
  A[Start retry loop] --> B{retryConfig nil?}
  B -- Yes --> C[Mark FAILED + Publish C]
  B -- No --> D[for attempt=1..MaxAttempts]

  D --> E[Sleep WaitDuration]
  E --> F[Call SMB API Advice PLN Token]

  F --> G{advice error?}
  G -- Yes --> H[Log error, continue retry]
  G -- No --> I{rcPPS from response_code}

  I -- 0 --> J[DB: Update SUCCESS]
  J --> J2[Publish F with token]
  I -- 1 --> K[DB: Update FAILED]
  K --> K2[Publish C]
  I -- 9 --> D

  D --> L{max attempts reached}
  L -- Yes --> M[DB: Update FAILED + Publish C]
```

## 5) Downstream Publish Format

Semua publish ke downstream consumer menggunakan format wrapper:

```json
{
  "source": "PROVIDER",
  "data": {
    "msg_id": 123,
    "status_to_be": "F",
    "serial_number": "TOKEN_PLN_12345",
    "client_number": "12345678901",
    "nominal": "50000",
    "conversation_id": "SMB-MID-123-20260422120000",
    "message_to_customer": "Token PLN: TOKEN_PLN_12345",
    "queue_name": "queue_smb_pln_token"
  }
}
```

## 6) Impact ke pps-services-tokopedia

Tokopedia **tidak perlu perubahan kode** untuk mendukung SMB gateway. Alur:

1. Tokopedia menerima payment request dari user
2. Tokopedia publish ke RabbitMQ (format legacy `PRE_ORDER||...`)
3. Consumer (pps-services-consumer) consume dan proses PRE-ORDER
4. Consumer call Publisher-Provider
5. Publisher-Provider route ke queue SMB berdasarkan `remarkImsi`
6. **Gateway SMB** consume dari queue provider, call SMB API, publish result
7. Consumer consume result `{source: "PROVIDER", data: {...}}`
8. Consumer call SP_SetTransactionStatus di Oracle

Yang perlu dikonfigurasi:
- Publisher-Provider: tambah routing rule untuk produk PLN Token ke queue SMB
- Environment: set `QUEUE_NAME_PROVIDER` di gateway SMB sesuai queue yang di-route

## SMB API Response Code Mapping

| SMB Code | Deskripsi | RC PPS | status_to_be |
|----------|-----------|--------|--------------|
| 00 | Success | 0 | F |
| 28 | Timeout/Pending | 9 | S (retry advice) |
| 68 | Timeout/Pending | 9 | S (retry advice) |
| empty | Unknown | 9 | S (retry advice) |
| lainnya | Failed | 1 | C |
