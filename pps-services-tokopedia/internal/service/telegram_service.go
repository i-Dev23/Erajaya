package service

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"os"
	"time"
)

// TelegramService defines the interface for Telegram notification service
type TelegramService interface {
	SendMessage(ctx context.Context, message string) error
	SendErrorAlert(ctx context.Context, errorMsg string, metadata map[string]interface{}) error
}

// telegramServiceImpl implements TelegramService
type telegramServiceImpl struct {
	botToken string
	chatID   string
	client   *http.Client
}

// NewTelegramService creates a new Telegram service
func NewTelegramService(botToken, chatID string) TelegramService {
	return &telegramServiceImpl{
		botToken: botToken,
		chatID:   chatID,
		client: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// SendMessage sends a simple text message to Telegram
func (s *telegramServiceImpl) SendMessage(ctx context.Context, message string) error {
	// Skip if bot token or chat ID not configured
	if s.botToken == "" || s.chatID == "" {
		return nil // Silently skip, don't error
	}

	url := fmt.Sprintf("https://api.telegram.org/bot%s/sendMessage", s.botToken)

	payload := map[string]interface{}{
		"chat_id":    s.chatID,
		"text":       message,
		"parse_mode": "HTML",
	}

	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal telegram payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create telegram request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	resp, err := s.client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send telegram message: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("telegram API returned non-OK status: %d", resp.StatusCode)
	}

	return nil
}

// SendErrorAlert sends a formatted error alert to Telegram
func (s *telegramServiceImpl) SendErrorAlert(ctx context.Context, errorMsg string, metadata map[string]interface{}) error {
	// Format timestamp
	timestamp := time.Now().Format("2006-01-02 15:04:05")

	// Get server info
	hostname, _ := os.Hostname()
	localIP := getLocalIP()
	port := os.Getenv("APP_PORT")
	if port == "" {
		port = "3001" // default port
	}

	// Build message with HTML formatting
	message := "🚨 <b>ERROR ALERT</b> 🚨\n\n"
	message += fmt.Sprintf("⏰ <b>Time:</b> %s\n", timestamp)
	message += fmt.Sprintf("❌ <b>Error:</b> %s\n\n", errorMsg)

	// Add metadata if provided
	if len(metadata) > 0 {
		message += "<b>📋 Details:</b>\n"
		for key, value := range metadata {
			message += fmt.Sprintf("• <b>%s:</b> %v\n", key, value)
		}
	}

	// Add environment info
	message += "\n🖥 <b>Service:</b> PPS Services Tokopedia\n"
	message += fmt.Sprintf("🌐 <b>Server:</b> %s\n", hostname)
	message += fmt.Sprintf("📍 <b>IP:</b> %s\n", localIP)
	message += fmt.Sprintf("🔌 <b>Port:</b> %s", port)

	return s.SendMessage(ctx, message)
}

// getLocalIP returns the local IP address of the server
func getLocalIP() string {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "unknown"
	}

	for _, addr := range addrs {
		if ipnet, ok := addr.(*net.IPNet); ok && !ipnet.IP.IsLoopback() {
			if ipnet.IP.To4() != nil {
				return ipnet.IP.String()
			}
		}
	}

	return "unknown"
}
