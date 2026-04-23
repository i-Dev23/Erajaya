package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"pps-services-tokopedia/internal/config"
	"pps-services-tokopedia/internal/domain"
	"pps-services-tokopedia/internal/usecase/testmocks"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestInquiryUsecase_Inquiry(t *testing.T) {
	type fields struct {
		productRepo  *mockProductRepository
		ultimaSvc    *mockUltimaService
		postgresRepo *mockPostgresInquiryRepository
	}
	type args struct {
		ctx context.Context
		req *domain.InquiryRequestDomain
	}

	tests := []struct {
		name      string
		fields    fields
		args      args
		mockSetup func(*mockProductRepository, *mockUltimaService, *mockPostgresInquiryRepository)
		wantResp  *domain.InquiryResponseDomain
		wantErr   error
	}{
		{
			name: "success flow",
			fields: fields{
				productRepo:  &mockProductRepository{},
				ultimaSvc:    &mockUltimaService{},
				postgresRepo: &mockPostgresInquiryRepository{},
			},
			args: args{
				ctx: context.Background(),
				req: &domain.InquiryRequestDomain{
					RefID:        "REF123",
					ClientNumber: "C1",
					Category:     "cat",
					Rsid:         "TOKOPEDIA",
					ProductCode:  "P1",
					Timestamp:    "2026-01-23 12:00:00",
				},
			},
			mockSetup: func(mpr *mockProductRepository, mus *mockUltimaService, mir *mockPostgresInquiryRepository) {
				mpr.GetProductByUserAndProductCodeFunc = func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
					return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
				}
			},
			wantResp: &domain.InquiryResponseDomain{ResponseCode: "00", Message: "Success"},
			wantErr:  nil,
		},
		{
			name: "product repo error",
			fields: fields{
				productRepo:  &mockProductRepository{},
				ultimaSvc:    &mockUltimaService{},
				postgresRepo: &mockPostgresInquiryRepository{},
			},
			args: args{
				ctx: context.Background(),
				req: &domain.InquiryRequestDomain{
					RefID:        "REF123",
					ClientNumber: "C1",
					Category:     "cat",
					Rsid:         "TOKOPEDIA",
					ProductCode:  "P1",
					Timestamp:    "2026-01-23 12:00:00",
				},
			},
			mockSetup: func(mpr *mockProductRepository, mus *mockUltimaService, mir *mockPostgresInquiryRepository) {
				mpr.GetProductByUserAndProductCodeFunc = func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
					return nil, errors.New("db error")
				}
			},
			wantResp: &domain.InquiryResponseDomain{ResponseCode: "62", Message: "Server error"},
			wantErr:  errors.New("db error"),
		},
		{
			name: "pln cache missing mandatory client number",
			fields: fields{
				productRepo:  &mockProductRepository{},
				ultimaSvc:    &mockUltimaService{},
				postgresRepo: &mockPostgresInquiryRepository{},
			},
			args: args{
				ctx: context.Background(),
				req: &domain.InquiryRequestDomain{
					RefID:       "REF123",
					Category:    "cat",
					Rsid:        "TOKOPEDIA",
					ProductCode: "P1",
					Timestamp:   "2026-01-23 12:00:00",
				},
			},
			mockSetup: func(mpr *mockProductRepository, mus *mockUltimaService, mir *mockPostgresInquiryRepository) {
				mpr.GetProductByUserAndProductCodeFunc = func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
					return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
				}
			},
			wantResp: &domain.InquiryResponseDomain{ResponseCode: "42", Message: "Invalid parameter"},
			wantErr:  nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cachedPLN, _ := json.Marshal(domain.PLNTransactionInquiry{ // ensure PLN data available when client number present
				Name:        "Test User",
				MeterNumber: "123",
				IDPelanggan: "456",
				TarifDaya:   "R1",
			})
			productCache := `{"kode_voucher":"KV1","price":1000,"status":"00"}`
			store := map[string]string{
				"C1": string(cachedPLN),
			}
			if tt.name != "product repo error" {
				store["product_with_status:P1"] = productCache
			}
			redisClient := &mockRedisClient{store: store}
			if tt.mockSetup != nil {
				tt.mockSetup(tt.fields.productRepo, tt.fields.ultimaSvc, tt.fields.postgresRepo)
			}
			mockConfig := &config.Config{TPClientID: "test-client"}
			u := NewInquiryUsecase(
				mockConfig,
				&mockStatusLogger{},
				redisClient,
				tt.fields.productRepo,
				tt.fields.ultimaSvc,
				tt.fields.postgresRepo,
				&mockCutOffRepository{}, // cutOffRepo
				nil,                     // errorMappingRepo
			).(*inquiryUsecaseImpl)
			resp, err := u.Inquiry(tt.args.ctx, tt.args.req)
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

func TestInquiryUsecase_ProcessInquiry(t *testing.T) {
	mockProductRepo := &testmocks.MockProductRepository{}
	mockUltimaService := &testmocks.MockUltimaService{}
	mockInquiryRepo := &testmocks.MockInquiryRepository{}
	mockLogger := &testmocks.MockLogger{}
	mockRedisClient := &testmocks.MockRedisClient{}
	mockCutOffRepo := &testmocks.MockCutOffRepository{}
	cfg := config.Config{ /* fill with test config values as needed */ }
	u := NewInquiryUsecase(
		&cfg,
		mockLogger,
		mockRedisClient,
		mockProductRepo,
		mockUltimaService,
		mockInquiryRepo,
		mockCutOffRepo,
		nil,
	)
	_ = u
	// TODO: add your test cases and assertions here
}
