# ---------- STAGE 1: Build ----------
FROM golang:1.26-alpine AS builder

WORKDIR /src

# Required for downloading private/public git modules when needed.
RUN apk add --no-cache git ca-certificates

# Cache dependency download.
COPY go.mod go.sum ./
RUN go mod download

# Copy source and build binary.
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/pps-services-gateway-telkomsel ./cmd/app

# ---------- STAGE 2: Runtime ----------
FROM alpine:3.22.3

ARG APP_USER=app
ARG APP_UID=10001
ARG APP_GID=10001

WORKDIR /app

# Upgrade base packages to patch known CVEs (zlib >= 1.3.2-r0, musl >= 1.2.5-r12),
# then install runtime dependencies.
RUN apk update && apk upgrade --no-cache && \
    apk add --no-cache ca-certificates tzdata

ENV TZ=Asia/Jakarta
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

# Create non-root user.
RUN addgroup -g ${APP_GID} -S ${APP_USER} && \
    adduser -u ${APP_UID} -S ${APP_USER} -G ${APP_USER}

# Copy binary from builder.
COPY --from=builder /out/pps-services-gateway-telkomsel /app/pps-services-gateway-telkomsel

USER ${APP_USER}

ENTRYPOINT ["/app/pps-services-gateway-telkomsel"]
