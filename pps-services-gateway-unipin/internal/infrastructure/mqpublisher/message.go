package mqpublisher

// ProviderPublishData berisi data transaksi yang dipublikasikan ke downstream consumer.
type ProviderPublishData struct {
	MsgID                  int    `json:"msg_id"`
	StatusToBe             string `json:"status_to_be"`
	SerialNumber           string `json:"serial_number"`
	ClientNumber           string `json:"client_number"`
	Nominal                string `json:"nominal"`
	OriginalConversationID string `json:"original_conversation_id"`
	ConversationID         string `json:"conversation_id"`
	MessageToCustomer      string `json:"message_to_customer"`
	AdditionalMessage      string `json:"additional_message"`
	QueueName              string `json:"queue_name"`
}

// ProviderPublishMessage adalah wrapper JSON yang dipublikasikan ke RabbitMQ.
// Format: {"source": "PROVIDER", "data": {...}}
type ProviderPublishMessage struct {
	Source string              `json:"source"`
	Data   ProviderPublishData `json:"data"`
}

// NewProviderPublishMessage membuat ProviderPublishMessage dengan source = "PROVIDER".
func NewProviderPublishMessage(data ProviderPublishData) ProviderPublishMessage {
	return ProviderPublishMessage{Source: "PROVIDER", Data: data}
}
