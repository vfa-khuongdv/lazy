package notification

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestNewSlackNotifier(t *testing.T) {
	config := SlackConfig{
		WebhookURL: "https://hooks.slack.com/test",
		Channel:    "#general",
		Username:   "backup-bot",
		IconEmoji:  ":robot_face:",
		IconURL:    "https://example.com/icon.png",
	}

	notifier := NewSlackNotifier(config)
	assert.NotNil(t, notifier)
	assert.Equal(t, config.WebhookURL, notifier.config.WebhookURL)
	assert.Equal(t, config.Channel, notifier.config.Channel)
	assert.Equal(t, config.Username, notifier.config.Username)
	assert.Equal(t, config.IconEmoji, notifier.config.IconEmoji)
	assert.Equal(t, config.IconURL, notifier.config.IconURL)
}

func TestSlackNotifier_GetChannelType(t *testing.T) {
	config := SlackConfig{WebhookURL: "https://hooks.slack.com/test"}
	notifier := NewSlackNotifier(config)

	channelType := notifier.GetChannelType()
	assert.Equal(t, ChannelSlack, channelType)
}

func TestSlackNotifier_ValidateConfig(t *testing.T) {
	notifier := &SlackNotifier{}

	tests := []struct {
		name        string
		config      map[string]interface{}
		expectError bool
	}{
		{
			name: "valid config",
			config: map[string]interface{}{
				"webhook_url": "https://hooks.slack.com/test",
			},
			expectError: false,
		},
		{
			name: "missing webhook_url",
			config: map[string]interface{}{
				"channel": "#general",
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

func TestSlackNotifier_Send(t *testing.T) {
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
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create test server
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				// Verify request headers
				assert.Equal(t, "application/json", r.Header.Get("Content-Type"))
				assert.Equal(t, "DB-Backup-GDrive/1.0", r.Header.Get("User-Agent"))

				// Verify request method
				assert.Equal(t, "POST", r.Method)

				// Parse and verify payload
				var payload SlackWebhookPayload
				err := json.NewDecoder(r.Body).Decode(&payload)
				assert.NoError(t, err)

				// Verify payload structure
				assert.NotEmpty(t, payload.Attachments)
				assert.Len(t, payload.Attachments, 1)

				attachment := payload.Attachments[0]
				assert.Equal(t, tt.message.Title, attachment.Title)

				w.WriteHeader(tt.serverResponse)
			}))
			defer server.Close()

			// Create notifier with test server URL
			config := SlackConfig{
				WebhookURL: server.URL,
				Channel:    "#test",
				Username:   "test-bot",
				IconEmoji:  ":test:",
			}
			notifier := NewSlackNotifier(config)

			// Send message
			err := notifier.Send(tt.message)
			if tt.expectError {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

func TestSlackNotifier_Send_NetworkError(t *testing.T) {
	config := SlackConfig{
		WebhookURL: "http://localhost:99999/invalid", // Use a port that's definitely not in use
	}
	notifier := NewSlackNotifier(config)

	message := &Message{
		Type:      MessageTypeInfo,
		Title:     "Test",
		Text:      "Test message",
		Timestamp: time.Now(),
	}

	err := notifier.Send(message)
	assert.Error(t, err)
}

func TestSlackNotifier_createPayload(t *testing.T) {
	config := SlackConfig{
		WebhookURL: "https://hooks.slack.com/test",
		Channel:    "#general",
		Username:   "backup-bot",
		IconEmoji:  ":robot_face:",
		IconURL:    "https://example.com/icon.png",
	}
	notifier := NewSlackNotifier(config)

	timestamp := time.Now()
	message := &Message{
		Type:       MessageTypeSuccess,
		Title:      "Test Title",
		Text:       "Test message",
		Timestamp:  timestamp,
		ConfigName: "test-config",
		Fields: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
	}

	payload := notifier.createPayload(message)

	// Verify basic payload structure
	assert.Equal(t, config.Channel, payload.Channel)
	assert.Equal(t, config.Username, payload.Username)
	assert.Equal(t, config.IconEmoji, payload.IconEmoji)
	assert.Equal(t, config.IconURL, payload.IconURL)

	// Verify attachments
	assert.Len(t, payload.Attachments, 1)

	attachment := payload.Attachments[0]
	assert.Equal(t, message.Title, attachment.Title)
	assert.Equal(t, message.Text, attachment.Text)
	assert.Equal(t, "good", attachment.Color)
	assert.Equal(t, "Database Backup Service", attachment.Footer)
	assert.Equal(t, timestamp.Unix(), attachment.Timestamp)

	// Verify fields (including message fields + config name)
	expectedFieldCount := len(message.Fields) + 1 // +1 for ConfigName
	assert.Len(t, attachment.Fields, expectedFieldCount)

	// Check if config name field exists
	configFieldFound := false
	for _, field := range attachment.Fields {
		if field.Title == "Configuration" && field.Value == message.ConfigName {
			configFieldFound = true
			break
		}
	}
	assert.True(t, configFieldFound)
}

func TestSlackNotifier_createPayload_EmptyConfig(t *testing.T) {
	config := SlackConfig{
		WebhookURL: "https://hooks.slack.com/test",
		// All optional fields empty
	}
	notifier := NewSlackNotifier(config)

	message := &Message{
		Type:      MessageTypeInfo,
		Title:     "Test",
		Text:      "Test message",
		Timestamp: time.Now(),
	}

	payload := notifier.createPayload(message)

	// Verify optional fields are not set
	assert.Empty(t, payload.Channel)
	assert.Empty(t, payload.Username)
	assert.Empty(t, payload.IconEmoji)
	assert.Empty(t, payload.IconURL)
}

func TestSlackNotifier_getColorForType(t *testing.T) {
	notifier := &SlackNotifier{}

	tests := []struct {
		msgType       MessageType
		expectedColor string
	}{
		{MessageTypeSuccess, "good"},
		{MessageTypeError, "danger"},
		{MessageTypeWarning, "warning"},
		{MessageTypeInfo, "#36a64f"},
		{MessageType("unknown"), "#808080"},
	}

	for _, tt := range tests {
		t.Run(string(tt.msgType), func(t *testing.T) {
			color := notifier.getColorForType(tt.msgType)
			assert.Equal(t, tt.expectedColor, color)
		})
	}
}

func TestCreateSlackBackupSuccessMessage(t *testing.T) {
	startTime := time.Now().Add(-5 * time.Minute)
	endTime := time.Now()

	data := &BackupNotificationData{
		ConfigName:   "test-db",
		DatabaseType: "mysql",
		BackupSize:   1024 * 1024, // 1MB
		FileName:     "backup.sql",
		WebViewLink:  "https://drive.google.com/file/d/123",
		StartedAt:    startTime,
		CompletedAt:  endTime,
	}

	message := CreateSlackBackupSuccessMessage(data)

	assert.Equal(t, MessageTypeSuccess, message.Type)

	expectedTitle := ":white_check_mark: Backup Completed: test-db"
	assert.Equal(t, expectedTitle, message.Title)

	expectedText := "Database backup completed successfully for *test-db*"
	assert.Equal(t, expectedText, message.Text)

	assert.Equal(t, data.ConfigName, message.ConfigName)
	assert.Equal(t, data.CompletedAt, message.Timestamp)

	// Verify fields
	expectedFields := []string{"Database Type", "File Name", "File Size", "Duration", "Google Drive"}
	for _, field := range expectedFields {
		assert.Contains(t, message.Fields, field)
	}

	// Verify Google Drive link format
	driveLink, exists := message.Fields["Google Drive"]
	assert.True(t, exists)

	expectedLink := "<https://drive.google.com/file/d/123|View File>"
	assert.Equal(t, expectedLink, driveLink)
}

func TestCreateSlackBackupSuccessMessage_NoWebViewLink(t *testing.T) {
	data := &BackupNotificationData{
		ConfigName:   "test-db",
		DatabaseType: "mysql",
		BackupSize:   1024,
		FileName:     "backup.sql",
		StartedAt:    time.Now().Add(-1 * time.Minute),
		CompletedAt:  time.Now(),
	}

	message := CreateSlackBackupSuccessMessage(data)

	// Verify Google Drive field is not present when WebViewLink is empty
	_, exists := message.Fields["Google Drive"]
	assert.False(t, exists)
}

func TestCreateSlackBackupErrorMessage(t *testing.T) {
	startTime := time.Now().Add(-3 * time.Minute)
	endTime := time.Now()

	data := &BackupNotificationData{
		ConfigName:   "test-db",
		DatabaseType: "postgresql",
		ErrorMessage: "Connection timeout",
		StartedAt:    startTime,
		CompletedAt:  endTime,
	}

	message := CreateSlackBackupErrorMessage(data)

	assert.Equal(t, MessageTypeError, message.Type)

	expectedTitle := ":x: Backup Failed: test-db"
	assert.Equal(t, expectedTitle, message.Title)

	expectedText := "Database backup failed for *test-db*"
	assert.Equal(t, expectedText, message.Text)

	assert.Equal(t, data.ConfigName, message.ConfigName)
	assert.Equal(t, data.CompletedAt, message.Timestamp)

	// Verify fields
	expectedFields := []string{"Database Type", "Duration", "Error"}
	for _, field := range expectedFields {
		assert.Contains(t, message.Fields, field)
	}

	// Verify error message
	assert.Equal(t, data.ErrorMessage, message.Fields["Error"])
}

func TestCreateSlackBackupErrorMessage_NoCompletedAt(t *testing.T) {
	startTime := time.Now().Add(-2 * time.Minute)

	data := &BackupNotificationData{
		ConfigName:   "test-db",
		DatabaseType: "mysql",
		ErrorMessage: "Disk full",
		StartedAt:    startTime,
		// CompletedAt is zero value
	}

	message := CreateSlackBackupErrorMessage(data)

	// Should use current time when CompletedAt is zero
	assert.False(t, message.Timestamp.IsZero())

	// Duration should be calculated from StartedAt to now
	duration, exists := message.Fields["Duration"]
	assert.True(t, exists)
	durationStr := duration.(string)
	assert.NotEqual(t, "0s", durationStr)
}

func TestSlackNotifier_Send_InvalidURL(t *testing.T) {
	config := SlackConfig{WebhookURL: "://invalid-url"}
	notifier := NewSlackNotifier(config)

	message := &Message{
		Type:      MessageTypeInfo,
		Title:     "Test",
		Text:      "Test message",
		Timestamp: time.Now(),
	}

	err := notifier.Send(message)
	assert.Error(t, err)
}

func TestSlackNotifier_createPayload_NoConfigName(t *testing.T) {
	config := SlackConfig{WebhookURL: "https://hooks.slack.com/test"}
	notifier := NewSlackNotifier(config)

	message := &Message{
		Type:      MessageTypeInfo,
		Title:     "Test",
		Text:      "Test message",
		Timestamp: time.Now(),
		// ConfigName is empty
	}

	payload := notifier.createPayload(message)

	attachment := payload.Attachments[0]

	// Should not have Configuration field when ConfigName is empty
	for _, field := range attachment.Fields {
		assert.NotEqual(t, "Configuration", field.Title)
	}
}

func TestSlackNotifier_createPayload_NoFields(t *testing.T) {
	config := SlackConfig{WebhookURL: "https://hooks.slack.com/test"}
	notifier := NewSlackNotifier(config)

	message := &Message{
		Type:      MessageTypeInfo,
		Title:     "Test",
		Text:      "Test message",
		Timestamp: time.Now(),
		// No Fields and no ConfigName
	}

	payload := notifier.createPayload(message)

	attachment := payload.Attachments[0]

	// Should have no fields
	assert.Len(t, attachment.Fields, 0)
}

func TestSlackNotifier_createPayload_WithFieldsNoConfigName(t *testing.T) {
	config := SlackConfig{WebhookURL: "https://hooks.slack.com/test"}
	notifier := NewSlackNotifier(config)

	message := &Message{
		Type:      MessageTypeInfo,
		Title:     "Test",
		Text:      "Test message",
		Timestamp: time.Now(),
		Fields: map[string]interface{}{
			"key1": "value1",
			"key2": 123,
		},
		// ConfigName is empty
	}

	payload := notifier.createPayload(message)

	attachment := payload.Attachments[0]

	// Should have only the message fields, no Configuration field
	assert.Len(t, attachment.Fields, len(message.Fields))

	// Verify no Configuration field
	for _, field := range attachment.Fields {
		assert.NotEqual(t, "Configuration", field.Title)
	}
}
