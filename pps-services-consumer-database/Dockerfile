# Stage 1: Build
FROM golang:1.23-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build binary worker consumer (consumer hanya punya cmd/worker)
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/worker ./cmd/worker/main.go

# Stage 2: Runtime
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

ENV TZ=Asia/Jakarta

WORKDIR /app

COPY --from=builder /app/worker /app/worker
COPY config.json /app/config.json

RUN mkdir -p /app/logs

RUN adduser -D -g '' appuser
USER appuser

CMD ["/app/worker"]
