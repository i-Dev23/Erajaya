package unipin

// GameListResponse is the response body for In-Game Topup Game List API.
type GameListResponse struct {
	GameList []Game    `json:"game_list"`
	Status   int       `json:"status"`
	Reason   string    `json:"reason"`
	Error    *APIError `json:"error,omitempty"`
}

// Game represents a single game entry in the game list response.
type Game struct {
	GameCategory string `json:"game_category"`
	GameCode     string `json:"game_code"`
	GameName     string `json:"game_name"`
	IconURL      string `json:"icon_url"`
	GameStatus   string `json:"game_status"`
	UpdatedAt    string `json:"updated_at"`
	ProductName  string `json:"product_name"`
	CategoryName string `json:"category_name"`
}

// gameDetailRequestBody is the internal request body for Game Detail API.
type gameDetailRequestBody struct {
	GameCode string `json:"game_code"`
}

// GameDetailResponse is the response body for In-Game Topup Game Detail API.
type GameDetailResponse struct {
	HelpImageURL  string         `json:"help_image_url"`
	Game          GameInfo       `json:"game"`
	Denominations []Denomination `json:"denominations"`
	Fields        []Field        `json:"fields"`
	Status        int            `json:"status"`
	Reason        string         `json:"reason"`
	Error         *APIError      `json:"error,omitempty"`
}

// GameInfo represents game info in the game detail response.
type GameInfo struct {
	Name     string `json:"name"`
	Code     string `json:"code"`
	Category string `json:"category"`
}

// Denomination represents a denomination entry in the game detail response.
type Denomination struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Currency string `json:"currency"`
	Amount   string `json:"amount"`
}

// Field represents a required field for in-game topup.
type Field struct {
	Name string `json:"name"`
	Type string `json:"type"`
}

// ValidateUserRequest is the request for In-Game Topup Validate User API.
type ValidateUserRequest struct {
	GameCode string         `json:"game_code"`
	Fields   map[string]any `json:"fields"`
}

// ValidateUserResponse is the response body for In-Game Topup Validate User API.
type ValidateUserResponse struct {
	Username        string    `json:"username"`
	ValidationToken string    `json:"validation_token"`
	Status          int       `json:"status"`
	Reason          string    `json:"reason"`
	Error           *APIError `json:"error,omitempty"`
}

// CreateOrderRequest is the request for In-Game Topup Create Order API.
type CreateOrderRequest struct {
	GameCode        string `json:"game_code"`
	ValidationToken string `json:"validation_token"`
	ReferenceNo     string `json:"reference_no"`
	DenominationID  string `json:"denomination_id"`
}

// CreateOrderResponse is the response body for In-Game Topup Create Order API.
type CreateOrderResponse struct {
	TransactionNumber string    `json:"transaction_number"`
	ReferenceNo       string    `json:"reference_no"`
	Amount            int       `json:"amount"`
	Currency          string    `json:"currency"`
	ItemName          string    `json:"item_name"`
	Status            int       `json:"status"`
	Reason            string    `json:"reason"`
	Error             *APIError `json:"error,omitempty"`
}

// orderInquiryRequestBody is the internal request body for Order Inquiry API.
type orderInquiryRequestBody struct {
	ReferenceNo string `json:"reference_no"`
}

// OrderInquiryResponse is the response body for In-Game Topup Order Inquiry API.
type OrderInquiryResponse struct {
	TransactionNumber string    `json:"transaction_number"`
	ReferenceNo       string    `json:"reference_no"`
	Amount            int       `json:"amount"`
	Currency          string    `json:"currency"`
	ItemName          string    `json:"item_name"`
	Status            int       `json:"status"`
	Reason            string    `json:"reason"`
	Error             *APIError `json:"error,omitempty"`
}
