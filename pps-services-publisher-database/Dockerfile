# Stage 1: Build
FROM golang:1.26-alpine AS builder

RUN apk add --no-cache git

WORKDIR /app

COPY go.mod go.sum ./
RUN go mod download

COPY . .

# Build binary web server (publisher hanya punya cmd/web)
RUN CGO_ENABLED=0 GOOS=linux go build -o /app/web ./cmd/web/main.go

# Stage 2: Runtime
FROM alpine:3.19

RUN apk --no-cache add ca-certificates tzdata

ENV TZ=Asia/Jakarta

WORKDIR /app

COPY --from=builder /app/web /app/web
COPY config.json /app/config.json

RUN mkdir -p /app/logs

RUN adduser -D -g '' appuser
USER appuser

EXPOSE 3000

CMD ["/app/web"]
