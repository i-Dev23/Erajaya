// Package logger provides logging helpers separated from config to avoid import cycles.
package logger

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"

	"github.com/rs/zerolog"
	"github.com/spf13/viper"
)

// ContextLogger returns a logger enriched with a trace_id field.
func ContextLogger(log zerolog.Logger, traceID string) zerolog.Logger {
	return log.With().Str("trace_id", traceID).Logger()
}

// SendTelegramLog sends a fire-and-forget notification to a Telegram bot.
func SendTelegramLog(config *viper.Viper, log zerolog.Logger, message string) {
	if !config.GetBool("log.telegram.enabled") {
		return
	}

	botToken := config.GetString("log.telegram.bot_token")
	chatID := config.GetString("log.telegram.chat_id")

	if botToken == "" || chatID == "" {
		log.Warn().Msg("Telegram bot_token or chat_id is empty, skipping notification")
		return
	}

	apiURL := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", botToken)

	resp, err := http.PostForm(apiURL, url.Values{
		"chat_id": {chatID},
		"text":    {message},
	})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send Telegram notification")
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Error().Msgf("Telegram API returned status %d", resp.StatusCode)
	}
}

var piiPatterns = []struct {
	regex       *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile(`(\+62|62|0)8[1-9][0-9]{6,10}`), "***PHONE***"},
	{regexp.MustCompile(`[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`), "***EMAIL***"},
	{regexp.MustCompile(`\b[0-9]{16}\b`), "***NIK***"},
}

// MaskPII replaces PII patterns (phone, email, NIK) in a string.
func MaskPII(s string) string {
	result := s
	for _, p := range piiPatterns {
		result = p.regex.ReplaceAllString(result, p.replacement)
	}
	return result
}
