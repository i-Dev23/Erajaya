# Flowchart Gateway Telkomsel

Dokumen ini merangkum alur utama `pps-services-gateway-telkomsel` berdasarkan kode saat ini (April 2026) untuk membantu validasi & koreksi logic.

## 1) Startup & Komponen

```mermaid
flowchart TD
  A([Process start]) --> B[Load config: RABBITMQ_URL, QUEUE_NAME_PROVIDER/QUEUE_NAME, CONSUMER_TAG]
  B --> C[Load retry config: RETRY_MAX_ATTEMPTS, RETRY_WAIT_SECONDS]
  C --> D[Create context with SIGINT/SIGTERM]

  D --> E[Init RabbitMQ ConsumerService]

  D --> F{POSTGRES_DSN configured?}
  F -- No --> G[Skip transaction logging + error mapping + API logging]
  F -- Yes --> H[Init Postgres TransactionLogger]
  H --> I[Run migration transaction tables]
  I --> J[Init ErrorMappingResolver]
  J --> K[Init Telkomsel APILogger]
  K --> L[Inject TransactionLogger into ConsumerService]

  E --> M[Inject RetryConfig into ConsumerService]
  M --> N[Init MQ Publisher (AMQPPublisher)]
  N --> O[Inject MQ Publisher into ConsumerService]

  O --> P[Load callback server config: CALLBACK_PORT]
  P --> Q[Init CallbackHandler(logger, txLogger, mqPub, queueName)]
  Q --> R[Start HTTP server: GET /callback/ext, GET /health]

  O --> S[Start RabbitMQ consumer loop]

  R --> T[errgroup: run HTTP + consumer]
  S --> T
  T --> U([Exit on fatal error / context cancel])
```

## 2) RabbitMQ Consumer — Top-level Flow

Sumber utama: consumer membaca queue `cfg.QueueName` dan memproses tiap message satu per satu (QoS prefetch=1). Setelah diproses, message selalu `Ack()`.

```mermaid
flowchart TD
  A[Start()] --> B{ctx canceled?}
  B -- Yes --> Z([Stop])
  B -- No --> C[consumeSession(): Dial RabbitMQ, Channel, QueueDeclare, QoS(1), Consume]

  C --> D[for delivery in deliveries]
  D --> E[Unmarshal JSON -> consumePayload]
  E -->|fail| E1[Log error parse payload]
  E1 --> ACK

  E -->|ok| F[Derive queueName override (payload.queue_name if set)]
  F --> G[Derive msgID: payload.msgid else delivery.MessageId else CorrelationId]
  G --> H{product_type? (lowercase)}

  H -- pulsa --> P[Pulsa flow]
  H -- data --> Q[Data flow]
  H -- other/empty --> W[Warn unsupported product type]

  P --> ACK
  Q --> ACK
  W --> ACK

  ACK[delivery.Ack(false)] --> D
```

## 3) Pulsa Flow (Initiate Regular Recharge)

```mermaid
flowchart TD
  A[Pulsa branch] --> B[requestedAt = now]
  B --> C[Generate ourTrxID = Telkomsel.GenerateTransactionID(mid, msgID, requestedAt)
  fallback: ourTrxID=msgID]

  C --> D[DB: InsertTransaction(PROCESSING)
  msg_id=msgID, our_trx_id=ourTrxID, msisdn, mid, amount, stockType, queueName, mq_transaction]

  D --> E[Call Telkomsel: InitiateRegularRechargeOnConsumeWithTransactionID(
  msisdn, mid, queueName, msgID, ourTrxID, amount, stockType)]

  E --> F{Call error?}
  F -- No --> G[DB: InsertSyncResponse(SYNC)
  status_code=resp.transaction.status_code]
  G --> H[rcPPS = ResolveRCPPS(httpStatus, esbStatusCode)]

  F -- Yes --> F1[DB: InsertSyncResponse(SYNC)
  status_code=ERROR, status_desc=err]
  F1 --> H1[Determine httpStatus + esbStatusCode from error types]
  H1 --> I[rcPPS = ResolveRCPPS(httpStatus, esbStatusCode)]

  H --> J{rcPPS}
  I --> J

  J -- 0 (success) --> K[DB: UpdateTransactionStatus SUCCESS]
  K --> K2[Publish downstream: status_to_be = F]

  J -- 1 (failed) --> L[DB: UpdateTransactionStatus FAILED]
  L --> L2[Publish downstream: status_to_be = C]

  J -- 9 (pending/unknown) --> M[Async retryCheckStatusSync()]
  M --> M2[Publish happens later (success/failed) or after max retry]
```

## 4) Data Flow (Browse Offer → Order Dealer → Waiting Callback/Retry)

```mermaid
flowchart TD
  A[Data branch] --> B[processingAt = now]
  B --> C[Generate ourTrxID = Telkomsel.GenerateTransactionID(mid, msgID, processingAt)
  fallback: ourTrxID=msgID]
  C --> D[DB: InsertTransaction(PROCESSING)]

  D --> E[Call Telkomsel: BrowseOfferOnConsume(msisdn, mid, queueName, msgID, productID)]
  E --> F{browse ok AND status_code==00000?}

  F -- No --> G[DB: InsertSyncResponse(ERROR) + UpdateStatus FAILED]
  G --> G2[Publish downstream: status_to_be = C]

  F -- Yes --> H[DB: InsertSyncResponse(SYNC) for BrowseOffer]
  H --> I[Call Telkomsel: OrderDealerOnConsumeWithTransactionID(
  msisdn, mid, queueName, msgID, ourTrxID, productID, stockType, storeID, callbackURL)]

  I --> J{order error?}
  J -- Yes --> K[DB: InsertSyncResponse(ERROR) + UpdateStatus FAILED]
  K --> K2[Publish downstream: status_to_be = C]

  J -- No --> L[DB: InsertSyncResponse(SYNC) for OrderDealer]
  L --> M[rcPPS = ResolveRCPPS(httpStatus, esbStatusCode)]

  M --> N{rcPPS}
  N -- 0 (accepted/processing) --> O[DB: UpdateStatus PROCESSING]
  O --> O2[Publish downstream: status_to_be = S]
  O2 --> O3[Async retryCheckStatusPaketDataSync()]

  N -- 1 (failed) --> P[DB: UpdateStatus FAILED]
  P --> P2[Publish downstream: status_to_be = C]

  N -- 9 (pending/unknown) --> Q[Async retryCheckStatusPaketDataSync()]
```

## 5) Async Retry — Check Order Status

Ada 2 jalur retry:
- `retryCheckStatusSync()` (dipakai di pulsa saat rcPPS=9).
- `retryCheckStatusPaketDataSync()` (dipakai di data; sebelum call API dia cek DB apakah sudah resolved via callback).

```mermaid
flowchart TD
  A[Start retry loop] --> B{retryConfig nil?}
  B -- Yes --> C[Mark FAILED + Publish C]
  B -- No --> D[for attempt=1..MaxAttempts]

  D --> E[Sleep WaitDuration]
  E --> F{data retry?}

  F -- Yes --> G[If txLogger: GetTransactionStatusByMsgID(msgID)]
  G --> H{DB status SUCCESS/FAILED?}
  H -- Yes --> Z([Stop retry; no publish])
  H -- No --> I[Call Telkomsel CheckOrderStatus]

  F -- No --> I[Call Telkomsel CheckOrderStatus]

  I --> J{rcPPS from response (or parsed error response)}
  J -- 0 --> K[DB: Update SUCCESS]
  K --> K2[Publish F]
  J -- 1 --> L[DB: Update FAILED]
  L --> L2[Publish C]
  J -- 9 --> D

  D --> M{max attempts reached}
  M -- Yes --> N[DB: Update FAILED + Publish C]
```

## 6) HTTP Callback Flow (`GET /callback/ext`)

```mermaid
flowchart TD
  A[HTTP GET /callback/ext] --> B[Parse query params]
  B --> C{Validate mandatory fields:
  transaction_id, organization_code(6-13), service_id(13), status SUCCESS/FAILED, message}
  C -- invalid --> D[Return 400 JSON]
  C -- valid --> E[Decode message]

  E --> F[Lookup transaction by our_trx_id = transaction_id (if txLogger exists)]
  F --> G{Found?}
  G -- Yes --> H[Use msgID, amount, msisdn/clientNumber, mq_transaction, queueName from DB]
  G -- No --> I[Fallback: msgID = transaction_id; mq_transaction empty]

  H --> J[DB: InsertCallbackResponse(status_code 0/1, status_desc message)
  this will also update transaction status SUCCESS/FAILED]
  I --> J

  J --> K{mqPublisher exists AND mq_transaction exists?}
  K -- No --> L[Skip publish]
  K -- Yes --> M[Publish downstream callback message]
  M --> N[Return 200 JSON OK]
  L --> N
```

## Validasi cepat (titik rawan logic)

1) **Perbedaan `status_to_be`**
- Consumer publish memakai konstanta `F/C/S`.
- Callback publish memakai literal `SUCCESS/FAILED` sebagai `statusToBe`.
  Jika downstream consumer mengharapkan `F/C/S`, ini mismatch dan perlu diseragamkan.

2) **BrowseOffer gagal tanpa error**
- Kondisi gagal mencakup `status_code != "00000"` walau `callErr == nil`.
- Pastikan error handling tidak dereference error nil (sudah diproteksi di consumer).

3) **Sumber `client_number` pada callback publish**
- Callback publish mengisi `ClientNumber` dengan `service_id` dari query.
- Sementara `messageToCustomer` dibuat pakai `clientNumber` dari DB (MSISDN).
  Kalau `service_id` bukan MSISDN atau formatnya beda, bisa bikin inkonsistensi payload.

4) **`msg_id` int**
- Publish downstream memakai `msg_id` bertipe int; kalau `msgID` bukan angka, nilainya jadi 0.

Jika Anda mau, saya bisa bantu bikin versi flowchart yang “expected behavior” (yang seharusnya) lalu kita bandingkan dengan “as-is” dari kode ini.
