package telkomsel

import (
	"os"
	"strings"
)

// GenerateCallbackURL builds the callback URL for order dealer async notification.
func GenerateCallbackURL() string {
	return strings.TrimSpace(os.Getenv("CALLBACK_URL_TELKOMSEL"))
}
