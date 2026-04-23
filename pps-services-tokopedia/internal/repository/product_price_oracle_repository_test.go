package repository

import (
	"context"
	"database/sql"
	"regexp"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"

	"pps-services-tokopedia/internal/domain"
)

// mockOracleService implements service.OracleService using sqlmock DB.
type mockOracleService struct {
	db *sql.DB
}

func (m *mockOracleService) Query(ctx context.Context, query string, args ...any) (*sql.Rows, error) {
	return m.db.QueryContext(ctx, query, args...)
}

func (m *mockOracleService) Exec(ctx context.Context, query string, args ...any) (sql.Result, error) {
	return m.db.ExecContext(ctx, query, args...)
}

func (m *mockOracleService) Ping(ctx context.Context) error { return m.db.PingContext(ctx) }
func (m *mockOracleService) Close() error                   { return m.db.Close() }

// testLogger is a no-op logger for testing.
type testLogger struct{}

func (l *testLogger) Info(msg string, args ...any)  {}
func (l *testLogger) Error(msg string, args ...any) {}
func (l *testLogger) Warn(msg string, args ...any)  {}
func (l *testLogger) Debug(msg string, args ...any) {}

func TestGetProductByUser(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	username := "USERTOKPEDDEV"

	rows := sqlmock.NewRows([]string{"OUTERRCODE", "OUTERRMSG", "KODEVOUCHER", "PRICE", "STATUSPRODUCT"}).
		AddRow("00", "OK", "PLN1", 1000.0, "ACTIVE").
		AddRow("00", "OK", "PLN2", 2000.0, "INACTIVE")

	query := "SELECT outerrcode, outerrmsg, kodevoucher, price, StatusProduct FROM TABLE(PKG_TOKPED_PRODUCT.getProductByUser(:1))"
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(username).
		WillReturnRows(rows)

	repo := &ProductPriceOracleRepository{
		oracleService: &mockOracleService{db: db},
		logger:        &testLogger{},
		redisClient:   nil,
	}

	resp, err := repo.GetProductByUser(context.Background(), username)
	require.NoError(t, err)
	require.Len(t, *resp, 2)

	require.Equal(t, domain.ProductPriceResponseDomain{
		OuterRCode:  "00",
		OuterRMsg:   "OK",
		KodeVoucher: "PLN1",
		Price:       1000.0,
		Status:      "ACTIVE",
	}, (*resp)[0])

	require.Equal(t, domain.ProductPriceResponseDomain{
		OuterRCode:  "00",
		OuterRMsg:   "OK",
		KodeVoucher: "PLN2",
		Price:       2000.0,
		Status:      "INACTIVE",
	}, (*resp)[1])

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestGetProductByUserAndProductCode(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	username := "USERTOKPEDDEV"
	productCode := "PLNT1JT"

	rows := sqlmock.NewRows([]string{"OUTERRCODE", "OUTERRMSG", "KODEVOUCHER", "PRICE", "STATUSPRODUCT"}).
		AddRow("00", "OK", "PLNT1JT", 12345.0, "ACTIVE")

	query := "SELECT outerrcode, outerrmsg, kodevoucher, price, StatusProduct FROM TABLE(PKG_TOKPED_PRODUCT.getProductByUserAndProductCode(:1, :2))"
	mock.ExpectQuery(regexp.QuoteMeta(query)).
		WithArgs(username, productCode).
		WillReturnRows(rows)

	repo := &ProductPriceOracleRepository{
		oracleService: &mockOracleService{db: db},
		logger:        &testLogger{},
		redisClient:   nil,
	}

	resp, err := repo.GetProductByUserAndProductCode(context.Background(), username, productCode)
	require.NoError(t, err)
	require.NotNil(t, resp)

	require.Equal(t, "00", resp.OuterRCode)
	require.Equal(t, "OK", resp.OuterRMsg)
	require.Equal(t, "PLNT1JT", resp.KodeVoucher)
	require.Equal(t, 12345.0, resp.Price)
	require.Equal(t, "ACTIVE", resp.Status)

	require.NoError(t, mock.ExpectationsWereMet())
}
