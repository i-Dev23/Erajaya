package util

import (
	"fmt"
	"os"
	"strings"
	"time"
)

func GenerateMessage(amount int, client_number string, requestedAt time.Time, statusToBe string, serialNumber string, statusCode string) string {
	const DDMMYYYYhhmmss = "02/01/2006 15:04:05"
	msg := fmt.Sprintf("Pengisian Voucher sebesar %d ke nomor %s pada tanggal %s",
		amount, client_number, requestedAt.Format(DDMMYYYYhhmmss))

	switch statusToBe {
	case "F":
		msg = fmt.Sprintf("%s telah berhasil dengan no ref <%s>", msg, serialNumber)
	case "C":
		if strings.TrimSpace(statusCode) == "" {
			msg = fmt.Sprintf("%s GAGAL", msg)
			break
		}
		msg = fmt.Sprintf("%s GAGAL, keterangan = Status Code : %s ", msg, statusCode)
	case "S":
		msg = strings.TrimSpace(os.Getenv("PROCESSING_MESSAGE"))
		if msg == "" {
			msg = "Menunggu Response Telkomsel"
		}
	}

	return msg
}
