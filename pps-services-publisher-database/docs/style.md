# Go Style Guide — AZEC Backend Services

Panduan gaya kode yang dipakai di proyek ini. Bisa di-copy ke proyek Go lain sebagai template.

---

## 1. Struktur Proyek (Clean Architecture)

```
cmd/app/              → Entry point & DI (Wire)
internal/
  config/             → Application config
  domain/
    entity/           → Domain entities (pure struct, no import luar)
    contract/
      repository/     → Interface repository
      service/        → Interface infrastructure service
      usecase/        → Interface usecase
    errors/           → Domain-specific error types
    valueobject/      → Value objects (Money, ResponseCode, Timestamp)
  dto/
    request/          → HTTP request DTO
    response/         → HTTP response DTO
    mapper/           → Mapper: DTO ↔ Entity (stateless function)
  usecase/            → Business logic per fitur (inquiry/, payment/, dll.)
  delivery/http/
    router.go         → Route registration & middleware chain
    handler/          → HTTP handler per fitur
    presenter/        → Response presenter helper
  middleware/
    chain.go          → ChainBuilder (compose middleware)
    auth/             → Auth middleware (BearerToken)
    security/         → Security (IP whitelist, rate limiter, encrypt/decrypt)
    observability/    → Logging, recovery, request ID
    resilience/       → Circuit breaker, timeout, DB error handler
    transform/        → Response formatting
  repository/         → Implementasi contract repository (postgres/, oracle/, cache/)
  infrastructure/     → Infra adapter (DB, Redis, RabbitMQ, Crypto, Logger, Scheduler)
  pkg/                → Shared utility packages (pure, no domain import)
test/
  mock/               → Mock implementations
  fixture/            → Test fixtures
  testutil/           → Test helpers
```

### Dependency Rule

Arah dependensi **hanya ke dalam** (inner layer tidak pernah import outer layer):

```
Delivery → Usecase contract + DTO
Usecase  → Domain (entity + contract)
Repository/Infrastructure → Domain (implements contract)
cmd/app  → Wires everything (Google Wire)
```

---

## 2. Naming Conventions

### File

| Layer          | Pattern                      | Contoh                  |
|---------------|------------------------------|-------------------------|
| Entity        | `<feature>.go`               | `inquiry.go`            |
| Contract      | `<feature>_<type>.go`        | `inquiry_repository.go` |
| Error         | `<jenis>_error.go`           | `business_error.go`     |
| Handler       | `<feature>_handler.go`       | `inquiry_handler.go`    |
| Usecase       | `<feature>_usecase.go`       | `inquiry_usecase.go`    |
| Repository    | `<feature>_repository.go`    | `inquiry_repository.go` |
| DTO Request   | `<feature>_request.go`       | `inquiry_request.go`    |
| DTO Response  | `<feature>_response.go`      | `inquiry_response.go`   |
| Mapper        | `<feature>_mapper.go`        | `inquiry_mapper.go`     |
| Middleware     | `<concern>.go`               | `bearer_token.go`       |
| Test          | `<target>_test.go`           | `bearer_token_test.go`  |

### Package

- Gunakan **lowercase, single word** jika memungkinkan.
- Import alias hanya jika konflik: `contractsvc "…/contract/service"`.

### Struct & Interface

```go
// Interface — di domain/contract/, tanpa prefix "I"
type InquiryUsecase interface { ... }
type InquiryRepository interface { ... }
type Logger interface { ... }

// Implementasi — di layer luar
type InquiryUsecaseImpl struct { ... }
```

### Constructor

```go
// Selalu prefix New + nama struct
func NewInquiryHandler(uc contractuc.InquiryUsecase, logger contractsvc.Logger) *InquiryHandler
func NewBusinessError(code, message string) *BusinessError
```

---

## 3. Pola Handler (Delivery Layer)

Setiap handler mengikuti pola **5 langkah**:

```go
func (h *InquiryHandler) handleInquiry(c *fiber.Ctx) error {
    // 1. Parse DTO from decrypted body
    decryptedBody, ok := c.Locals("decryptedBody").([]byte)
    if !ok || decryptedBody == nil {
        return presenter.Error(c, nil)
    }
    var req request.InquiryRequest
    if err := json.Unmarshal(decryptedBody, &req); err != nil {
        return presenter.Error(c, err)
    }

    // 2. Map DTO → Domain Entity
    domainReq := mapper.InquiryRequestToEntity(&req)

    // 3. Call Usecase
    domainResp, err := h.usecase.Inquiry(c.UserContext(), domainReq)
    if err != nil {
        return presenter.Error(c, err)
    }

    // 4. Map Domain Entity → Response DTO
    respDto := mapper.InquiryResponseFromEntity(domainResp)

    // 5. Return JSON
    return presenter.Success(c, respDto)
}
```

### Route Registration

Handler mendaftarkan endpoint via method `RegisterRoutes`:

```go
func (h *InquiryHandler) RegisterRoutes(router fiber.Router) {
    router.Post("/inquiry", h.handleInquiry)
}
```

Router memanggil `RegisterRoutes` setelah middleware ter-apply ke group:

```go
apiGroup := r.app.Group("/api/v1", apiChain...)
r.inquiryHandler.RegisterRoutes(apiGroup)
```

---

## 4. Pola Middleware

### Chain Builder

Middleware di-compose dengan `ChainBuilder`, bukan langsung `app.Use()`:

```go
apiChain := middleware.NewChainBuilder().
    Use(resilience.DatabaseErrorHandler(r.logger)).
    Use(security.IPWhitelist(r.cache, r.productRepo, r.logger)).
    Use(security.RateLimiter(r.cache, r.logger)).
    Use(auth.BearerToken(r.tokenUsecase, r.crypto, r.digitalSig, r.logger)).
    Use(security.DecryptRequest(r.crypto, r.digitalSig, r.logger)).
    Use(observability.HTTPLogger(r.logger, r.httpLoggingRepo, r.crypto)).
    Use(security.EncryptResponse(r.crypto, r.digitalSig, r.logger)).
    Use(transform.ResponseFormatter(r.logger)).
    Build()
```

### Urutan Middleware (penting!)

1. **Resilience** — DB error handler, circuit breaker (paling luar, tangkap panic/error infra)
2. **Security** — IP whitelist, rate limiter
3. **Auth** — Bearer token verification
4. **Security** — Decrypt request body
5. **Observability** — HTTP logger (log setelah body terdecrypt)
6. **Security** — Encrypt response body
7. **Transform** — Format response akhir

### Middleware Signature

Setiap middleware me-return `fiber.Handler`:

```go
func BearerToken(tokenUC contractuc.TokenUsecase, ...) fiber.Handler {
    return func(c *fiber.Ctx) error {
        // logic ...
        return c.Next()
    }
}
```

---

## 5. Pola Domain & Error

### Entity

- Struct murni, tidak import package luar domain.
- Tidak ada JSON tag (JSON tag ada di DTO saja).

```go
type InquiryRequest struct {
    RefID        string
    ClientNumber string
    ProductCode  string
    // ...
}
```

### Custom Error

Semua error domain ada di `internal/domain/errors/`:

| Error Type          | Kegunaan                                |
|--------------------|-----------------------------------------|
| `BusinessError`    | Error bisnis dengan response code       |
| `ValidationError`  | Input tidak valid                       |
| `NotFoundError`    | Resource tidak ditemukan                |
| `DuplicateError`   | Duplikasi data                          |
| `InfraError`       | Error infrastruktur (DB, cache, dll.)   |

```go
// Implements error interface + Unwrap()
func (e *BusinessError) Error() string { ... }
func (e *BusinessError) Unwrap() error { return e.Cause }
```

---

## 6. Pola DTO & Mapper

- **DTO** (`dto/request/`, `dto/response/`) = struct dengan JSON tag, hanya untuk HTTP boundary.
- **Mapper** (`dto/mapper/`) = **pure function**, stateless, tidak ada side effect.

```go
// mapper/inquiry_mapper.go
func InquiryRequestToEntity(dto *request.InquiryRequest) *entity.InquiryRequest { ... }
func InquiryResponseFromEntity(e *entity.InquiryResponse) *response.InquiryResponse { ... }
```

---

## 7. Compile-Time Interface Check

Semua implementasi wajib punya compile-time check:

```go
var _ contractrepo.InquiryRepository = (*InquiryRepositoryImpl)(nil)
```

---

## 8. Testing

### Konvensi

- File test: `<target>_test.go` di package yang sama.
- Mock: `test/mock/` (shared mock untuk repository, service, usecase).
- Fixture: `test/fixture/` (data dummy reusable).
- Gunakan `testify/assert` dan `testify/mock`.

### Pola Table-Driven Test

```go
func TestSomething(t *testing.T) {
    tests := []struct {
        name     string
        input    SomeInput
        expected SomeOutput
        wantErr  bool
    }{
        { name: "success case", ... },
        { name: "validation error", ..., wantErr: true },
    }
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // arrange, act, assert
        })
    }
}
```

### Makefile targets

```bash
make test          # Semua test dengan race detector
make test-unit     # Unit test usecase saja
make coverage      # Generate coverage HTML report
make lint          # Jalankan golangci-lint
make fmt           # gofmt -s -w .
```

---

## 9. Dependency Injection

- Gunakan **Google Wire** (`cmd/app/wire.go`).
- Provider set diorganisir per layer.
- Tidak boleh ada `init()` function untuk DI.

---

## 10. Import Ordering

Ikuti konvensi `goimports`:

```go
import (
    // 1. Standard library
    "context"
    "fmt"

    // 2. Internal packages (module path)
    "azec-backend-services/internal/domain/entity"
    contractuc "azec-backend-services/internal/domain/contract/usecase"

    // 3. External packages
    "github.com/gofiber/fiber/v2"
)
```

---

## 11. Komentar

- Setiap **exported type/function** wajib punya doc comment.
- Format: `// TypeName does ...` atau `// FunctionName creates ...`

```go
// InquiryHandler handles inquiry HTTP requests.
type InquiryHandler struct { ... }

// NewInquiryHandler creates a new InquiryHandler.
func NewInquiryHandler(...) *InquiryHandler { ... }

// RegisterRoutes registers inquiry routes on the given router.
func (h *InquiryHandler) RegisterRoutes(router fiber.Router) { ... }
```
