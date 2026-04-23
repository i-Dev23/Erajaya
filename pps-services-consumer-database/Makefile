.PHONY: build run test test-cover docker-up docker-down docker-build docker-logs clean deps

# Build binary
build:
	go build -o bin/worker ./cmd/worker/main.go

# Jalankan worker consumer
run:
	go run ./cmd/worker/main.go

# Jalankan semua test
test:
	go test -v ./test/...

# Test dengan coverage report
test-cover:
	go test -v -coverprofile=coverage.out ./test/...
	go tool cover -html=coverage.out -o coverage.html

# Docker compose up (rabbitmq + consumer)
docker-up:
	docker-compose up -d

# Docker compose down
docker-down:
	docker-compose down

# Docker build
docker-build:
	docker-compose build

# Docker logs
docker-logs:
	docker-compose logs -f

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html logs/

# Download dependencies
deps:
	go mod download
	go mod tidy
