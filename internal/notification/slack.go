package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// SlackNotifier implements the Notifier interface for Slack
type SlackNotifier struct {
	config SlackConfig
}

// NewSlackNotifier creates a new Slack notifier
func NewSlackNotifier(config SlackConfig) *SlackNotifier {
	return &SlackNotifier{
		config: config,
	}
}

// SlackWebhookPayload represents the payload structure for Slack webhooks
type SlackWebhookPayload struct {
	Channel     string            `json:"channel,omitempty"`
	Username    string            `json:"username,omitempty"`
	IconEmoji   string            `json:"icon_emoji,omitempty"`
	IconURL     string            `json:"icon_url,omitempty"`
	Text        string            `json:"text,omitempty"`
	Attachments []SlackAttachment `json:"attachments,omitempty"`
}

// SlackAttachment represents an attachment in Slack message
type SlackAttachment struct {
	Color      string       `json:"color,omitempty"`
	Title      string       `json:"title,omitempty"`
	Text       string       `json:"text,omitempty"`
	Fields     []SlackField `json:"fields,omitempty"`
	Footer     string       `json:"footer,omitempty"`
	FooterIcon string       `json:"footer_icon,omitempty"`
	Timestamp  int64        `json:"ts,omitempty"`
}

// SlackField represents a field in Slack attachment
type SlackField struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}

// Send sends a notification message to Slack
func (s *SlackNotifier) Send(message *Message) error {
	// Create Slack payload
	payload := s.createPayload(message)

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Slack payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", s.config.WebhookURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	// Set headers
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "DB-Backup-GDrive/1.0")

	// Send request
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// Check response
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// ValidateConfig validates the Slack configuration
func (s *SlackNotifier) ValidateConfig(config map[string]interface{}) error {
	webhookURL, ok := config["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("webhook_url is required for Slack")
	}

	return nil
}

// GetChannelType returns the notification channel type
func (s *SlackNotifier) GetChannelType() NotificationChannel {
	return ChannelSlack
}

// createPayload creates a Slack webhook payload from a message
func (s *SlackNotifier) createPayload(message *Message) *SlackWebhookPayload {
	attachment := SlackAttachment{
		Color:     s.getColorForType(message.Type),
		Title:     message.Title,
		Text:      message.Text,
		Footer:    "Database Backup Service",
		Timestamp: message.Timestamp.Unix(),
	}

	// Add fields if present
	if len(message.Fields) > 0 {
		for key, value := range message.Fields {
			attachment.Fields = append(attachment.Fields, SlackField{
				Title: key,
				Value: fmt.Sprintf("%v", value),
				Short: true,
			})
		}
	}

	// Add configuration name if present
	if message.ConfigName != "" {
		attachment.Fields = append(attachment.Fields, SlackField{
			Title: "Configuration",
			Value: message.ConfigName,
			Short: true,
		})
	}

	payload := &SlackWebhookPayload{
		Attachments: []SlackAttachment{attachment},
	}

	// Set channel-specific config if provided
	if s.config.Channel != "" {
		payload.Channel = s.config.Channel
	}
	if s.config.Username != "" {
		payload.Username = s.config.Username
	}
	if s.config.IconEmoji != "" {
		payload.IconEmoji = s.config.IconEmoji
	}
	if s.config.IconURL != "" {
		payload.IconURL = s.config.IconURL
	}

	return payload
}

// getColorForType returns an appropriate color for the message type
func (s *SlackNotifier) getColorForType(msgType MessageType) string {
	switch msgType {
	case MessageTypeSuccess:
		return "good" // Green
	case MessageTypeError:
		return "danger" // Red
	case MessageTypeWarning:
		return "warning" // Yellow
	case MessageTypeInfo:
		return "#36a64f" // Blue-green
	default:
		return "#808080" // Gray
	}
}

// CreateSlackBackupSuccessMessage creates a Slack-optimized success message
func CreateSlackBackupSuccessMessage(data *BackupNotificationData) *Message {
	duration := data.CompletedAt.Sub(data.StartedAt)

	fields := map[string]interface{}{
		"Database Type": data.DatabaseType,
		"File Name":     data.FileName,
		"File Size":     formatFileSize(data.BackupSize),
		"Duration":      duration.Round(time.Second).String(),
	}

	if data.WebViewLink != "" {
		fields["Google Drive"] = fmt.Sprintf("<%s|View File>", data.WebViewLink)
	}

	return &Message{
		Type:       MessageTypeSuccess,
		Title:      fmt.Sprintf(":white_check_mark: Backup Completed: %s", data.ConfigName),
		Text:       fmt.Sprintf("Database backup completed successfully for *%s*", data.ConfigName),
		Fields:     fields,
		Timestamp:  data.CompletedAt,
		ConfigName: data.ConfigName,
	}
}

// CreateSlackBackupErrorMessage creates a Slack-optimized error message
func CreateSlackBackupErrorMessage(data *BackupNotificationData) *Message {
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
		Title:      fmt.Sprintf(":x: Backup Failed: %s", data.ConfigName),
		Text:       fmt.Sprintf("Database backup failed for *%s*", data.ConfigName),
		Fields:     fields,
		Timestamp:  completedAt,
		ConfigName: data.ConfigName,
	}
}
