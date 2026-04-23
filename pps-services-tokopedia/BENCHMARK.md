# Benchmark Guide for PPS Services Tokopedia

This document describes the benchmark tests for all API endpoints in the PPS Services Tokopedia application.

## Production Load Expectations

The service is expected to handle the following load in production:

| API Endpoint | Expected Load | Description |
|-------------|---------------|-------------|
| `/auth/token` | 2 hits/hour | Token generation (~0.0006 TPS) |
| `/api/v1/health` | Every 5 minutes | Health check monitoring (~0.003 TPS) |
| `/api/v1/inquiry` | 300 TPS | High-throughput inquiry processing |
| `/api/v1/payment` | 250 TPS | High-throughput payment processing |
| `/api/v1/check-status` | 250 TPS | High-throughput status checking |

## Running Benchmarks

### Run All Benchmarks

```powershell
go test -bench=. -benchmem ./internal/delivery/http/...
```

### Run Specific Endpoint Benchmarks

#### Token API
```powershell
go test -bench=BenchmarkTokenHandler -benchmem ./internal/delivery/http/
```

#### Health Check API
```powershell
go test -bench=BenchmarkHealthCheckHandler -benchmem ./internal/delivery/http/
```

#### Inquiry API
```powershell
go test -bench=BenchmarkInquiryHandler -benchmem ./internal/delivery/http/
```

#### Payment API
```powershell
go test -bench=BenchmarkPaymentHandler -benchmem ./internal/delivery/http/
```

#### Check Status API
```powershell
go test -bench=BenchmarkCheckStatusHandler -benchmem ./internal/delivery/http/
```

### Run Parallel Benchmarks Only

To test concurrent request handling:

```powershell
go test -bench=Parallel -benchmem ./internal/delivery/http/
```

## Benchmark Options

### Set Benchmark Duration
```powershell
go test -bench=. -benchtime=10s ./internal/delivery/http/...
```

### Set Number of Iterations
```powershell
go test -bench=. -benchtime=1000x ./internal/delivery/http/...
```

### CPU Profiling
```powershell
go test -bench=. -cpuprofile=cpu.prof ./internal/delivery/http/...
go tool pprof cpu.prof
```

### Memory Profiling
```powershell
go test -bench=. -memprofile=mem.prof ./internal/delivery/http/...
go tool pprof mem.prof
```

### Generate Benchmark Report
```powershell
go test -bench=. -benchmem ./internal/delivery/http/... | tee benchmark_results.txt
```

## Understanding Benchmark Results

Example output:
```
BenchmarkInquiryHandler_Inquiry-8              10000    123456 ns/op    45678 B/op    123 allocs/op
BenchmarkInquiryHandler_Inquiry_Parallel-8     50000     25678 ns/op    45678 B/op    123 allocs/op
```

- `BenchmarkInquiryHandler_Inquiry-8`: Test name with GOMAXPROCS=8
- `10000`: Number of iterations completed
- `123456 ns/op`: Average nanoseconds per operation
- `45678 B/op`: Average bytes allocated per operation
- `123 allocs/op`: Average number of allocations per operation

## Performance Targets

Based on production expectations:

### Token API (2 hits/hour)
- **Target**: < 100ms per request
- **Reason**: Low traffic, occasional use

### Health Check API (every 5 minutes)
- **Target**: < 50ms per request
- **Reason**: Should respond quickly for monitoring

### Inquiry API (300 TPS)
- **Target**: < 10ms per request
- **Throughput**: Should handle 300+ concurrent requests/second
- **Memory**: Minimal allocations per request

### Payment API (250 TPS)
- **Target**: < 15ms per request
- **Throughput**: Should handle 250+ concurrent requests/second
- **Memory**: Efficient memory usage for high volume

### Check Status API (250 TPS)
- **Target**: < 10ms per request
- **Throughput**: Should handle 250+ concurrent requests/second
- **Memory**: Minimal database queries

## Optimization Tips

1. **Reduce Allocations**: Focus on reducing `allocs/op` for high-traffic endpoints
2. **Memory Pooling**: Use `sync.Pool` for frequently allocated objects
3. **Database Connections**: Ensure proper connection pool sizing
4. **Caching**: Implement caching for frequently accessed data
5. **Middleware Optimization**: Minimize middleware overhead for critical paths

## Continuous Monitoring

Run benchmarks regularly to:
- Detect performance regressions
- Validate optimization efforts
- Ensure production readiness
- Compare performance across versions

## Example Benchmark Comparison

To compare two versions:

```powershell
# Run benchmarks on old code
go test -bench=. -benchmem ./internal/delivery/http/... > old.txt

# Make changes...

# Run benchmarks on new code
go test -bench=. -benchmem ./internal/delivery/http/... > new.txt

# Compare results
go install golang.org/x/perf/cmd/benchstat@latest
benchstat old.txt new.txt
```

## Load Testing

For realistic load testing, use tools like:
- **Apache Bench (ab)**
- **wrk**
- **Gatling**
- **k6**

Example with wrk:
```bash
# Test inquiry endpoint at 300 TPS for 30 seconds
wrk -t12 -c300 -d30s --latency -s inquiry.lua http://localhost:8080/api/v1/inquiry
```
