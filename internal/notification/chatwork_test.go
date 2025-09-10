package notification

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestFormatMessage(t *testing.T) {
	config := ChatworkConfig{APIToken: "dummy", RoomID: "123"}
	n := NewChatworkNotifier(config)
	msg := &Message{
		Type:       MessageTypeSuccess,
		Title:      "Test Title",
		Text:       "Test Body",
		Fields:     map[string]any{"Key": "Value"},
		Timestamp:  time.Now(),
		ConfigName: "TestConfig",
	}
	formatted := n.formatMessage(msg)
	assert.NotEmpty(t, formatted, "formatted message should not be empty")
}

func TestValidateConfig(t *testing.T) {
	c := &ChatworkNotifier{}

	valid := map[string]any{"api_token": "token", "room_id": "room"}
	err := c.ValidateConfig(valid)
	assert.NoError(t, err)

	invalid := map[string]any{"api_token": "", "room_id": ""}
	err = c.ValidateConfig(invalid)
	assert.Error(t, err)
}

func TestValidateConfig_MissingRoomID(t *testing.T) {
	c := &ChatworkNotifier{}
	config := map[string]any{"api_token": "token", "room_id": ""}
	err := c.ValidateConfig(config)
	assert.Error(t, err)
}

func TestValidateConfig_MissingAPIToken(t *testing.T) {
	c := &ChatworkNotifier{}
	config := map[string]any{"api_token": "", "room_id": "123456"}
	err := c.ValidateConfig(config)
	assert.Error(t, err)
}

func TestGetChannelType(t *testing.T) {
	c := &ChatworkNotifier{}
	assert.Equal(t, ChannelChatwork, c.GetChannelType())
}

func TestGetEmojiForType(t *testing.T) {
	c := &ChatworkNotifier{}
	assert.Equal(t, "✅", c.getEmojiForType(MessageTypeSuccess))
	assert.Equal(t, "❌", c.getEmojiForType(MessageTypeError))
	assert.Equal(t, "⚠️", c.getEmojiForType(MessageTypeWarning))
	assert.Equal(t, "ℹ️", c.getEmojiForType(MessageTypeInfo))
	assert.Equal(t, "📝", c.getEmojiForType("other"))
}

func TestCreateBackupSuccessMessage(t *testing.T) {
	data := &BackupNotificationData{
		ConfigName:   "test-config",
		DatabaseType: "mysql",
		BackupSize:   1024 * 1024,
		FileName:     "backup.sql",
		WebViewLink:  "https://drive.google.com/file",
		StartedAt:    time.Now().Add(-time.Minute),
		CompletedAt:  time.Now(),
	}
	msg := CreateBackupSuccessMessage(data)
	assert.Equal(t, MessageTypeSuccess, msg.Type)
	assert.Equal(t, "https://drive.google.com/file", msg.Fields["Google Drive Link"])
}

func TestCreateBackupErrorMessage(t *testing.T) {
	data := &BackupNotificationData{
		ConfigName:   "test-config",
		DatabaseType: "mysql",
		ErrorMessage: "error",
		StartedAt:    time.Now().Add(-time.Minute),
		CompletedAt:  time.Now(),
	}
	msg := CreateBackupErrorMessage(data)
	assert.Equal(t, MessageTypeError, msg.Type)
	assert.Equal(t, "error", msg.Fields["Error"])
}

func TestCreateBackupErrorMessage_CompletedAtZero(t *testing.T) {
	data := &BackupNotificationData{
		ConfigName:   "test-config",
		DatabaseType: "mysql",
		ErrorMessage: "error",
		StartedAt:    time.Now().Add(-time.Minute),
		CompletedAt:  time.Time{},
	}
	msg := CreateBackupErrorMessage(data)
	assert.Equal(t, MessageTypeError, msg.Type)
	assert.Equal(t, "error", msg.Fields["Error"])
}

func TestFormatFileSize(t *testing.T) {
	assert.Equal(t, "999 B", formatFileSize(999))
	assert.Equal(t, "1.0 KB", formatFileSize(1024))
	assert.Equal(t, "1.0 MB", formatFileSize(1024*1024))
	assert.Equal(t, "1.0 GB", formatFileSize(1024*1024*1024))
}

func TestSend_UnreachableAPI(t *testing.T) {
	config := ChatworkConfig{APIToken: "token", RoomID: "room"}
	n := &ChatworkNotifier{config: config}
	msg := &Message{
		Type:      MessageTypeInfo,
		Title:     "Test",
		Text:      "Body",
		Timestamp: time.Now(),
	}
	err := n.Send(msg)
	assert.Error(t, err, "expected error for unreachable API")
}
