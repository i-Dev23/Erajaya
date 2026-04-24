package util

import (
	"fmt"
	"time"
)

// GenerateTransactionID menghasilkan ID transaksi unik berdasarkan mid, msgID, dan timestamp.
// Format: SMB-{mid}-{msgID}-{timestamp}
func GenerateTransactionID(mid, msgID string, t time.Time) string {
	if mid == "" {
		mid = "UNKNOWN"
	}
	if msgID == "" {
		msgID = "0"
	}
	return fmt.Sprintf("SMB-%s-%s-%s", mid, msgID, t.Format("20060102150405"))
}
