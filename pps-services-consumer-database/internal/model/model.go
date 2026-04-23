package model

import "encoding/json"

// TransactionEvent is the message consumed from RabbitMQ, matching publisher's format.
type TransactionEvent struct {
	Id        string              `json:"id"`
	QueueName string              `json:"queue_name"`
	Headers   map[string][]string `json:"headers"`
	Payload   json.RawMessage     `json:"payload"`
}

// TransactionPayload is the decoded body from CallbackEvent, matching publisher's CallbackRequest format.
type TransactionPayload struct {
	Source string          `json:"source"`
	Data   json.RawMessage `json:"data"`
}

// Source constants
const (
	SourcePreOrder = "PRE_ORDER"
	SourceProvider = "PROVIDER"
	SourceOrder    = "ORDER"
)

// TopupPayload matches publisher's TopupPayload for PROVIDER source.
type TopupDataPayload struct {
	MsgID                  string `json:"msgId"`
	StatusToBe             string `json:"statusToBe"`
	SerialNumber           string `json:"serialNumber"`
	ClientNumber           string `json:"clientNumber"`
	Nominal                string `json:"nominal"`
	OriginalConversationID string `json:"originalConversationID"`
	ConversationID         string `json:"conversationID"`
	MessageToCustomer      string `json:"messageToCustomer"`
	AdditionalMessage      string `json:"additionalMessage"`
	QueueName              string `json:"queueName"`
}

// OrderPayload for ORDER source — data needed for PreOrderConsume flow.
type OrderPayload struct {
	MsgID         string `json:"msgId"`
	ConsumeStatus string `json:"consumeStatus"`
	User          string `json:"user"`
	ClientNumber  string `json:"clientNumber"`
	VoucherCode   string `json:"voucherCode"`
	TrxNo         string `json:"trxNo"`
	Signature     string `json:"signature"`
	IP            string `json:"ip"`
	Status        string `json:"status"`
}

// PreOrderResult holds output from CallUpdPreOrderConsume SP.
type PreOrderResult struct {
	IMSI        string `json:"imsi"`
	RemarkIMSI  string `json:"remarkImsi"`
	MID         string `json:"mid"`
	StoreID     int    `json:"storeId"`
	QueueName   string `json:"queueName"`
	TypeVoucher string `json:"typeVoucher"`
	BID         int    `json:"bid"`
	TypeOfStock string `json:"typeOfStock"`
	Provider    string `json:"provider"`
}

// DownstreamRequest is the payload sent to the REST API after order processing.
type DownstreamRequest struct {
	MsgID         string `json:"msgId"`
	ClientNumber  string `json:"clientNumber"`
	IMSI          string `json:"imsi"`
	RemarkIMSI    string `json:"remarkImsi"`
	MID           string `json:"mid"`
	StoreID       int    `json:"storeId"`
	QueueName     string `json:"queueName"`
	TypeVoucher   string `json:"typeVoucher"`
	VoucherCode   string `json:"voucherCode"`
	BID           int    `json:"bid"`
	TypeOfStock   string `json:"typeOfStock"`
	Provider      string `json:"provider"`
	MQTransaction string `json:"mqTransaction"`
}

// SPResult holds the output parameters from the Oracle stored procedure.
type SPResult struct {
	ID      int    `json:"out_id"`
	Error   int    `json:"out_error"`
	Message string `json:"out_message"`
}
