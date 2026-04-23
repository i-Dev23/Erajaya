# Architecture Overview

## Clean Architecture Layers

```
┌──────────────────────────────────────────────────────┐
│                    cmd/app                           │
│           (main.go + wire.go - Entry Point)          │
├──────────────────────────────────────────────────────┤
│                 Delivery Layer                        │
│  ┌────────────┐  ┌──────────┐  ┌──────────────────┐ │
│  │  Handlers  │  │  Router  │  │   Middleware      │ │
│  │            │  │          │  │ auth/security/    │ │
│  │            │  │          │  │ observability/    │ │
│  │            │  │          │  │ transform/        │ │
│  │            │  │          │  │ resilience/       │ │
│  └────────────┘  └──────────┘  └──────────────────┘ │
├──────────────────────────────────────────────────────┤
│                 Application Layer                     │
│  ┌────────────────────┐  ┌────────────────────────┐  │
│  │     Usecases       │  │       DTOs             │  │
│  │ inquiry/payment/   │  │ request/response/      │  │
│  │ check_status/      │  │ mapper/                │  │
│  │ balance/token/     │  │                        │  │
│  │ callback/health/   │  │                        │  │
│  │ cleanup/           │  │                        │  │
│  └────────────────────┘  └────────────────────────┘  │
├──────────────────────────────────────────────────────┤
│                  Domain Layer (Core)                  │
│  ┌──────────┐  ┌───────────┐  ┌──────────────────┐  │
│  │ Entities │  │ Contracts │  │  Value Objects   │  │
│  │          │  │ (usecase/ │  │  (response_code/ │  │
│  │          │  │  repo/    │  │   money/         │  │
│  │          │  │  service/)│  │   timestamp/)    │  │
│  └──────────┘  └───────────┘  └──────────────────┘  │
├──────────────────────────────────────────────────────┤
│               Infrastructure Layer                    │
│  ┌──────────┐  ┌───────┐  ┌────────┐  ┌──────────┐ │
│  │ Database │  │ Cache │  │Messagi-│  │ External │ │
│  │ postgres │  │ redis │  │  ng    │  │ ultima   │ │
│  │ oracle   │  │       │  │rabbitmq│  │          │ │
│  └──────────┘  └───────┘  └────────┘  └──────────┘ │
│  ┌──────────┐  ┌───────┐  ┌─────────────────────┐  │
│  │  Crypto  │  │Logger │  │    Scheduler        │  │
│  │ rsa/sig  │  │ file/ │  │    cron             │  │
│  │          │  │ tele  │  │                     │  │
│  └──────────┘  └───────┘  └─────────────────────┘  │
├──────────────────────────────────────────────────────┤
│               Repository Layer                        │
│  ┌──────────┐  ┌────────┐  ┌────────────────────┐   │
│  │ Postgres │  │ Oracle │  │      Cache         │   │
│  │ inquiry  │  │balance │  │  token_cache       │   │
│  │ payment  │  │cutoff  │  │  product_cache     │   │
│  │ err_map  │  │preorder│  │                    │   │
│  │ logging  │  │product │  │                    │   │
│  │ cleanup  │  │        │  │                    │   │
│  └──────────┘  └────────┘  └────────────────────┘   │
└──────────────────────────────────────────────────────┘
```

## Dependency Rule

Dependencies flow **inward only**:
- Infrastructure → Domain (implements contracts)
- Repository → Domain (implements contracts)
- Usecase → Domain (uses entities + contracts)
- Delivery → Usecase contracts + DTOs + Domain entities
- `cmd/app` → wires everything together

**No inner layer ever imports from an outer layer.**

## Key Design Decisions

1. **Contracts in Domain**: All interfaces live in `internal/domain/contract/` ensuring the domain is the single source of truth
2. **Feature-based Usecases**: Each feature has its own subfolder (inquiry/, payment/, etc.)
3. **Middleware Categories**: Grouped by concern (auth, security, observability, transform, resilience)
4. **Compile-time Checks**: All implementations use `var _ Interface = (*Impl)(nil)` pattern
5. **Google Wire DI**: Provider sets organized by layer in `cmd/app/wire.go`
6. **Table-driven Tests**: All usecases tested with mock dependencies
