.PHONY: all build run test lint wire clean mock migrate-up migrate-down migrate-create

include .env
export

# === Build & Run ===
build:
	go build -o bin/app ./cmd/app

run:
	go run ./cmd/app

# === Testing ===
test:
	go test ./internal/... -v -cover -race

test-unit:
	go test ./internal/usecase/... -v -cover

test-integration:
	go test ./test/integration/... -v -tags=integration

test-benchmark:
	go test ./internal/... -bench=. -benchmem

coverage:
	go test ./internal/... -coverprofile=coverage.out
	go tool cover -html=coverage.out -o coverage.html

# === Code Quality ===
lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

# === Docker ===
docker-build:
	docker build -t azec-backend-services .

docker-up:
	docker compose up --build

# === Clean ===
clean:
	rm -rf bin/ coverage.out coverage.html

# === Database Migration ===
# Requires: go install -tags 'postgres' github.com/golang-migrate/migrate/v4/cmd/migrate@latest
# Or in Docker: migrate is pre-installed in the image

migrate-up:
	migrate -path database/migrations -database "$(POSTGRES_DSN)" up

migrate-down:
	migrate -path database/migrations -database "$(POSTGRES_DSN)" down 1

migrate-status:
	migrate -path database/migrations -database "$(POSTGRES_DSN)" version

migrate-create:
	@read -p "Migration name: " name; \
	migrate create -ext sql -dir database/migrations -seq $$name
