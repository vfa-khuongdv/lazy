package notification

import (
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// ChatworkNotifier implements the Notifier interface for Chatwork
type ChatworkNotifier struct {
	config ChatworkConfig
}

// NewChatworkNotifier creates a new Chatwork notifier
func NewChatworkNotifier(config ChatworkConfig) *ChatworkNotifier {
	return &ChatworkNotifier{
		config: config,
	}
}

// Send sends a notification message to Chatwork
func (c *ChatworkNotifier) Send(message *Message) error {
	// Format message for Chatwork
	text := c.formatMessage(message)

	// Prepare API request
	apiURL := fmt.Sprintf("https://api.chatwork.com/v2/rooms/%s/messages", c.config.RoomID)

	// Prepare form data
	data := url.Values{}
	data.Set("body", text)
	data.Set("self_unread", "0") // Don't mark as unread for sender

	// Create HTTP request
	req, err := http.NewRequest("POST", apiURL, strings.NewReader(data.Encode()))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("X-ChatWorkToken", c.config.APIToken)

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		log.Printf("Error sending request to Chatwork: %v", err)
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("chatwork API returned status %d", resp.StatusCode)
	}

	return nil
}

// ValidateConfig validates the Chatwork configuration
func (c *ChatworkNotifier) ValidateConfig(config map[string]interface{}) error {
	apiToken, ok := config["api_token"].(string)
	if !ok || apiToken == "" {
		return fmt.Errorf("api_token is required for Chatwork")
	}

	roomID, ok := config["room_id"].(string)
	if !ok || roomID == "" {
		return fmt.Errorf("room_id is required for Chatwork")
	}

	return nil
}

// GetChannelType returns the notification channel type
func (c *ChatworkNotifier) GetChannelType() NotificationChannel {
	return ChannelChatwork
}

// formatMessage formats a message for Chatwork
func (c *ChatworkNotifier) formatMessage(message *Message) string {
	var builder strings.Builder

	// === Title Section ===
	emoji := c.getEmojiForType(message.Type)
	builder.WriteString(fmt.Sprintf("[info][title]%s %s[/title]\n", emoji, message.Title))

	// Time
	builder.WriteString(fmt.Sprintf("⏰ Time: %s\n", message.Timestamp.Format("2006-01-02 15:04:05")))

	// Separator
	builder.WriteString("[hr]\n")

	// === Body Section ===
	if message.Text != "" {
		builder.WriteString(fmt.Sprintf("📝 %s\n", message.Text))
	}

	// Fields
	if len(message.Fields) > 0 {
		builder.WriteString("\n📌 Details:\n")
		for key, value := range message.Fields {
			builder.WriteString(fmt.Sprintf("• %s: %v\n", key, value))
		}
	}

	// Separator
	builder.WriteString("[hr]\n")

	// === Footer Section ===
	if message.ConfigName != "" {
		builder.WriteString(fmt.Sprintf("⚙️ Config: %s\n", message.ConfigName))
	}

	// Final footer
	builder.WriteString("[hr]\n")
	builder.WriteString("From Chatwork, sending all our love 💖🤝💬\n")

	// Close info tag
	builder.WriteString("[/info]")

	return builder.String()
}

// getEmojiForType returns an appropriate emoji for the message type
func (c *ChatworkNotifier) getEmojiForType(msgType MessageType) string {
	switch msgType {
	case MessageTypeSuccess:
		return "✅"
	case MessageTypeError:
		return "❌"
	case MessageTypeWarning:
		return "⚠️"
	case MessageTypeInfo:
		return "ℹ️"
	default:
		return "📝"
	}
}

// CreateBackupSuccessMessage creates a formatted success message for backup completion
func CreateBackupSuccessMessage(data *BackupNotificationData) *Message {
	duration := data.CompletedAt.Sub(data.StartedAt)

	fields := map[string]interface{}{
		"Database Type": data.DatabaseType,
		"File Name":     data.FileName,
		"File Size":     formatFileSize(data.BackupSize),
		"Duration":      duration.Round(time.Second).String(),
	}

	if data.WebViewLink != "" {
		fields["Google Drive Link"] = data.WebViewLink
	}

	return &Message{
		Type:       MessageTypeSuccess,
		Title:      fmt.Sprintf("Backup Completed: %s", data.ConfigName),
		Text:       fmt.Sprintf("Database backup completed successfully for %s", data.ConfigName),
		Fields:     fields,
		Timestamp:  data.CompletedAt,
		ConfigName: data.ConfigName,
	}
}

// CreateBackupErrorMessage creates a formatted error message for backup failure
func CreateBackupErrorMessage(data *BackupNotificationData) *Message {
	var duration time.Duration
	if !data.CompletedAt.IsZero() {
		duration = data.CompletedAt.Sub(data.StartedAt)
	} else {
		duration = time.Since(data.StartedAt)
	}

	fields := map[string]interface{}{
		"Database Type": data.DatabaseType,
		"Duration":      duration.Round(time.Second).String(),
		"Error":         data.ErrorMessage,
	}

	completedAt := data.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}

	return &Message{
		Type:       MessageTypeError,
		Title:      fmt.Sprintf("Backup Failed: %s", data.ConfigName),
		Text:       fmt.Sprintf("Database backup failed for %s", data.ConfigName),
		Fields:     fields,
		Timestamp:  completedAt,
		ConfigName: data.ConfigName,
	}
}

// formatFileSize formats a file size in bytes to a human-readable string
func formatFileSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
