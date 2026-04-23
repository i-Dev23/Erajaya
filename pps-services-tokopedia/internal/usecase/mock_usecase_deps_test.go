package usecase

import (
	"context"
	"fmt"
	"pps-services-tokopedia/internal/domain"
	"time"

	"github.com/rabbitmq/amqp091-go"
	"github.com/redis/go-redis/v9"
)

// --- RedisClient ---
type mockRedisClient struct {
	store map[string]string
}

func (m *mockRedisClient) ensureStore() {
	if m.store == nil {
		m.store = make(map[string]string)
	}
}

func (m *mockRedisClient) Ping(ctx context.Context) *redis.StatusCmd {
	return redis.NewStatusResult("PONG", nil)
}

func (m *mockRedisClient) Set(ctx context.Context, key string, value interface{}, expiration time.Duration) *redis.StatusCmd {
	m.ensureStore()
	switch v := value.(type) {
	case []byte:
		m.store[key] = string(v)
	default:
		m.store[key] = fmt.Sprint(v)
	}
	return redis.NewStatusResult("OK", nil)
}

func (m *mockRedisClient) Del(ctx context.Context, keys ...string) *redis.IntCmd {
	m.ensureStore()
	deleted := int64(0)
	for _, k := range keys {
		if _, ok := m.store[k]; ok {
			delete(m.store, k)
			deleted++
		}
	}
	return redis.NewIntResult(deleted, nil)
}

func (m *mockRedisClient) Incr(ctx context.Context, key string) *redis.IntCmd {
	m.ensureStore()
	m.store[key] = fmt.Sprint(int64(1))
	return redis.NewIntResult(1, nil)
}

func (m *mockRedisClient) Expire(ctx context.Context, key string, expiration time.Duration) *redis.BoolCmd {
	m.ensureStore()
	_, ok := m.store[key]
	return redis.NewBoolResult(ok, nil)
}

func (m *mockRedisClient) Get(ctx context.Context, key string) *redis.StringCmd {
	m.ensureStore()
	if val, ok := m.store[key]; ok {
		return redis.NewStringResult(val, nil)
	}
	return redis.NewStringResult("", redis.Nil)
}

// --- ProductRepository ---
type mockProductRepository struct {
	GetPriceFunc                       func(ctx context.Context, productCode string) (float64, error)
	GetProductByUserAndProductCodeFunc func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error)
	GetPriceByUserAndProductCodeFunc   func(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error)
	GetProductByUserFunc               func(ctx context.Context, username string) (*[]domain.ProductPriceResponseDomain, error)
}

func (m *mockProductRepository) GetPrice(ctx context.Context, productCode string) (float64, error) {
	if m.GetPriceFunc != nil {
		return m.GetPriceFunc(ctx, productCode)
	}
	return 1000, nil
}

// Implement all other methods as stubs
func (m *mockProductRepository) GetPriceByUser(ctx context.Context, username string, provider string) (*[]domain.ProductPriceResponseDomain, error) {
	return nil, nil
}

func (m *mockProductRepository) GetPriceByUserAndProductCode(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
	if m.GetPriceByUserAndProductCodeFunc != nil {
		return m.GetPriceByUserAndProductCodeFunc(ctx, username, productCode)
	}
	return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
}

func (m *mockProductRepository) GetProductByUser(ctx context.Context, username string) (*[]domain.ProductPriceResponseDomain, error) {
	if m.GetProductByUserFunc != nil {
		return m.GetProductByUserFunc(ctx, username)
	}
	products := []domain.ProductPriceResponseDomain{{Price: 1000, Status: "00", OuterRCode: "0"}}
	return &products, nil
}

func (m *mockProductRepository) GetProductByUserAndProductCode(ctx context.Context, username string, productCode string) (*domain.ProductPriceResponseDomain, error) {
	if m.GetProductByUserAndProductCodeFunc != nil {
		return m.GetProductByUserAndProductCodeFunc(ctx, username, productCode)
	}
	return &domain.ProductPriceResponseDomain{Price: 1000, Status: "00", OuterRCode: "0"}, nil
}
func (m *mockProductRepository) GetIpByUser(ctx context.Context, username string) (*domain.WhitelistedIpResponseDomain, error) {
	return nil, nil
}
func (m *mockProductRepository) GetCutOff(ctx context.Context) (*domain.CutOffDataResponseDomain, error) {
	return &domain.CutOffDataResponseDomain{OutErrCode: "0", OutErrMsg: "OK"}, nil
}

// --- PostgresInquiryRepository ---
type mockPostgresInquiryRepository struct {
	SaveFunc func(ctx context.Context, req *domain.InquiryRequestDomain) error
}

func (m *mockPostgresInquiryRepository) Save(ctx context.Context, req *domain.InquiryRequestDomain) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, req)
	}
	return nil
}

// Implement all other methods as stubs
func (m *mockPostgresInquiryRepository) InsertBillDetail(ctx context.Context, req *domain.BillDetailInsertRequest) (*domain.BillDetailInsertResponse, error) {
	return &domain.BillDetailInsertResponse{BillDetailID: 1}, nil
}
func (m *mockPostgresInquiryRepository) InsertInquiryRequest(ctx context.Context, req *domain.InquiryRequestInsertRequest) (*domain.InquiryRequestInsertResponse, error) {
	return &domain.InquiryRequestInsertResponse{InquiryRequestID: 1}, nil
}
func (m *mockPostgresInquiryRepository) InsertInquiryResponse(ctx context.Context, req *domain.InquiryResponseInsertRequest) (*domain.InquiryResponseInsertResponse, error) {
	return &domain.InquiryResponseInsertResponse{InquiryResponseID: 1}, nil
}
func (m *mockPostgresInquiryRepository) ValidateInquiryId(ctx context.Context, inquiryRequestId string, productCode string, clientNumber string) error {
	return nil
}
func (m *mockPostgresInquiryRepository) CheckRefIDExists(ctx context.Context, refID string) (bool, error) {
	return false, nil
}
func (m *mockPostgresInquiryRepository) GetBillDetailsByInquiryID(ctx context.Context, ppsInquiryID string) ([]domain.InquiryBillDetail, error) {
	return nil, nil
}
func (m *mockPostgresInquiryRepository) GetInquiryByPartnerInquiryID(ctx context.Context, partnerInquiryID string) (*domain.InquiryData, error) {
	return nil, nil
}

// --- PostgresPaymentRepository ---
type mockPostgresPaymentRepository struct {
	SaveFunc                    func(ctx context.Context, req *domain.PaymentRequestDomain) error
	FindByRefIDFunc             func(ctx context.Context, refID string) (*domain.PaymentResponseDomain, error)
	GetPaymentStatusByRefIDFunc func(ctx context.Context, refID string) (*domain.PaymentStatusResult, error)
}

func (m *mockPostgresPaymentRepository) Save(ctx context.Context, req *domain.PaymentRequestDomain) error {
	if m.SaveFunc != nil {
		return m.SaveFunc(ctx, req)
	}
	return nil
}
func (m *mockPostgresPaymentRepository) FindByRefID(ctx context.Context, refID string) (*domain.PaymentResponseDomain, error) {
	if m.FindByRefIDFunc != nil {
		return m.FindByRefIDFunc(ctx, refID)
	}
	return nil, nil
}

// Implement all other methods as stubs
func (m *mockPostgresPaymentRepository) InsertPaymentBillDetail(ctx context.Context, req *domain.PaymentBillDetailInsertRequest) (*domain.PaymentBillDetailInsertResponse, error) {
	return &domain.PaymentBillDetailInsertResponse{PaymentBillDetailID: 1}, nil
}
func (m *mockPostgresPaymentRepository) InsertPaymentRequest(ctx context.Context, req *domain.PaymentRequestInsertRequest) (*domain.PaymentRequestInsertResponse, error) {
	return &domain.PaymentRequestInsertResponse{PaymentRequestID: 1}, nil
}
func (m *mockPostgresPaymentRepository) InsertPaymentResponse(ctx context.Context, req *domain.PaymentResponseInsertRequest) (*domain.PaymentResponseInsertResponse, error) {
	return &domain.PaymentResponseInsertResponse{PaymentResponseID: 1}, nil
}
func (m *mockPostgresPaymentRepository) GetPaymentStatusByRefID(ctx context.Context, refID string) (*domain.PaymentStatusResult, error) {
	if m.GetPaymentStatusByRefIDFunc != nil {
		return m.GetPaymentStatusByRefIDFunc(ctx, refID)
	}
	return nil, nil
}
func (m *mockPostgresPaymentRepository) CheckRefIDExists(ctx context.Context, refID string) (bool, error) {
	return false, nil
}
func (m *mockPostgresPaymentRepository) CheckPartnerInquiryIDExists(ctx context.Context, partnerInquiryID string) (bool, error) {
	return false, nil
}
func (m *mockPostgresPaymentRepository) ValidatePartnerInquiryID(ctx context.Context, partnerInquiryID string) error {
	return nil
}

// --- UltimaService ---
type mockUltimaService struct {
	CheckInquiryFunc     func(ctx context.Context, req *domain.InquiryRequestDomain) (*domain.InquiryResponseDomain, error)
	CheckIdPlnUltimaFunc func(ctx context.Context, req *domain.UltimaCheckIdPlnRequestDomain) (*domain.UltimaBaseResponseDomain, *domain.PLNTransactionInquiry, error)
}

func (m *mockUltimaService) CheckInquiry(ctx context.Context, req *domain.InquiryRequestDomain) (*domain.InquiryResponseDomain, error) {
	if m.CheckInquiryFunc != nil {
		return m.CheckInquiryFunc(ctx, req)
	}
	return nil, nil
}

// Add stub for interface compliance
func (m *mockUltimaService) CheckIdPlnUltima(ctx context.Context, req *domain.UltimaCheckIdPlnRequestDomain) (*domain.UltimaBaseResponseDomain, *domain.PLNTransactionInquiry, error) {
	if m.CheckIdPlnUltimaFunc != nil {
		return m.CheckIdPlnUltimaFunc(ctx, req)
	}
	return nil, nil, nil
}

func (m *mockUltimaService) Ping(ctx context.Context) error {
	return nil
}

// --- OracleService ---
type mockOracleService struct{}

func (m *mockOracleService) Query(ctx context.Context, query string, args ...any) (interface{}, error) {
	return nil, nil
}
func (m *mockOracleService) Exec(ctx context.Context, query string, args ...any) (interface{}, error) {
	return nil, nil
}
func (m *mockOracleService) Ping(ctx context.Context) error { return nil }
func (m *mockOracleService) Close() error                   { return nil }

// --- RedisClient ---

// Add other methods as needed for full interface compliance

// --- Add other mocks as needed for other interfaces ---
type mockCutOffRepository struct{}

func (m *mockCutOffRepository) CutOff(ctx context.Context, flag string) (*domain.CutOffResponseDomain, error) {
	return &domain.CutOffResponseDomain{OutErrCode: "00", OutMsgErr: "OK"}, nil
}

// --- PreorderRepository ---
type mockPreorderRepository struct{}

func (m *mockPreorderRepository) Preorder(ctx context.Context, req *domain.PreorderRequestDomain) (*domain.PreorderResponseDomain, error) {
	return &domain.PreorderResponseDomain{OuterRCode: 0, OuterRMsg: "OK", ServerId: "S123"}, nil
}
func (m *mockPreorderRepository) UpdatePreorderStatus(ctx context.Context, msgid string, status string, message string) (*domain.UpdatePreorderStatusResponseDomain, error) {
	return &domain.UpdatePreorderStatusResponseDomain{OuterRCode: 0, OuterRMsg: "OK"}, nil
}

// --- RabbitMQService ---
type mockRabbitMQService struct{}

func (m *mockRabbitMQService) Publish(ctx context.Context, exchange, routingKey string, body []byte, headers amqp091.Table) error {
	return nil
}
func (m *mockRabbitMQService) Consume(ctx context.Context, queueName string) (<-chan amqp091.Delivery, error) {
	return nil, nil
}
func (m *mockRabbitMQService) Ping(ctx context.Context) error { return nil }
func (m *mockRabbitMQService) Close() error                   { return nil }

// --- ErrorMappingRepository ---
type mockErrorMappingRepository struct{}

func (m *mockErrorMappingRepository) GetMapping(ctx context.Context, systemType string, errorMessage string) (*domain.ErrorMessageMapping, error) {
	return &domain.ErrorMessageMapping{ResponseCode: "00", Description: "OK", Found: true}, nil
}
