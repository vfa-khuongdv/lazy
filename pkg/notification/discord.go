package notification

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// DiscordNotifier implements the Notifier interface for Discord
type DiscordNotifier struct {
	config DiscordConfig
}

// NewDiscordNotifier creates a new Discord notifier
func NewDiscordNotifier(config DiscordConfig) *DiscordNotifier {
	return &DiscordNotifier{
		config: config,
	}
}

// DiscordWebhookPayload represents the payload structure for Discord webhooks
type DiscordWebhookPayload struct {
	Username  string         `json:"username,omitempty"`
	AvatarURL string         `json:"avatar_url,omitempty"`
	Content   string         `json:"content,omitempty"`
	Embeds    []DiscordEmbed `json:"embeds,omitempty"`
}

// DiscordEmbed represents an embed in Discord message
type DiscordEmbed struct {
	Title       string              `json:"title,omitempty"`
	Description string              `json:"description,omitempty"`
	Color       int                 `json:"color,omitempty"`
	Fields      []DiscordEmbedField `json:"fields,omitempty"`
	Footer      *DiscordEmbedFooter `json:"footer,omitempty"`
	Timestamp   string              `json:"timestamp,omitempty"`
}

// DiscordEmbedField represents a field in Discord embed
type DiscordEmbedField struct {
	Name   string `json:"name"`
	Value  string `json:"value"`
	Inline bool   `json:"inline,omitempty"`
}

// DiscordEmbedFooter represents footer in Discord embed
type DiscordEmbedFooter struct {
	Text    string `json:"text"`
	IconURL string `json:"icon_url,omitempty"`
}

// Send sends a notification message to Discord
func (d *DiscordNotifier) Send(message *Message) error {
	// Create Discord payload
	payload := d.createPayload(message)

	// Marshal to JSON
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal Discord payload: %w", err)
	}

	// Create HTTP request
	req, err := http.NewRequest("POST", d.config.WebhookURL, bytes.NewBuffer(jsonData))
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
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("discord webhook returned status %d", resp.StatusCode)
	}

	return nil
}

// ValidateConfig validates the Discord configuration
func (d *DiscordNotifier) ValidateConfig(config map[string]interface{}) error {
	webhookURL, ok := config["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return fmt.Errorf("webhook_url is required for Discord")
	}

	return nil
}

// GetChannelType returns the notification channel type
func (d *DiscordNotifier) GetChannelType() NotificationChannel {
	return ChannelDiscord
}

// createPayload creates a Discord webhook payload from a message
func (d *DiscordNotifier) createPayload(message *Message) *DiscordWebhookPayload {
	embed := DiscordEmbed{
		Title:       message.Title,
		Description: message.Text,
		Color:       d.getColorForType(message.Type),
		Timestamp:   message.Timestamp.Format(time.RFC3339),
		Footer: &DiscordEmbedFooter{
			Text: "Database Backup Service",
		},
	}

	// Add fields if present
	if len(message.Fields) > 0 {
		for key, value := range message.Fields {
			embed.Fields = append(embed.Fields, DiscordEmbedField{
				Name:   key,
				Value:  fmt.Sprintf("%v", value),
				Inline: true,
			})
		}
	}

	// Add configuration name if present
	if message.ConfigName != "" {
		embed.Fields = append(embed.Fields, DiscordEmbedField{
			Name:   "Configuration",
			Value:  message.ConfigName,
			Inline: true,
		})
	}

	payload := &DiscordWebhookPayload{
		Embeds: []DiscordEmbed{embed},
	}

	// Set username and avatar if configured
	if d.config.Username != "" {
		payload.Username = d.config.Username
	}
	if d.config.AvatarURL != "" {
		payload.AvatarURL = d.config.AvatarURL
	}

	return payload
}

// getColorForType returns an appropriate color for the message type
func (d *DiscordNotifier) getColorForType(msgType MessageType) int {
	switch msgType {
	case MessageTypeSuccess:
		return 0x00FF00 // Green
	case MessageTypeError:
		return 0xFF0000 // Red
	case MessageTypeWarning:
		return 0xFFFF00 // Yellow
	default:
		return 0x0099FF // Blue
	}
}

// CreateDiscordBackupSuccessMessage creates a Discord-optimized success message
func CreateDiscordBackupSuccessMessage(data *BackupNotificationData) *Message {
	duration := data.CompletedAt.Sub(data.StartedAt)

	fields := map[string]interface{}{
		"🗄️ Database Type": data.DatabaseType,
		"📁 File Name":      data.FileName,
		"📊 File Size":      formatFileSize(data.BackupSize),
		"⏱️ Duration":      duration.Round(time.Second).String(),
	}

	if data.WebViewLink != "" {
		fields["🔗 Google Drive"] = fmt.Sprintf("[View File](%s)", data.WebViewLink)
	}

	return &Message{
		Type:       MessageTypeSuccess,
		Title:      fmt.Sprintf("✅ Backup Completed: %s", data.ConfigName),
		Text:       fmt.Sprintf("Database backup completed successfully for **%s**", data.ConfigName),
		Fields:     fields,
		Timestamp:  data.CompletedAt,
		ConfigName: data.ConfigName,
	}
}

// CreateDiscordBackupErrorMessage creates a Discord-optimized error message
func CreateDiscordBackupErrorMessage(data *BackupNotificationData) *Message {
	var duration time.Duration
	if !data.CompletedAt.IsZero() {
		duration = data.CompletedAt.Sub(data.StartedAt)
	} else {
		duration = time.Since(data.StartedAt)
	}

	fields := map[string]interface{}{
		"🗄️ Database Type": data.DatabaseType,
		"⏱️ Duration":      duration.Round(time.Second).String(),
		"❌ Error":          data.ErrorMessage,
	}

	completedAt := data.CompletedAt
	if completedAt.IsZero() {
		completedAt = time.Now()
	}

	return &Message{
		Type:       MessageTypeError,
		Title:      fmt.Sprintf("❌ Backup Failed: %s", data.ConfigName),
		Text:       fmt.Sprintf("Database backup failed for **%s**", data.ConfigName),
		Fields:     fields,
		Timestamp:  completedAt,
		ConfigName: data.ConfigName,
	}
}
