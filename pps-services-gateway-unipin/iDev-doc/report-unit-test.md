# API Documentation — pps-services-gateway-unipin

---

## 1. Health Check

| Field                        | Value                                                          |
|------------------------------|----------------------------------------------------------------|
| **Endpoint**                 | `/health`                                                      |
| **Method**                   | GET                                                            |
| **Description**              | Cek status service apakah berjalan normal                      |
| **Authentication**           | None (public endpoint)                                         |
| **Headers**                  | —                                                              |
| **Request Body**             | —                                                              |
| **Response 200 OK**          | `{"status":"ok"}`                                              |
| **Response 500 Internal Error** | Service tidak berjalan                                      |
| **Rate Limit**               | —                                                              |
| **Link Swagger (Optional)**  | —                                                              |

---

## 2. Game List

| Field                        | Value                                                          |
|------------------------------|----------------------------------------------------------------|
| **Endpoint**                 | `/api/v1/ingame/list`                                          |
| **Method**                   | GET                                                            |
| **Description**              | Mengambil daftar game dari UniPin In-Game Topup API            |
| **Authentication**           | Internal (HMAC-SHA256 ke UniPin via server-side)               |
| **Headers**                  | Content-Type: application/json                                 |
| **Request Body**             | —                                                              |
| **Response 200 OK**          | `{"game_list":[{"game_code":"MLBBD_ID","game_name":"Mobile Legends Diamonds","game_category":"MLBB_ID","game_status":"active",...}],"status":1,"reason":"Successful"}` |
| **Response 422 Unprocessable** | `{"status":0,"reason":"<reason dari UniPin>"}`               |
| **Response 502 Bad Gateway** | `{"error":"failed to fetch game list from provider"}`          |
| **Rate Limit**               | —                                                              |
| **Link Swagger (Optional)**  | —                                                              |

---

## 3. Game Detail

| Field                        | Value                                                          |
|------------------------------|----------------------------------------------------------------|
| **Endpoint**                 | `/api/v1/ingame/detail?game_code=XXX`                          |
| **Method**                   | GET                                                            |
| **Description**              | Mengambil detail game beserta denominations dan fields          |
| **Authentication**           | Internal (HMAC-SHA256 ke UniPin via server-side)               |
| **Headers**                  | Content-Type: application/json                                 |
| **Request Body**             | —                                                              |
| **Query Parameters**         | `game_code` (required) — kode game UniPin                      |
| **Response 200 OK**          | `{"game":{"name":"...","code":"...","category":"..."},"denominations":[{"id":324,"name":"154 Diamonds","currency":"IDR","amount":"58696.00"}],"fields":[{"name":"userid","type":"string"}],"status":1,"reason":"Successful"}` |
| **Response 400 Bad Request** | `{"error":"game_code query parameter is required"}`            |
| **Response 422 Unprocessable** | `{"status":0,"reason":"<reason dari UniPin>"}`               |
| **Response 502 Bad Gateway** | `{"error":"failed to fetch game detail from provider"}`        |
| **Rate Limit**               | —                                                              |
| **Link Swagger (Optional)**  | —                                                              |

---

## 4. Sync Game List (Full)

| Field                        | Value                                                          |
|------------------------------|----------------------------------------------------------------|
| **Endpoint**                 | `/api/v1/ingame/sync`                                          |
| **Method**                   | POST                                                           |
| **Description**              | Sinkronisasi seluruh game list + denominations ke Oracle DB    |
| **Authentication**           | Internal (HMAC-SHA256 ke UniPin via server-side)               |
| **Headers**                  | Content-Type: application/json                                 |
| **Request Body**             | —                                                              |
| **Response 200 OK**          | `{"status":"ok","message":"game list synced to database"}`     |
| **Response 500 Internal Error** | `{"error":"game list sync failed"}`                         |
| **Rate Limit**               | —                                                              |
| **Link Swagger (Optional)**  | —                                                              |

---

## 5. Sync Single Game

| Field                        | Value                                                          |
|------------------------------|----------------------------------------------------------------|
| **Endpoint**                 | `/api/v1/ingame/sync/:game_code`                               |
| **Method**                   | POST                                                           |
| **Description**              | Sinkronisasi satu game beserta semua denominations ke Oracle DB |
| **Authentication**           | Internal (HMAC-SHA256 ke UniPin via server-side)               |
| **Headers**                  | Content-Type: application/json                                 |
| **Request Body**             | —                                                              |
| **Path Parameters**          | `game_code` (required) — kode game UniPin, contoh: `MLBBD_ID` |
| **Response 200 OK**          | `{"status":"ok","message":"game synced to database","game_code":"MLBBD_ID"}` |
| **Response 400 Bad Request** | `{"error":"game_code is required"}`                            |
| **Response 500 Internal Error** | `{"error":"sync failed","detail":"<detail>","game_code":"MLBBD_ID"}` |
| **Rate Limit**               | —                                                              |
| **Link Swagger (Optional)**  | —                                                              |

---

## 6. Sync Single Denomination

| Field                        | Value                                                          |
|------------------------------|----------------------------------------------------------------|
| **Endpoint**                 | `/api/v1/ingame/sync/:game_code/:denomination_id`              |
| **Method**                   | POST                                                           |
| **Description**              | Sinkronisasi satu denomination dari satu game ke Oracle DB     |
| **Authentication**           | Internal (HMAC-SHA256 ke UniPin via server-side)               |
| **Headers**                  | Content-Type: application/json                                 |
| **Request Body**             | —                                                              |
| **Path Parameters**          | `game_code` (required), `denomination_id` (required, integer)  |
| **Response 200 OK**          | `{"status":"ok","message":"denomination synced to database","game_code":"MLBBD_ID","denomination_id":324}` |
| **Response 400 Bad Request** | `{"error":"game_code and denomination_id are required"}`       |
| **Response 500 Internal Error** | `{"error":"sync failed","detail":"<detail>","game_code":"MLBBD_ID","denomination_id":324}` |
| **Rate Limit**               | —                                                              |
| **Link Swagger (Optional)**  | —                                                              |

---

## 7. RabbitMQ Consumer — processMessage (Internal)

| Field                        | Value                                                          |
|------------------------------|----------------------------------------------------------------|
| **Endpoint**                 | RabbitMQ Queue (bukan HTTP)                                    |
| **Method**                   | Consumer (subscribe ke queue)                                  |
| **Description**              | Menerima pesan dari RabbitMQ dan memproses berdasarkan `product_type` |
| **Authentication**           | RabbitMQ credentials via `RABBITMQ_URL`                        |
| **Input (Message Body)**     | JSON payload dengan field: `product_type`, `command`, `msisdn`, `msgid`, `mq_transaction`, `amount`, `mid`, `store_id`, `queue_name` |
| **Supported Product Types**  | `unipin-voucher`, `unipin-game`                                |
| **Output**                   | Publish status ke downstream RabbitMQ via `forwardCallback`    |
| **Output Format**            | `{"source":"PROVIDER","data":{"msg_id":123,"status_to_be":"SUCCESS/FAILED","serial_number":"...","client_number":"...","nominal":"50000","message_to_customer":"..."}}` |

---

## 8. RabbitMQ Consumer — processGame (Internal)

| Field                        | Value                                                          |
|------------------------------|----------------------------------------------------------------|
| **Trigger**                  | `product_type = "unipin-game"`                                 |
| **Description**              | Alur in-game topup: parse Command → parse MSISDN JSON → ValidateUser → CreateOrder → fallback OrderInquiry |
| **Input: Command**           | Format: `gamecode*denomination_id` (contoh: `MLBB*123`)        |
| **Input: MSISDN**            | JSON string (contoh: `{"userid":"789","zone":"ID"}`)           |
| **UniPin API: ValidateUser** | `POST /in-game-topup/user/validate` — validasi user game       |
| **UniPin API: CreateOrder**  | `POST /in-game-topup/order/create` — buat order topup          |
| **UniPin API: OrderInquiry** | `POST /in-game-topup/order/inquiry` — fallback saat timeout    |
| **Output SUCCESS**           | `status_to_be=SUCCESS`, `serial_number=ReferenceNo`            |
| **Output FAILED**            | `status_to_be=FAILED`, `message=<reason/error>`                |
| **Timeout Handling**         | CreateOrder timeout → fallback ke OrderInquiry                 |

---

## 9. RabbitMQ Consumer — processVoucher (Internal)

| Field                        | Value                                                          |
|------------------------------|----------------------------------------------------------------|
| **Trigger**                  | `product_type = "unipin-voucher"`                              |
| **Description**              | Alur voucher: parse Command → VoucherRequest → fallback VoucherInquiry |
| **Input: Command**           | Format: `voucher_code*denomination_code` (contoh: `STEAM*STEAM-100K`) |
| **Input: MSISDN**            | Nomor telepon biasa (bukan JSON)                               |
| **UniPin API: VoucherRequest** | `POST /voucher/request` — beli voucher                       |
| **UniPin API: VoucherInquiry** | `POST /voucher/inquiry` — fallback saat timeout              |
| **Output SUCCESS**           | `status_to_be=SUCCESS`, `serial_number=ReferenceNo`            |
| **Output FAILED**            | `status_to_be=FAILED`, `message=<reason/error>`                |
| **Timeout Handling**         | VoucherRequest timeout → fallback ke VoucherInquiry            |

---

## Unit Test Coverage

| Package                               | Coverage | Total Test | Status  |
|---------------------------------------|----------|------------|---------|
| `internal/infrastructure/rabbitmq`    | 96.1%    | 80         | ✅ PASS |
| `internal/infrastructure/mqpublisher` | 100%     | 4          | ✅ PASS |
| `internal/usecase/gamesync`           | 96.7%    | 23         | ✅ PASS |
| **Total**                             |          | **107**    | ✅ PASS |

---

## Cara Menjalankan Test

```bash
# Semua unit test
go test ./internal/infrastructure/rabbitmq/ ./internal/infrastructure/mqpublisher/ ./internal/usecase/gamesync/ -v -count=1 -timeout 120s

# Dengan coverage
go test ./internal/infrastructure/rabbitmq/ -cover -timeout 120s
go test ./internal/infrastructure/mqpublisher/ -cover
go test ./internal/usecase/gamesync/ -cover
```
