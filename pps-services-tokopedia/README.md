# PPS Services Tokopedia – Clean Architecture Template

This repository is a production-ready reference implementing Clean Architecture in Go. Use it as a template to bootstrap new services or to refactor existing ones. It demonstrates clear layering, dependency injection with Wire, context propagation, parameterized queries, and table-driven tests.

---

## ⚡ Quickstart

### Run locally (without Docker)
```bash
go mod download

# Generate DI code (if not generated yet)
cd cmd/app && wire && cd -

# Export minimal env (adjust to your environment)
export APP_PORT=3001
export POSTGRES_DSN="postgres://user:pass@localhost:5432/dbname"
export ORACLE_DSN="oracle://user:pass@localhost:1521/SERVICE"
export REDIS_ADDR="localhost:6379"
export RABBITMQ_URL="amqp://guest:guest@localhost:5672/"

# Run
go run ./cmd/app
```

### Run with Docker
```bash
# Build & run default compose (DEV)
docker compose up --build

# Or SIT variant
docker compose -f docker-compose-sit.yml up --build
```

Healthcheck: `GET http://localhost:3001/health` (container maps to host port 3002 or 3003 per compose).

---

## 🔑 Environment Variables
These are commonly used (see `docker-compose.yml` and `docker-compose-sit.yml` for full values):

- APP_PORT: HTTP port (default 3001)
- ORACLE_DSN, ORACLE_MAX_OPEN_CONNS, ORACLE_MAX_IDLE_CONNS, ORACLE_CONN_MAX_LIFETIME
- POSTGRES_DSN, POSTGRES_MAX_CONNS, POSTGRES_MIN_CONNS, POSTGRES_MAX_CONN_LIFETIME
- REDIS_ADDR, REDIS_DB, REDIS_PASSWORD
- RABBITMQ_URL, RABBITMQ_PUBLISH_TIMEOUT, RABBITMQ_QUEUE_NAME, RABBITMQ_CALLBACK_QUEUE_NAME
- CONSUMER_CALLBACK (Y|N): enable/disable RabbitMQ callback consumer
- TP_CLIENT_ID, TP_CLIENT_SECRET
- PRIVATE_KEY, PUBLIC_KEY: RSA keys (PEM base64 string)
- ULTIMA_GATEWAY_BASE_URL
- HTTP_LOG_RETENTION_DAYS, CALLBACK_LOG_RETENTION_DAYS, INQUIRY_AND_PAYMENT_LOG_RETENTION_DAYS
- RUN_JOBS_LOG_RETENTION: Y|N to enable scheduler jobs
- PROVIDER_CODE_GET_PRICE
- TELEGRAM_BOT_TOKEN, TELEGRAM_CHAT_ID
- CALLBACK_URL, GUID
- RATE_LIMIT_TOKEN(S), RATE_LIMIT_HEALTH_CHECK, RATE_LIMIT_INQUIRY, RATE_LIMIT_PAYMENT, RATE_LIMIT_CHECK_STATUS

File log cleanup:
- FILE_LOG_RETENTION_DAYS: days to keep log files (default 50)
- FILE_LOG_RETENTION_CRON: cron for file log cleanup (default `0 40 1 * * *`)

---

## 📝 Logging

This service writes logs to files by default (daily rotation). It can also log to stdout if desired.

- Default: file logging enabled. Configure directory via `PATH_LOG` (defaults to `./logs`).
- Files are named `YYYY-MM-DD.log` and rotate automatically each day.
- Disable file logging by setting `LOG_TO_FILE=N` (accepted: `N`, `NO`, `FALSE`).

Windows PowerShell examples:

Enable (default) with custom directory:
```powershell
$env:PATH_LOG = "C:\\logs\\pps-tokopedia"
go run ./cmd/app
```

Disable file logging (use stdout only):
```powershell
$env:LOG_TO_FILE = "N"
go run ./cmd/app
```

---

## 🧭 Request Flow (Handler → Usecase → Repository/Service → Domain)
1. Delivery (`internal/delivery/http`): parse/validate request, map to domain, call usecase, map response to DTO.
2. Usecase (`internal/usecase`): orchestrate business rules using repositories and services; always pass `context.Context`.
3. Repository (`internal/repository`): data access with parameterized queries; no business logic.
4. Service (`internal/service`): external integrations (Postgres, Oracle, Redis, RabbitMQ, Ultima, Crypto, Logger, Scheduler).
5. Domain (`internal/domain`): entities and interfaces; no external deps.

Middleware (`internal/middleware`) handles cross-cutting concerns: IP whitelist, rate limit, crypto, HTTP logging.

---

## 🚀 Apply This Structure to New Projects
1. Copy the folder structure under `internal/` and `cmd/app/`.
2. Define entities and interfaces in `internal/domain` first.
3. Implement services (DB/client wrappers) in `internal/service` and repositories in `internal/repository` using parameterized queries.
4. Put business logic in `internal/usecase`; accept interfaces; inject via constructors; pass `context.Context`.
5. Keep `internal/delivery/http` thin; map DTOs ↔ domain; no business logic.
6. Configure providers in `cmd/app/wire.go`; run `wire` to generate DI.
7. Add middleware as needed; keep it stateless and fast.
8. Write table-driven tests for usecases; mock interfaces; add handler tests where helpful.

Tip: Keep module name updated in `go.mod`, and regenerate Wire after adding constructors.

---

---

## 📋 Table of Contents

1. [Architecture Overview](#architecture-overview)
2. [Layer Responsibilities](#layer-responsibilities)
3. [Project Structure](#project-structure)
4. [Refactoring Strategy](#refactoring-strategy)
5. [Implementation Guide](#implementation-guide)
6. [Dependency Injection with Wire](#dependency-injection-with-wire)
7. [Best Practices & Patterns](#best-practices--patterns)
8. [Testing Strategy](#testing-strategy)
9. [Examples & Code Patterns](#examples--code-patterns)

---

## 🏛️ Architecture Overview

This project implements **Clean Architecture** with the following principles:

### Core Principles
1. **Separation of Concerns** - Each layer has a single responsibility
2. **Dependency Rule** - Dependencies point inward (Domain ← Use Case ← Repository/Service)
3. **Interface-Based Design** - Use interfaces for testability and flexibility
4. **Context Propagation** - Always use `context.Context` for DB and external calls
5. **Dependency Injection** - Use Google Wire for automatic DI code generation

### Architectural Layers (Inner → Outer)

```
┌─────────────────────────────────────────────────────┐
│                   Delivery Layer                    │
│           (HTTP Handlers, gRPC, CLI)                │
│                                                     │
│  ┌───────────────────────────────────────────────┐ │
│  │              Use Case Layer                   │ │
│  │          (Business Logic)                     │ │
│  │                                               │ │
│  │  ┌─────────────────────────────────────────┐ │ │
│  │  │         Domain Layer                    │ │ │
│  │  │   (Entities & Interfaces)               │ │ │
│  │  └─────────────────────────────────────────┘ │ │
│  │                                               │ │
│  │  ┌─────────────────┐  ┌──────────────────┐  │ │
│  │  │   Repository    │  │     Service      │  │ │
│  │  │      Layer      │  │      Layer       │  │ │
│  │  │  (Data Access)  │  │   (External)     │  │ │
│  │  └─────────────────┘  └──────────────────┘  │ │
│  └───────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

---

## 📦 Layer Responsibilities

### 1. Domain Layer (`internal/domain/`)
**Purpose:** Core business entities and contracts (interfaces)

**Contains:**
- Entity structs (business objects)
- Repository interfaces
- Service interfaces
- Business rules and validations

**Rules:**
- ✅ Pure business logic, no external dependencies
- ✅ No framework imports (no fiber, no database drivers)
- ✅ Define interfaces that outer layers implement
- ❌ Never import from other layers

**Example:**
```go
// internal/domain/inquiry_domain.go
package domain

import "context"

// Entity
type InquiryRequest struct {
    ID            int64
    ClientNumber  string
    ProductCode   string
    Amount        float64
}

// Repository Interface (implemented by repository layer)
type InquiryRepository interface {
    Save(ctx context.Context, req *InquiryRequest) error
    GetByID(ctx context.Context, id int64) (*InquiryRequest, error)
}

// Service Interface (implemented by service layer)
type UltimaService interface {
    CheckInquiry(ctx context.Context, req *InquiryRequest) (*InquiryResponse, error)
}
```

### 2. Use Case Layer (`internal/usecase/`)
**Purpose:** Business logic and orchestration

**Contains:**
- Business logic implementation
- Use case structs
- Orchestration between repositories and services
- Transaction management

**Rules:**
- ✅ Depends on domain interfaces (not implementations)
- ✅ Orchestrates multiple repositories/services
- ✅ Always use `context.Context` as first parameter
- ✅ Returns domain entities or errors
- ❌ No HTTP-specific code (no fiber.Ctx)
- ❌ No direct database access (use repositories)

**Example:**
```go
// internal/usecase/inquiry_usecase.go
package usecase

import (
    "context"
    "pps-services-tokopedia/internal/domain"
)

type InquiryUsecase struct {
    logger      Logger
    redisClient RedisClient
    productRepo domain.ProductRepository
    ultimaService domain.UltimaService
    inquiryRepo domain.InquiryRepository
}

func NewInquiryUsecase(
    logger Logger,
    redisClient RedisClient,
    productRepo domain.ProductRepository,
    ultimaService domain.UltimaService,
    inquiryRepo domain.InquiryRepository,
) *InquiryUsecase {
    return &InquiryUsecase{
        logger:        logger,
        redisClient:   redisClient,
        productRepo:   productRepo,
        ultimaService: ultimaService,
        inquiryRepo:   inquiryRepo,
    }
}

func (u *InquiryUsecase) ProcessInquiry(ctx context.Context, req *domain.InquiryRequest) (*domain.InquiryResponse, error) {
    // 1. Get product price (with cache)
    price, err := u.productRepo.GetPrice(ctx, req.ProductCode)
    if err != nil {
        return nil, err
    }
    
    // 2. Call external service
    response, err := u.ultimaService.CheckInquiry(ctx, req)
    if err != nil {
        return nil, err
    }
    
    // 3. Save to database (best-effort)
    _ = u.inquiryRepo.Save(ctx, req)
    
    return response, nil
}
```

### 3. Repository Layer (`internal/repository/`)
**Purpose:** Data access implementation

**Contains:**
- Database query implementation
- Cache implementation
- Data mapping (DB ↔ Domain)

**Rules:**
- ✅ Implements domain repository interfaces
- ✅ Use parameterized queries (sqlc or $1, $2 placeholders)
- ✅ Always use `context.Context`
- ✅ Handle database connections via service layer
- ❌ No business logic
- ❌ No HTTP code

**Example:**
```go
// internal/repository/inquiry_postgres_repository.go
package repository

import (
    "context"
    "pps-services-tokopedia/internal/domain"
    "pps-services-tokopedia/internal/service"
)

type PostgresInquiryRepository struct {
    db     service.PostgresService
    logger service.Logger
}

func NewPostgresInquiryRepository(db service.PostgresService, logger service.Logger) domain.InquiryRepository {
    return &PostgresInquiryRepository{
        db:     db,
        logger: logger,
    }
}

func (r *PostgresInquiryRepository) Save(ctx context.Context, req *domain.InquiryRequest) error {
    query := `
        SELECT transaction.inquiry_request_oninsert(
            $1, $2, $3, $4, $5
        )
    `
    var id int64
    err := r.db.QueryRow(ctx, query,
        req.ClientNumber,
        req.ProductCode,
        req.Amount,
        req.CreatedAt,
        req.Status,
    ).Scan(&id)
    
    if err != nil {
        r.logger.Error("Failed to save inquiry", "error", err)
        return err
    }
    
    req.ID = id
    return nil
}

func (r *PostgresInquiryRepository) GetByID(ctx context.Context, id int64) (*domain.InquiryRequest, error) {
    query := `
        SELECT id, client_number, product_code, amount, created_at, status
        FROM transaction.inquiry_requests
        WHERE id = $1
    `
    
    var req domain.InquiryRequest
    err := r.db.QueryRow(ctx, query, id).Scan(
        &req.ID,
        &req.ClientNumber,
        &req.ProductCode,
        &req.Amount,
        &req.CreatedAt,
        &req.Status,
    )
    
    if err != nil {
        return nil, err
    }
    
    return &req, nil
}
```

### 4. Service Layer (`internal/service/`)
**Purpose:** External integrations and infrastructure

**Contains:**
- Database connection management
- Redis client setup
- External API clients
- Logging service
- Crypto services
- Message queue services

**Rules:**
- ✅ Implements domain service interfaces
- ✅ Handles connection pooling
- ✅ Provides abstraction over third-party libraries
- ✅ Configuration and initialization
- ❌ No business logic

**Example:**
```go
// internal/service/postgres_service.go
package service

import (
    "context"
    "github.com/jackc/pgx/v5/pgxpool"
)

type PostgresService interface {
    QueryRow(ctx context.Context, query string, args ...interface{}) Row
    Query(ctx context.Context, query string, args ...interface{}) (Rows, error)
    Exec(ctx context.Context, query string, args ...interface{}) error
    Close()
}

type postgresService struct {
    pool *pgxpool.Pool
}

func NewPostgresService(ctx context.Context, dsn string, maxConns int) (PostgresService, error) {
    config, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, err
    }
    
    config.MaxConns = int32(maxConns)
    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, err
    }
    
    return &postgresService{pool: pool}, nil
}

// Implement interface methods...
```

### 5. Delivery Layer (`internal/delivery/http/`)
**Purpose:** HTTP request/response handling

**Contains:**
- HTTP handlers
- Route registration
- Request validation
- DTO (Data Transfer Object) mapping
- Response formatting

**Rules:**
- ✅ Depends on use cases (via interfaces)
- ✅ Handles HTTP-specific concerns (status codes, headers)
- ✅ Validates input and maps to domain entities
- ✅ Maps domain entities to DTOs for response
- ❌ No business logic
- ❌ No direct database access

**Example:**
```go
// internal/delivery/http/inquiry_handler.go
package http

import (
    "pps-services-tokopedia/internal/domain"
    "pps-services-tokopedia/internal/dto"
    "github.com/gofiber/fiber/v2"
)

type InquiryHandler struct {
    usecase InquiryUsecase
    logger  Logger
}

func NewInquiryHandler(usecase InquiryUsecase, logger Logger) *InquiryHandler {
    return &InquiryHandler{
        usecase: usecase,
        logger:  logger,
    }
}

func (h *InquiryHandler) HandleInquiry(c *fiber.Ctx) error {
    var req dto.InquiryRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "invalid request"})
    }
    
    // Map DTO to domain entity
    domainReq := &domain.InquiryRequest{
        ClientNumber: req.ClientNumber,
        ProductCode:  req.ProductCode,
        Amount:       req.Amount,
    }
    
    // Call use case
    response, err := h.usecase.ProcessInquiry(c.Context(), domainReq)
    if err != nil {
        h.logger.Error("Failed to process inquiry", "error", err)
        return c.Status(500).JSON(fiber.Map{"error": "internal server error"})
    }
    
    // Map domain response to DTO
    dtoResponse := dto.InquiryResponse{
        InquiryID: response.ID,
        Status:    response.Status,
        Amount:    response.Amount,
    }
    
    return c.JSON(dtoResponse)
}

func (h *InquiryHandler) RegisterRoutes(router fiber.Router) {
    router.Post("/inquiry", h.HandleInquiry)
}
```

### 6. DTO Layer (`internal/dto/`)
**Purpose:** Data transfer between layers

**Contains:**
- Request/Response structs
- JSON tags for serialization
- Validation tags

**Rules:**
- ✅ Used for external communication (HTTP, gRPC)
- ✅ Can have different structure from domain entities
- ✅ Include serialization tags (json, xml, etc.)
- ❌ No business logic

### 7. Middleware Layer (`internal/middleware/`)
**Purpose:** Cross-cutting concerns

**Contains:**
- Authentication/Authorization
- Logging middleware
- Encryption/Decryption
- CORS handling
- Rate limiting

**Example:**
```go
// internal/middleware/http_logging_middleware.go
package middleware

import (
    "github.com/gofiber/fiber/v2"
    "pps-services-tokopedia/internal/domain"
)

type HTTPLoggingConfig struct {
    Logger     Logger
    Repository domain.HTTPLoggingRepository
    Next       func(c *fiber.Ctx) bool
}

func HTTPLoggingMiddlewareWithConfig(config HTTPLoggingConfig) fiber.Handler {
    return func(c *fiber.Ctx) error {
        // Log request
        log := &domain.HTTPLog{
            Method:    c.Method(),
            Path:      c.Path(),
            RequestBody: string(c.Body()),
        }
        
        // Process request
        err := c.Next()
        
        // Log response
        log.StatusCode = c.Response().StatusCode()
        log.ResponseBody = string(c.Response().Body())
        
        // Save to repository (best-effort)
        go config.Repository.Save(c.Context(), log)
        
        return err
    }
}
```

---

## 📁 Project Structure

```
project-name/
├── cmd/
│   └── app/
│       ├── main.go              # Entry point with graceful shutdown
│       ├── wire.go              # Dependency injection providers
│       └── wire_gen.go          # Auto-generated (don't edit)
│
├── internal/
│   ├── domain/                  # Core business entities & interfaces
│   │   ├── user_domain.go
│   │   ├── product_domain.go
│   │   └── ...
│   │
│   ├── usecase/                 # Business logic implementation
│   │   ├── user_usecase.go
│   │   ├── user_usecase_test.go
│   │   └── ...
│   │
│   ├── repository/              # Data access implementation
│   │   ├── user_postgres_repository.go
│   │   ├── product_oracle_repository.go
│   │   └── ...
│   │
│   ├── delivery/                # Delivery mechanisms
│   │   └── http/
│   │       ├── handler.go       # Route registration
│   │       ├── user_handler.go
│   │       ├── user_handler_test.go
│   │       └── ...
│   │
│   ├── service/                 # External services & infrastructure
│   │   ├── postgres_service.go
│   │   ├── redis_service.go
│   │   ├── logger_service.go
│   │   └── ...
│   │
│   ├── middleware/              # HTTP middleware
│   │   ├── auth_middleware.go
│   │   ├── logging_middleware.go
│   │   └── ...
│   │
│   ├── dto/                     # Data Transfer Objects
│   │   ├── user_dto.go
│   │   ├── product_dto.go
│   │   └── ...
│   │
│   └── utils/                   # Utility functions
│       ├── id_generator.go
│       ├── error_utils.go
│       └── ...
│
├── database/                    # Database schemas and migrations
│   ├── schema.sql
│   └── migrations/
│
├── docker-compose.yml
├── Dockerfile
├── go.mod
├── go.sum
└── README.md
```

---

## 🔄 Refactoring Strategy

### Phase 1: Analysis & Planning

1. **Identify Current Architecture**
   - Map existing code to layers
   - Identify mixed responsibilities
   - List external dependencies
   - Document data flow

2. **Define Domain Entities**
   - Extract core business objects
   - Identify relationships
   - Define business rules

3. **Design Interfaces**
   - Repository interfaces for data access
   - Service interfaces for external dependencies
   - Use case interfaces (if needed for multiple implementations)

### Phase 2: Create Clean Structure

1. **Setup Directory Structure**
   ```bash
   mkdir -p internal/{domain,usecase,repository,delivery/http,service,middleware,dto,utils}
   mkdir -p cmd/app
   mkdir -p database
   ```

2. **Create Domain Layer**
   - Move entities to `internal/domain/`
   - Define repository interfaces
   - Define service interfaces
   - Remove all external dependencies

3. **Create Service Layer**
   - Implement database connection management
   - Setup Redis client
   - Create logger service
   - Implement external API clients

### Phase 3: Refactor Core Logic

1. **Extract Use Cases**
   - Identify business operations
   - Create use case structs
   - Move business logic from handlers
   - Inject dependencies via constructor

2. **Implement Repositories**
   - Create repository structs
   - Implement domain interfaces
   - Move database queries
   - Use parameterized queries

3. **Refactor Handlers**
   - Keep only HTTP concerns
   - Delegate to use cases
   - Map DTOs ↔ Domain entities
   - Handle errors appropriately

### Phase 4: Dependency Injection

1. **Install Google Wire**
   ```bash
   go get github.com/google/wire/cmd/wire
   ```

2. **Create Wire Providers** (`cmd/app/wire.go`)
   ```go
   //go:build wireinject
   // +build wireinject

   package main

   import (
       "context"
       "github.com/google/wire"
       // Import your packages
   )

   func InitializeApp() (*AppContainer, error) {
       wire.Build(
           // Service providers
           service.NewPostgresService,
           service.NewRedisService,
           service.NewLogger,
           
           // Repository providers
           repository.NewUserRepository,
           
           // Use case providers
           usecase.NewUserUsecase,
           
           // Handler providers
           http.NewHandler,
           
           // App container
           NewAppContainer,
       )
       return nil, nil
   }
   ```

3. **Generate Wire Code**
   ```bash
   cd cmd/app
   wire
   ```

### Phase 5: Testing & Migration

1. **Write Tests**
   - Unit tests for use cases (table-driven)
   - Mock repositories and services
   - Integration tests for repositories
   - Handler tests with mocked use cases

2. **Gradual Migration**
   - Migrate one feature at a time
   - Keep old code until new is tested
   - Run both versions in parallel if needed
   - Update one endpoint/feature at a time

---

## 🛠️ Implementation Guide

### Step-by-Step Refactoring Example: User Management

#### 1. Current Code (Monolithic Handler)
```go
// OLD: handlers/user.go
func HandleGetUser(c *fiber.Ctx) error {
    id := c.Params("id")
    
    // Direct database access in handler (BAD)
    var user User
    db.QueryRow("SELECT * FROM users WHERE id = ?", id).Scan(&user.ID, &user.Name)
    
    // Business logic in handler (BAD)
    if user.Status == "inactive" {
        return c.Status(403).JSON(fiber.Map{"error": "user inactive"})
    }
    
    return c.JSON(user)
}
```

#### 2. Refactored to Clean Architecture

**Step 1: Create Domain Entity**
```go
// internal/domain/user_domain.go
package domain

import "context"

type User struct {
    ID       int64
    Name     string
    Email    string
    Status   string
}

type UserRepository interface {
    GetByID(ctx context.Context, id int64) (*User, error)
    Save(ctx context.Context, user *User) error
    Update(ctx context.Context, user *User) error
    Delete(ctx context.Context, id int64) error
}
```

**Step 2: Implement Repository**
```go
// internal/repository/user_postgres_repository.go
package repository

import (
    "context"
    "pps-services-tokopedia/internal/domain"
    "pps-services-tokopedia/internal/service"
)

type userPostgresRepository struct {
    db     service.PostgresService
    logger service.Logger
}

func NewUserRepository(db service.PostgresService, logger service.Logger) domain.UserRepository {
    return &userPostgresRepository{
        db:     db,
        logger: logger,
    }
}

func (r *userPostgresRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
    query := `SELECT id, name, email, status FROM users WHERE id = $1`
    
    var user domain.User
    err := r.db.QueryRow(ctx, query, id).Scan(
        &user.ID,
        &user.Name,
        &user.Email,
        &user.Status,
    )
    
    if err != nil {
        r.logger.Error("Failed to get user", "error", err, "id", id)
        return nil, err
    }
    
    return &user, nil
}

// Implement other methods...
```

**Step 3: Create Use Case**
```go
// internal/usecase/user_usecase.go
package usecase

import (
    "context"
    "errors"
    "pps-services-tokopedia/internal/domain"
)

var ErrUserInactive = errors.New("user is inactive")

type UserUsecase struct {
    userRepo domain.UserRepository
    logger   Logger
}

func NewUserUsecase(userRepo domain.UserRepository, logger Logger) *UserUsecase {
    return &UserUsecase{
        userRepo: userRepo,
        logger:   logger,
    }
}

func (u *UserUsecase) GetUser(ctx context.Context, id int64) (*domain.User, error) {
    user, err := u.userRepo.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Business logic: check if user is active
    if user.Status == "inactive" {
        u.logger.Warn("Attempted to access inactive user", "id", id)
        return nil, ErrUserInactive
    }
    
    return user, nil
}
```

**Step 4: Create Handler**
```go
// internal/delivery/http/user_handler.go
package http

import (
    "strconv"
    "pps-services-tokopedia/internal/usecase"
    "github.com/gofiber/fiber/v2"
)

type UserHandler struct {
    usecase *usecase.UserUsecase
    logger  Logger
}

func NewUserHandler(usecase *usecase.UserUsecase, logger Logger) *UserHandler {
    return &UserHandler{
        usecase: usecase,
        logger:  logger,
    }
}

func (h *UserHandler) HandleGetUser(c *fiber.Ctx) error {
    id, err := strconv.ParseInt(c.Params("id"), 10, 64)
    if err != nil {
        return c.Status(400).JSON(fiber.Map{"error": "invalid id"})
    }
    
    user, err := h.usecase.GetUser(c.Context(), id)
    if err != nil {
        if err == usecase.ErrUserInactive {
            return c.Status(403).JSON(fiber.Map{"error": "user inactive"})
        }
        h.logger.Error("Failed to get user", "error", err)
        return c.Status(500).JSON(fiber.Map{"error": "internal server error"})
    }
    
    return c.JSON(user)
}

func (h *UserHandler) RegisterRoutes(router fiber.Router) {
    router.Get("/users/:id", h.HandleGetUser)
}
```

**Step 5: Wire Dependencies**
```go
// cmd/app/wire.go
//go:build wireinject
// +build wireinject

package main

import (
    "context"
    "github.com/google/wire"
    "pps-services-tokopedia/internal/delivery/http"
    "pps-services-tokopedia/internal/repository"
    "pps-services-tokopedia/internal/service"
    "pps-services-tokopedia/internal/usecase"
)

func InitializeApp() (*AppContainer, error) {
    wire.Build(
        // Context
        context.Background,
        
        // Services
        service.NewPostgresService,
        service.NewLogger,
        
        // Repositories
        repository.NewUserRepository,
        
        // Use Cases
        usecase.NewUserUsecase,
        
        // Handlers
        http.NewUserHandler,
        http.NewHandler,
        
        // App
        NewAppContainer,
    )
    return nil, nil
}
```

**Step 6: Main Application**
```go
// cmd/app/main.go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"
    "time"
    "github.com/gofiber/fiber/v2"
)

type AppContainer struct {
    App             *fiber.App
    Handler         *http.Handler
    PostgresService service.PostgresService
    Logger          service.Logger
}

func NewAppContainer(
    handler *http.Handler,
    postgresService service.PostgresService,
    logger service.Logger,
) *AppContainer {
    app := fiber.New()
    
    return &AppContainer{
        App:             app,
        Handler:         handler,
        PostgresService: postgresService,
        Logger:          logger,
    }
}

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()
    
    // Initialize app with Wire
    appContainer, err := InitializeApp()
    if err != nil {
        log.Fatalf("Failed to initialize app: %v", err)
    }
    
    // Cleanup on exit
    defer func() {
        if appContainer.PostgresService != nil {
            appContainer.PostgresService.Close()
        }
    }()
    
    // Register routes
    appContainer.Handler.RegisterRoutes(appContainer.App)
    
    // Handle graceful shutdown
    handleShutdown(ctx, cancel, appContainer.Logger)
    
    // Start server
    go func() {
        port := os.Getenv("APP_PORT")
        if port == "" {
            port = "3000"
        }
        if err := appContainer.App.Listen(":" + port); err != nil {
            appContainer.Logger.Error("Server error", "error", err)
            cancel()
        }
    }()
    
    // Wait for shutdown
    <-ctx.Done()
    appContainer.Logger.Info("Shutting down...")
    
    shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer shutdownCancel()
    
    if err := appContainer.App.ShutdownWithContext(shutdownCtx); err != nil {
        appContainer.Logger.Error("Shutdown error", "error", err)
    }
    
    appContainer.Logger.Info("Graceful shutdown complete")
}

func handleShutdown(ctx context.Context, cancel context.CancelFunc, logger service.Logger) {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)
    
    go func() {
        select {
        case sig := <-sigCh:
            logger.Info("Received signal", "signal", sig)
            cancel()
        case <-ctx.Done():
        }
    }()
}
```

---

## 🔌 Dependency Injection with Wire

### Why Wire?

- ✅ **Compile-time DI** - Errors caught during compilation
- ✅ **No reflection** - Better performance
- ✅ **Type-safe** - Full type checking
- ✅ **Auto-generated** - Less boilerplate

### Wire Setup

**1. Install Wire**
```bash
go install github.com/google/wire/cmd/wire@latest
```

**2. Create Provider Functions**
   ```go
// internal/service/postgres_service.go
func NewPostgresService(ctx context.Context) (PostgresService, error) {
    dsn := os.Getenv("POSTGRES_DSN")
    maxConns := 25
    
    return newPostgresService(ctx, dsn, maxConns)
}
```

**3. Create Wire Configuration**
   ```go
// cmd/app/wire.go
//go:build wireinject
// +build wireinject

package main

import (
    "github.com/google/wire"
    // ... imports
)

// Define provider sets for logical grouping
var ServiceSet = wire.NewSet(
    service.NewPostgresService,
    service.NewRedisService,
    service.NewLogger,
    service.NewCryptoService,
)

var RepositorySet = wire.NewSet(
    repository.NewUserRepository,
    repository.NewProductRepository,
)

var UsecaseSet = wire.NewSet(
    usecase.NewUserUsecase,
    usecase.NewProductUsecase,
)

var HandlerSet = wire.NewSet(
    http.NewUserHandler,
    http.NewProductHandler,
    http.NewHandler, // Main handler that aggregates all handlers
)

// Main wire injector
func InitializeApp() (*AppContainer, error) {
    wire.Build(
        context.Background,
        ServiceSet,
        RepositorySet,
        UsecaseSet,
        HandlerSet,
        NewAppContainer,
    )
    return nil, nil
}
```

**4. Generate Wire Code**
```bash
cd cmd/app
wire
# This generates wire_gen.go - don't edit it manually!
```

**5. Using Wire with Interfaces**
   ```go
// When you have interface binding
var RepositorySet = wire.NewSet(
    repository.NewUserRepository,
    wire.Bind(new(domain.UserRepository), new(*repository.userPostgresRepository)),
)
```

### Wire Best Practices

1. **Group Related Providers**
   - Create provider sets for each layer
   - Makes wire.go more readable

2. **Use Constructor Functions**
   - All constructors should be `New*` functions
   - Return concrete types or interfaces

3. **Handle Errors**
   - Providers can return `(Type, error)`
   - Wire handles error propagation

4. **Cleanup Functions**
   - Wire supports cleanup with `wire.Cleanup`
   - Use for closing connections

5. **Build Tags**
   - Always use `//go:build wireinject` tag
   - Prevents wire.go from being compiled

---

## ✅ Best Practices & Patterns

### 1. Context Propagation

**Always use context as the first parameter:**
   ```go
// ✅ GOOD
func (u *UserUsecase) GetUser(ctx context.Context, id int64) (*domain.User, error)

// ❌ BAD
func (u *UserUsecase) GetUser(id int64) (*domain.User, error)
```

**Pass context to all downstream calls:**
   ```go
func (u *UserUsecase) ProcessOrder(ctx context.Context, order *domain.Order) error {
    // Pass context to repository
    user, err := u.userRepo.GetByID(ctx, order.UserID)
    if err != nil {
        return err
    }
    
    // Pass context to external service
    payment, err := u.paymentService.Charge(ctx, order.Amount)
    if err != nil {
        return err
    }
    
    return nil
}
```

### 2. Error Handling

**Define domain-specific errors:**
```go
// internal/domain/errors.go
package domain

import "errors"

var (
    ErrUserNotFound     = errors.New("user not found")
    ErrUserInactive     = errors.New("user inactive")
    ErrInvalidInput     = errors.New("invalid input")
    ErrUnauthorized     = errors.New("unauthorized")
)
```

**Handle errors at appropriate layers:**
```go
// Use case returns domain errors
func (u *UserUsecase) GetUser(ctx context.Context, id int64) (*domain.User, error) {
    user, err := u.userRepo.GetByID(ctx, id)
    if err != nil {
        if errors.Is(err, sql.ErrNoRows) {
            return nil, domain.ErrUserNotFound
        }
        return nil, err
    }
    return user, nil
}

// Handler maps to HTTP status codes
func (h *UserHandler) HandleGetUser(c *fiber.Ctx) error {
    user, err := h.usecase.GetUser(c.Context(), id)
    if err != nil {
        if errors.Is(err, domain.ErrUserNotFound) {
            return c.Status(404).JSON(fiber.Map{"error": "user not found"})
        }
        if errors.Is(err, domain.ErrUnauthorized) {
            return c.Status(401).JSON(fiber.Map{"error": "unauthorized"})
        }
        return c.Status(500).JSON(fiber.Map{"error": "internal server error"})
    }
    return c.JSON(user)
}
```

### 3. Logging

**Use structured logging:**
```go
// ✅ GOOD - Structured logging with key-value pairs
logger.Error("Failed to get user",
    "error", err,
    "userId", id,
    "operation", "GetUser",
)

// ❌ BAD - String concatenation
logger.Error("Failed to get user " + id + ": " + err.Error())
```

**Log at appropriate levels:**
```go
logger.Debug("Processing request", "requestId", reqId)  // Development debugging
logger.Info("User created", "userId", user.ID)          // Normal operations
logger.Warn("Rate limit exceeded", "userId", user.ID)   // Warnings
logger.Error("Database error", "error", err)            // Errors
```

### 4. Testing Patterns

**Use table-driven tests:**
```go
func TestUserUsecase_GetUser(t *testing.T) {
    tests := []struct {
        name          string
        userId        int64
        mockUser      *domain.User
        mockError     error
        expectedUser  *domain.User
        expectedError error
    }{
        {
            name:   "successful get",
            userId: 1,
            mockUser: &domain.User{
                ID:   1,
                Name: "John",
            },
            mockError:    nil,
            expectedUser: &domain.User{ID: 1, Name: "John"},
            expectedError: nil,
        },
        {
            name:          "user not found",
            userId:        999,
            mockUser:      nil,
            mockError:     sql.ErrNoRows,
            expectedUser:  nil,
            expectedError: domain.ErrUserNotFound,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            // Setup mocks
            mockRepo := &mockUserRepository{
                user:  tt.mockUser,
                err:   tt.mockError,
            }
            mockLogger := &mockLogger{}
            
            usecase := usecase.NewUserUsecase(mockRepo, mockLogger)
            
            // Execute
            user, err := usecase.GetUser(context.Background(), tt.userId)
            
            // Assert
            if !errors.Is(err, tt.expectedError) {
                t.Errorf("expected error %v, got %v", tt.expectedError, err)
            }
            if user != nil && tt.expectedUser != nil {
                if user.ID != tt.expectedUser.ID {
                    t.Errorf("expected user ID %d, got %d", tt.expectedUser.ID, user.ID)
                }
            }
        })
    }
}
```

**Mock interfaces for testing:**
```go
// Test mock
type mockUserRepository struct {
    user *domain.User
    err  error
}

func (m *mockUserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
    return m.user, m.err
}

func (m *mockUserRepository) Save(ctx context.Context, user *domain.User) error {
    return m.err
}
```

### 5. Repository Patterns

**Use parameterized queries:**
```go
// ✅ GOOD - Parameterized query (safe from SQL injection)
query := `SELECT * FROM users WHERE id = $1 AND status = $2`
db.QueryRow(ctx, query, userID, "active")

// ❌ BAD - String concatenation (SQL injection risk)
query := fmt.Sprintf("SELECT * FROM users WHERE id = %d", userID)
```

**Handle NULL values properly:**
```go
import "database/sql"

var user domain.User
var phoneNumber sql.NullString

err := db.QueryRow(ctx, query, id).Scan(
    &user.ID,
    &user.Name,
    &phoneNumber, // Can be NULL
)

if phoneNumber.Valid {
    user.PhoneNumber = phoneNumber.String
}
```

**Use stored procedures for complex operations:**
```go
// Call stored procedure
query := `SELECT transaction.inquiry_request_oninsert($1, $2, $3)`
var id int64
err := db.QueryRow(ctx, query, req.ClientNumber, req.Amount, req.Status).Scan(&id)
```

### 6. Service Patterns

**Connection pooling:**
```go
func NewPostgresService(ctx context.Context, dsn string, maxConns int) (PostgresService, error) {
    config, err := pgxpool.ParseConfig(dsn)
    if err != nil {
        return nil, err
    }
    
    config.MaxConns = int32(maxConns)
    config.MinConns = 5
    config.MaxConnLifetime = time.Hour
    config.MaxConnIdleTime = 30 * time.Minute
    
    pool, err := pgxpool.NewWithConfig(ctx, config)
    if err != nil {
        return nil, err
    }
    
    return &postgresService{pool: pool}, nil
}
```

**Graceful shutdown:**
```go
func (s *postgresService) Close() {
    if s.pool != nil {
        s.pool.Close()
    }
}
```

### 7. Middleware Patterns

**Chain multiple middleware:**
```go
api := app.Group("/api/v1",
    middleware.CheckBearerToken(),
    middleware.Logging(),
    middleware.Encryption(),
)
```

**Best-effort logging (don't fail request):**
```go
func HTTPLoggingMiddleware(repo domain.HTTPLoggingRepository) fiber.Handler {
    return func(c *fiber.Ctx) error {
        log := &domain.HTTPLog{
            Method: c.Method(),
            Path:   c.Path(),
        }
        
        err := c.Next()
        
        log.StatusCode = c.Response().StatusCode()
        
        // Best-effort logging - don't fail request if logging fails
        go func() {
            _ = repo.Save(context.Background(), log)
        }()
        
        return err
    }
}
```

### 8. Configuration Management

**Use environment variables:**
```go
type Config struct {
    AppPort         string
    PostgresDSN     string
    PostgresMaxConns int
    RedisAddr       string
    RedisPassword   string
}

func LoadConfig() *Config {
    return &Config{
        AppPort:         getEnv("APP_PORT", "3000"),
        PostgresDSN:     getEnv("POSTGRES_DSN", ""),
        PostgresMaxConns: getEnvAsInt("POSTGRES_MAX_CONNS", 25),
        RedisAddr:       getEnv("REDIS_ADDR", "localhost:6379"),
        RedisPassword:   getEnv("REDIS_PASSWORD", ""),
    }
}

func getEnv(key, defaultValue string) string {
    if value := os.Getenv(key); value != "" {
        return value
    }
    return defaultValue
}

func getEnvAsInt(key string, defaultValue int) int {
    if value := os.Getenv(key); value != "" {
        if intVal, err := strconv.Atoi(value); err == nil {
            return intVal
        }
    }
    return defaultValue
}
```

---

## 🧪 Testing Strategy

### Test Pyramid

```
        ┌─────────────┐
        │   E2E Tests │  ← Few, expensive
        ├─────────────┤
        │ Integration │  ← Some, medium cost
        │    Tests    │
        ├─────────────┤
        │  Unit Tests │  ← Many, cheap
        └─────────────┘
```

### Unit Tests (Use Case Layer)

**Goal:** Test business logic in isolation

```go
// internal/usecase/user_usecase_test.go
package usecase

import (
    "context"
    "errors"
    "testing"
    "pps-services-tokopedia/internal/domain"
)

type mockUserRepository struct {
    getUserFunc func(ctx context.Context, id int64) (*domain.User, error)
}

func (m *mockUserRepository) GetByID(ctx context.Context, id int64) (*domain.User, error) {
    return m.getUserFunc(ctx, id)
}

func TestUserUsecase_GetUser(t *testing.T) {
    tests := []struct {
        name          string
        userId        int64
        setupMock     func() *mockUserRepository
        expectedError error
    }{
        {
            name:   "successful get",
            userId: 1,
            setupMock: func() *mockUserRepository {
                return &mockUserRepository{
                    getUserFunc: func(ctx context.Context, id int64) (*domain.User, error) {
                        return &domain.User{ID: 1, Name: "John", Status: "active"}, nil
                    },
                }
            },
            expectedError: nil,
        },
        {
            name:   "inactive user",
            userId: 2,
            setupMock: func() *mockUserRepository {
                return &mockUserRepository{
                    getUserFunc: func(ctx context.Context, id int64) (*domain.User, error) {
                        return &domain.User{ID: 2, Name: "Jane", Status: "inactive"}, nil
                    },
                }
            },
            expectedError: ErrUserInactive,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            mockRepo := tt.setupMock()
            mockLogger := &mockLogger{}
            usecase := NewUserUsecase(mockRepo, mockLogger)
            
            user, err := usecase.GetUser(context.Background(), tt.userId)
            
            if !errors.Is(err, tt.expectedError) {
                t.Errorf("expected error %v, got %v", tt.expectedError, err)
            }
            
            if err == nil && user.Status != "active" {
                t.Error("expected active user")
            }
        })
    }
}
```

### Integration Tests (Repository Layer)

**Goal:** Test database interactions

```go
// internal/repository/user_postgres_repository_test.go
package repository

import (
    "context"
    "testing"
    "pps-services-tokopedia/internal/domain"
    "pps-services-tokopedia/internal/service"
)

func TestUserRepository_GetByID_Integration(t *testing.T) {
    if testing.Short() {
        t.Skip("Skipping integration test")
    }
    
    // Setup test database
    ctx := context.Background()
    db, err := service.NewPostgresService(ctx, "postgres://localhost/testdb", 5)
    if err != nil {
        t.Fatalf("Failed to connect to test database: %v", err)
    }
    defer db.Close()
    
    // Create repository
    logger := service.NewLogger()
    repo := NewUserRepository(db, logger)
    
    // Create test user
    testUser := &domain.User{
        Name:   "Test User",
        Email:  "test@example.com",
        Status: "active",
    }
    
    err = repo.Save(ctx, testUser)
    if err != nil {
        t.Fatalf("Failed to save user: %v", err)
    }
    
    // Test retrieval
    retrievedUser, err := repo.GetByID(ctx, testUser.ID)
    if err != nil {
        t.Fatalf("Failed to get user: %v", err)
    }
    
    if retrievedUser.Name != testUser.Name {
        t.Errorf("expected name %s, got %s", testUser.Name, retrievedUser.Name)
    }
    
    // Cleanup
    _ = repo.Delete(ctx, testUser.ID)
}
```

### Handler Tests

**Goal:** Test HTTP request/response handling

```go
// internal/delivery/http/user_handler_test.go
package http

import (
    "bytes"
    "encoding/json"
    "errors"
    "net/http/httptest"
    "testing"
    "github.com/gofiber/fiber/v2"
    "pps-services-tokopedia/internal/domain"
)

type mockUserUsecase struct {
    getUserFunc func(ctx context.Context, id int64) (*domain.User, error)
}

func (m *mockUserUsecase) GetUser(ctx context.Context, id int64) (*domain.User, error) {
    return m.getUserFunc(ctx, id)
}

func TestUserHandler_HandleGetUser(t *testing.T) {
    tests := []struct {
        name           string
        userId         string
        setupMock      func() *mockUserUsecase
        expectedStatus int
    }{
        {
            name:   "successful get",
            userId: "1",
            setupMock: func() *mockUserUsecase {
                return &mockUserUsecase{
                    getUserFunc: func(ctx context.Context, id int64) (*domain.User, error) {
                        return &domain.User{ID: 1, Name: "John"}, nil
                    },
                }
            },
            expectedStatus: 200,
        },
        {
            name:   "user not found",
            userId: "999",
            setupMock: func() *mockUserUsecase {
                return &mockUserUsecase{
                    getUserFunc: func(ctx context.Context, id int64) (*domain.User, error) {
                        return nil, domain.ErrUserNotFound
                    },
                }
            },
            expectedStatus: 404,
        },
        {
            name:           "invalid id",
            userId:         "invalid",
            setupMock:      func() *mockUserUsecase { return &mockUserUsecase{} },
            expectedStatus: 400,
        },
    }
    
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
            app := fiber.New()
            mockUsecase := tt.setupMock()
            mockLogger := &mockLogger{}
            handler := NewUserHandler(mockUsecase, mockLogger)
            
            app.Get("/users/:id", handler.HandleGetUser)
            
            req := httptest.NewRequest("GET", "/users/"+tt.userId, nil)
            resp, _ := app.Test(req)
            
            if resp.StatusCode != tt.expectedStatus {
                t.Errorf("expected status %d, got %d", tt.expectedStatus, resp.StatusCode)
            }
        })
    }
}
```

### Benchmark Tests

**Goal:** Measure performance

```go
// internal/usecase/user_usecase_benchmark_test.go
package usecase

import (
    "context"
    "testing"
    "pps-services-tokopedia/internal/domain"
)

func BenchmarkUserUsecase_GetUser(b *testing.B) {
    mockRepo := &mockUserRepository{
        getUserFunc: func(ctx context.Context, id int64) (*domain.User, error) {
            return &domain.User{ID: 1, Name: "John", Status: "active"}, nil
        },
    }
    mockLogger := &mockLogger{}
    usecase := NewUserUsecase(mockRepo, mockLogger)
    ctx := context.Background()
    
    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        _, _ = usecase.GetUser(ctx, 1)
    }
}
```

---

## 📚 Examples & Code Patterns

### Pattern 1: CRUD Operations

**Domain:**
```go
type Product struct {
    ID          int64
    Name        string
    Price       float64
    Description string
    CreatedAt   time.Time
    UpdatedAt   time.Time
}

type ProductRepository interface {
    Create(ctx context.Context, product *Product) error
    GetByID(ctx context.Context, id int64) (*Product, error)
    Update(ctx context.Context, product *Product) error
    Delete(ctx context.Context, id int64) error
    List(ctx context.Context, limit, offset int) ([]*Product, error)
}
```

**Use Case:**
```go
type ProductUsecase struct {
    productRepo domain.ProductRepository
    logger      Logger
}

func (u *ProductUsecase) CreateProduct(ctx context.Context, product *domain.Product) error {
    // Validation
    if product.Name == "" {
        return domain.ErrInvalidInput
    }
    
    product.CreatedAt = time.Now()
    product.UpdatedAt = time.Now()
    
    return u.productRepo.Create(ctx, product)
}
```

### Pattern 2: Caching

**Use Case with Cache:**
```go
func (u *ProductUsecase) GetProduct(ctx context.Context, id int64) (*domain.Product, error) {
    // Try cache first
    cacheKey := fmt.Sprintf("product:%d", id)
    var product domain.Product
    
    err := u.cache.Get(ctx, cacheKey, &product)
    if err == nil {
        u.logger.Debug("Cache hit", "productId", id)
        return &product, nil
    }
    
    // Cache miss, get from database
    product, err := u.productRepo.GetByID(ctx, id)
    if err != nil {
        return nil, err
    }
    
    // Save to cache
    _ = u.cache.Set(ctx, cacheKey, product, 1*time.Hour)
    
    return product, nil
}
```

### Pattern 3: External Service Integration

**Domain Interface:**
```go
type PaymentService interface {
    Charge(ctx context.Context, amount float64, cardToken string) (*PaymentResult, error)
    Refund(ctx context.Context, transactionId string) error
}
```

**Service Implementation:**
```go
type paymentService struct {
    baseURL    string
    apiKey     string
    httpClient *http.Client
    logger     Logger
}

func (s *paymentService) Charge(ctx context.Context, amount float64, cardToken string) (*PaymentResult, error) {
    payload := map[string]interface{}{
        "amount": amount,
        "token":  cardToken,
    }
    
    body, _ := json.Marshal(payload)
    req, err := http.NewRequestWithContext(ctx, "POST", s.baseURL+"/charge", bytes.NewReader(body))
    if err != nil {
        return nil, err
    }
    
    req.Header.Set("Authorization", "Bearer "+s.apiKey)
    req.Header.Set("Content-Type", "application/json")
    
    resp, err := s.httpClient.Do(req)
    if err != nil {
        s.logger.Error("Payment charge failed", "error", err)
        return nil, err
    }
    defer resp.Body.Close()
    
    var result PaymentResult
    if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
        return nil, err
    }
    
    return &result, nil
}
```

### Pattern 4: Transaction Management

**Use Case with Transaction:**
```go
func (u *OrderUsecase) CreateOrder(ctx context.Context, order *domain.Order) error {
    // Start transaction
    tx, err := u.db.BeginTx(ctx)
    if err != nil {
        return err
    }
    defer tx.Rollback()
    
    // Save order
    if err := u.orderRepo.CreateWithTx(ctx, tx, order); err != nil {
        return err
    }
    
    // Update inventory
    for _, item := range order.Items {
        if err := u.inventoryRepo.DecrementWithTx(ctx, tx, item.ProductID, item.Quantity); err != nil {
            return err
        }
    }
    
    // Commit transaction
    return tx.Commit()
}
```

### Pattern 5: Middleware Chain

**Authentication + Logging + Encryption:**
```go
api := app.Group("/api/v1",
    middleware.CheckBearerToken(tokenUsecase, cryptoService, logger),
    middleware.DecryptRequest(cryptoService, logger),
    middleware.HTTPLogging(httpLoggingRepo, logger),
    middleware.EncryptResponse(cryptoService, logger),
)
```

---

## 🎯 Refactoring Checklist

Use this checklist when refactoring a project:

### Phase 1: Analysis
- [ ] Identify all external dependencies (databases, APIs, caches)
- [ ] List all business operations
- [ ] Map current code to architectural layers
- [ ] Identify mixed responsibilities
- [ ] Document data flow

### Phase 2: Structure
- [ ] Create directory structure (`domain`, `usecase`, `repository`, `delivery`, `service`)
- [ ] Move files to appropriate directories
- [ ] Separate concerns (HTTP → business logic → data access)

### Phase 3: Domain Layer
- [ ] Extract entities to `domain` package
- [ ] Define repository interfaces
- [ ] Define service interfaces
- [ ] Remove all framework imports from domain

### Phase 4: Service Layer
- [ ] Create database service with connection pooling
- [ ] Create Redis client service
- [ ] Create logger service
- [ ] Implement external API clients
- [ ] All services implement domain interfaces

### Phase 5: Repository Layer
- [ ] Implement repository interfaces
- [ ] Convert to parameterized queries
- [ ] Add context.Context to all methods
- [ ] Handle NULL values properly
- [ ] Remove business logic from repositories

### Phase 6: Use Case Layer
- [ ] Extract business logic from handlers
- [ ] Create use case structs
- [ ] Inject dependencies via constructors
- [ ] Add context.Context to all methods
- [ ] Define domain-specific errors

### Phase 7: Delivery Layer
- [ ] Keep only HTTP concerns in handlers
- [ ] Delegate to use cases
- [ ] Map DTOs ↔ Domain entities
- [ ] Handle HTTP status codes
- [ ] Validate input

### Phase 8: Dependency Injection
- [ ] Install Google Wire
- [ ] Create wire.go with providers
- [ ] Generate wire_gen.go
- [ ] Create AppContainer struct
- [ ] Update main.go to use Wire

### Phase 9: Middleware
- [ ] Extract cross-cutting concerns
- [ ] Create middleware functions
- [ ] Chain middleware appropriately
- [ ] Use best-effort for non-critical middleware (logging)

### Phase 10: Testing
- [ ] Write unit tests for use cases
- [ ] Write integration tests for repositories
- [ ] Write handler tests with mocks
- [ ] Add benchmark tests for critical paths
- [ ] Achieve >80% coverage for business logic

### Phase 11: Configuration
- [ ] Extract configuration to environment variables
- [ ] Create config struct
- [ ] Add default values
- [ ] Document all config options

### Phase 12: Documentation
- [ ] Update README with architecture
- [ ] Document API endpoints
- [ ] Add code examples
- [ ] Create architecture diagrams

---

## 🔧 Tools & Libraries

### Recommended Stack

**Web Framework:**
- `github.com/gofiber/fiber/v2` - Fast HTTP framework

**Database:**
- `github.com/jackc/pgx/v5` - PostgreSQL driver
- `github.com/redis/go-redis/v9` - Redis client

**Dependency Injection:**
- `github.com/google/wire` - Compile-time DI

**Logging:**
- `log/slog` - Structured logging (Go 1.21+)

**Testing:**
- `testing` - Standard library
- `github.com/stretchr/testify` - Assertions (optional)

**Crypto:**
- `crypto/rsa` - RSA encryption
- `github.com/golang-jwt/jwt/v5` - JWT tokens

**Scheduling:**
- `github.com/robfig/cron/v3` - Cron jobs

---

## 📖 Additional Resources

### Clean Architecture
- [The Clean Architecture by Uncle Bob](https://blog.cleancoder.com/uncle-bob/2012/08/13/the-clean-architecture.html)
- [Hexagonal Architecture](https://alistair.cockburn.us/hexagonal-architecture/)

### Go Best Practices
- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://github.com/golang/go/wiki/CodeReviewComments)

### Design Patterns
- [Repository Pattern](https://martinfowler.com/eaaCatalog/repository.html)
- [Dependency Injection](https://martinfowler.com/articles/injection.html)

---

## 💡 Tips for AI Agent Refactoring

When using this guide with an AI coding assistant (like Cursor):

1. **Start Small**: Refactor one feature/module at a time
2. **Ask for Analysis First**: "Analyze the current architecture of [feature]"
3. **Request Layer by Layer**: "Create the domain layer for user management"
4. **Verify Each Step**: Test after each major change
5. **Use Examples**: "Refactor this handler following the pattern in the guide"
6. **Iterate**: Don't try to refactor everything at once

### Example Prompts:

```
"Following the clean architecture guide in README.md, refactor the user 
management module. Start by creating the domain entities and interfaces."

"Implement the UserRepository following the repository pattern in the guide. 
Use parameterized queries and context propagation."

"Create the UserUsecase with dependency injection. Include error handling 
as shown in the best practices section."

"Set up Google Wire for dependency injection following the guide's Wire setup section."

"Write table-driven tests for the UserUsecase following the testing strategy in the guide."
```

---

## 🎉 Success Criteria

Your refactoring is successful when:

- ✅ Each layer has a single responsibility
- ✅ Dependencies point inward (Domain ← Use Case ← Repository/Service)
- ✅ All interfaces are defined in the domain layer
- ✅ Business logic is isolated in use cases
- ✅ Handlers only contain HTTP concerns
- ✅ All database calls use context.Context
- ✅ Tests are easy to write with mocks
- ✅ New features can be added without modifying existing code
- ✅ Code is maintainable and readable

---

## 📞 Support

For questions or issues with this refactoring guide:
1. Review the examples in this README
2. Check the code patterns in the project
3. Refer to the clean architecture resources
4. Ask specific questions with context

---

**Happy Refactoring! 🚀**
