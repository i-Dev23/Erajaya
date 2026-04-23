package dto

type BalanceRequestDTO struct {
	Timestamp string `json:"timestamp" validate:"required,datetime=2006-01-02 15:04:05"`
}

type BalanceDetailDTO struct {
	Label string  `json:"label"`
	Value float64 `json:"value"`
}

type BalanceResponseDTO struct {
	ResponseCode   string             `json:"response_code"`
	Message        string             `json:"message"`
	BalanceDetails []BalanceDetailDTO `json:"balance_details"`
	Timestamp      string             `json:"timestamp"`
}
