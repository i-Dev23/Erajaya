package usecase

import (
	"context"
	"errors"
	"testing"

	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/utils"
)

// stubErrorMappingRepo is a simple stub for ErrorMessageMappingRepository.
type stubErrorMappingRepo struct {
	mapping *domain.ErrorMessageMapping
	err     error
}

func (s *stubErrorMappingRepo) GetMapping(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error) {
	return s.mapping, s.err
}

// mockLogger is a simple logger implementation for testing
type mockLogger struct{}

func (m *mockLogger) Info(msg string, args ...any)  {}
func (m *mockLogger) Error(msg string, args ...any) {}
func (m *mockLogger) Warn(msg string, args ...any)  {}
func (m *mockLogger) Debug(msg string, args ...any) {}

func TestResolveErrorMapping_DBFound(t *testing.T) {
	repo := &stubErrorMappingRepo{mapping: &domain.ErrorMessageMapping{ResponseCode: "20", Description: "", Found: true}}
	logger := &mockLogger{}

	code, message := resolveErrorMapping(context.Background(), "ultima", "any", repo, logger)

	rc, ok := utils.GetResponseCode("20")
	if !ok {
		t.Fatalf("response code 20 not found in static mapping")
	}
	if code != rc.Code || message != rc.Message {
		t.Fatalf("expected %s/%s, got %s/%s", rc.Code, rc.Message, code, message)
	}
}

func TestResolveErrorMapping_DBNotFound_Return99(t *testing.T) {
	repo := &stubErrorMappingRepo{mapping: &domain.ErrorMessageMapping{Found: false}}
	logger := &mockLogger{}

	code, message := resolveErrorMapping(context.Background(), "ultima", "unknown", repo, logger)

	if code != "99" {
		t.Fatalf("expected code 99 when DB mapping not found, got %s", code)
	}
	if message != "Other error" {
		t.Fatalf("expected 'Other error', got %s", message)
	}
}

func TestResolveErrorMapping_DBError_Return99(t *testing.T) {
	repo := &stubErrorMappingRepo{err: errors.New("db down")}
	logger := &mockLogger{}

	code, message := resolveErrorMapping(context.Background(), "oracle", "err", repo, logger)

	if code != "62" {
		t.Fatalf("expected code 62 when DB returns error, got %s", code)
	}
	if message != "Server error" {
		t.Fatalf("expected 'Server error', got %s", message)
	}
}

func TestResolveErrorMapping_DBInvalidCode_Return99(t *testing.T) {
	repo := &stubErrorMappingRepo{mapping: &domain.ErrorMessageMapping{ResponseCode: "999", Found: true}}
	logger := &mockLogger{}

	code, message := resolveErrorMapping(context.Background(), "oracle", "err", repo, logger)

	if code != "99" {
		t.Fatalf("expected code 99 when DB returns unknown response code, got %s", code)
	}
	if message != "Other error" {
		t.Fatalf("expected 'Other error', got %s", message)
	}
}
