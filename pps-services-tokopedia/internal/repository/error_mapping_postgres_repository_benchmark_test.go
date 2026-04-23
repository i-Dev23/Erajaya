package repository

import (
	"context"
	"database/sql"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

// BenchmarkMockPostgresService for testing error mapping
type BenchmarkMockPostgresService struct {
	queryCount int
}

func (m *BenchmarkMockPostgresService) Query(ctx context.Context, query string, args ...any) (pgx.Rows, error) {
	m.queryCount++
	return &BenchmarkMockRows{found: true, responseCode: "14", description: "Auto-inserted"}, nil
}

func (m *BenchmarkMockPostgresService) Exec(ctx context.Context, query string, args ...any) (pgconn.CommandTag, error) {
	return pgconn.NewCommandTag("INSERT 0 1"), nil
}

func (m *BenchmarkMockPostgresService) Ping(ctx context.Context) error {
	return nil
}

func (m *BenchmarkMockPostgresService) IsAvailable() bool {
	return true
}

func (m *BenchmarkMockPostgresService) Close() {}

type BenchmarkMockRows struct {
	found        bool
	responseCode string
	description  string
	nextCount    int
	closed       bool
}

func (m *BenchmarkMockRows) Next() bool {
	if m.closed {
		return false
	}
	if m.nextCount == 0 {
		m.nextCount++
		return true
	}
	return false
}

func (m *BenchmarkMockRows) Scan(dest ...interface{}) error {
	if len(dest) >= 3 {
		// Support *sql.NullString and *string
		switch v := dest[0].(type) {
		case *string:
			*v = m.responseCode
		case *sql.NullString:
			v.String = m.responseCode
			v.Valid = true
		}
		switch v := dest[1].(type) {
		case *string:
			*v = m.description
		case *sql.NullString:
			v.String = m.description
			v.Valid = true
		}
		switch v := dest[2].(type) {
		case *bool:
			*v = m.found
		}
	}
	return nil
}

func (m *BenchmarkMockRows) Err() error {
	return nil
}

func (m *BenchmarkMockRows) Close() {
	m.closed = true
}

func (m *BenchmarkMockRows) CommandTag() pgconn.CommandTag {
	return pgconn.NewCommandTag("SELECT 1")
}

func (m *BenchmarkMockRows) FieldDescriptions() []pgconn.FieldDescription {
	return []pgconn.FieldDescription{}
}

func (m *BenchmarkMockRows) Values() ([]any, error) {
	return []any{m.responseCode, m.description, m.found}, nil
}

func (m *BenchmarkMockRows) RawValues() [][]byte {
	return [][]byte{}
}

func (m *BenchmarkMockRows) Conn() *pgx.Conn {
	return nil
}

// mockLogger is a simple logger implementation for testing
type mockLogger struct{}

func (m *mockLogger) Info(msg string, args ...any)  {}
func (m *mockLogger) Error(msg string, args ...any) {}
func (m *mockLogger) Warn(msg string, args ...any)  {}
func (m *mockLogger) Debug(msg string, args ...any) {}

// BenchmarkErrorMappingRepository_GetMapping_ExactMatch benchmarks exact match scenario
// Expected: < 1ms per query (with index)
func BenchmarkErrorMappingRepository_GetMapping_ExactMatch(b *testing.B) {
	mockPostgres := &BenchmarkMockPostgresService{}
	mockLoggerImpl := &mockLogger{}
	repo := NewErrorMappingPostgresRepository(mockPostgres, mockLoggerImpl)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = repo.GetMapping(ctx, "ultima", "yang anda masukkan salah")
	}

	b.Logf("Total queries: %d | Avg per op: %.2f ns/op", mockPostgres.queryCount, float64(b.Elapsed().Nanoseconds())/float64(b.N))
}

// BenchmarkErrorMappingRepository_GetMapping_PartialMatch benchmarks partial match scenario
// Expected: < 2ms per query (with LIKE pattern)
func BenchmarkErrorMappingRepository_GetMapping_PartialMatch(b *testing.B) {
	mockPostgres := &BenchmarkMockPostgresService{}
	mockLoggerImpl := &mockLogger{}
	repo := NewErrorMappingPostgresRepository(mockPostgres, mockLoggerImpl)
	ctx := context.Background()

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		_, _ = repo.GetMapping(ctx, "oracle", "error 02 : signature tidak benar")
	}

	b.Logf("Total queries: %d | Avg per op: %.2f ns/op", mockPostgres.queryCount, float64(b.Elapsed().Nanoseconds())/float64(b.N))
}

// BenchmarkErrorMappingRepository_GetMapping_Parallel simulates concurrent queries
// Expected: handle 300+ concurrent queries per second
func BenchmarkErrorMappingRepository_GetMapping_Parallel(b *testing.B) {
	mockPostgres := &BenchmarkMockPostgresService{}
	mockLoggerImpl := &mockLogger{}
	repo := NewErrorMappingPostgresRepository(mockPostgres, mockLoggerImpl)
	ctx := context.Background()

	patterns := []string{
		"yang anda masukkan salah",
		"kwh melebihi batas maksimum",
		"no payment",
		"failed system (timeout)",
		"sistem sedang kendala",
		"cut-off",
	}

	b.ResetTimer()
	b.ReportAllocs()

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			pattern := patterns[i%len(patterns)]
			_, _ = repo.GetMapping(ctx, "ultima", pattern)
			i++
		}
	})

	b.Logf("Total queries: %d | Avg per op: %.2f ns/op", mockPostgres.queryCount, float64(b.Elapsed().Nanoseconds())/float64(b.N))
}

// TestErrorMappingRepository_GetMapping validates mapping results
func TestErrorMappingRepository_GetMapping_Mock(t *testing.T) {
	mockPostgres := &BenchmarkMockPostgresService{}
	mockLoggerImpl := &mockLogger{}
	repo := NewErrorMappingPostgresRepository(mockPostgres, mockLoggerImpl)
	ctx := context.Background()

	result, err := repo.GetMapping(ctx, "ultima", "yang anda masukkan salah")

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if !result.Found {
		t.Fatalf("Expected found=true, got false")
	}

	if result.ResponseCode != "14" {
		t.Fatalf("Expected response_code=14, got %s", result.ResponseCode)
	}

	if mockPostgres.queryCount == 0 {
		t.Fatalf("Expected database query to be called")
	}

	t.Logf("✓ Query executed %d times", mockPostgres.queryCount)
	t.Logf("✓ Result: found=%v, code=%s, description=%s", result.Found, result.ResponseCode, result.Description)
}
