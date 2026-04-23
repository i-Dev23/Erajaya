package repository

import (
	"encoding/json"
	"fmt"
	"pps-services-consumer/constanta"
	"pps-services-consumer/model"
	"pps-services-consumer/util"
	log "pps-services-consumer/util"
	"strconv"
	"strings"

	amqp "github.com/rabbitmq/amqp091-go"
)

// sourceResult menentukan hasil pemrosesan pesan untuk Ack/Nack.
type sourceResult int

const (
	resultOK          sourceResult = iota // Ack
	resultNackRequeue                     // Nack + requeue
	resultNackDiscard                     // Nack tanpa requeue (pesan invalid)
)

func ConsumerFIFO(connection *amqp.Connection, queueName string, mqTransactionURL string) {
	channel, err := connection.Channel()
	if err != nil {
		util.ComposeMessageTelegramNotification(err.Error())
		log.Println("Error connection channel => " + err.Error())
		return
	}
	defer channel.Close()

	// Prefetch = 1 → hanya satu pesan dikirim ke consumer, untuk FIFO
	err = channel.Qos(1, 0, false)
	if err != nil {
		util.ComposeMessageTelegramNotification("Qos error: " + err.Error())
		log.Println("Error setting Qos => " + err.Error())
		return
	}

	_, errQueue := channel.QueueDeclare(
		queueName,
		true,  // durable
		false, // auto delete
		false, // exclusive
		false, // no wait
		nil,
	)
	if errQueue != nil {
		util.ComposeMessageTelegramNotification("Queue declare error: " + errQueue.Error())
		log.Println("Error queue declare => " + errQueue.Error())
		return
	}

	msgs, errConsume := channel.Consume(
		queueName,
		"",
		false, // manual ack → untuk FIFO
		false,
		false,
		false,
		nil,
	)
	if errConsume != nil {
		util.ComposeMessageTelegramNotification("Consume error: " + errConsume.Error())
		log.Println("Error Consume => " + errConsume.Error())
		return
	}

	log.Println(" [*] Waiting for FIFO messages in queue:", queueName)

	for msg := range msgs {
		log.Println("Received:", string(msg.Body))
		res := ProsesDataFIFO(string(msg.Body), mqTransactionURL)
		switch res {
		case resultNackDiscard:
			log.Println("Nack (discard) message")
			msg.Nack(false, false) // requeue=false
		case resultNackRequeue:
			log.Println("Nack (requeue) message")
			msg.Nack(false, true)
		default:
			msg.Ack(false)
		}
	}
}

// extractSource mengekstrak field "source" dari JSON message.
// Return source string (uppercase trimmed) dan error jika bukan JSON valid.
func extractSource(data string) (string, error) {
	var envelope struct {
		Source string `json:"source"`
	}
	if err := json.Unmarshal([]byte(data), &envelope); err != nil {
		return "", err
	}
	return strings.ToUpper(strings.TrimSpace(envelope.Source)), nil
}

// ProsesDataFIFO memproses pesan dari RabbitMQ dengan routing berdasarkan source.
// mqTransactionURL adalah URL RabbitMQ dari pair config, diteruskan ke publisher-provider.
func ProsesDataFIFO(data string, mqTransactionURL string) sourceResult {
	source, err := extractSource(data)
	if err != nil {
		// Bukan JSON valid, coba legacy pipe-delimited format
		log.Println("Cannot extract source, fallback to legacy flow: " + err.Error())
		return processLegacy(data)
	}

	switch source {
	case "PRE-ORDER", "PREORDER":
		return processPreOrder(data, mqTransactionURL)
	case "PROVIDER":
		return processProvider(data)
	default:
		// Source kosong atau tidak dikenali → legacy flow
		log.Println("Unknown source '" + source + "', fallback to legacy flow")
		return processLegacy(data)
	}
}

// processPreOrder menangani pesan PRE-ORDER: UpdPreOrderConsume + SellWithId + HTTP call publisher-provider.
func processPreOrder(data string, mqTransactionURL string) sourceResult {
	request, err := parseMessage(data)
	if err != nil {
		log.Println("Data format invalid (PRE-ORDER) => " + data + " error: " + err.Error())
		return resultOK // ack, data memang invalid
	}

	result := UpdPreOrderConsume(strconv.FormatInt(request.Msgid, 10), constanta.FLAG_SUCCESS_CONSUME)
	if result.OutError == 1 {
		log.Println("Skip SellWithId karena outError UpdPreOrderConsume = 1, msgid: " + strconv.FormatInt(request.Msgid, 10))
		return resultOK
	}

	response := SellWithId(request)

	// Jika SellWithId error (status = FAILED), skip HTTP call
	if response.Status == constanta.FAILED_STATUS {
		log.Printf("Skip CallPublisherProvider karena SellWithId gagal, msgid: %d, message: %s",
			request.Msgid, response.Message)
		return resultOK
	}

	// Jika RemarkImsi kosong dari SP, tidak perlu forward ke provider
	if result.OutRemarkImsi == "" {
		log.Printf("Skip CallPublisherProvider karena OutRemarkImsi kosong, msgid: %d", request.Msgid)
		return resultOK
	}

	// Mapping data dari SP output + request ke PublishProviderRequest
	publishReq := model.PublishProviderRequest{
		MsgID:        strconv.FormatInt(request.Msgid, 10),
		ClientNumber: request.MDN,
		IMSI:         result.OutImsi,
		RemarkIMSI:   result.OutRemarkImsi,
		MID:          result.OutMid,
		StoreID:      strings.TrimSpace(result.OutStoreId),
		QueueName:    result.OutQueueName,
		TypeVoucher:  result.OutTypeVoucher,
		VoucherCode:  request.Produk,
		Command:      result.OutCommand,
		Provider:     result.OutProvider,
		QTransaction: mqTransactionURL,
	}

	if err := CallPublisherProvider(publishReq); err != nil {
		log.Printf("CallPublisherProvider error, msgid: %d, error: %s", request.Msgid, err.Error())
	}

	return resultOK
}

// parseProviderMessage mem-parsing JSON pesan PROVIDER dengan auto-deteksi format.
// Mengembalikan ProviderMessage, format yang terdeteksi ("wrapper" atau "flat"), dan error.
func parseProviderMessage(data string) (model.ProviderMessage, string, error) {
	// 1. Try wrapper format first
	var wrapper model.ProviderWrapperMessage
	if err := json.Unmarshal([]byte(data), &wrapper); err == nil && wrapper.Data.MsgId != 0 {
		return wrapper.Data, "wrapper", nil
	}

	// 2. Fallback to flat format
	var flat model.ProviderMessage
	if err := json.Unmarshal([]byte(data), &flat); err == nil {
		return flat, "flat", nil
	}

	return model.ProviderMessage{}, "", fmt.Errorf("failed to parse provider message in both wrapper and flat format")
}

// processProvider menangani pesan PROVIDER: parse ProviderMessage + SetTransactionStatus.
// Mendukung dua format: wrapper {"source":"PROVIDER","data":{...}} dan flat {...}.
func processProvider(data string) sourceResult {
	provMsg, format, err := parseProviderMessage(data)
	if err != nil {
		log.Println("Error parse ProviderMessage => " + err.Error() + " data: " + data)
		return resultNackDiscard
	}

	log.Printf("Processing PROVIDER message (%s format), msg_id: %d", format, provMsg.MsgId)

	outError, outMessage := SetTransactionStatus(provMsg)
	if outError != 0 {
		log.Printf("SetTransactionStatus warning, msg_id: %d, outError: %d, outMessage: %s",
			provMsg.MsgId, outError, outMessage)
	}

	return resultOK
}

// processLegacy menangani pesan tanpa source (backward compatible).
func processLegacy(data string) sourceResult {
	request, err := parseMessage(data)
	if err != nil {
		log.Println("Data format invalid (legacy) => " + data + " error: " + err.Error())
		return resultOK
	}

	result := UpdPreOrderConsume(strconv.FormatInt(request.Msgid, 10), constanta.FLAG_SUCCESS_CONSUME)
	if result.OutError == 1 {
		log.Println("Skip SellWithId karena outError UpdPreOrderConsume = 1 (legacy)")
		return resultOK
	}

	SellWithId(request)
	return resultOK
}
