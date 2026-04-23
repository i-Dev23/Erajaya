# ---------- STAGE 1: Build ----------
FROM golang:1.26-alpine AS builder

WORKDIR /src

RUN apk add --no-cache git ca-certificates

COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 \
    go build -trimpath -ldflags="-s -w" -o /out/pps-services-gateway-unipin ./cmd/app

# ---------- STAGE 2: Runtime ----------
FROM alpine:3.22

ARG APP_USER=app
ARG APP_UID=10001
ARG APP_GID=10001

WORKDIR /app

RUN apk add --no-cache ca-certificates tzdata && apk upgrade --no-cache musl zlib

ENV TZ=Asia/Jakarta
RUN ln -snf /usr/share/zoneinfo/$TZ /etc/localtime && echo $TZ > /etc/timezone

RUN addgroup -g ${APP_GID} -S ${APP_USER} && \
    adduser -u ${APP_UID} -S ${APP_USER} -G ${APP_USER}

COPY --from=builder /out/pps-services-gateway-unipin /app/pps-services-gateway-unipin

USER ${APP_USER}

ENTRYPOINT ["/app/pps-services-gateway-unipin"]
