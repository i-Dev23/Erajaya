# 🔍 Deep Architectural Audit Report

**Auditor:** Claude Opus 4.6 (Senior Software Architect)  
**Date:** 6 Februari 2026  
**Project:** PPS Services Tokopedia  
**Stack:** Go 1.23, Fiber v2, pgx v5, go-ora v2, Redis, RabbitMQ, Google Wire

---

## Ringkasan Eksekutif

Secara keseluruhan, project ini sudah mengikuti prinsip Clean Architecture dengan baik — layering yang jelas, interface-based design, context propagation, dan dependency injection via Wire. Namun, setelah deep audit terhadap seluruh source code, ditemukan **25+ area perbaikan** yang dikategorikan ke dalam: **Critical (harus segera diperbaiki)**, **High (penting untuk production)**, **Medium (best practice)**, dan **Low (nice to have)**.

---

## 🔴 CRITICAL — Harus Segera Diperbaiki

### 1. Dependency Injection Leak di `checkIdPlnUltima` — Membuat Instance Baru di Dalam Usecase

**File:** `internal/usecase/inquiry_usecase.go` (fungsi `checkIdPlnUltima`)

```go
// ❌ MASALAH: Membuat instance service baru langsung di dalam usecase
ultimaService := service.NewUltimaService(u.logger)
ultimaResp, parsedData, err := ultimaService.CheckIdPlnUltima(ctx, ...)
```

**Masalah:**  
- Melanggar Dependency Injection — usecase seharusnya menggunakan `u.ultimaService` yang sudah di-inject via Wire.
- Service baru dibuat setiap kali fungsi dipanggil → tidak reuse HTTP client & connection.
- Tidak bisa di-mock saat testing → test menjadi tightly coupled ke implementasi.

**Solusi:**
```go
// ✅ Gunakan yang sudah di-inject
ultimaResp, parsedData, err := u.ultimaService.CheckIdPlnUltima(ctx, ...)
```

---

### 2. Sync.Pool Misuse di `InquiryHandler` — Use After Return to Pool (Race Condition)

**File:** `internal/delivery/http/inquiry_handler.go`

```go
func (h *InquiryHandler) convertDomainToDtoOptimized(domain *domain.InquiryResponseDomain) dto.InquiryResponseDto {
    billDetails := billDetailPool.Get().([]dto.BillDetailDto)
    billDetails = billDetails[:0]
    
    for _, detail := range domain.BillDetails {
        billDetails = append(billDetails, ...)
    }
    
    response := dto.InquiryResponseDto{
        BillDetails: billDetails,  // ← response memegang reference ke slice
    }
    
    billDetailPool.Put(billDetails)  // ❌ RACE CONDITION: slice dikembalikan ke pool tapi masih dipakai oleh response
    
    return response
}
```

**Masalah:**
- Setelah `billDetailPool.Put(billDetails)`, slice bisa diambil goroutine lain dan di-overwrite.
- Response yang dikembalikan masih memegang reference ke slice yang sama → **data corruption** saat concurrent requests.
- Ini adalah **race condition** yang sangat sulit di-debug di production.

**Solusi:**  
Hapus penggunaan `sync.Pool` untuk case ini. Overhead allocation slice kecil jauh lebih aman daripada race condition:
```go
func (h *InquiryHandler) convertDomainToDto(resp *domain.InquiryResponseDomain) dto.InquiryResponseDto {
    billDetails := make([]dto.BillDetailDto, 0, len(resp.BillDetails))
    for _, detail := range resp.BillDetails {
        billDetails = append(billDetails, dto.BillDetailDto{...})
    }
    return dto.InquiryResponseDto{BillDetails: billDetails, ...}
}
```

---

### 3. Singleton Pattern di Logger & Redis — Menghambat Testability & Hot Reload

**File:** `internal/service/logger_service.go`, `internal/service/redis_service.go`

```go
var (
    once     sync.Once
    instance Logger
)

func NewLogger() Logger {
    once.Do(func() { ... })
    return instance
}
```

**Masalah:**
- Singleton `sync.Once` membuat service hanya bisa diinstansiasi 1x sepanjang lifecycle process.
- Saat unit test, **tidak bisa di-reset atau di-replace** — test bisa saling mempengaruhi.
- Jika Redis/Logger gagal saat init, tidak bisa retry atau reconnect — stuck selamanya.
- Wire sudah menangani lifecycle management → singleton di Wire level, bukan di service level.

**Solusi:**
Hapus singleton pattern, biarkan Wire yang mengelola lifecycle:
```go
func NewLogger(telegramService TelegramService) Logger {
    // Langsung return instance baru, Wire menjamin hanya 1x dipanggil
    return &loggerImpl{...}
}
```

---

### 4. Hardcoded Credentials & Sensitive Data in Logs

**File:** `internal/usecase/token_usecase.go`

```go
func (u *tokenUsecaseImpl) validateClientCredentials(clientID, clientSecret string) error {
    // ❌ Logging client_secret yang gagal validasi
    u.logger.Warn("Invalid client_secret provided", "provided", clientSecret)
    ...
}
```

**File:** `internal/delivery/http/payment_handler.go`

```go
h.logger.Info("Payment API Request Started",
    ...
    "request_body", string(decryptedBody))  // ❌ Log decrypted body (bisa mengandung PII)
```

**Masalah:**
- Client secret di-log — jika log terekspos, credential bocor.
- Decrypted request body (yang berisi data pelanggan PLN) di-log tanpa masking.
- Melanggar PCI-DSS dan data protection compliance.

**Solusi:**
```go
// Jangan log credential
u.logger.Warn("Invalid client_secret provided")

// Mask sensitive fields
h.logger.Info("Payment API Request Started",
    "ref_id", reqDto.RefID,
    "product_code", reqDto.ProductCode)
```

---

### 5. Unsafe Type Assertion Tanpa Check — Panic di Production

**File:** `internal/delivery/http/inquiry_handler.go`

```go
decryptedBody := c.Locals("decryptedBody").([]byte)  // ❌ Panic jika nil
```

**File:** `internal/delivery/http/payment_handler.go`

```go
decryptedBody := c.Locals("decryptedBody").([]byte)  // ❌ Panic jika nil
```

**Masalah:**
- Jika middleware decrypt gagal atau di-skip, `c.Locals("decryptedBody")` return `nil`.
- Type assertion ke `.([]byte)` pada nil → **panic** → server crash.

**Solusi:**
```go
rawBody, ok := c.Locals("decryptedBody").([]byte)
if !ok || rawBody == nil {
    return c.Status(400).JSON(fiber.Map{"error": "missing decrypted body"})
}
```

---

## 🟠 HIGH — Penting untuk Production Readiness

### 6. `main.go` Terlalu Besar — 541 Baris dengan Business Logic

**File:** `cmd/app/main.go`

**Masalah:**
- `setupScheduledJobs()` berisi ~300 baris logic termasuk reconciliation report generation (CSV export, query DB, format data).
- Reconciliation export logic (line 430-540) seharusnya ada di usecase layer, bukan di `main.go`.
- Environment variable parsing (cron schedule, retention days) dilakukan berulang kali di dalam job handler.
- `main.go` seharusnya hanya: init app → register routes → start server → handle shutdown.

**Solusi:**
- Pindahkan job handlers ke `usecase/scheduler_usecase.go` atau `usecase/reconciliation_usecase.go`.
- Pindahkan cron configuration ke `config.Config`.
- `main.go` hanya memanggil `appContainer.SchedulerUsecase.RegisterJobs()`.

---

### 7. Goroutine Tanpa Error Handling di Startup Cache

**File:** `cmd/app/main.go` (line 119-195)

```go
go func() {
    products, err := appContainer.ProductRepo.GetProductByUser(ctx, username)
    if err != nil {
        appContainer.Logger.Error(...)
    } else {
        for _, product := range *products {
            go func(p domain.ProductPriceResponseDomain) {  // ❌ Goroutine di dalam goroutine
                ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
                defer cancel()
                // Tidak ada error handling untuk Set
                appContainer.RedisClient.Set(ctx, cacheKey, productJSON, 24*time.Hour)
            }(product)
        }
    }
}()
```

**Masalah:**
- Nested goroutines tanpa WaitGroup → tidak ada jaminan semua cache tersimpan sebelum server mulai menerima request.
- Jika ada 1000 produk, spawn 1000 goroutines sekaligus → bisa overwhelm Redis.
- Menggunakan `fmt.Sprintf` untuk build JSON (`productJSON := fmt.Sprintf(...)`) → rentan injection & format error.

**Solusi:**
- Gunakan `errgroup` atau `sync.WaitGroup` dengan semaphore.
- Gunakan `json.Marshal` instead of `fmt.Sprintf`.
- Batasi concurrency (e.g., max 10 goroutines).

---

### 8. Tidak Ada Request Validation Library — Manual Validation

**File:** `internal/usecase/inquiry_usecase.go` → `validateMandatoryParamsBeforeInsert()`

DTO sudah punya tag `validate:"required"` tapi tidak pernah digunakan. Validation dilakukan manual satu per satu.

**Masalah:**
- Duplikasi validation logic antara DTO tags dan usecase manual validation.
- DTO tags `validate:"required"` tidak pernah di-enforce → misleading.
- Rentan missed validation saat menambah field baru.

**Solusi:**
Gunakan `github.com/go-playground/validator/v10`:
```go
validate := validator.New()
if err := validate.Struct(reqDto); err != nil {
    return buildErrorResponse("42")
}
```

---

### 9. Middleware `encryptAndSignBearerErrorResponse` — Path-Based Switch Case

**File:** `internal/middleware/crypto_middlerware.go`

```go
if pathUrlApi == "/auth/token" {
    // ...
} else if pathUrlApi == "/api/v1/inquiry" {
    // ...
} else if pathUrlApi == "/api/v1/payment" {
    // ...
} else if pathUrlApi == "/api/v1/check-status" {
    // ...
}
```

**Masalah:**
- Middleware tightly coupled ke route paths → setiap endpoint baru harus update middleware.
- Typo pada path string tidak akan terdeteksi saat compile time.
- Open/Closed Principle violation — harus modify existing code untuk extend.

**Solusi:**
Gunakan strategy pattern atau middleware config yang mendaftarkan response builder per route:
```go
type ErrorResponseBuilder func(c *fiber.Ctx, code, message string) interface{}

var responseBuilders = map[string]ErrorResponseBuilder{
    "/auth/token":          buildTokenErrorResponse,
    "/api/v1/inquiry":      buildInquiryErrorResponse,
    // ...
}
```

---

### 10. Duplikasi Kode Cut-Off Validation (Inquiry & Payment)

**File:** `internal/usecase/inquiry_usecase.go` dan `internal/usecase/payment_usecase.go`

Kedua file memiliki **copy-paste yang identik** untuk:
- `validateCutOffRedis()` (~60 baris)
- `validateCutOffFromOracle()` (~80 baris)
- `within()` helper function (~20 baris)
- `buildErrorResponse()` (~15 baris)

**Masalah:**
- DRY violation — perubahan di satu tempat harus diubah di tempat lain.
- Bug fix di inquiry cut-off bisa missed di payment cut-off.
- ~160 baris kode duplikat.

**Solusi:**
Extract ke shared service atau utility:
```go
// internal/usecase/cutoff_validator.go
type CutOffValidator struct {
    redisClient service.RedisClient
    productRepo domain.ProductRepository
    cutOffRepo  domain.CutOffRepository
    logger      service.Logger
}

func (v *CutOffValidator) Validate(ctx context.Context) (bool, error) {
    // Shared logic
}
```

---

### 11. Typo di Filename — `crypto_middlerware.go` (extra 'r')

**File:** `internal/middleware/crypto_middlerware.go`

**Masalah:** Typo `middlerware` → seharusnya `middleware`.

**Dampak:** Membingungkan developer lain, mencerminkan kurangnya code review.

**Solusi:** Rename ke `crypto_middleware.go`.

---

### 12. `healty_domain.go` — Typo di Filename

**File:** `internal/domain/healty_domain.go`

**Masalah:** Typo `healty` → seharusnya `health` atau `healthy`.

**Solusi:** Rename ke `health_domain.go`.

---

## 🟡 MEDIUM — Best Practice Improvements

### 13. Service Interface Didefinisikan di Service Package, Bukan di Domain

**File:** `internal/service/logger_service.go`, `internal/service/redis_service.go`, `internal/service/postgres_service.go`, dll.

```go
package service

type Logger interface { ... }
type RedisClient interface { ... }
type PostgresService interface { ... }
```

**Masalah (Nuansa):**
Menurut Clean Architecture, interface seharusnya didefinisikan oleh **consumer** (domain/usecase layer), bukan oleh implementor (service layer). Ini membuat usecase layer import dari service layer → inner layer depend ke outer layer.

**Impact saat ini:**
- `internal/usecase/inquiry_usecase.go` import `pps-services-tokopedia/internal/service` hanya untuk type `service.Logger`, `service.RedisClient`.
- Ini menciptakan coupling yang seharusnya tidak ada.

**Solusi (Phased):**
1. Definisikan interface `Logger`, `Cache`, `MessageQueue` di `internal/domain/` atau buat `internal/port/` package.
2. Service layer mengimplementasikan interface dari domain/port.
3. Usecase hanya depend ke domain interfaces.

> **Catatan:** Ini adalah refactoring besar. Bisa dilakukan bertahap — mulai dari `Logger` interface dulu.

---

### 14. Tidak Ada Graceful Shutdown untuk Goroutines

**File:** `cmd/app/main.go`

Saat shutdown signal diterima, goroutines yang sedang berjalan (cache warmup, async Redis save, callback consumer) tidak di-cancel secara eksplisit.

**Masalah:**
- Goroutine leak saat graceful shutdown.
- Cache save yang sedang berjalan bisa terpotong.

**Solusi:**
- Gunakan `context.WithCancel` yang sama untuk semua startup goroutines.
- Tambahkan `sync.WaitGroup` untuk menunggu goroutines selesai sebelum exit.

---

### 15. `CleanupUsecase` Langsung Menggunakan `PostgresService` — Bypass Repository Pattern

**File:** `internal/usecase/cleanup_usecase.go`

```go
type cleanupUsecaseImpl struct {
    postgresService service.PostgresService  // ❌ Direct DB access di usecase
    logger          service.Logger
}
```

**Masalah:**
- Usecase langsung memanggil `postgresService.Query()` → bypass repository layer.
- Melanggar arsitektur → usecase seharusnya hanya akses data via repository interface.

**Solusi:**
Buat `CleanupRepository` interface di domain layer, implementasi di repository layer.

---

### 16. Reconciliation Report Logic di `main.go` — Business Logic di Entry Point

**File:** `cmd/app/main.go` (line ~430-540)

```go
// Di dalam setupScheduledJobs:
query := "SELECT * FROM payment.get_daily_reconciliation_report($1)"
rows, err := appContainer.PostgresService.Query(ctx, query, reportDate)
// ... CSV generation logic ...
```

**Masalah:**
- Full business logic (query DB, generate CSV, write file) ada di `main.go`.
- Tidak bisa di-test secara unit.
- Mixing infrastructure concerns (file I/O) dengan business logic (report generation).

**Solusi:**
Buat `ReconciliationUsecase` dan `ReconciliationRepository`.

---

### 17. Redis Key Naming Tidak Konsisten

**Dari analisis seluruh codebase:**

| Key Pattern | Contoh | Layer |
|---|---|---|
| Plain product code | `PLN20` | inquiry_usecase (product price cache) |
| `product_with_status:{code}` | `product_with_status:PLN20` | inquiry_usecase |
| Plain client number | `08123456789` | inquiry_usecase (PLN cache) |
| `WHITELISTED_IP` | uppercase constant | main.go startup |
| `CUT_OFF_TIME_START` | uppercase constant | main.go startup |
| `rate_limit:{ip}:{path}` | structured | rate_limit_middleware |

**Masalah:**
- Tidak ada namespace/prefix standar → risiko key collision.
- Plain product code `PLN20` sebagai key bisa collide dengan key lain.
- Tidak ada documentation tentang key schema.

**Solusi:**
Standarisasi prefix: `pps:{entity}:{identifier}`
```
pps:product:price:PLN20
pps:product:status:PLN20
pps:pln:inquiry:08123456789
pps:config:whitelist_ip
pps:config:cutoff:*
pps:ratelimit:{ip}:{path}
```

---

### 18. Tidak Ada Timeout pada Context di Usecase Layer

**File:** `internal/usecase/inquiry_usecase.go`, `internal/usecase/payment_usecase.go`

Usecase method menerima `ctx` dari handler tapi tidak pernah menambahkan timeout:

```go
func (u *inquiryUsecaseImpl) Inquiry(ctx context.Context, req *domain.InquiryRequestDomain) (*domain.InquiryResponseDomain, error) {
    // ctx langsung digunakan tanpa timeout
    productPriceResp, fromCache, err := u.getProductWithStatus(ctx, req.ProductCode)
    plnData, err := u.getPLNInquiry(ctx, req.ClientNumber)
    // ...
}
```

**Masalah:**
- Jika Oracle atau Ultima service hang, request bisa menggantung indefinitely.
- Fiber default timeout bisa terlalu lama untuk chain of calls.

**Solusi:**
```go
func (u *inquiryUsecaseImpl) Inquiry(ctx context.Context, req *domain.InquiryRequestDomain) (...) {
    ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
    defer cancel()
    // ...
}
```

---

### 19. Mock Files di Production Package — Bukan `_test.go`

**File:** `internal/usecase/mock_deps.go`, `internal/usecase/mock_extra_deps.go`, `internal/usecase/mock_error_mapping_repo.go`, `internal/usecase/mock_rabbitmq_service.go`

**Masalah:**
- Mock files tanpa suffix `_test.go` → ikut di-compile ke production binary.
- Menambah ukuran binary dan attack surface.
- Best practice: mock harus di file `*_test.go` atau di package terpisah (`internal/usecase/testmocks/`).

**Solusi:**
- Rename semua mock file ke `mock_*_test.go` (agar hanya di-compile saat test), atau
- Pindahkan ke `internal/usecase/testmocks/` (sudah ada folder-nya tapi belum digunakan konsisten).

---

### 20. Dockerfile Formatting — Indentasi Tidak Konsisten

**File:** `Dockerfile`

```dockerfile
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o pps-services-tokopedia ./cmd/app
    
    
    # ---------- STAGE 2: Final ----------
    FROM alpine:3.22
    
    # Build arguments...
    ARG APP_USER=it
```

**Masalah:**
- Stage 2 menggunakan indentasi 4 spaces → tidak standar Dockerfile.
- Bisa menyebabkan masalah di beberapa Docker build tools.

**Solusi:**
Hapus indentasi pada stage 2. Semua instruction di Dockerfile harus flush-left.

---

## 🟢 LOW — Nice to Have

### 21. Tidak Ada Structured Error Types — String Matching untuk Error Handling

**File:** `internal/utils/db_error_utils.go`

```go
if strings.Contains(errMsg, "connection refused") ||
   strings.Contains(errMsg, "cannot connect") ||
   strings.Contains(errMsg, "connection reset") || ...
```

**Masalah:**
- String matching fragile — error message bisa berubah di library update.
- Tidak idiomatic Go — seharusnya pakai `errors.Is()` / `errors.As()`.

**Solusi (Long term):**
Wrap errors di service layer dengan custom error types:
```go
type ConnectionError struct {
    Service string
    Err     error
}
```

---

### 22. `time.Now()` Tanpa Timezone Explicit

**Multiple files**

```go
time.Now().Format("2006-01-02 15:04:05")
```

**Masalah:**
- Bergantung pada timezone server/container → bisa berbeda antar environment.
- Bisa inconsistent antara Docker (UTC) dan local dev (WIB).

**Solusi:**
```go
loc, _ := time.LoadLocation("Asia/Jakarta")
time.Now().In(loc).Format("2006-01-02 15:04:05")
```

Atau set `TZ=Asia/Jakarta` di semua environment (sudah ada di Dockerfile, tapi belum tentu di local dev).

---

### 23. Logger Tidak Support Structured Key-Value dengan Format String

**File:** `internal/service/ultima_service.go`

```go
u.logger.Info("Starting CheckIdPlnUltima request for idPel: %s, idTrx: %s", req.IdPel, req.IdTrx)
```

**Masalah:**
Logger menggunakan key-value pattern (`msg, key1, val1, key2, val2`), bukan `fmt.Sprintf`. Tapi di beberapa tempat, format string digunakan → placeholder `%s` akan terprint literally.

**Solusi:**
```go
u.logger.Info("Starting CheckIdPlnUltima request", "idPel", req.IdPel, "idTrx", req.IdTrx)
```

---

### 24. Tidak Ada Health Check untuk Dependencies

**File:** `internal/usecase/health_check_usecase.go`

Health check endpoint hanya return "OK" tanpa cek koneksi ke Postgres, Oracle, Redis, RabbitMQ.

**Solusi:**
Tambahkan deep health check (opsional endpoint terpisah `/health/deep`):
```go
func (u *healthCheckUsecase) DeepHealthCheck(ctx context.Context) map[string]string {
    results := map[string]string{}
    if err := u.postgres.Ping(ctx); err != nil {
        results["postgres"] = "unhealthy"
    } else {
        results["postgres"] = "healthy"
    }
    // ... redis, oracle, rabbitmq
    return results
}
```

---

### 25. Tidak Ada Rate Limit per Client/Token — Hanya per IP

**File:** `internal/middleware/rate_limit_middleware.go`

```go
func getClientIdentifier(c *fiber.Ctx) string {
    clientIP := c.Get("X-Real-IP")
    if clientIP == "" {
        clientIP = c.IP()
    }
    return clientIP
}
```

**Masalah:**
- Rate limit hanya berdasarkan IP.
- Di belakang load balancer/CDN, banyak request bisa datang dari IP yang sama.
- Bisa di-bypass dengan rotating proxy.

**Solusi:**
Kombinasikan IP + Bearer token/Client ID:
```go
key := fmt.Sprintf("rate_limit:%s:%s:%s", clientID, clientIP, path)
```

---

### 26. `BillCount` Type Inconsistency

**Domain:**
```go
// inquiry_domain.go
BillCount int  // int

// postgres_inquiry_domain.go  
BillCount float64  // float64 di insert request
```

**Masalah:** `BillCount` seharusnya selalu `int`, tapi di `InquiryResponseInsertRequest` didefinisikan sebagai `float64`.

---

## 📊 Ringkasan Prioritas

| Prioritas | Jumlah | Action |
|---|---|---|
| 🔴 Critical | 5 | Harus diperbaiki sebelum production deployment berikutnya |
| 🟠 High | 7 | Harus diperbaiki dalam 1-2 sprint |
| 🟡 Medium | 8 | Plan dalam backlog, perbaiki bertahap |
| 🟢 Low | 6 | Nice to have, perbaiki saat menyentuh kode terkait |

---

## 🛣️ Recommended Roadmap

### Sprint 1 (Immediate)
1. ✅ Fix `checkIdPlnUltima` — gunakan injected `u.ultimaService`
2. ✅ Fix `sync.Pool` race condition di `InquiryHandler`
3. ✅ Fix unsafe type assertions di semua handler
4. ✅ Remove sensitive data dari log output
5. ✅ Fix typo filename (`crypto_middlerware.go`, `healty_domain.go`)

### Sprint 2 (Short-term)
6. ✅ Extract scheduled job logic dari `main.go` ke usecase layer
7. ✅ Standarisasi Redis key naming
8. ✅ Add context timeout di usecase layer
9. ✅ Pindahkan mock files ke `_test.go` atau `testmocks/`
10. ✅ Fix Dockerfile indentation

### Sprint 3 (Medium-term)
11. ✅ Extract shared cut-off validation logic
12. ✅ Implement proper request validation (go-playground/validator)
13. ✅ Refactor `CleanupUsecase` menggunakan repository pattern
14. ✅ Refactor middleware error response builder ke strategy pattern
15. ✅ Add deep health check endpoint

### Sprint 4+ (Long-term)
16. ✅ Migrate service interfaces ke domain/port layer
17. ✅ Remove singleton pattern dari Logger & Redis
18. ✅ Implement structured error types
19. ✅ Add rate limit per client ID
20. ✅ Add graceful shutdown untuk background goroutines

---

## ✅ Hal yang Sudah Baik

Agar seimbang, berikut aspek yang **sudah dikerjakan dengan baik**:

1. **Clean Architecture layering** — Domain, Usecase, Repository, Service, Delivery terpisah dengan jelas.
2. **Dependency Injection via Wire** — Proper compile-time DI, provider grouping yang rapi.
3. **Context propagation** — Hampir semua DB/external calls menerima `context.Context`.
4. **Parameterized queries** — Tidak ada SQL injection risk di repository layer.
5. **Table-driven tests** — Test pattern yang konsisten dan mudah di-extend.
6. **Graceful shutdown** — Signal handling dan resource cleanup yang proper.
7. **Multi-stage Docker build** — Binary kecil, non-root user, health check.
8. **Structured logging** — Key-value pattern yang konsisten (dengan catatan beberapa tempat masih pakai format string).
9. **Error mapping system** — DB-first fallback ke static map untuk error code resolution.
10. **RabbitMQ reconnection logic** — Auto-reconnect dengan retry yang robust.
11. **Crypto middleware chain** — Decrypt → Verify → Process → Sign → Encrypt pipeline yang benar.
12. **Database partitioning** — Maintenance job untuk monthly partition management.

---

*Report ini dibuat berdasarkan analisis statis terhadap seluruh source code. Untuk audit lebih lengkap, disarankan juga melakukan: load testing, security penetration testing, dan dependency vulnerability scanning (`govulncheck`).*
