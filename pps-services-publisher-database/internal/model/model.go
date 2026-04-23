package model

import (
	"encoding/json"
)

type HasMsgIdAndQueueName interface {
	GetMsgID() string
	GetQueueName() string
}

// WebResponse is a generic HTTP response wrapper.
type WebResponse[T any] struct {
	Data   T      `json:"data"`
	Errors string `json:"errors,omitempty"`
}

type HealthResponse struct {
	Status   string            `json:"status"`
	Services map[string]string `json:"services"`
}

type CallbackRequest[T HasMsgIdAndQueueName] struct {
	Source string `json:"source"`
	Data   T      `json:"data"`
}

func (r *CallbackRequest[T]) ToCallbackEvent(Headers map[string][]string) *CallbackEvent {
	event := &CallbackEvent{
		ID:        r.Data.GetMsgID(),
		QueueName: r.Data.GetQueueName(),
		Headers:   Headers,
	}

	payloadBytes, err := json.Marshal(r)
	if err != nil {
		return event
	}

	// // UseNumber() agar angka tetap json.Number (bukan float64)
	// dec := json.NewDecoder(bytes.NewReader(payloadBytes))
	// dec.UseNumber()

	// var payloadMap map[string]any
	// if err := dec.Decode(&payloadMap); err != nil {
	// 	return event
	// }

	// event.Payload = payloadMap

	event.Payload = payloadBytes
	return event
}

type TopupDataPayload struct {
	MsgID                  string `json:"msgId" validate:"required,min=1"`
	StatusToBe             string `json:"statusToBe" validate:"required,max=1"`
	SerialNumber           string `json:"serialNumber,omitempty" validate:"omitempty,max=100"`
	ClientNumber           string `json:"clientNumber" validate:"required,min=1,max=100"`
	Nominal                string `json:"nominal,omitempty" validate:"omitempty,max=50"`
	OriginalConversationID string `json:"originalConversationID" validate:"required,max=100"`
	ConversationID         string `json:"conversationID" validate:"required,min=1,max=100"`
	MessageToCustomer      string `json:"messageToCustomer" validate:"required,max=500"`
	AdditionalMessage      string `json:"additionalMessage,omitempty" validate:"omitempty,max=500"`
	QueueName              string `json:"queueName" validate:"required,min=1,max=100"`
}

func (t TopupDataPayload) GetMsgID() string     { return t.MsgID }
func (t TopupDataPayload) GetQueueName() string { return t.QueueName }

type GamePayload struct {
	MsgID     string `json:"msgId" validate:"required,min=1"`
	QueueName string `json:"queueName" validate:"required,min=1,max=100"`
}

func (g GamePayload) GetMsgID() string     { return g.MsgID }
func (g GamePayload) GetQueueName() string { return g.QueueName }

// type PlnTokenPayload struct {
// }

// type PlnPostpaidPayload struct {
// }

type CallbackResponse struct {
	MsgID  string `json:"msgId"`
	Status string `json:"status"`
}

// CallbackEvent is the message published to RabbitMQ for Oracle processing.
type CallbackEvent struct {
	ID        string              `json:"id"`
	QueueName string              `json:"queueName"`
	Headers   map[string][]string `json:"headers"`
	Payload   json.RawMessage     `json:"payload"`
}
