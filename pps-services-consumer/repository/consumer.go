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

func Consumer(connection *amqp.Connection, queueName string) {
	channel, err := connection.Channel()
	if err != nil {
		util.ComposeMessageTelegramNotification(err.Error())
		log.Println("Error connection channel => " + err.Error())
	} else {
		defer channel.Close()

		// declaring queue with its properties over the the channel opened
		_, errQueue := channel.QueueDeclare(
			queueName, // name
			false,     // durable
			false,     // auto delete
			false,     // exclusive
			false,     // no wait
			nil,       // args
		)
		if errQueue != nil {
			util.ComposeMessageTelegramNotification(err.Error())
			log.Println("Error queue declare => " + errQueue.Error())
		}

		// declaring consumer with its properties over channel opened
		msgs, errDeclareConsume := channel.Consume(
			queueName, // queue
			"",        // consumer
			true,      // auto ack
			false,     // exclusive
			false,     // no local
			false,     // no wait
			nil,       //args
		)
		if errDeclareConsume != nil {
			util.ComposeMessageTelegramNotification(errDeclareConsume.Error())
			log.Println("Error Declare Consume => " + errDeclareConsume.Error())
		} else {
			// print consumed messages from queue
			forever := make(chan bool)
			//go func() { // remark go func()

			for msg := range msgs {
				log.Printf("Received Message (queue=%s exchange=%s routingKey=%s): %s", queueName, msg.Exchange, msg.RoutingKey, string(msg.Body))
				ProsesData(string(msg.Body))
			}

			//}()

			log.Println("Waiting for messages...")

			<-forever
		}
	}
}

func ProsesData(data string) {
	request, err := parseMessage(data)
	if err != nil {
		log.Println("Error parsing message: " + err.Error() + " data: " + data)
		return
	}

	result := UpdPreOrderConsume(strconv.FormatInt(request.Msgid, 10), constanta.FLAG_SUCCESS_CONSUME)
	if result.OutError == 1 {
		log.Println("tidak call request2JualRandomWithID karena outError UpdPreOrderConsume = 1")
	} else {
		SellWithId(request)
	}
}

// parseMessage parses both JSON and pipe-delimited message formats.
func parseMessage(data string) (model.SellRequestModel, error) {
	var request model.SellRequestModel
	trimmed := strings.TrimSpace(data)

	// Try JSON format first
	if strings.HasPrefix(trimmed, "{") {
		var msg model.PublishMessage
		if err := json.Unmarshal([]byte(trimmed), &msg); err == nil && msg.Data.ServerId != "" {
			i, _ := strconv.ParseInt(strings.TrimSpace(msg.Data.ServerId), 10, 64)
			request.Msgid = i
			request.User = msg.Data.User
			request.Produk = msg.Data.Produk
			request.MDN = msg.Data.MDN
			request.NoTrx = msg.Data.NoTrx
			request.Signature = msg.Data.Signature
			request.Addr = msg.Data.Addr
			return request, nil
		}
	}

	// Fallback to pipe-delimited format
	// Format: PREORDER||addr||mdn||notrx||produk||signature||serverId||user
	datas := strings.Split(data, "||")
	if len(datas) < 8 {
		return request, fmt.Errorf("invalid pipe-delimited format, expected 8 fields got %d", len(datas))
	}

	i, _ := strconv.ParseInt(datas[6], 10, 64)
	request.Msgid = i
	request.Addr = datas[1]
	request.MDN = datas[2]
	request.NoTrx = datas[3]
	request.Produk = datas[4]
	request.Signature = datas[5]
	request.User = datas[7]

	return request, nil
}
