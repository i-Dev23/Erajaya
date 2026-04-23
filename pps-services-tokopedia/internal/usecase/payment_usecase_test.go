package usecase

import (
	"context"
	"errors"
	"testing"

	"pps-services-tokopedia/internal/config"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/usecase/testmocks"

	"github.com/stretchr/testify/assert"
)

func TestPaymentUsecase_Payment(t *testing.T) {
	type fields struct {
		productRepo         *testmocks.MockProductRepository
		postgresPaymentRepo *testmocks.MockPostgresPaymentRepository
	}
	type args struct {
		ctx context.Context
		req *domain.PaymentRequestDomain
	}

	tests := []struct {
		name      string
		fields    fields
		args      args
		mockSetup func(*testmocks.MockProductRepository, *testmocks.MockPostgresPaymentRepository)
		wantResp  *domain.PaymentResponseDomain
		wantErr   error
	}{
		{
			name: "success flow",
			fields: fields{
				productRepo: &testmocks.MockProductRepository{
					GetProductByUserAndProductCodeFunc: func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
						return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
					},
					GetPriceByUserAndProductCodeFunc: func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
						return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
					},
					GetCutOffFunc: func(ctx context.Context) (*domain.CutOffDataResponseDomain, error) {
						return &domain.CutOffDataResponseDomain{
							OutErrCode:               "0",
							OutErrMsg:                "OK",
							CutOffTimeStartTokopedia: "2026-01-23T12:00:00Z",
							CutOffDurationTokopedia:  "3600",
							CutOffMessageTokopedia:   "No cut-off",
							CutOffTimeStart:          "2026-01-23T12:00:00Z",
							CutOffDuration:           "3600",
							CutOffMessage:            "No cut-off",
						}, nil
					},
				},
				postgresPaymentRepo: &testmocks.MockPostgresPaymentRepository{},
			},
			args: args{
				ctx: context.Background(),
				req: &domain.PaymentRequestDomain{
					RefID:            "REF-PAY-1",
					PartnerInquiryID: "INQ123",
					ClientNumber:     "C1",
					Category:         "cat",
					Rsid:             "TOKOPEDIA",
					ProductCode:      "P1",
					TotalAmount:      1000,
					Timestamp:        "2026-01-23 12:00:00",
					ClientIP:         "127.0.0.1",
				},
			},
			mockSetup: func(mpr *testmocks.MockProductRepository, mppr *testmocks.MockPostgresPaymentRepository) {
				mpr.GetProductByUserAndProductCodeFunc = func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
					return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
				}
				mpr.GetPriceByUserAndProductCodeFunc = func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
					return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
				}
			},
			wantResp: &domain.PaymentResponseDomain{ResponseCode: "01", Message: "On process"},
			wantErr:  nil,
		},
		{
			name: "product repo error",
			fields: fields{
				productRepo: &testmocks.MockProductRepository{
					GetProductByUserAndProductCodeFunc: func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
						return nil, errors.New("db error")
					},
					GetCutOffFunc: func(ctx context.Context) (*domain.CutOffDataResponseDomain, error) {
						return &domain.CutOffDataResponseDomain{
							OutErrCode:               "0",
							OutErrMsg:                "OK",
							CutOffTimeStartTokopedia: "2026-01-23T12:00:00Z",
							CutOffDurationTokopedia:  "3600",
							CutOffMessageTokopedia:   "No cut-off",
							CutOffTimeStart:          "2026-01-23T12:00:00Z",
							CutOffDuration:           "3600",
							CutOffMessage:            "No cut-off",
						}, nil
					},
				},
				postgresPaymentRepo: &testmocks.MockPostgresPaymentRepository{},
			},
			args: args{
				ctx: context.Background(),
				req: &domain.PaymentRequestDomain{
					RefID:            "REF-PAY-2",
					PartnerInquiryID: "INQ123",
					ClientNumber:     "C1",
					Category:         "cat",
					Rsid:             "TOKOPEDIA",
					ProductCode:      "P1",
					TotalAmount:      1000,
					Timestamp:        "2026-01-23 12:00:00",
				},
			},
			mockSetup: func(mpr *testmocks.MockProductRepository, mppr *testmocks.MockPostgresPaymentRepository) {
				mpr.GetProductByUserAndProductCodeFunc = func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
					return nil, errors.New("db error")
				}
			},
			wantResp: &domain.PaymentResponseDomain{ResponseCode: "62", Message: "Server error"},
			wantErr:  nil,
		},
		{
			name: "missing mandatory parameters",
			fields: fields{
				productRepo: &testmocks.MockProductRepository{
					GetCutOffFunc: func(ctx context.Context) (*domain.CutOffDataResponseDomain, error) {
						return &domain.CutOffDataResponseDomain{
							OutErrCode:               "0",
							OutErrMsg:                "OK",
							CutOffTimeStartTokopedia: "2026-01-23T12:00:00Z",
							CutOffDurationTokopedia:  "3600",
							CutOffMessageTokopedia:   "No cut-off",
							CutOffTimeStart:          "2026-01-23T12:00:00Z",
							CutOffDuration:           "3600",
							CutOffMessage:            "No cut-off",
						}, nil
					},
				},
				postgresPaymentRepo: &testmocks.MockPostgresPaymentRepository{},
			},
			args: args{
				ctx: context.Background(),
				req: &domain.PaymentRequestDomain{},
			},
			mockSetup: nil,
			wantResp:  &domain.PaymentResponseDomain{ResponseCode: "42", Message: "Invalid parameter"},
			wantErr:   nil,
		},
		{
			name: "success flow with cut off",
			fields: fields{
				productRepo: &testmocks.MockProductRepository{
					GetProductByUserAndProductCodeFunc: func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
						return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
					},
					GetPriceByUserAndProductCodeFunc: func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
						return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
					},
					GetCutOffFunc: func(ctx context.Context) (*domain.CutOffDataResponseDomain, error) {
						return &domain.CutOffDataResponseDomain{
							OutErrCode:               "0",
							OutErrMsg:                "OK",
							CutOffTimeStartTokopedia: "2026-01-23T12:00:00Z",
							CutOffDurationTokopedia:  "3600",
							CutOffMessageTokopedia:   "No cut-off",
							CutOffTimeStart:          "2026-01-23T12:00:00Z",
							CutOffDuration:           "3600",
							CutOffMessage:            "No cut-off",
						}, nil
					},
				},
				postgresPaymentRepo: &testmocks.MockPostgresPaymentRepository{},
			},
			args: args{
				ctx: context.Background(),
				req: &domain.PaymentRequestDomain{
					RefID:            "REF-PAY-3",
					PartnerInquiryID: "INQ123",
					ClientNumber:     "C1",
					Category:         "cat",
					Rsid:             "TOKOPEDIA",
					ProductCode:      "P1",
					TotalAmount:      1000,
					Timestamp:        "2026-01-23 12:00:00",
					ClientIP:         "127.0.0.1",
				},
			},
			mockSetup: func(mpr *testmocks.MockProductRepository, mppr *testmocks.MockPostgresPaymentRepository) {
				mpr.GetProductByUserAndProductCodeFunc = func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
					return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
				}
				mpr.GetPriceByUserAndProductCodeFunc = func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
					return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
				}
			},
			wantResp: &domain.PaymentResponseDomain{ResponseCode: "01", Message: "On process"},
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.mockSetup != nil {
				tt.mockSetup(tt.fields.productRepo, tt.fields.postgresPaymentRepo)
			}
			// Add all required mocks for NewPaymentUsecase
			mockConfig := &config.Config{TPClientID: "test-client", TPClientSecret: "test-secret"}
			mockPreorderRepo := &testmocks.MockPreorderRepository{}
			mockCutOffRepo := &testmocks.MockCutOffRepository{
				CutOffFunc: func(ctx context.Context, flag string) (*domain.CutOffResponseDomain, error) {
					return &domain.CutOffResponseDomain{OutErrCode: "0", OutMsgErr: "OK"}, nil
				},
			}
			mockPostgresInquiryRepo := &testmocks.MockInquiryRepository{}
			mockErrorMappingRepo := &testmocks.MockErrorMappingRepository{}
			mockRabbitMQService := &testmocks.MockRabbitMQService{}
			u := NewPaymentUsecase(
				mockConfig,
				&testmocks.MockLogger{},
				&testmocks.MockRedisClient{},
				tt.fields.productRepo,
				mockPreorderRepo,
				mockCutOffRepo,
				mockPostgresInquiryRepo,
				tt.fields.postgresPaymentRepo,
				mockErrorMappingRepo,
				mockRabbitMQService,
			)
			resp, err := u.Payment(tt.args.ctx, tt.args.req)
			if tt.wantErr != nil {
				assert.EqualError(t, err, tt.wantErr.Error())
			} else {
				assert.NoError(t, err)
			}
			if tt.wantResp != nil {
				assert.Equal(t, tt.wantResp.ResponseCode, resp.ResponseCode)
				assert.Equal(t, tt.wantResp.Message, resp.Message)
			} else {
				assert.Nil(t, resp)
			}
		})
	}
}

func TestPaymentUsecase_ProcessPayment(t *testing.T) {
	t.Run("success", func(t *testing.T) {
		// setup mocks
		// call ProcessPayment
		// assert results
	})
	t.Run("failure", func(t *testing.T) {
		// setup mocks to return error
		// call ProcessPayment
		// assert results
	})
}
