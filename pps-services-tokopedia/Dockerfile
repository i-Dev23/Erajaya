# ---------- STAGE 1: Build ----------
FROM golang:1.24-alpine AS builder

WORKDIR /go/src/pps-services-tokopedia

# Install tools yang dibutuhkan untuk build
RUN apk add --no-cache git

# Copy go mod files first for better caching
COPY go.mod go.sum ./

# Download dependencies
RUN go mod download

# Copy semua source code
COPY . .

# Build binary statis (ukuran kecil, tanpa debug info) - menggunakan struktur baru
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o pps-services-tokopedia ./cmd/app

# ---------- STAGE 2: Final ----------
FROM alpine:3.22

# Build arguments untuk user configuration (bisa di-override saat build)
ARG APP_USER=it
ARG APP_UID=1000
ARG APP_GID=1000

WORKDIR /app

# Install timezone & curl untuk healthcheck
RUN apk add --no-cache tzdata curl

ENV TZ=Asia/Jakarta

# Buat user & group non-root dengan UID/GID yang configurable
RUN addgroup -g ${APP_GID} -S appgroup && \
    adduser -u ${APP_UID} -S ${APP_USER} -G appgroup

RUN mkdir -p /app/keyPPS && chown -R ${APP_USER}:appgroup /app/keyPPS

# Buat directory logs dengan permission yang proper untuk volume mount
RUN mkdir -p /app/logs && \
    chown -R ${APP_USER}:appgroup /app/logs && \
    chmod -R 775 /app/logs

# Directory for reconciliation exports (default ./reports under WORKDIR=/app)
RUN mkdir -p /app/reports && \
    chown -R ${APP_USER}:appgroup /app/reports && \
    chmod -R 775 /app/reports

# Copy binary dari stage builder, langsung set owner & permission
COPY --from=builder --chown=${APP_USER}:appgroup /go/src/pps-services-tokopedia/pps-services-tokopedia .

# Jalankan sebagai user non-root
USER ${APP_USER}

# Health check tiap 30 detik (uses public liveness endpoint)
HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD curl -fsS "http://localhost:${APP_PORT:-3001}/health" || exit 1

EXPOSE 3001

ENTRYPOINT ["./pps-services-tokopedia"]
    