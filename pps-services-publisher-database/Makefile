.PHONY: build run test test-cover docker-up docker-down docker-build docker-logs swagger k6 clean deps

# Build binary
build:
	go build -o bin/web ./cmd/web/main.go

# Jalankan web server
run:
	go run ./cmd/web/main.go

# Jalankan semua test
test:
	go test -v ./test/...

# Test dengan coverage report
test-cover:
	go test -v -coverprofile=coverage.out ./test/...
	go tool cover -html=coverage.out -o coverage.html

# Docker compose up (rabbitmq + publisher)
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

# Generate swagger docs
swagger:
	swag init -g cmd/web/main.go -o docs

# Jalankan k6 load test
k6:
	k6 run test/loadtest/k6_load_test.js

# Clean build artifacts
clean:
	rm -rf bin/ coverage.out coverage.html logs/

# Download dependencies
deps:
	go mod download
	go mod tidy
