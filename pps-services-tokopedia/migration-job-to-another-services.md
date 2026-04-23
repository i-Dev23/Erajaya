# Migration Plan: Memisahkan Cron Job ke Service Terpisah

## 📌 Tujuan

| Concern | Main Service (`pps-api`) | Job Service (`pps-job`) |
|---------|--------------------------|-------------------------|
| Scaling | Horizontal **+ Vertical** (HPA + VPA) | **Horizontal only** (max 1 replica aktif via leader election) |
| Fungsi | REST API, RabbitMQ consumer | Cron scheduler, cleanup, partition maintenance, reconciliation export |
| Downtime tolerance | Tidak boleh downtime (high availability) | Boleh restart tanpa impact ke user |

**Prinsip utama:** Job hanya boleh berjalan **satu instance aktif** pada satu waktu (singleton), sedangkan API service bisa di-scale bebas tanpa khawatir job duplicate.

---

## 🗺️ Current State Analysis

### Komponen yang terkait Job di main service saat ini:

```
cmd/app/main.go
├── InitializeApp()          ← Wire DI (termasuk job dependencies)
├── setupScheduledJobs()     ← trigger job setup
├── SchedulerService.Start() ← start cron
└── defer SchedulerService.Stop()

internal/
├── service/scheduler_service.go      ← robfig/cron wrapper
├── usecase/scheduled_jobs_usecase.go ← 7 job definitions
├── usecase/cleanup_usecase.go        ← DB cleanup logic
├── repository/cleanup_postgres_repository.go
└── domain/cleanup_domain.go          ← CleanupRepository interface
```

### 7 Job yang harus dipindahkan:

| # | Job | Dependencies |
|---|-----|-------------|
| 1 | HTTP logs cleanup | `CleanupRepository` → Postgres |
| 2 | Callback logs cleanup | `CleanupRepository` → Postgres |
| 3 | Inquiry logs cleanup | `CleanupRepository` → Postgres |
| 4 | Payment logs cleanup | `CleanupRepository` → Postgres |
| 5 | File logs cleanup | Filesystem (local logs) |
| 6 | Partition maintenance | `PostgresService` (direct query) |
| 7 | Reconciliation export | `PostgresService` (direct query) → CSV file |

### Shared Dependencies (dipakai job & API):

- `service.PostgresService`
- `service.Logger` + `service.TelegramService`
- `domain.CleanupRepository` / `repository.CleanupPostgresRepository`
- `internal/config`
- `internal/utils`

### Dependencies yang **hanya** dipakai API (tidak perlu di job service):

- `service.OracleService`
- `service.RedisClient`
- `service.RabbitMQService`
- `service.CryptoService` / `service.DigitalSignatureService`
- `service.UltimaService`
- Semua handler, middleware, token usecase, dsb.

---

## 🚀 Step-by-Step Migration

### Phase 1: Persiapan — Shared Module (Go internal packages)

> **Goal:** Pastikan kode yang dipakai bersama oleh API dan Job bisa di-import tanpa duplikasi.

Karena kedua service ada dalam **satu repository** (monorepo), kamu bisa langsung import package `internal/` dari masing-masing entrypoint.

#### Step 1.1 — Buat entrypoint baru untuk Job service

```
cmd/
├── api/                ← rename dari cmd/app (main service)
│   ├── main.go
│   ├── wire.go
│   └── wire_gen.go
│
└── job/                ← NEW: job service entrypoint
    ├── main.go
    ├── wire.go
    └── wire_gen.go
```

```bash
# Buat folder baru
mkdir -p cmd/job

# Rename cmd/app → cmd/api (opsional, bisa tetap cmd/app)
# Atau biarkan cmd/app untuk API, tambah cmd/job untuk Job
```

#### Step 1.2 — Buat `cmd/job/main.go`

```go
package main

import (
    "context"
    "log"
    "os"
    "os/signal"
    "syscall"

    "pps-services-tokopedia/internal/service"
)

func main() {
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // Initialize job container using Wire DI
    jobContainer, err := InitializeJobApp()
    if err != nil {
        log.Fatalf("failed to initialize job application: %v", err)
    }

    // Cleanup resources on exit
    defer func() {
        if jobContainer.SchedulerService != nil {
            shutdownCtx := jobContainer.SchedulerService.Stop()
            <-shutdownCtx.Done()
        }
        if jobContainer.PostgresService != nil {
            jobContainer.PostgresService.Close()
        }
    }()

    // Handle graceful shutdown
    handleSigterm(ctx, cancel, jobContainer.Logger)

    // Setup & start scheduled jobs
    jobContainer.ScheduledJobsUsecase.SetupScheduledJobs()
    jobContainer.SchedulerService.Start()
    jobContainer.Logger.Info("Job scheduler started successfully")

    // Block until shutdown signal
    <-ctx.Done()
    jobContainer.Logger.Info("Job service shutting down...")
}

func handleSigterm(ctx context.Context, cancel context.CancelFunc, logger service.Logger) {
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGTERM, syscall.SIGINT)

    go func() {
        defer signal.Stop(sigCh)
        select {
        case sig := <-sigCh:
            logger.Info("Received signal, shutting down...", "signal", sig)
            cancel()
        case <-ctx.Done():
        }
    }()
}
```

#### Step 1.3 — Buat `cmd/job/wire.go`

```go
//go:build wireinject
// +build wireinject

package main

import (
    "pps-services-tokopedia/internal/repository"
    "pps-services-tokopedia/internal/service"
    "pps-services-tokopedia/internal/usecase"

    "github.com/google/wire"
)

// JobContainer — hanya dependencies yang dibutuhkan oleh Job
type JobContainer struct {
    Logger               service.Logger
    PostgresService      service.PostgresService
    SchedulerService     service.SchedulerService
    ScheduledJobsUsecase usecase.ScheduledJobsUsecase
}

func NewJobContainer(
    logger service.Logger,
    postgresService service.PostgresService,
    schedulerService service.SchedulerService,
    scheduledJobsUsecase usecase.ScheduledJobsUsecase,
) *JobContainer {
    return &JobContainer{
        Logger:               logger,
        PostgresService:      postgresService,
        SchedulerService:     schedulerService,
        ScheduledJobsUsecase: scheduledJobsUsecase,
    }
}

// Provider set khusus job (tanpa Oracle, Redis, RabbitMQ, Crypto, handlers, dsb.)
var JobProviderSet = wire.NewSet(
    // Services (minimal)
    NewTelegramService,
    NewLoggerWithTelegram,
    service.NewPostgresService,
    service.NewSchedulerService,

    // Repository (cleanup only)
    repository.NewCleanupPostgresRepository,

    // Usecases (cleanup + scheduled jobs only)
    usecase.NewCleanupUsecase,
    usecase.NewScheduledJobsUsecase,
)

func InitializeJobApp() (*JobContainer, error) {
    wire.Build(JobProviderSet, NewJobContainer)
    return nil, nil
}
```

#### Step 1.4 — Generate Wire untuk Job

```bash
cd cmd/job && wire && cd -
```

---

### Phase 2: Refactor Main Service — Hapus Job dari API

> **Goal:** `cmd/app/main.go` tidak lagi menjalankan scheduler/cron.

#### Step 2.1 — Hapus job-related code dari `cmd/app/main.go`

Hapus bagian berikut dari `main()`:

```go
// HAPUS:
// Setup scheduled jobs
setupScheduledJobs(appContainer)

// Start the scheduler
appContainer.SchedulerService.Start()
appContainer.Logger.Info("Scheduler service started successfully")
```

Dan di bagian `defer`:

```go
// HAPUS:
// Stop scheduler
if appContainer.SchedulerService != nil {
    shutdownCtx := appContainer.SchedulerService.Stop()
    <-shutdownCtx.Done()
}
```

Dan hapus function `setupScheduledJobs()` di akhir file.

#### Step 2.2 — Hapus job dependencies dari `cmd/app/wire.go`

Hapus dari `ProviderSet`:

```go
// HAPUS dari ProviderSet:
service.NewSchedulerService,
repository.NewCleanupPostgresRepository,
usecase.NewCleanupUsecase,
usecase.NewScheduledJobsUsecase,
```

Hapus dari `AppContainer`:

```go
// HAPUS field:
SchedulerService     service.SchedulerService
CleanupUsecase       usecase.CleanupUsecase
ScheduledJobsUsecase usecase.ScheduledJobsUsecase
```

#### Step 2.3 — Regenerate Wire untuk API

```bash
cd cmd/app && wire && cd -
```

---

### Phase 3: Dockerfile — Multi-Binary Build

> **Goal:** Satu Dockerfile menghasilkan dua binary: `pps-api` dan `pps-job`.

#### Step 3.1 — Update `Dockerfile`

```dockerfile
# ---------- STAGE 1: Build ----------
FROM golang:1.24-alpine AS builder

WORKDIR /go/src/pps-services-tokopedia
RUN apk add --no-cache git

COPY go.mod go.sum ./
RUN go mod download
COPY . .

# Build API binary
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o pps-api ./cmd/app

# Build Job binary
RUN CGO_ENABLED=0 go build -ldflags="-s -w" -o pps-job ./cmd/job

# ---------- STAGE 2: API Image ----------
FROM alpine:3.22 AS api

ARG APP_USER=it
ARG APP_UID=1000
ARG APP_GID=1000

WORKDIR /app
RUN apk add --no-cache tzdata curl
ENV TZ=Asia/Jakarta

RUN addgroup -g ${APP_GID} -S appgroup && \
    adduser -u ${APP_UID} -S ${APP_USER} -G appgroup
RUN mkdir -p /app/logs && chown -R ${APP_USER}:appgroup /app/logs && chmod -R 775 /app/logs

COPY --from=builder --chown=${APP_USER}:appgroup /go/src/pps-services-tokopedia/pps-api .
USER ${APP_USER}

HEALTHCHECK --interval=30s --timeout=5s --retries=3 \
    CMD curl -fsS "http://localhost:${APP_PORT:-3001}/health" || exit 1

EXPOSE 3001
ENTRYPOINT ["./pps-api"]

# ---------- STAGE 3: Job Image ----------
FROM alpine:3.22 AS job

ARG APP_USER=it
ARG APP_UID=1000
ARG APP_GID=1000

WORKDIR /app
RUN apk add --no-cache tzdata
ENV TZ=Asia/Jakarta

RUN addgroup -g ${APP_GID} -S appgroup && \
    adduser -u ${APP_UID} -S ${APP_USER} -G appgroup
RUN mkdir -p /app/logs /app/reports && \
    chown -R ${APP_USER}:appgroup /app/logs /app/reports && \
    chmod -R 775 /app/logs /app/reports

COPY --from=builder --chown=${APP_USER}:appgroup /go/src/pps-services-tokopedia/pps-job .
USER ${APP_USER}

ENTRYPOINT ["./pps-job"]
```

#### Step 3.2 — Build command

```bash
# Build API image
docker build --target api -t pps-api:latest .

# Build Job image
docker build --target job -t pps-job:latest .
```

---

### Phase 4: Deployment & Scaling Strategy

#### Step 4.1 — Kubernetes Deployment (API) — Horizontal + Vertical

```yaml
# k8s/api-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pps-api
  labels:
    app: pps-api
spec:
  replicas: 3          # Bisa di-scale horizontal
  selector:
    matchLabels:
      app: pps-api
  template:
    metadata:
      labels:
        app: pps-api
    spec:
      containers:
        - name: pps-api
          image: pps-api:latest
          ports:
            - containerPort: 3001
          resources:
            requests:
              cpu: "250m"       # VPA bisa adjust ini
              memory: "256Mi"
            limits:
              cpu: "1000m"
              memory: "1Gi"
          env:
            - name: APP_PORT
              value: "3001"
            - name: RUN_JOBS_LOG_RETENTION
              value: "N"          # ← PENTING: disable job di API
            # ... env lainnya (DB, Redis, RabbitMQ, dsb.)
          livenessProbe:
            httpGet:
              path: /health
              port: 3001
            initialDelaySeconds: 10
            periodSeconds: 30
          readinessProbe:
            httpGet:
              path: /health
              port: 3001
            initialDelaySeconds: 5
            periodSeconds: 10
---
apiVersion: autoscaling/v2
kind: HorizontalPodAutoscaler
metadata:
  name: pps-api-hpa
spec:
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: pps-api
  minReplicas: 2
  maxReplicas: 10
  metrics:
    - type: Resource
      resource:
        name: cpu
        target:
          type: Utilization
          averageUtilization: 70
    - type: Resource
      resource:
        name: memory
        target:
          type: Utilization
          averageUtilization: 80
```

#### Step 4.2 — Kubernetes Deployment (Job) — Singleton / Leader Election

```yaml
# k8s/job-deployment.yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: pps-job
  labels:
    app: pps-job
spec:
  replicas: 1              # ← Selalu 1 (singleton)
  strategy:
    type: Recreate          # ← Jangan RollingUpdate, hindari 2 instance jalan bareng
  selector:
    matchLabels:
      app: pps-job
  template:
    metadata:
      labels:
        app: pps-job
    spec:
      containers:
        - name: pps-job
          image: pps-job:latest
          resources:
            requests:
              cpu: "100m"
              memory: "128Mi"
            limits:
              cpu: "500m"
              memory: "512Mi"
          env:
            - name: RUN_JOBS_LOG_RETENTION
              value: "Y"        # ← Enable job
            - name: POSTGRES_DSN
              valueFrom:
                secretKeyRef:
                  name: pps-secrets
                  key: postgres-dsn
            # ... env lainnya (cron schedules, retention days, dsb.)
```

> **Kenapa `replicas: 1` + `strategy: Recreate`?**
> - `replicas: 1` → hanya satu pod yang jalan → tidak ada duplicate job execution.
> - `Recreate` → saat deploy ulang, pod lama dihentikan dulu sebelum pod baru dibuat, sehingga **tidak pernah ada 2 pod jalan bersamaan**.

#### Step 4.3 — (Opsional) Leader Election untuk High Availability

Jika butuh HA (pod job auto-replace kalau mati), bisa pakai **Kubernetes Lease-based leader election**:

```go
// cmd/job/main.go — tambahkan leader election
import "k8s.io/client-go/tools/leaderelection"

// Hanya leader yang menjalankan scheduler
// Follower standby, siap take over jika leader mati
```

Atau lebih sederhana, gunakan **Kubernetes CronJob** (lihat Phase 5 alternatif).

---

### Phase 5: (Alternatif) Gunakan Kubernetes CronJob

Jika ingin fully cloud-native, ganti `robfig/cron` dengan Kubernetes CronJob resources:

```yaml
# k8s/cronjob-http-cleanup.yaml
apiVersion: batch/v1
kind: CronJob
metadata:
  name: pps-http-logs-cleanup
spec:
  schedule: "0 1 * * *"          # setiap hari 01:00 (5 field, bukan 6)
  concurrencyPolicy: Forbid       # jangan jalankan 2 job bersamaan
  successfulJobsHistoryLimit: 3
  failedJobsHistoryLimit: 3
  jobTemplate:
    spec:
      backoffLimit: 2
      template:
        spec:
          restartPolicy: OnFailure
          containers:
            - name: cleanup
              image: pps-job:latest
              command: ["./pps-job"]
              args: ["--run-once", "--job=http-cleanup"]
              env:
                - name: HTTP_LOG_RETENTION_DAYS
                  value: "31"
                - name: POSTGRES_DSN
                  valueFrom:
                    secretKeyRef:
                      name: pps-secrets
                      key: postgres-dsn
```

> Ini memerlukan modifikasi `cmd/job/main.go` untuk support flag `--run-once` dan `--job=<nama>` agar bisa menjalankan satu job lalu exit.

---

### Phase 6: CI/CD Pipeline Update

#### Step 6.1 — Jenkinsfile (atau CI pipeline)

Pipeline harus build **2 image** dari satu repo:

```groovy
// Jenkinsfile (contoh)
pipeline {
    stages {
        stage('Build') {
            parallel {
                stage('Build API') {
                    steps {
                        sh 'docker build --target api -t pps-api:${BUILD_TAG} .'
                    }
                }
                stage('Build Job') {
                    steps {
                        sh 'docker build --target job -t pps-job:${BUILD_TAG} .'
                    }
                }
            }
        }
        stage('Deploy API') {
            steps {
                sh 'kubectl set image deployment/pps-api pps-api=pps-api:${BUILD_TAG}'
                // API bisa rolling update (zero downtime)
            }
        }
        stage('Deploy Job') {
            steps {
                sh 'kubectl set image deployment/pps-job pps-job=pps-job:${BUILD_TAG}'
                // Job pakai Recreate strategy (brief downtime OK)
            }
        }
    }
}
```

---

### Phase 7: Testing & Validation

#### Step 7.1 — Validasi API service tanpa job

```bash
# Set env: RUN_JOBS_LOG_RETENTION=N
# Start API service
# Pastikan:
# ✅ API endpoint berfungsi normal
# ✅ Tidak ada log "scheduler" / "cron"
# ✅ RabbitMQ consumer tetap jalan (jika enabled)
```

#### Step 7.2 — Validasi Job service tanpa API

```bash
# Start Job service
# Pastikan:
# ✅ Scheduler start & job registered
# ✅ Job execute sesuai cron schedule
# ✅ Tidak ada Fiber/HTTP listener
# ✅ Graceful shutdown bekerja
```

#### Step 7.3 — Validasi scaling behavior

```bash
# API: scale ke 3 replicas
kubectl scale deployment pps-api --replicas=3
# ✅ Semua replica serve traffic
# ✅ Tidak ada duplicate job execution

# Job: tetap 1 replica
kubectl get pods -l app=pps-job
# ✅ Hanya 1 pod
```

---

## 📋 Checklist Migrasi

### Preparation
- [ ] Buat folder `cmd/job/`
- [ ] Buat `cmd/job/main.go` (tanpa Fiber, hanya scheduler)
- [ ] Buat `cmd/job/wire.go` (minimal dependencies)
- [ ] Copy helper functions yang dipakai (e.g., `NewTelegramService`, `NewLoggerWithTelegram`)
- [ ] Generate `cmd/job/wire_gen.go` via `wire`

### Refactor API
- [ ] Hapus `setupScheduledJobs()` dari `cmd/app/main.go`
- [ ] Hapus `SchedulerService.Start()` / `Stop()` dari `cmd/app/main.go`
- [ ] Hapus job-related providers dari `cmd/app/wire.go`
- [ ] Hapus `SchedulerService`, `CleanupUsecase`, `ScheduledJobsUsecase` dari `AppContainer`
- [ ] Regenerate `cmd/app/wire_gen.go`
- [ ] Set `RUN_JOBS_LOG_RETENTION=N` di API deployment env

### Docker & CI/CD
- [ ] Update `Dockerfile` dengan multi-target build (api + job)
- [ ] Update CI pipeline untuk build 2 image
- [ ] Buat Kubernetes manifest: `api-deployment.yaml` + `api-hpa.yaml`
- [ ] Buat Kubernetes manifest: `job-deployment.yaml` (replicas=1, Recreate)
- [ ] Test build lokal: `docker build --target api` dan `docker build --target job`

### Testing
- [ ] API berjalan normal tanpa scheduler
- [ ] Job service berjalan dan execute sesuai jadwal
- [ ] Scale API ke >1 replica, pastikan tidak ada duplicate job
- [ ] Graceful shutdown kedua service bekerja
- [ ] Monitoring/alerting (Telegram) bekerja di kedua service

### Post-Migration
- [ ] Update `README.md` dengan informasi service baru
- [ ] Update `cronjob.md` — job sekarang dijalankan oleh `pps-job`
- [ ] Update docker-compose (jika ada) untuk development lokal

---

## ⚠️ Catatan Penting

### Job #5 (File logs cleanup)
Job ini membersihkan file log di **filesystem lokal**. Setelah dipisah:
- File log yang perlu dibersihkan ada di **pod API** (bukan pod Job).
- **Solusi:** Jika API dan Job share volume (via PVC), job bisa bersihkan. Jika tidak, pertimbangkan:
  - Pindahkan file log cleanup ke **sidecar container** di API pod, atau
  - Gunakan **log rotation** bawaan container runtime (Docker/containerd).

### Job #7 (Reconciliation export)
Job ini menghasilkan file CSV di filesystem. Pastikan:
- Output directory (`/app/reports`) di-mount ke **PersistentVolumeClaim** agar file tidak hilang saat pod restart.
- Atau kirim file ke object storage (S3/MinIO) setelah generate.

### Database Connection Pooling
Job service hanya butuh koneksi Postgres (tanpa Oracle/Redis). Set pool size kecil:
```
POSTGRES_MAX_CONNS=5
POSTGRES_MIN_CONNS=1
```

### Timezone
Pastikan kedua service (API + Job) pakai timezone yang sama (`Asia/Jakarta`) agar jadwal cron sesuai ekspektasi.
