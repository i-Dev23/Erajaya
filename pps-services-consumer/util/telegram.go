package util

import (
	"bytes"
	"io"
	"log"
	"os"
	"pps-services-consumer/constanta"
	"strconv"
	"strings"

	tgbotapi "github.com/go-telegram-bot-api/telegram-bot-api/v5"
)

func ComposeMessageTelegramNotification(errorMessages string) {
	var buffer bytes.Buffer
	buffer.WriteString("Environment: " + os.Getenv(constanta.OS_ENV_Env))
	buffer.WriteString("\n")
	buffer.WriteString("\n")
	buffer.WriteString("Error Message: " + errorMessages)

	SendMessageTelegramViaSdk(buffer.String())
}

func SendMessageTelegramViaSdk(bodyMessage string) {
	bot, err := tgbotapi.NewBotAPI(os.Getenv(constanta.OS_ENV_TokenBotTelagram))
	if err != nil {
		log.Println("error Bot API = " + err.Error())
	} else {
		i, _ := strconv.ParseInt(os.Getenv(constanta.OS_ENV_ChatId), 10, 64)
		msg := tgbotapi.NewMessage(i, CharLimiter(bodyMessage, 4096))
		bot.Send(msg)
		bot.Debug = true
	}
}

func CharLimiter(s string, limit int) string {

	reader := strings.NewReader(s)

	// create buffer with specified limit of chraracters
	buff := make([]byte, limit)

	n, _ := io.ReadAtLeast(reader, buff, limit)

	if n != 0 {
		//fmt.Printf("\n %s ", buff)
		return string(buff)
	} else {
		// nothing happens, return original string
		return s
	}

}
