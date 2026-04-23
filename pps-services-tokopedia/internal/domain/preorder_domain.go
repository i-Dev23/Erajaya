package domain

import "context"

type PreorderRequestDomain struct {
	User      string
	MDN       string
	Product   string
	NoTrx     string
	Signature string
	Addr      string
}

type PreorderResponseDomain struct {
	ServerId   string
	Status     string
	QueueName  string
	OuterRCode int64
	OuterRMsg  string
}

type UpdatePreorderStatusResponseDomain struct {
	OuterRCode int64
	OuterRMsg  string
}

type PreorderRepository interface {
	Preorder(ctx context.Context, req *PreorderRequestDomain) (*PreorderResponseDomain, error)
	UpdatePreorderStatus(ctx context.Context, msgid string, status string, message string) (*UpdatePreorderStatusResponseDomain, error)
}
