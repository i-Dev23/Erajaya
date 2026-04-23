package model

type SellRequestModel struct {
	Msgid     int64  `json:"msgid"`
	User      string `json:"user"`
	Produk    string `json:"produk"`
	MDN       string `json:"mdn"`
	NoTrx     string `json:"noTrx"`
	Signature string `json:"signature"`
	Addr      string `json:"addr"`
}

// PublishMessage is the JSON format published by pps-services-publisher.
type PublishMessage struct {
	Source string      `json:"source"`
	Data   PublishData `json:"data"`
}

// PublishData is the data block inside PublishMessage.
type PublishData struct {
	User      string `json:"user"`
	Produk    string `json:"produk"`
	MDN       string `json:"mdn"`
	NoTrx     string `json:"noTrx"`
	Signature string `json:"signature"`
	Addr      string `json:"addr"`
	ServerId  string `json:"serverId"`
}

type BaseH2hResponse struct {
	Status      string `json:"Status"`
	ServerIDTrx string `json:"ServerIDTrx"`
	ClientNoTrx string `json:"ClientNoTrx"`
	Message     string `json:"Message"`
}

// PreOrderConsumeResult menampung output dari SP MSG.updPreOrderConsume
// yang sekarang juga mengembalikan data provider untuk diteruskan ke publisher-provider.
type PreOrderConsumeResult struct {
	OutError       int64
	OutMessage     string
	OutImsi        string
	OutRemarkImsi  string
	OutMid         string
	OutStoreId     string
	OutQueueName   string
	OutTypeVoucher string
	OutProvider    string
	OutCommand     string
}

// ProviderMessage adalah pesan PROVIDER yang dikonsumsi dari RabbitMQ,
// dikirim oleh pps-services-publisher-database setelah gateway mendapat response provider.
// Struktur ini sesuai dengan TransactionEvent di pps-services-consumer-database.
type ProviderMessage struct {
	MsgId                  int    `json:"msg_id"`
	StatusToBe             string `json:"status_to_be"`
	SerialNumber           string `json:"serial_number"`
	ClientNumber           string `json:"client_number"`
	Nominal                string `json:"nominal"`
	OriginalConversationID string `json:"original_conversation_id"`
	ConversationID         string `json:"conversation_id"`
	MessageToCustomer      string `json:"message_to_customer"`
	AdditionalMessage      string `json:"additional_message"`
	QueueName              string `json:"queue_name"`
	Source                 string `json:"source"`
}

// ProviderWrapperMessage adalah format wrapper untuk pesan PROVIDER dari RabbitMQ.
// Format: {"source": "PROVIDER", "data": {"msg_id": ..., ...}}
// Digunakan oleh gateway yang mengirim status akhir transaksi ke consumer.
type ProviderWrapperMessage struct {
	Source string          `json:"source"`
	Data   ProviderMessage `json:"data"`
}

// PublishProviderRequest adalah HTTP request body untuk POST ke
// pps-services-publisher-provider /api/v1/publish.
// JSON tag harus sama persis dengan PublishRequest di publisher-provider.
type PublishProviderRequest struct {
	MsgID        string `json:"msgid"`
	ClientNumber string `json:"clientNumber"`
	IMSI         string `json:"imsi"`
	RemarkIMSI   string `json:"remarkImsi"`
	MID          string `json:"mid"`
	StoreID      string `json:"storeId,omitempty"`
	QueueName    string `json:"queueName,omitempty"`
	TypeVoucher  string `json:"typeVoucher"`
	VoucherCode  string `json:"voucherCode"`
	Command      string `json:"command,omitempty"`
	Provider     string `json:"provider,omitempty"`
	QTransaction string `json:"MQTransaction"`
}
