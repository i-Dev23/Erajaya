package domain

type BillDetailDomain struct {
	Name   string `json:"name"`    // Bill detail name
	Value  string `json:"value"`   // Bill detail value
	IsPII  bool   `json:"is_pii"`  // Defines if given detail contains PII value
	IsShow bool   `json:"is_show"` // Defines if given detail needs to be shown to the user
}
