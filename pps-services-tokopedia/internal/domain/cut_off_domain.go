package domain

import "context"

type CutOffResponseDomain struct {
	OutErrCode string
	OutMsgErr  string
}

const (
	CutOffFlagActive   = "1"
	CutOffFlagInactive = "0"
	CutOffFlagH2H      = "H2H_TOKOPEDIA" // flag for tokopedia services
)

type CutOffRepository interface {
	CutOff(ctx context.Context, flag string) (*CutOffResponseDomain, error)
}
