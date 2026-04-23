# List API — pps-services-gateway-unipin

---

## 1. Health Check

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Endpoint**                    | `/health`                                                   |
| **Method**                      | GET                                                         |
| **Description**                 | Cek status service apakah berjalan normal                   |
| **Authentication**              | None (public endpoint)                                      |
| **Headers**                     | —                                                           |
| **Request Body**                | —                                                           |
| **Response 200 OK**             | `{"status":"ok"}`                                           |
| **Response 500 Internal Error** | Service tidak berjalan                                      |
| **Rate Limit**                  | —                                                           |
| **Link Swagger (Optional)**     | —                                                           |

---

## 2. Game List API (dari Oracle DB)

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Endpoint**                    | `/api/v1/game-list`                                         |
| **Method**                      | POST                                                        |
| **Description**                 | Mengambil daftar game dari Oracle DB dengan validasi signature |
| **Authentication**              | Signature validation via Oracle SP `MSG.PKG_UNIPIN.validateSignatureGameList` |
| **Headers**                     | Content-Type: application/json                              |
| **Request Body**                | `{"user":"string","timestamp":"string","signature":"string"}` |
| **Response 200 OK**             | `{"status":"0","message":"Successfully","data":[{"product":"MLBBD_ID","product_desc":"Mobile Legends","fields":[{"name":"userid","type":"string"}]}]}` |
| **Response 400 Bad Request**    | `{"status":"1","message":"invalid request body"}` atau `{"status":"1","message":"user and signature are required"}` |
| **Response 401 Unauthorized**   | `{"status":"1","message":"<outMessage dari SP>"}`           |
| **Response 500 Internal Error** | `{"status":"1","message":"internal server error"}` atau `{"status":"1","message":"failed to retrieve game list"}` |
| **Rate Limit**                  | —                                                           |
| **Link Swagger (Optional)**     | —                                                           |

---

## 3. Game List (dari UniPin API — proxy)

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Endpoint**                    | `/api/v1/ingame/list`                                       |
| **Method**                      | GET                                                         |
| **Description**                 | Mengambil daftar game dari UniPin In-Game Topup API         |
| **Authentication**              | Internal (HMAC-SHA256 ke UniPin via server-side)            |
| **Headers**                     | Content-Type: application/json                              |
| **Request Body**                | —                                                           |
| **Response 200 OK**             | `{"game_list":[{"game_code":"MLBBD_ID","game_name":"Mobile Legends Diamonds","game_category":"MLBB_ID","game_status":"active"}],"status":1,"reason":"Successful"}` |
| **Response 422 Unprocessable**  | `{"status":0,"reason":"<reason dari UniPin>"}`              |
| **Response 502 Bad Gateway**    | `{"error":"failed to fetch game list from provider"}`       |
| **Rate Limit**                  | —                                                           |
| **Link Swagger (Optional)**     | —                                                           |

---

## 4. Game Detail

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Endpoint**                    | `/api/v1/ingame/detail?game_code=XXX`                       |
| **Method**                      | GET                                                         |
| **Description**                 | Mengambil detail game beserta denominations dan fields       |
| **Authentication**              | Internal (HMAC-SHA256 ke UniPin via server-side)            |
| **Headers**                     | Content-Type: application/json                              |
| **Request Body**                | —                                                           |
| **Query Parameters**            | `game_code` (required) — kode game UniPin                   |
| **Response 200 OK**             | `{"game":{"name":"...","code":"..."},"denominations":[{"id":324,"name":"154 Diamonds","currency":"IDR","amount":"58696.00"}],"fields":[{"name":"userid","type":"string"}],"status":1,"reason":"Successful"}` |
| **Response 400 Bad Request**    | `{"error":"game_code query parameter is required"}`         |
| **Response 422 Unprocessable**  | `{"status":0,"reason":"<reason dari UniPin>"}`              |
| **Response 502 Bad Gateway**    | `{"error":"failed to fetch game detail from provider"}`     |
| **Rate Limit**                  | —                                                           |
| **Link Swagger (Optional)**     | —                                                           |

---

## 5. Sync Game List (Full)

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Endpoint**                    | `/api/v1/ingame/sync`                                       |
| **Method**                      | POST                                                        |
| **Description**                 | Sinkronisasi seluruh game list + denominations ke Oracle DB |
| **Authentication**              | Internal (HMAC-SHA256 ke UniPin via server-side)            |
| **Headers**                     | Content-Type: application/json                              |
| **Request Body**                | —                                                           |
| **Response 200 OK**             | `{"status":"ok","message":"game list synced to database"}`  |
| **Response 500 Internal Error** | `{"error":"game list sync failed"}`                         |
| **Rate Limit**                  | —                                                           |
| **Link Swagger (Optional)**     | —                                                           |

---

## 6. Sync Single Game

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Endpoint**                    | `/api/v1/ingame/sync/:game_code`                            |
| **Method**                      | POST                                                        |
| **Description**                 | Sinkronisasi satu game beserta semua denominations ke Oracle DB |
| **Authentication**              | Internal (HMAC-SHA256 ke UniPin via server-side)            |
| **Headers**                     | Content-Type: application/json                              |
| **Request Body**                | —                                                           |
| **Path Parameters**             | `game_code` (required) — contoh: `MLBBD_ID`                |
| **Response 200 OK**             | `{"status":"ok","message":"game synced to database","game_code":"MLBBD_ID"}` |
| **Response 400 Bad Request**    | `{"error":"game_code is required"}`                         |
| **Response 500 Internal Error** | `{"error":"sync failed","detail":"<detail>","game_code":"MLBBD_ID"}` |
| **Rate Limit**                  | —                                                           |
| **Link Swagger (Optional)**     | —                                                           |

---

## 7. Sync Single Denomination

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Endpoint**                    | `/api/v1/ingame/sync/:game_code/:denomination_id`           |
| **Method**                      | POST                                                        |
| **Description**                 | Sinkronisasi satu denomination dari satu game ke Oracle DB  |
| **Authentication**              | Internal (HMAC-SHA256 ke UniPin via server-side)            |
| **Headers**                     | Content-Type: application/json                              |
| **Request Body**                | —                                                           |
| **Path Parameters**             | `game_code` (required), `denomination_id` (required, int)   |
| **Response 200 OK**             | `{"status":"ok","message":"denomination synced to database","game_code":"MLBBD_ID","denomination_id":324}` |
| **Response 400 Bad Request**    | `{"error":"game_code and denomination_id are required"}`    |
| **Response 500 Internal Error** | `{"error":"sync failed","detail":"<detail>","game_code":"MLBBD_ID","denomination_id":324}` |
| **Rate Limit**                  | —                                                           |
| **Link Swagger (Optional)**     | —                                                           |

---

## 8. RabbitMQ Consumer — processMessage (Internal)

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Endpoint**                    | RabbitMQ Queue (bukan HTTP)                                 |
| **Method**                      | Consumer (subscribe ke queue)                               |
| **Description**                 | Menerima pesan dari RabbitMQ, proses berdasarkan `product_type` |
| **Authentication**              | RabbitMQ credentials via `RABBITMQ_URL`                     |
| **Input (Message Body)**        | JSON: `product_type`, `command`, `msisdn`, `msgid`, `mq_transaction`, `amount`, `mid`, `store_id`, `queue_name` |
| **Supported Product Types**     | `unipin-voucher`, `unipin-game`                             |
| **Output**                      | Publish status ke downstream RabbitMQ via `forwardCallback`  |
| **Output Format**               | `{"source":"PROVIDER","data":{"msg_id":123,"status_to_be":"SUCCESS/FAILED","serial_number":"...","client_number":"...","nominal":"50000","message_to_customer":"..."}}` |

---

## 9. Game Direct Topup — processGame (Internal, RabbitMQ Consumer)

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Trigger**                     | `product_type = "unipin-game"`                              |
| **Description**                 | Alur in-game topup: parse Command → parse MSISDN JSON → ValidateUser → CreateOrder → fallback OrderInquiry |
| **Input: Command**              | Format: `gamecode*denomination_id` (contoh: `MLBB*123`)     |
| **Input: MSISDN**               | JSON string (contoh: `{"userid":"789","zone":"ID"}`)        |
| **UniPin API: ValidateUser**    | `POST /in-game-topup/user/validate`                         |
| **UniPin API: CreateOrder**     | `POST /in-game-topup/order/create`                          |
| **UniPin API: OrderInquiry**    | `POST /in-game-topup/order/inquiry` — fallback saat timeout |
| **Output SUCCESS**              | `status_to_be=SUCCESS`, `serial_number=ReferenceNo`         |
| **Output FAILED**               | `status_to_be=FAILED`, `message=<reason/error>`             |
| **Timeout Handling**            | CreateOrder timeout → fallback ke OrderInquiry              |

---

## 10. RabbitMQ Consumer — processVoucher (Internal)

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Trigger**                     | `product_type = "unipin-voucher"`                           |
| **Description**                 | Alur voucher: parse Command → VoucherRequest → fallback VoucherInquiry |
| **Input: Command**              | Format: `voucher_code*denomination_code` (contoh: `STEAM*STEAM-100K`) |
| **Input: MSISDN**               | Nomor telepon biasa (bukan JSON)                            |
| **UniPin API: VoucherRequest**  | `POST /voucher/request`                                     |
| **UniPin API: VoucherInquiry**  | `POST /voucher/inquiry` — fallback saat timeout             |
| **Output SUCCESS**              | `status_to_be=SUCCESS`, `serial_number=ReferenceNo`         |
| **Output FAILED**               | `status_to_be=FAILED`, `message=<reason/error>`             |
| **Timeout Handling**            | VoucherRequest timeout → fallback ke VoucherInquiry         |

---

## 11. API Logging ke Postgres (Internal, Otomatis)

| Field                           | Value                                                       |
|---------------------------------|-------------------------------------------------------------|
| **Trigger**                     | Setiap HTTP request ke UniPin API (otomatis via LoggingTransport) |
| **Description**                 | Mencatat semua request/response ke UniPin API ke tabel `log_unipin.api_log` di Postgres |
| **Database**                    | Postgres (`POSTGRES_DSN`)                                   |
| **Tabel**                       | `log_unipin.api_log`                                        |
| **Field yang dicatat**          | `endpoint`, `method`, `request_url`, `request_headers`, `request_body`, `response_code`, `response_body`, `duration_ms`, `error_message`, `created_at` |
| **Behavior**                    | Async — tidak memblokir request utama. Jika Postgres tidak tersedia, logging dinonaktifkan dan service tetap jalan |
| **Konfigurasi**                 | `POSTGRES_DSN` (optional — jika tidak diset, logging disabled) |
