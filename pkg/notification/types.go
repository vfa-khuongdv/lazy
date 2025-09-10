package notification

import (
	"time"
)

// NotificationChannel represents the type of notification channel
type NotificationChannel string

const (
	ChannelChatwork NotificationChannel = "chatwork"
	ChannelDiscord  NotificationChannel = "discord"
	ChannelSlack    NotificationChannel = "slack"
)

// MessageType represents the type of notification message
type MessageType string

const (
	MessageTypeSuccess MessageType = "success"
	MessageTypeError   MessageType = "error"
	MessageTypeInfo    MessageType = "info"
	MessageTypeWarning MessageType = "warning"
)

// Message represents a notification message to be sent
type Message struct {
	Type        MessageType            `json:"type"`
	Title       string                 `json:"title"`
	Text        string                 `json:"text"`
	Fields      map[string]interface{} `json:"fields,omitempty"`
	Timestamp   time.Time              `json:"timestamp"`
	ConfigName  string                 `json:"config_name,omitempty"`
	DatabaseURL string                 `json:"database_url,omitempty"`
}

// BackupNotificationData contains backup-specific data for notifications
type BackupNotificationData struct {
	ConfigName   string        `json:"config_name"`
	DatabaseType string        `json:"database_type"`
	BackupSize   int64         `json:"backup_size"`
	Duration     time.Duration `json:"duration"`
	FileName     string        `json:"file_name"`
	FileID       string        `json:"file_id,omitempty"`
	WebViewLink  string        `json:"web_view_link,omitempty"`
	ErrorMessage string        `json:"error_message,omitempty"`
	StartedAt    time.Time     `json:"started_at"`
	CompletedAt  time.Time     `json:"completed_at"`
}

// Notifier interface defines the methods that all notification implementations must provide
type Notifier interface {
	// Send sends a notification message
	Send(message *Message) error

	// ValidateConfig validates the configuration for this notifier
	ValidateConfig(config map[string]interface{}) error

	// GetChannelType returns the notification channel type
	GetChannelType() NotificationChannel
}

// ChatworkConfig holds Chatwork-specific configuration
type ChatworkConfig struct {
	APIToken string `json:"api_token"`
	RoomID   string `json:"room_id"`
}

// DiscordConfig holds Discord-specific configuration
type DiscordConfig struct {
	WebhookURL string `json:"webhook_url"`
	Username   string `json:"username,omitempty"`
	AvatarURL  string `json:"avatar_url,omitempty"`
}

// SlackConfig holds Slack-specific configuration
type SlackConfig struct {
	WebhookURL string `json:"webhook_url"`
	Channel    string `json:"channel,omitempty"`
	Username   string `json:"username,omitempty"`
	IconEmoji  string `json:"icon_emoji,omitempty"`
	IconURL    string `json:"icon_url,omitempty"`
}

// NotificationResult represents the result of sending a notification
type NotificationResult struct {
	Channel   NotificationChannel `json:"channel"`
	Success   bool                `json:"success"`
	Error     string              `json:"error,omitempty"`
	SentAt    time.Time           `json:"sent_at"`
	MessageID string              `json:"message_id,omitempty"`
}

type NotificationConfig struct {
	Name            string                 `json:"name"`
	Channel         string                 `json:"channel"`
	Config          map[string]interface{} `json:"config"`
	NotifyOnSuccess bool                   `json:"notify_on_success"`
	NotifyOnError   bool                   `json:"notify_on_error"`
	Enabled         bool                   `json:"enabled"`
}
