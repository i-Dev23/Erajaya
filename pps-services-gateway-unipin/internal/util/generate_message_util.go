package util

import (
	"fmt"
	"os"
	"strings"
	"time"
)

// GenerateMessage membentuk message_to_customer untuk downstream.
//
// Catatan implementasi untuk UniPin:
//   - statusToBe diharapkan sudah dinormalisasi ke salah satu dari: F / C / S
//   - serialNumber diisi dengan ReferenceNo (jika ada)
//   - statusCode diisi dengan resp.Status (string) (jika ada); untuk resp.Status==0,
//     statusCode sebaiknya diisi dengan resp.error.error_code (atau resp.error.code) bila tersedia.
func GenerateMessage(amount int, clientNumber string, requestedAt time.Time, statusToBe string, serialNumber string, statusCode string) string {
	const DDMMYYYYhhmmss = "02/01/2006 15:04:05"
	msg := fmt.Sprintf("Pengisian Voucher sebesar %d ke nomor %s pada tanggal %s",
		amount, clientNumber, requestedAt.Format(DDMMYYYYhhmmss))

	switch strings.ToUpper(strings.TrimSpace(statusToBe)) {
	case "F":
		msg = fmt.Sprintf("%s telah berhasil dengan no ref <%s>", msg, serialNumber)
	case "C":
		if strings.TrimSpace(statusCode) == "" {
			msg = fmt.Sprintf("%s GAGAL", msg)
			break
		}
		msg = fmt.Sprintf("%s GAGAL, keterangan = Status Code : %s", msg, statusCode)
	case "S":
		msg = strings.TrimSpace(os.Getenv("PROCESSING_MESSAGE"))
		if msg == "" {
			msg = "Menunggu Response Unipin"
		}
	}

	return msg
}
