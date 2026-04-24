# List API — pps-services-gateway-smb

## 1. Internal HTTP Endpoints (Service Ini)

Service ini cuma punya 1 HTTP endpoint — untuk health check:

| Method | Endpoint | Deskripsi | Auth |
|--------|----------|-----------|------|
| GET | `/health` | Health check liveness probe | Tidak ada |

Response:
```json
{"status": "ok", "service": "pps-services-gateway-smb"}
```

> Service ini **bukan HTTP API server**. Dia RabbitMQ consumer.
> Endpoint `/health` cuma untuk Kubernetes/Docker health check.

---

## 2. SMB / Loket Bayar API (External — yang dipanggil service ini)

Base URL: dari env `SMB_BASE_URL`
Auth: MD5 signature `md5(partner_id + secret_key + ref_id)`

### 2.1 Inquiry PLN Token

```
POST {SMB_BASE_URL}/api/v1/pln-prepaid/inquiry
```

Request:
```json
{
  "partner_id": "PARTNER123",
  "client_number": "12345678901",
  "product_code": "PLN50",
  "sign": "md5(partner_id + secret_key + client_number)"
}
```

Response (sukses):
```json
{
  "response_code": "00",
  "message": "Success",
  "data": {
    "ref_id": "REF-001",
    "client_number": "12345678901",
    "client_name": "BUDI SANTOSO",
    "tarif_daya": "R1/900VA",
    "admin_fee": 2500,
    "total_amount": 52500
  }
}
```

Response (gagal):
```json
{
  "response_code": "94",
  "message": "Error Inquiry Data"
}
```

File kode: `pkg/smb/pln_token.go` → `InquiryPLNToken()`

---

### 2.2 Payment PLN Token

```
POST {SMB_BASE_URL}/api/v1/pln-prepaid/payment
```

Request:
```json
{
  "partner_id": "PARTNER123",
  "client_number": "12345678901",
  "product_code": "PLN50",
  "ref_id": "REF-001",
  "total_amount": 52500,
  "sign": "md5(partner_id + secret_key + ref_id)"
}
```

Response (sukses):
```json
{
  "response_code": "00",
  "message": "Success",
  "data": {
    "ref_id": "REF-001",
    "client_number": "12345678901",
    "client_name": "BUDI SANTOSO",
    "token": "1234-5678-9012-3456-7890",
    "serial_number": "SN123456789",
    "total_amount": 52500,
    "admin_fee": 2500
  }
}
```

Response (pending/timeout):
```json
{
  "response_code": "28",
  "message": "Timeout atau Pending Transaksi"
}
```

File kode: `pkg/smb/pln_token.go` → `PaymentPLNToken()`

---

### 2.3 Advice PLN Token (Check Status)

```
POST {SMB_BASE_URL}/api/v1/pln-prepaid/advice
```

Request:
```json
{
  "partner_id": "PARTNER123",
  "client_number": "12345678901",
  "ref_id": "REF-001",
  "sign": "md5(partner_id + secret_key + ref_id)"
}
```

Response (sukses):
```json
{
  "response_code": "00",
  "message": "Success",
  "data": {
    "ref_id": "REF-001",
    "client_number": "12345678901",
    "client_name": "BUDI SANTOSO",
    "token": "1234-5678-9012-3456-7890",
    "serial_number": "SN123456789",
    "total_amount": 52500,
    "admin_fee": 2500
  }
}
```

File kode: `pkg/smb/pln_token.go` → `AdvicePLNToken()`

---

## 3. RabbitMQ — Format Pesan

### 3.1 Pesan Masuk (yang di-consume gateway SMB)

Queue: dari env `QUEUE_NAME_PROVIDER`

```json
{
  "msg_id": "12345",
  "client_number": "12345678901",
  "product_code": "PLN50",
  "product_type": "pln_token",
  "amount": 50000,
  "mid": "MERCHANT_001",
  "queue_name": "queue_smb_pln_token",
  "MQTransaction": "amqp://user:pass@rabbitmq:5672/"
}
```

| Field | Tipe | Wajib | Keterangan |
|-------|------|-------|------------|
| msg_id | string | ✅ | ID unik transaksi |
| client_number | string | ✅ | Nomor meter PLN |
| product_code | string | ✅ | Kode produk (PLN20, PLN50, PLN100) |
| product_type | string | ✅ | Harus `"pln_token"` |
| amount | int | ✅ | Nominal dalam rupiah |
| mid | string | ❌ | ID merchant |
| queue_name | string | ❌ | Override queue name |
| MQTransaction | string | ✅ | URL RabbitMQ untuk publish hasil balik |

File kode: `internal/infrastructure/rabbitmq/consumer_service.go` → `consumePayload`

---

### 3.2 Pesan Keluar (yang di-publish ke downstream)

Queue: dari field `MQTransaction` + `queue_name` di pesan masuk

```json
{
  "source": "PROVIDER",
  "data": {
    "msg_id": 12345,
    "status_to_be": "F",
    "serial_number": "1234-5678-9012-3456-7890",
    "client_number": "12345678901",
    "nominal": "52500",
    "original_conversation_id": "",
    "conversation_id": "SMB-MERCHANT_001-12345-20260422120000",
    "message_to_customer": "Token PLN: 1234-5678-9012-3456-7890",
    "additional_message": "",
    "queue_name": "queue_smb_pln_token"
  }
}
```

| Field | Nilai | Keterangan |
|-------|-------|------------|
| source | `"PROVIDER"` | Selalu "PROVIDER" |
| status_to_be | `"F"` / `"C"` / `"S"` | F=sukses, C=gagal, S=masih proses |
| serial_number | token PLN | Hanya ada kalau sukses |
| msg_id | int | ID transaksi (angka) |
| conversation_id | string | ID transaksi internal gateway |
| message_to_customer | string | Pesan untuk pelanggan |

File kode: `internal/infrastructure/mqpublisher/message.go` → `ProviderPublishMessage`

---

## 4. Response Code Mapping SMB → PPS

| SMB Code | Deskripsi | RC PPS | status_to_be | Aksi |
|----------|-----------|--------|--------------|------|
| 00 | Success | 0 | F | Sukses, token didapat |
| 28 | Timeout/Pending | 9 | S | Retry advice |
| 68 | Timeout/Pending | 9 | S | Retry advice |
| (kosong) | Unknown | 9 | S | Retry advice |
| 93 | Error Payment | 1 | C | Gagal |
| 94 | Error Inquiry Data | 1 | C | Gagal |
| 95 | Error Price Setting | 1 | C | Gagal |
| 97 | Error Client Data | 1 | C | Gagal |
| 98 | Error Parameter | 1 | C | Gagal |
| 99 | Error Credential/IP | 1 | C | Gagal |
| lainnya | Error lain | 1 | C | Gagal |

File kode: `internal/util/rc.go` → `ResolveRCPPS()`

---

## 5. Testing Manual dengan cURL

### Health Check
```bash
curl http://localhost:8080/health
```

### Publish Pesan ke RabbitMQ (simulasi trigger gateway)
Pakai `rabbitmqadmin` atau RabbitMQ Management UI:
```bash
rabbitmqadmin publish \
  exchange="" \
  routing_key="queue_smb_pln_token" \
  payload='{"msg_id":"99001","client_number":"12345678901","product_code":"PLN50","product_type":"pln_token","amount":50000,"mid":"TEST","MQTransaction":"amqp://guest:guest@localhost:5672/","queue_name":"queue_smb_pln_token"}'
```
