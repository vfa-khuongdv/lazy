package notification

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewDiscordNotifier(t *testing.T) {
	config := DiscordConfig{
		WebhookURL: "https://discord.com/api/webhooks/test",
		Username:   "backup-bot",
		AvatarURL:  "https://example.com/avatar.png",
	}

	notifier := NewDiscordNotifier(config)

	assert.NotNil(t, notifier, "Expected notifier to be created")
	assert.Equal(t, config.WebhookURL, notifier.config.WebhookURL)
	assert.Equal(t, config.Username, notifier.config.Username)
	assert.Equal(t, config.AvatarURL, notifier.config.AvatarURL)
}

func TestDiscordNotifier_GetChannelType(t *testing.T) {
	config := DiscordConfig{WebhookURL: "https://discord.com/api/webhooks/test"}
	notifier := NewDiscordNotifier(config)

	assert.Equal(t, ChannelDiscord, notifier.GetChannelType())
}

func TestDiscordNotifier_ValidateConfig(t *testing.T) {
	notifier := &DiscordNotifier{}

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"webhook_url": "https://discord.com/api/webhooks/test",
			},
			expectError: false,
		},
		{
			name: "missing webhook_url",
			config: map[string]interface{}{
				"username": "bot",
			},
			expectError: true,
		},
		{
			name: "empty webhook_url",
			config: map[string]interface{}{
				"webhook_url": "",
			},
			expectError: true,
		},
		{
			name: "invalid webhook_url type",
			config: map[string]interface{}{
				"webhook_url": 123,
			},
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := notifier.ValidateConfig(tt.config)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestDiscordNotifier_Send(t *testing.T) {
	tests := []struct {
		name           string
		message        *Message
		serverResponse int
		expectError    bool
	}{
		{
			name: "successful send",
			message: &Message{
				Type:       MessageTypeSuccess,
				Title:      "Test Title",
				Text:       "Test message",
				Timestamp:  time.Now(),
				ConfigName: "test-config",
				Fields: map[string]interface{}{
					"key1": "value1",
					"key2": 123,
				},
			},
			serverResponse: http.StatusOK,
			expectError:    false,
		},
		{
			name: "successful send with 204",
			message: &Message{
				Type:      MessageTypeInfo,
				Title:     "Info Title",
				Text:      "Info message",
				Timestamp: time.Now(),
			},
			serverResponse: http.StatusNoContent,
			expectError:    false,
		},
		{
			name: "server error",
			message: &Message{
				Type:      MessageTypeError,
				Title:     "Error Title",
				Text:      "Error message",
				Timestamp: time.Now(),
			},
			serverResponse: http.StatusInternalServerError,
			expectError:    true,
		},
		{
			name: "bad request",
			message: &Message{
				Type:      MessageTypeWarning,
				Title:     "Warning Title",
				Text:      "Warning message",
				Timestamp: time.Now(),
			},
			serverResponse: http.StatusBadRequest,
			expectError:    true,
		},
		{
			name: "rate limited",
			message: &Message{
				Type:      MessageTypeInfo,
				Title:     "Test Title",
				Text:      "Test message",
				Timestamp: time.Now(),
			},
			serverResponse: http.StatusTooManyRequests,
			expectError:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "DB-Backup-GDrive/1.0", r.Header.Get("User-Agent"))
				assert.Equal(t, "POST", r.Method)

				var payload DiscordWebhookPayload
				err := json.NewDecoder(r.Body).Decode(&payload)
				assert.NoError(t, err)
				assert.NotEmpty(t, payload.Embeds)

				embed := payload.Embeds[0]
				assert.Equal(t, tt.message.Title, embed.Title)

				w.WriteHeader(tt.serverResponse)
			}))
			defer server.Close()

			config := DiscordConfig{
				WebhookURL: server.URL,
				Username:   "test-bot",
				AvatarURL:  "https://example.com/avatar.png",
			}
			notifier := NewDiscordNotifier(config)

			err := notifier.Send(tt.message)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestCreateDiscordBackupSuccessMessage(t *testing.T) {
	data := &BackupNotificationData{
		ConfigName:   "test-config",
		DatabaseType: "mysql",
		BackupSize:   1024 * 1024,
		FileName:     "backup.sql",
		WebViewLink:  "https://drive.google.com/file/d/12345/view",
	}

	message := CreateDiscordBackupSuccessMessage(data)

	assert.NotEmpty(t, message, "message should not be empty")
	assert.Equal(t, MessageTypeSuccess, message.Type)
	assert.Contains(t, message.Title, "✅ Backup Completed: test-config")
	assert.Contains(t, message.Text, "Database backup completed successfully for **test-config**")
	assert.Contains(t, message.Fields["🗄️ Database Type"], "mysql")
	assert.Contains(t, message.Fields["📊 File Size"], "1.0 MB")
	assert.Contains(t, message.Fields["📁 File Name"], "backup.sql")
	assert.Contains(t, message.Fields["🔗 Google Drive"], "[View File](https://drive.google.com/file/d/12345/view)")
}

func TestCreateDiscordBackupErrorMessage_CompletedAtZero(t *testing.T) {
	data := &BackupNotificationData{
		ConfigName:   "test-config",
		DatabaseType: "mysql",
		ErrorMessage: "error",
		StartedAt:    time.Now().Add(-time.Minute),
		CompletedAt:  time.Time{},
	}
	msg := CreateDiscordBackupErrorMessage(data)
	assert.Equal(t, MessageTypeError, msg.Type)
	assert.Equal(t, "error", msg.Fields["❌ Error"])
	assert.Contains(t, msg.Title, "❌ Backup Failed: test-config")
	assert.Contains(t, msg.Text, "Database backup failed for **test-config**")
	assert.Contains(t, msg.Fields["🗄️ Database Type"], "mysql")
	assert.Contains(t, msg.Fields["❌ Error"], "error")
	assert.Contains(t, msg.Fields["⏱️ Duration"], "1m0s")
}
