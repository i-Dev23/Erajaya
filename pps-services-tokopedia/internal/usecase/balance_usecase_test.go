package usecase

import (
	"context"
	"errors"
	"pps-services-tokopedia/internal/config"
	"pps-services-tokopedia/internal/domain"
	"testing"

	"github.com/stretchr/testify/assert"
)

type mockBalanceRepository struct {
	GetDepositBalanceCreditFunc func(ctx context.Context, username string) (*domain.BalanceResponseDomain, error)
}

func (m *mockBalanceRepository) GetDepositBalanceCredit(ctx context.Context, username string) (*domain.BalanceResponseDomain, error) {
	if m.GetDepositBalanceCreditFunc != nil {
		return m.GetDepositBalanceCreditFunc(ctx, username)
	}
	return nil, nil
}

func TestBalanceUsecase_GetBalance(t *testing.T) {
	tests := []struct {
		name        string
		req         *domain.BalanceGetRequestDomain
		repoResp    *domain.BalanceResponseDomain
		repoErr     error
		wantCode    string
		wantMsg     string
		wantDetails int
		wantErr     bool
	}{
		{
			name:        "missing timestamp",
			req:         &domain.BalanceGetRequestDomain{Timestamp: ""},
			repoResp:    nil,
			repoErr:     nil,
			wantCode:    "42",
			wantMsg:     "Invalid parameter",
			wantDetails: 0,
			wantErr:     false,
		},
		{
			name: "success",
			req:  &domain.BalanceGetRequestDomain{Timestamp: "2021-12-12 12:12:12"},
			repoResp: &domain.BalanceResponseDomain{
				OutErrCode:       "0",
				OutErrMsg:        "OK",
				DepositBalance:   1500000000.00,
				DepositGroupName: "General",
			},
			repoErr:     nil,
			wantCode:    "00",
			wantMsg:     "Success",
			wantDetails: 1,
			wantErr:     false,
		},
		{
			name:     "repo error",
			req:      &domain.BalanceGetRequestDomain{Timestamp: "2021-12-12 12:12:12"},
			repoResp: nil,
			repoErr:  errors.New("oracle down"),
			wantCode: "62",
			wantMsg:  "Server error",
			wantErr:  true,
		},
		{
			name: "oracle non-success outerrcode",
			req:  &domain.BalanceGetRequestDomain{Timestamp: "2021-12-12 12:12:12"},
			repoResp: &domain.BalanceResponseDomain{
				OutErrCode:       "1",
				OutErrMsg:        "Some oracle error",
				DepositBalance:   0,
				DepositGroupName: "",
			},
			repoErr:     nil,
			wantCode:    "62",
			wantMsg:     "Some oracle error",
			wantDetails: 0,
			wantErr:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo := &mockBalanceRepository{}
			repo.GetDepositBalanceCreditFunc = func(ctx context.Context, username string) (*domain.BalanceResponseDomain, error) {
				return tt.repoResp, tt.repoErr
			}

			cfg := &config.Config{TPClientID: "USERTOKPEDDEV"}
			u := NewBalanceUsecase(cfg, &mockStatusLogger{}, repo)

			resp, err := u.GetBalance(context.Background(), tt.req)
			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			if assert.NotNil(t, resp) {
				assert.Equal(t, tt.wantCode, resp.ResponseCode)
				assert.Equal(t, tt.wantMsg, resp.Message)
				assert.Len(t, resp.BalanceDetails, tt.wantDetails)
				if tt.wantDetails > 0 {
					assert.NotEmpty(t, resp.BalanceDetails[0].Label)
				}
			}
		})
	}
}
