# Tutorial Unit Test Go — Dari Nol Sampai Paham

> Ditulis khusus buat yang baru belajar Go testing.
> Semua contoh diambil dari kode project `pps-services-gateway-smb` ini.

---

## LEVEL 1: Dasar-Dasar — "Test Itu Apa Sih?"

### Apa itu Unit Test?

Unit test = **kode yang ngetes kode**.
Daripada lo jalanin program terus cek manual "ini bener gak ya?",
lo bikin kode yang otomatis ngecek hasilnya.

```
TANPA test:  Tulis kode → Jalanin → Cek manual → "Kayaknya bener"
DENGAN test: Tulis kode → Tulis test → `go test` → ✅ PASS / ❌ FAIL
```

### Aturan Dasar Go Test

| Aturan | Contoh |
|--------|--------|
| File test harus akhiran `_test.go` | `rc_test.go`, bukan `rc_testing.go` |
| Function test harus awalan `Test` | `TestResolveRCPPS`, bukan `testResolveRCPPS` |
| Parameter harus `*testing.T` | `func TestXxx(t *testing.T)` |
| File test di folder yang sama | `rc.go` dan `rc_test.go` satu folder |

### Cara Jalanin Test

```bash
# Test semua package
go test ./...

# Test satu package
go test ./internal/util/

# Test dengan output detail (-v = verbose)
go test -v ./internal/util/

# Test satu function aja
go test -v -run TestResolveRCPPS ./internal/util/

# Test dengan coverage
go test -cover ./...
```

---

## LEVEL 2: Test Pertama — Function Sederhana

### Kode yang mau di-test

File: `internal/util/rc.go`
```go
func ResolveRCPPS(smbResponseCode string) int {
    switch smbResponseCode {
    case "00":
        return 0 // Success
    case "28", "68":
        return 9 // Pending
    case "":
        return 9 // Empty = pending
    default:
        return 1 // Failed
    }
}
```

### Test-nya

File: `internal/util/rc_test.go`
```go
package util          // ← HARUS sama dengan package yang di-test

import "testing"      // ← import package testing dari Go standard library

func TestResolveRCPPS(t *testing.T) {
    // Arrange: siapkan input dan expected output
    input := "00"
    expected := 0

    // Act: panggil function yang mau di-test
    got := ResolveRCPPS(input)

    // Assert: cek hasilnya
    if got != expected {
        t.Errorf("ResolveRCPPS(%q) = %d, want %d", input, got, expected)
        //       ^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^^
        //       Pesan error yang muncul kalau test FAIL
        //       %q = string dengan quotes, %d = angka
    }
}
```

### Penjelasan Baris per Baris

```go
package util
// ↑ Package HARUS sama dengan file yang di-test (rc.go juga package util)
// Ini supaya test bisa akses function yang di-test langsung

import "testing"
// ↑ Package bawaan Go untuk testing. Wajib di-import.

func TestResolveRCPPS(t *testing.T) {
// ↑    ↑                ↑
// |    |                └── Parameter: pointer ke testing.T
// |    |                    Dipakai untuk report error (t.Errorf, t.Fatalf, dll)
// |    └── Nama function HARUS awalan "Test" + huruf kapital
// └── func = deklarasi function

    got := ResolveRCPPS("00")
    // ↑ Panggil function yang mau di-test

    if got != 0 {
        t.Errorf("expected 0, got %d", got)
        // ↑ t.Errorf = test FAIL tapi lanjut ke assertion berikutnya
        //   t.Fatalf = test FAIL dan STOP (gak lanjut)
    }
}
```

---

## LEVEL 3: Table-Driven Test — "Test Banyak Case Sekaligus"

Daripada bikin 10 function test terpisah, pakai **table-driven test**:

```go
func TestResolveRCPPS(t *testing.T) {
    // "Tabel" berisi semua test case
    tests := []struct {
        name     string  // nama test case (buat log)
        input    string  // input ke function
        expected int     // output yang diharapkan
    }{
        {"success",       "00",  0},   // case 1
        {"pending 28",    "28",  9},   // case 2
        {"pending 68",    "68",  9},   // case 3
        {"empty string",  "",    9},   // case 4
        {"failed 93",     "93",  1},   // case 5
        {"failed random", "XYZ", 1},   // case 6
    }

    // Loop semua case
    for _, tt := range tests {
        t.Run(tt.name, func(t *testing.T) {
        //     ↑ t.Run = bikin sub-test dengan nama
        //       Output: TestResolveRCPPS/success
        //               TestResolveRCPPS/pending_28
        //               dst...

            got := ResolveRCPPS(tt.input)
            if got != tt.expected {
                t.Errorf("ResolveRCPPS(%q) = %d, want %d",
                    tt.input, got, tt.expected)
            }
        })
    }
}
```

### Kenapa Table-Driven?

```
❌ TANPA table-driven: 6 function terpisah, banyak copy-paste
✅ DENGAN table-driven: 1 function, tinggal tambah baris di tabel

Mau tambah case baru? Tinggal tambah 1 baris:
    {"new case", "07", 1},
```

---

## LEVEL 4: Mock — "Gimana Test Kode yang Panggil API External?"

### Masalah

Usecase `ProcessTransaction()` panggil `smbClient.InquiryPLNToken()`.
Kalau test langsung, dia bakal beneran call API SMB → **gak bisa di-test offline**.

### Solusi: Mock

Mock = **object palsu** yang pura-pura jadi dependency.
Kita kontrol response-nya supaya bisa test semua skenario.

### Step 1: Lihat Interface yang Mau Di-mock

File: `internal/domain/contract/service/smb_client.go`
```go
type SMBClient interface {
    InquiryPLNToken(ctx context.Context, req PLNTokenInquiryRequest) (*PLNTokenInquiryResponse, error)
    PaymentPLNToken(ctx context.Context, req PLNTokenPaymentRequest) (*PLNTokenPaymentResponse, error)
    AdvicePLNToken(ctx context.Context, req PLNTokenAdviceRequest) (*PLNTokenAdviceResponse, error)
}
```

### Step 2: Bikin Mock Struct

```go
type mockSMBClient struct {
    // Function fields — bisa di-set per test case
    inquiryFunc func(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error)
    paymentFunc func(ctx context.Context, req contractsvc.PLNTokenPaymentRequest) (*contractsvc.PLNTokenPaymentResponse, error)
    adviceFunc  func(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error)
}

// Implementasi interface — tinggal panggil function field
func (m *mockSMBClient) InquiryPLNToken(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error) {
    return m.inquiryFunc(ctx, req)
}
func (m *mockSMBClient) PaymentPLNToken(ctx context.Context, req contractsvc.PLNTokenPaymentRequest) (*contractsvc.PLNTokenPaymentResponse, error) {
    return m.paymentFunc(ctx, req)
}
func (m *mockSMBClient) AdvicePLNToken(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
    return m.adviceFunc(ctx, req)
}
```

### Step 3: Pakai Mock di Test

```go
func TestProcessTransaction_InquirySuccess_PaymentSuccess(t *testing.T) {
    // Bikin mock yang return sukses
    client := &mockSMBClient{
        inquiryFunc: func(ctx context.Context, req contractsvc.PLNTokenInquiryRequest) (*contractsvc.PLNTokenInquiryResponse, error) {
            // ↑ Ini function palsu — gak beneran call API
            return &contractsvc.PLNTokenInquiryResponse{
                ResponseCode: "00",       // pura-pura sukses
                RefID:        "REF-001",
                TotalAmount:  52500,
            }, nil  // nil = gak ada error
        },
        paymentFunc: func(ctx context.Context, req contractsvc.PLNTokenPaymentRequest) (*contractsvc.PLNTokenPaymentResponse, error) {
            return &contractsvc.PLNTokenPaymentResponse{
                ResponseCode: "00",
                Token:        "1234-5678-9012",
            }, nil
        },
    }

    // Inject mock ke usecase
    uc := NewUsecase(client, nil, &mockLogger{})

    // Panggil function yang mau di-test
    result, err := uc.ProcessTransaction(context.Background(), "12345678901", "PLN50", "MSG001", 50000)

    // Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if result.Status != "SUCCESS" {
        t.Errorf("expected SUCCESS, got %s", result.Status)
    }
    if result.Token != "1234-5678-9012" {
        t.Errorf("expected token 1234-5678-9012, got %s", result.Token)
    }
}
```

### Visualisasi: Kode Asli vs Test

```
KODE ASLI (production):
    Usecase → SMBClient (interface) → HTTP Client → SMB API Server
                                                     ↑ butuh internet

TEST (unit test):
    Usecase → SMBClient (interface) → mockSMBClient (palsu)
                                       ↑ gak butuh internet
                                       ↑ kita kontrol response-nya
```

### Mock Logger (No-Op)

Logger juga perlu di-mock supaya gak print ke console saat test:

```go
type mockLogger struct{}

func (m *mockLogger) Info(msg string, kv ...any)  {} // gak ngapa-ngapain
func (m *mockLogger) Warn(msg string, kv ...any)  {}
func (m *mockLogger) Error(msg string, kv ...any) {}
```

---

## LEVEL 5: httptest — "Mock HTTP Server"

### Masalah

`pkg/smb/pln_token.go` langsung call HTTP ke SMB API.
Gak bisa pakai mock interface karena ini **implementasi HTTP client** itu sendiri.

### Solusi: `httptest.NewServer`

Go punya package `net/http/httptest` yang bisa bikin **HTTP server palsu** di localhost.

```go
import (
    "net/http"
    "net/http/httptest"
    "encoding/json"
)

func TestInquiryPLNToken_Success(t *testing.T) {
    // 1. Bikin HTTP server palsu
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        // ↑ Ini handler palsu — pura-pura jadi SMB API

        // Cek request yang masuk
        if r.URL.Path != "/api/v1/pln-prepaid/inquiry" {
            t.Errorf("unexpected path: %s", r.URL.Path)
        }
        if r.Method != http.MethodPost {
            t.Errorf("unexpected method: %s", r.Method)
        }

        // Return response palsu
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(InquiryResponse{
            ResponseCode: "00",
            Message:      "Success",
            Data: &InquiryData{
                RefID:       "REF-001",
                ClientName:  "BUDI",
                TotalAmount: 52500,
            },
        })
    }))
    defer server.Close()  // ← PENTING: tutup server setelah test selesai

    // 2. Bikin client yang nembak ke server palsu (bukan ke SMB beneran)
    client := NewClient(
        server.URL,    // ← URL server palsu, misal http://127.0.0.1:54321
        "PARTNER1",
        "SECRET1",
        5*time.Second,
        newTestLogger(),
    )

    // 3. Panggil function yang mau di-test
    resp, rawBody, err := client.InquiryPLNToken(context.Background(), "12345678901", "PLN50")

    // 4. Assert
    if err != nil {
        t.Fatalf("unexpected error: %v", err)
    }
    if resp.ResponseCode != "00" {
        t.Errorf("expected 00, got %s", resp.ResponseCode)
    }
    if resp.Data.RefID != "REF-001" {
        t.Errorf("expected REF-001, got %s", resp.Data.RefID)
    }
    if len(rawBody) == 0 {
        t.Error("expected raw body")
    }
}
```

### Visualisasi

```
KODE ASLI (production):
    smb.Client → HTTP POST → https://api.smb.com/api/v1/pln-prepaid/inquiry
                              ↑ server beneran di internet

TEST (unit test):
    smb.Client → HTTP POST → http://127.0.0.1:54321/api/v1/pln-prepaid/inquiry
                              ↑ httptest.NewServer (server palsu di localhost)
                              ↑ kita kontrol response-nya
```

### Test Error Scenarios

```go
// Test: server down
func TestInquiryPLNToken_ServerDown(t *testing.T) {
    // Pakai URL yang gak ada server-nya
    client := NewClient("http://localhost:1", "P1", "S1", 1*time.Second, newTestLogger())
    _, _, err := client.InquiryPLNToken(context.Background(), "12345678901", "PLN50")

    if err == nil {
        t.Fatal("expected error, got nil")
        //      ↑ t.Fatal = test FAIL dan STOP
    }
}

// Test: server return bukan JSON
func TestInquiryPLNToken_InvalidJSON(t *testing.T) {
    server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        w.Write([]byte("not json"))  // ← return string biasa, bukan JSON
    }))
    defer server.Close()

    client := NewClient(server.URL, "P1", "S1", 5*time.Second, newTestLogger())
    _, rawBody, err := client.InquiryPLNToken(context.Background(), "12345678901", "PLN50")

    if err == nil {
        t.Fatal("expected error for invalid JSON")
    }
    if len(rawBody) == 0 {
        t.Error("expected raw body even on parse error")
        //      ↑ rawBody harus tetap ada walaupun parse gagal
        //        supaya bisa di-log untuk debugging
    }
}
```

---

## LEVEL 6: Test Retry Logic — "Gimana Test Function yang Loop?"

### Masalah

`RetryAdvice()` punya loop retry. Gimana test-nya?

### Teknik 1: Counter di Closure

```go
func TestRetryAdvice_SuccessOnThirdAttempt(t *testing.T) {
    attempt := 0  // ← counter di luar mock

    client := &mockSMBClient{
        adviceFunc: func(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
            attempt++  // ← increment setiap kali dipanggil

            if attempt < 3 {
                // Attempt 1 dan 2: return pending
                return &contractsvc.PLNTokenAdviceResponse{
                    ResponseCode: "28",
                }, nil
            }
            // Attempt 3: return sukses
            return &contractsvc.PLNTokenAdviceResponse{
                ResponseCode: "00",
                Token:        "FINAL-TOKEN",
            }, nil
        },
    }

    // WaitDuration kecil supaya test cepat (10ms bukan 10s)
    retryCfg := &config.RetryConfig{
        MaxAttempts:  4,
        WaitDuration: 10 * time.Millisecond,  // ← 10ms, bukan 10 detik!
    }

    uc := NewUsecase(client, retryCfg, &mockLogger{})
    result := uc.RetryAdvice(context.Background(), "12345678901", "REF-003", "MSG012", 50000)

    if result.Status != "SUCCESS" {
        t.Errorf("expected SUCCESS, got %s", result.Status)
    }
    if attempt != 3 {
        t.Errorf("expected 3 attempts, got %d", attempt)
    }
}
```

### Teknik 2: Context Cancellation

```go
func TestRetryAdvice_ContextCancelled(t *testing.T) {
    client := &mockSMBClient{
        adviceFunc: func(ctx context.Context, req contractsvc.PLNTokenAdviceRequest) (*contractsvc.PLNTokenAdviceResponse, error) {
            return &contractsvc.PLNTokenAdviceResponse{ResponseCode: "28"}, nil
        },
    }

    ctx, cancel := context.WithCancel(context.Background())
    cancel()  // ← cancel LANGSUNG sebelum retry jalan

    retryCfg := &config.RetryConfig{MaxAttempts: 3, WaitDuration: 10 * time.Millisecond}
    uc := NewUsecase(client, retryCfg, &mockLogger{})
    result := uc.RetryAdvice(ctx, "12345678901", "REF-007", "MSG016", 50000)

    if result.Status != "FAILED" {
        t.Errorf("expected FAILED, got %s", result.Status)
    }
}
```

---

## LEVEL 7: JSON Roundtrip Test — "Cek Format Pesan"

Test bahwa struct bisa di-marshal ke JSON dan di-unmarshal balik dengan benar:

```go
func TestProviderPublishMessage_JSONFormat(t *testing.T) {
    // 1. Bikin message
    msg := NewProviderPublishMessage(ProviderPublishData{
        MsgID:      99,
        StatusToBe: "C",
        QueueName:  "q1",
    })

    // 2. Marshal ke JSON
    body, err := json.Marshal(msg)
    if err != nil {
        t.Fatalf("marshal error: %v", err)
    }

    // 3. Unmarshal balik
    var parsed ProviderPublishMessage
    if err := json.Unmarshal(body, &parsed); err != nil {
        t.Fatalf("unmarshal error: %v", err)
    }

    // 4. Cek hasilnya sama
    if parsed.Source != "PROVIDER" {
        t.Errorf("expected PROVIDER, got %s", parsed.Source)
    }
    if parsed.Data.MsgID != 99 {
        t.Errorf("expected 99, got %d", parsed.Data.MsgID)
    }

    // 5. Cek field names di JSON (bukan Go field names)
    var raw map[string]json.RawMessage
    json.Unmarshal(body, &raw)

    if _, ok := raw["source"]; !ok {
        t.Error("JSON missing 'source' field")
        //      ↑ Ini penting! Kalau json tag salah, field bisa hilang
    }
}
```

---

## CHEATSHEET: t.Errorf vs t.Fatalf vs t.Skip

| Method | Apa yang terjadi | Kapan pakai |
|--------|-----------------|-------------|
| `t.Errorf(...)` | Test FAIL, tapi **lanjut** ke assertion berikutnya | Cek banyak field sekaligus |
| `t.Fatalf(...)` | Test FAIL dan **STOP** | Error yang bikin assertion selanjutnya gak mungkin jalan (misal: nil pointer) |
| `t.Skip(...)` | Test di-**SKIP** (gak dijalanin) | Test yang butuh env tertentu |
| `t.Log(...)` | Print info (cuma muncul kalau `-v`) | Debugging |

### Contoh Kapan Pakai Fatalf vs Errorf

```go
func TestSomething(t *testing.T) {
    result, err := DoSomething()

    if err != nil {
        t.Fatalf("unexpected error: %v", err)
        // ↑ FATALF karena kalau error, result = nil
        //   assertion di bawah bakal panic (nil pointer)
    }

    // Kalau sampai sini, result pasti gak nil
    if result.Name != "expected" {
        t.Errorf("name: got %s, want expected", result.Name)
        // ↑ ERRORF karena masih bisa cek field lain
    }
    if result.Age != 25 {
        t.Errorf("age: got %d, want 25", result.Age)
    }
}
```

---

## CHEATSHEET: Cara Jalanin Test

```bash
# Semua test
go test ./...

# Verbose (lihat nama tiap test)
go test -v ./...

# Satu package
go test -v ./internal/usecase/plntoken/

# Satu function
go test -v -run TestProcessTransaction_InquirySuccess ./internal/usecase/plntoken/

# Pattern (semua test yang namanya mengandung "Retry")
go test -v -run Retry ./internal/usecase/plntoken/

# Dengan coverage percentage
go test -cover ./...

# Generate coverage HTML
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out -o coverage.html
open coverage.html

# Race detector (cek data race di goroutine)
go test -race ./...

# Timeout (kalau test hang)
go test -timeout 30s ./...
```

---

## LATIHAN: Coba Sendiri

### Latihan 1: Tambah Test Case di Table-Driven

Buka `internal/util/rc_test.go`, tambah case baru:
```go
{"failed 07", "07", 1},   // Layanan Sedang Gangguan
{"failed 1090", "1090", 1}, // Transaksi Cut-off
```

Jalanin: `go test -v -run TestResolveRCPPS ./internal/util/`

### Latihan 2: Bikin Test untuk Skenario Baru

Buka `internal/usecase/plntoken/pln_token_usecase_test.go`, tambah test:
```go
func TestProcessTransaction_InquiryPending(t *testing.T) {
    // Skenario: inquiry return code "28" (pending)
    // Expected: Status=FAILED (karena inquiry pending = gak bisa lanjut payment)
    //
    // Hint: bikin mockSMBClient dengan inquiryFunc return ResponseCode "28"
    // Cek result.Status dan result.StatusToBe
}
```

### Latihan 3: Bikin Test HTTP Client Baru

Buka `pkg/smb/pln_token_test.go`, tambah test:
```go
func TestPaymentPLNToken_Failed93(t *testing.T) {
    // Skenario: server return code "93" (Error Payment)
    // Hint: bikin httptest.NewServer yang return PaymentResponse dengan code "93"
    // Cek resp.ResponseCode == "93" dan resp.Data == nil
}
```
