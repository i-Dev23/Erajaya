package dto

type AdditionalParameterDto struct {
	Name  string `json:"name,omitempty"`  // Additional parameter name
	Value string `json:"value,omitempty"` // Additional parameter value
}

type BillDetailDto struct {
	Name   string `json:"name,omitempty"`    // Bill detail name
	Value  string `json:"value,omitempty"`   // Bill detail value
	IsPII  *bool  `json:"is_pii,omitempty"`  // Defines if given detail contains PII value
	IsShow *bool  `json:"is_show,omitempty"` // Defines if given detail needs to be shown to the user
}

type AdditionalDetailDto struct {
	Label   string          `json:"label,omitempty"`   // Additional label detail
	Details []BillDetailDto `json:"details,omitempty"` // Additional detail values
}

type BaseTokopediaResponseDto struct {
	ResponseCode string `json:"response_code" validate:"required"`                          // Response code as specified (see 10. Response Codes)
	Message      string `json:"message" validate:"required"`                                // Additional informational or error message
	Timestamp    string `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"` // In Jakarta Time GMT+7, Format: YYYY-MM-DD hh:mm:ss
}
