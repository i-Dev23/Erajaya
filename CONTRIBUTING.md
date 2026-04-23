# Contributing Guide

Terima kasih sudah berkontribusi! Panduan ini membantu menjaga konsistensi kode.

---

## Prerequisites

- **Go 1.24+**
- **Google Wire** — `go install github.com/google/wire/cmd/wire@latest`
- **golangci-lint** — `go install github.com/golangci/golangci-lint/cmd/golangci-lint@latest`
- **Docker** & **Docker Compose** (untuk environment lengkap)

---

## Setup

```bash
# Clone & install dependencies
git clone <repo-url>
cd azec-backend-services
go mod download

# Generate Wire DI
make wire

# Run
make run
```

---

## Development Workflow

### 1. Buat branch dari `main`

```bash
git checkout -b feature/nama-fitur
```

### 2. Tulis kode sesuai style guide

Lihat [`docs/style.md`](docs/style.md) untuk konvensi kode lengkap.

**Prinsip utama:**
- Ikuti Clean Architecture — dependency hanya mengalir ke dalam.
- Entity di domain layer = struct murni, tanpa JSON tag.
- DTO di dto layer = struct dengan JSON tag, hanya untuk HTTP boundary.
- Mapper = pure function, stateless.
- Handler ikuti pola 5 langkah: Parse → Map → Usecase → Map → Return.
- Semua exported type/function harus punya doc comment.

### 3. Jalankan checks sebelum commit

```bash
# Format kode
make fmt

# Linter
make lint

# Semua test dengan race detector
make test
```

**Semua 3 command di atas harus pass tanpa error sebelum push.**

### 4. Commit message

Gunakan format [Conventional Commits](https://www.conventionalcommits.org/):

```
<type>(<scope>): <deskripsi singkat>

Contoh:
feat(inquiry): add retry logic for external API call
fix(payment): handle nil pointer on empty response
refactor(middleware): extract chain builder to separate package
test(balance): add table-driven tests for edge cases
docs(style): update middleware ordering section
chore(deps): bump gofiber to v2.52.11
```

**Types:** `feat`, `fix`, `refactor`, `test`, `docs`, `chore`, `ci`, `perf`

### 5. Pull Request

- PR title = commit message format.
- Deskripsi singkat: apa yang berubah dan kenapa.
- Pastikan CI pipeline hijau (lint + test).
- Minimal 1 reviewer approval sebelum merge.

---

## Menambah Fitur Baru

Langkah khas saat menambahkan fitur (misal: `refund`):

1. **Domain entity** — `internal/domain/entity/refund.go`
2. **Contract interface** — `internal/domain/contract/usecase/refund_usecase.go`, `…/repository/refund_repository.go`
3. **DTO** — `internal/dto/request/refund_request.go`, `…/response/refund_response.go`
4. **Mapper** — `internal/dto/mapper/refund_mapper.go`
5. **Repository impl** — `internal/repository/postgres/refund_repository.go`
6. **Usecase impl** — `internal/usecase/refund/refund_usecase.go`
7. **Handler** — `internal/delivery/http/handler/refund_handler.go`
8. **Register route** — tambah di `internal/delivery/http/router.go`
9. **Wire provider** — tambah di `cmd/app/wire.go`
10. **Test** — unit test di usecase, mock di `test/mock/`
11. **Migration** (jika perlu) — `database/migrations/NNN_add_refund.up.sql`

---

## Testing Guidelines

```bash
make test            # Semua test + race detector
make test-unit       # Unit test usecase saja
make test-benchmark  # Benchmark test
make coverage        # Generate coverage report (HTML)
```

- Gunakan **table-driven tests**.
- Mock dependencies via interface (lihat `test/mock/`).
- Fixtures reusable di `test/fixture/`.
- Test file = `<target>_test.go` di package yang sama.

---

## Struktur Kode

Lihat [`docs/style.md`](docs/style.md) untuk detail lengkap dan [`docs/architecture.md`](docs/architecture.md) untuk diagram arsitektur.

---

## Makefile Cheatsheet

| Command              | Fungsi                              |
|---------------------|--------------------------------------|
| `make build`        | Build binary ke `bin/app`            |
| `make run`          | Wire generate + run                  |
| `make wire`         | Generate Wire DI                     |
| `make test`         | All tests + race detector            |
| `make test-unit`    | Unit tests (usecase only)            |
| `make coverage`     | Coverage report (HTML)               |
| `make lint`         | golangci-lint                        |
| `make fmt`          | gofmt -s -w                          |
| `make vet`          | go vet                               |
| `make docker-build` | Build Docker image                   |
| `make docker-up`    | Docker Compose up                    |
| `make clean`        | Remove build artifacts               |
