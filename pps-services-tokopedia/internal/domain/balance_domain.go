package domain

import "context"

type BalanceDetail struct {
	Label string
	Value float64
}

type BalanceGetRequestDomain struct {
	Timestamp string
}

type BalanceGetResponseDomain struct {
	ResponseCode   string
	Message        string
	BalanceDetails []BalanceDetail
	Timestamp      string
}

// BalanceResponseDomain represents the response from Oracle balance query
type BalanceResponseDomain struct {
	OutErrCode       string  `json:"outerrcode"`
	OutErrMsg        string  `json:"outerrmsg"`
	DepositBalance   float64 `json:"deposit_balance"`
	DepositGroupName string  `json:"deposit_group_name"`
}

// BalanceUsecase defines the interface for balance business logic
// * pointer because we don't want to pass by value, we want to pass by reference
type BalanceUsecase interface {
	GetBalance(ctx context.Context, req *BalanceGetRequestDomain) (*BalanceGetResponseDomain, error)
}

// BalanceRepository defines methods for balance operations
type BalanceRepository interface {
	GetDepositBalanceCredit(ctx context.Context, username string) (*BalanceResponseDomain, error)
}
