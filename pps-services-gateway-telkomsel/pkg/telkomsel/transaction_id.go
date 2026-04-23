package telkomsel

import (
	"fmt"
	"strings"
	"time"
)

// GenerateTransactionID generates the Telkomsel transaction_id used in request payloads.
//
// This is intentionally exported so outer layers (e.g. the RabbitMQ consumer) can
// log/store our_trx_id using the exact same generated ID (instead of msgID), and
// also use it to correlate Telkomsel callbacks (transaction_id) to msg_id.
func GenerateTransactionID(mid string, msgID string, at time.Time) (string, error) {
	if strings.TrimSpace(mid) == "" {
		return "", fmt.Errorf("mid is required")
	}

	orgCode, _, err := organizationCodeAndPINFromMIDEnv(mid)
	if err != nil {
		return "", err
	}

	seq := deriveSequence(msgID)
	return buildTelkomselTransactionID(orgCode, at, seq), nil
}
