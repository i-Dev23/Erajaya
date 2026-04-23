package usecase

import (
	"context"
	"errors"
	"testing"

	"pps-services-tokopedia/internal/domain"

	"github.com/stretchr/testify/assert"
)

// mockLogger implements service.Logger with no-op methods for testing
type mockStatusLogger struct{}

func (m *mockStatusLogger) Info(msg string, keysAndValues ...interface{})  {}
func (m *mockStatusLogger) Warn(msg string, keysAndValues ...interface{})  {}
func (m *mockStatusLogger) Error(msg string, keysAndValues ...interface{}) {}
func (m *mockStatusLogger) Debug(msg string, keysAndValues ...interface{}) {}

func TestCheckStatusUsecase_CheckStatus(t *testing.T) {
	type fields struct {
		postgresPaymentRepo *mockPostgresPaymentRepository
	}
	type args struct {
		ctx context.Context
		req *domain.CheckStatusRequestDomain
	}

	tests := []struct {
		name      string
		fields    fields
		args      args
		mockSetup func(*mockPostgresPaymentRepository)
		wantResp  *domain.CheckStatusResponseDomain
		wantErr   error
	}{
		{
			name:   "missing mandatory parameters",
			fields: fields{postgresPaymentRepo: &mockPostgresPaymentRepository{}},
			args: args{
				ctx: context.Background(),
				req: &domain.CheckStatusRequestDomain{},
			},
			mockSetup: nil,
			wantResp: &domain.CheckStatusResponseDomain{
				RefID:        "",
				ResponseCode: "42",
				Message:      "Invalid parameter",
			},
			wantErr: nil,
		},
		{
			name: "payment found",
			fields: fields{postgresPaymentRepo: &mockPostgresPaymentRepository{
				FindByRefIDFunc: func(ctx context.Context, refID string) (*domain.PaymentResponseDomain, error) {
					return &domain.PaymentResponseDomain{RefID: refID, ResponseCode: "00", Message: "OK"}, nil
				},
				GetPaymentStatusByRefIDFunc: func(ctx context.Context, refID string) (*domain.PaymentStatusResult, error) {
					return &domain.PaymentStatusResult{RefID: refID, ResponseCode: "00", Message: "OK"}, nil
				},
			}},
			args: args{
				ctx: context.Background(),
				req: &domain.CheckStatusRequestDomain{RefID: "abc", Timestamp: "2026-01-23 12:00:00", Category: "cat"},
			},
			mockSetup: nil,
			wantResp:  &domain.CheckStatusResponseDomain{RefID: "abc", ResponseCode: "00", Message: "OK"},
			wantErr:   nil,
		},
		{
			name: "payment not found",
			fields: fields{postgresPaymentRepo: &mockPostgresPaymentRepository{
				FindByRefIDFunc: func(ctx context.Context, refID string) (*domain.PaymentResponseDomain, error) {
					return nil, nil
				},
				GetPaymentStatusByRefIDFunc: func(ctx context.Context, refID string) (*domain.PaymentStatusResult, error) {
					return nil, errors.New("payment not found for ref_id: notfound")
				},
			}},
			args: args{
				ctx: context.Background(),
				req: &domain.CheckStatusRequestDomain{RefID: "notfound", Timestamp: "2026-01-23 12:00:00", Category: "cat"},
			},
			mockSetup: nil,
			wantResp:  &domain.CheckStatusResponseDomain{RefID: "notfound", ResponseCode: "12", Message: "Transaction not found"},
			wantErr:   nil,
		},
		{
			name: "repository error",
			fields: fields{postgresPaymentRepo: &mockPostgresPaymentRepository{
				FindByRefIDFunc: func(ctx context.Context, refID string) (*domain.PaymentResponseDomain, error) {
					return nil, errors.New("db error")
				},
				GetPaymentStatusByRefIDFunc: func(ctx context.Context, refID string) (*domain.PaymentStatusResult, error) {
					return nil, errors.New("db error")
				},
			}},
			args: args{
				ctx: context.Background(),
				req: &domain.CheckStatusRequestDomain{RefID: "err", Timestamp: "2026-01-23 12:00:00", Category: "cat"},
			},
			mockSetup: nil,
			wantResp:  &domain.CheckStatusResponseDomain{RefID: "err", ResponseCode: "62", Message: "Server error"},
			wantErr:   nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup(tt.fields.postgresPaymentRepo)
			}
			u := &checkStatusUsecaseImpl{
				logger:              &mockStatusLogger{},
				postgresPaymentRepo: tt.fields.postgresPaymentRepo,
			}
			resp, err := u.CheckStatus(tt.args.ctx, tt.args.req)
			if tt.wantErr != nil {
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
			if tt.wantResp != nil {
				if resp != nil {
					assert.Equal(t, tt.wantResp.RefID, resp.RefID)
					assert.Equal(t, tt.wantResp.ResponseCode, resp.ResponseCode)
					assert.Equal(t, tt.wantResp.Message, resp.Message)
				} else {
					assert.Fail(t, "expected non-nil response, got nil")
				}
			} else {
				assert.Nil(t, resp)
			}
		})
	}
}
