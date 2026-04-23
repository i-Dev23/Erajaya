# ---------- STAGE 1: Build ----------
  FROM golang:1.26-alpine AS builder

  WORKDIR /go/src/pps-services-consumer
  
  # Install tools yang dibutuhkan untuk build
  RUN apk add --no-cache git
  
  # Copy semua source code
  COPY . .
  
  # Build binary statis (ukuran kecil, tanpa debug info)
  RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o pps-services-consumer
  
  
  # ---------- STAGE 2: Final ----------
  FROM alpine:3.22
  
  WORKDIR /apps
  
  # Install timezone & curl untuk healthcheck
  RUN apk add --no-cache tzdata curl \
    && apk upgrade --no-cache
  
  ENV TZ=Asia/Jakarta
  
  # Buat user & group non-root
  RUN addgroup -S appgroup && adduser -S it -G appgroup
  
  # Buat logs directory dengan permission untuk non-root user
  RUN mkdir -p /apps/logs && chown it:appgroup /apps/logs
  
  # Copy binary dari stage builder, langsung set owner & permission
  COPY --from=builder --chown=it:appgroup /go/src/pps-services-consumer/pps-services-consumer .
  
  # Jalankan sebagai user non-root
  USER it
  
  # Health check tiap 30 detik
  #HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
  #  CMD curl -f http://localhost:9001/health || exit 1
  
  EXPOSE 9001
  
  ENTRYPOINT ["./pps-services-consumer"]
  