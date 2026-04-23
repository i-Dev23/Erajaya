.PHONY: build run test lint fmt vet clean docker-build

APP_NAME := pps-services-gateway-smb
BIN_DIR := bin

build:
	go build -o $(BIN_DIR)/app ./cmd/app

run: build
	./$(BIN_DIR)/app

test:
	go test -race -count=1 ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -s -w .

vet:
	go vet ./...

clean:
	rm -rf $(BIN_DIR)

docker-build:
	docker build -t $(APP_NAME) .
