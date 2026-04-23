package domain

import "context"

// ProductPriceResponseDomain represents the response from GetKodeVoucher function
type ProductPriceResponseDomain struct {
	OuterRCode  string  // outerrcode field from Oracle function
	OuterRMsg   string  // outerrmsg field from Oracle function
	KodeVoucher string  // kodevoucher field from Oracle function
	Price       float64 // price field from Oracle function
	Status      string  // statusproduct field from Oracle function
}

// WhitelistedIpResponseDomain represents the response from GetIpByUser function
type WhitelistedIpResponseDomain struct {
	OuterRCode string // outerrcode field from Oracle function
	OuterRMsg  string // outerrmsg field from Oracle function
	OutIp      string // outip field from Oracle function
}

// CutOffDataResponseDomain represents the response from GetCutOff function
type CutOffDataResponseDomain struct {
	OutErrCode               string // OutErrCode field from Oracle function
	OutErrMsg                string // OutErrMsg field from Oracle function
	CutOffTimeStartTokopedia string // Cut_off_time_start_tokopedia field from Oracle function
	CutOffDurationTokopedia  string // Cut_off_duration_tokopedia field from Oracle function
	CutOffMessageTokopedia   string // Cut_off_message_tokopedia field from Oracle function
	CutOffTimeStart          string // Cut_off_time_start field from Oracle function
	CutOffDuration           string // Cut_off_duration field from Oracle function
	CutOffMessage            string // Cut_off_message field from Oracle function
}

// ProductRepository defines the interface for product-related Oracle DB operations
type ProductRepository interface {
	GetPriceByUser(ctx context.Context, username string, provider string) (*[]ProductPriceResponseDomain, error)
	GetPriceByUserAndProductCode(ctx context.Context, username string, productCode string) (*ProductPriceResponseDomain, error)
	GetProductByUser(ctx context.Context, username string) (*[]ProductPriceResponseDomain, error)
	GetProductByUserAndProductCode(ctx context.Context, username string, productCode string) (*ProductPriceResponseDomain, error)
	GetIpByUser(ctx context.Context, username string) (*WhitelistedIpResponseDomain, error)
	GetCutOff(ctx context.Context) (*CutOffDataResponseDomain, error)
}
