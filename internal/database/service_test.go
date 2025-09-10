package database

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

type ServiceTestSuite struct {
	suite.Suite
	service *Service
	db      *gorm.DB
}

func (suite *ServiceTestSuite) SetupTest() {
	// Use in-memory SQLite database for testing
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	suite.NoError(err)

	// Run migrations
	err = AutoMigrate(db)
	suite.NoError(err)

	suite.db = db
	suite.service = &Service{db: db}
}

func (suite *ServiceTestSuite) TearDownTest() {
	if suite.db != nil {
		sqlDB, err := suite.db.DB()
		if err == nil {
			sqlDB.Close()
		}
	}
}

func TestServiceTestSuite(t *testing.T) {
	suite.Run(t, new(ServiceTestSuite))
}

// Test NewService with valid configuration
func (suite *ServiceTestSuite) TestNewService_ValidConfig() {
	config := &ServiceMySQLConfig{
		Host:     "localhost",
		Port:     "3306",
		User:     "test",
		Password: "password",
		Database: "testdb",
	}

	// Note: This will fail to connect to actual MySQL, which is expected in unit tests
	// We're testing the validation and structure creation
	service, err := NewService(config)
	suite.Error(err) // Expected to fail due to no actual MySQL connection
	suite.Nil(service)
	suite.Contains(err.Error(), "failed to connect to MySQL database")
}

// Test NewService with nil configuration
func (suite *ServiceTestSuite) TestNewService_NilConfig() {
	service, err := NewService(nil)
	suite.Error(err)
	suite.Nil(service)
	suite.Equal("MySQL configuration is required", err.Error())
}

// Test NewService with invalid configuration
func (suite *ServiceTestSuite) TestNewService_InvalidConfig() {
	config := &ServiceMySQLConfig{
		Host: "localhost",
		// Missing required fields
	}

	service, err := NewService(config)
	suite.Error(err)
	suite.Nil(service)
	suite.Contains(err.Error(), "invalid MySQL configuration")
}

// Test GetDB
func (suite *ServiceTestSuite) TestGetDB() {
	db := suite.service.GetDB()
	suite.NotNil(db)
	suite.Equal(suite.db, db)
}

// Test SaveTokenConfig - Create new
func (suite *ServiceTestSuite) TestSaveTokenConfig_CreateNew() {
	config := &TokenConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	err := suite.service.SaveTokenConfig(config)
	suite.NoError(err)
	suite.NotZero(config.ID)
}

// Test SaveTokenConfig - Update existing
func (suite *ServiceTestSuite) TestSaveTokenConfig_UpdateExisting() {
	// Create initial config
	config := &TokenConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	err := suite.service.SaveTokenConfig(config)
	suite.NoError(err)
	originalID := config.ID

	// Update with new values
	newConfig := &TokenConfig{
		ClientID:     "updated-client-id",
		ClientSecret: "updated-client-secret",
		AccessToken:  "updated-access-token",
		RefreshToken: "updated-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(2 * time.Hour),
	}

	err = suite.service.SaveTokenConfig(newConfig)
	suite.NoError(err)
	suite.Equal(originalID, newConfig.ID) // Should keep the same ID
}

// Test GetTokenConfig - Success
func (suite *ServiceTestSuite) TestGetTokenConfig_Success() {
	// Create config first
	config := &TokenConfig{
		ClientID:     "test-client-id",
		ClientSecret: "test-client-secret",
		AccessToken:  "test-access-token",
		RefreshToken: "test-refresh-token",
		TokenType:    "Bearer",
		Expiry:       time.Now().Add(time.Hour),
	}

	err := suite.service.SaveTokenConfig(config)
	suite.NoError(err)

	// Retrieve config
	retrieved, err := suite.service.GetTokenConfig()
	suite.NoError(err)
	suite.Equal(config.ClientID, retrieved.ClientID)
	suite.Equal(config.AccessToken, retrieved.AccessToken)
}

// Test GetTokenConfig - Not found
func (suite *ServiceTestSuite) TestGetTokenConfig_NotFound() {
	retrieved, err := suite.service.GetTokenConfig()
	suite.Error(err)
	suite.Nil(retrieved)
}

// Test SaveBackupHistory
func (suite *ServiceTestSuite) TestSaveBackupHistory() {
	history := &BackupHistory{
		DatabaseURL: "mysql://user:pass@localhost:3306/testdb",
		BackupType:  "mysql",
		FileName:    "backup_20231201.sql",
		FileID:      "drive-file-id",
		FileSize:    1024000,
		Status:      "success",
		StartedAt:   time.Now(),
		CompletedAt: func() *time.Time { t := time.Now().Add(time.Minute); return &t }(),
	}

	err := suite.service.SaveBackupHistory(history)
	suite.NoError(err)
	suite.NotZero(history.ID)
}

// Test UpdateBackupHistory
func (suite *ServiceTestSuite) TestUpdateBackupHistory() {
	// Create initial history
	history := &BackupHistory{
		DatabaseURL: "mysql://user:pass@localhost:3306/testdb",
		BackupType:  "mysql",
		FileName:    "backup_20231201.sql",
		Status:      "in_progress",
		StartedAt:   time.Now(),
	}

	err := suite.service.SaveBackupHistory(history)
	suite.NoError(err)

	// Update history
	history.Status = "success"
	history.FileID = "drive-file-id"
	history.FileSize = 1024000
	completedAt := time.Now()
	history.CompletedAt = &completedAt

	err = suite.service.UpdateBackupHistory(history)
	suite.NoError(err)

	// Verify update
	var retrieved BackupHistory
	err = suite.db.First(&retrieved, history.ID).Error
	suite.NoError(err)
	suite.Equal("success", retrieved.Status)
	suite.Equal("drive-file-id", retrieved.FileID)
}

// Test GetBackupHistory
func (suite *ServiceTestSuite) TestGetBackupHistory() {
	// Create multiple history records
	histories := []BackupHistory{
		{
			DatabaseURL: "mysql://user:pass@localhost:3306/testdb1",
			BackupType:  "mysql",
			FileName:    "backup1.sql",
			Status:      "success",
			StartedAt:   time.Now().Add(-2 * time.Hour),
		},
		{
			DatabaseURL: "mysql://user:pass@localhost:3306/testdb2",
			BackupType:  "mysql",
			FileName:    "backup2.sql",
			Status:      "success",
			StartedAt:   time.Now().Add(-1 * time.Hour),
		},
		{
			DatabaseURL: "mysql://user:pass@localhost:3306/testdb3",
			BackupType:  "mysql",
			FileName:    "backup3.sql",
			Status:      "failed",
			StartedAt:   time.Now(),
		},
	}

	for i := range histories {
		err := suite.service.SaveBackupHistory(&histories[i])
		suite.NoError(err)
	}

	// Test pagination
	retrieved, err := suite.service.GetBackupHistory(2, 0)
	suite.NoError(err)
	suite.Len(retrieved, 2)

	// Should be ordered by created_at DESC, so most recent first
	suite.Equal("backup3.sql", retrieved[0].FileName)
	suite.Equal("backup2.sql", retrieved[1].FileName)

	// Test offset
	retrieved, err = suite.service.GetBackupHistory(2, 1)
	suite.NoError(err)
	suite.Len(retrieved, 2)
	suite.Equal("backup2.sql", retrieved[0].FileName)
	suite.Equal("backup1.sql", retrieved[1].FileName)
}

// Test SaveBackupConfig - Create new
func (suite *ServiceTestSuite) TestSaveBackupConfig_CreateNew() {
	config := &BackupConfig{
		Name:         "daily-backup",
		DatabaseURL:  "mysql://user:pass@localhost:3306/testdb",
		DatabaseType: "mysql",
		CronSchedule: "0 2 * * *",
		Enabled:      true,
	}

	err := suite.service.SaveBackupConfig(config)
	suite.NoError(err)
	suite.NotZero(config.ID)
}

// Test SaveBackupConfig - Update existing
func (suite *ServiceTestSuite) TestSaveBackupConfig_UpdateExisting() {
	// Create initial config
	config := &BackupConfig{
		Name:         "daily-backup",
		DatabaseURL:  "mysql://user:pass@localhost:3306/testdb",
		DatabaseType: "mysql",
		CronSchedule: "0 2 * * *",
		Enabled:      true,
	}

	err := suite.service.SaveBackupConfig(config)
	suite.NoError(err)
	originalID := config.ID

	// Update with new values
	newConfig := &BackupConfig{
		Name:         "daily-backup", // Same name
		DatabaseURL:  "mysql://user:pass@localhost:3306/updated_db",
		DatabaseType: "mysql",
		CronSchedule: "0 3 * * *", // Different schedule
		Enabled:      false,
	}

	err = suite.service.SaveBackupConfig(newConfig)
	suite.NoError(err)
	suite.Equal(originalID, newConfig.ID)
}

// Test GetBackupConfigs
func (suite *ServiceTestSuite) TestGetBackupConfigs() {
	// Create enabled configs
	enabledConfigs := []BackupConfig{
		{
			Name:         "daily-backup",
			DatabaseURL:  "mysql://user:pass@localhost:3306/testdb1",
			DatabaseType: "mysql",
			CronSchedule: "0 2 * * *",
			Enabled:      true,
		},
		{
			Name:         "weekly-backup",
			DatabaseURL:  "mysql://user:pass@localhost:3306/testdb2",
			DatabaseType: "mysql",
			CronSchedule: "0 2 * * 0",
			Enabled:      true,
		},
	}

	for i := range enabledConfigs {
		err := suite.service.SaveBackupConfig(&enabledConfigs[i])
		suite.NoError(err)
	}

	// Create a disabled config
	disabledConfig := BackupConfig{
		Name:         "disabled-backup",
		DatabaseURL:  "mysql://user:pass@localhost:3306/testdb3",
		DatabaseType: "mysql",
		CronSchedule: "0 2 * * *",
		Enabled:      false,
	}
	err := suite.service.SaveBackupConfig(&disabledConfig)
	suite.NoError(err)

	// Manually update the enabled field to ensure it's disabled
	// (due to GORM default:true behavior)
	err = suite.db.Model(&disabledConfig).Update("enabled", false).Error
	suite.NoError(err)

	// Get enabled configs only (GetBackupConfigs already filters by enabled=true)
	retrieved, err := suite.service.GetBackupConfigs()
	suite.NoError(err)
	// Should only get 2 configs since GetBackupConfigs filters by enabled=true
	suite.Len(retrieved, 2) // Only enabled configs
}

// Test GetBackupConfigByName - Success
func (suite *ServiceTestSuite) TestGetBackupConfigByName_Success() {
	config := &BackupConfig{
		Name:         "test-backup",
		DatabaseURL:  "mysql://user:pass@localhost:3306/testdb",
		DatabaseType: "mysql",
		CronSchedule: "0 2 * * *",
		Enabled:      true,
	}

	err := suite.service.SaveBackupConfig(config)
	suite.NoError(err)

	retrieved, err := suite.service.GetBackupConfigByName("test-backup")
	suite.NoError(err)
	suite.Equal(config.Name, retrieved.Name)
	suite.Equal(config.DatabaseURL, retrieved.DatabaseURL)
}

// Test GetBackupConfigByName - Not found
func (suite *ServiceTestSuite) TestGetBackupConfigByName_NotFound() {
	retrieved, err := suite.service.GetBackupConfigByName("non-existent")
	suite.Error(err)
	suite.Nil(retrieved)
}

// Test UpdateBackupConfig
func (suite *ServiceTestSuite) TestUpdateBackupConfig() {
	config := &BackupConfig{
		Name:         "test-backup",
		DatabaseURL:  "mysql://user:pass@localhost:3306/testdb",
		DatabaseType: "mysql",
		CronSchedule: "0 2 * * *",
		Enabled:      true,
	}

	err := suite.service.SaveBackupConfig(config)
	suite.NoError(err)

	// Update config
	config.CronSchedule = "0 3 * * *"
	config.Enabled = false

	err = suite.service.UpdateBackupConfig(config)
	suite.NoError(err)

	// Verify update
	retrieved, err := suite.service.GetBackupConfigByName("test-backup")
	suite.NoError(err)
	suite.Equal("0 3 * * *", retrieved.CronSchedule)
	suite.False(retrieved.Enabled)
}

// Test DeleteBackupConfig
func (suite *ServiceTestSuite) TestDeleteBackupConfig() {
	config := &BackupConfig{
		Name:         "test-backup",
		DatabaseURL:  "mysql://user:pass@localhost:3306/testdb",
		DatabaseType: "mysql",
		CronSchedule: "0 2 * * *",
		Enabled:      true,
	}

	err := suite.service.SaveBackupConfig(config)
	suite.NoError(err)

	// Delete config
	err = suite.service.DeleteBackupConfig("test-backup")
	suite.NoError(err)

	// Verify deletion
	retrieved, err := suite.service.GetBackupConfigByName("test-backup")
	suite.Error(err)
	suite.Nil(retrieved)
}

// Test Close
func (suite *ServiceTestSuite) TestClose() {
	err := suite.service.Close()
	suite.NoError(err)
}

// Test SaveNotificationConfig - Create new
func (suite *ServiceTestSuite) TestSaveNotificationConfig_CreateNew() {
	config := &NotificationConfig{
		Name:            "slack-notifications",
		Channel:         "slack",
		Enabled:         true,
		Config:          map[string]interface{}{"webhook_url": "https://hooks.slack.com/test"},
		NotifyOnSuccess: true,
		NotifyOnError:   true,
	}

	err := suite.service.SaveNotificationConfig(config)
	suite.NoError(err)
	suite.NotZero(config.ID)
}

// Test SaveNotificationConfig - Update existing
func (suite *ServiceTestSuite) TestSaveNotificationConfig_UpdateExisting() {
	// Create initial config
	config := &NotificationConfig{
		Name:            "slack-notifications",
		Channel:         "slack",
		Enabled:         true,
		Config:          map[string]interface{}{"webhook_url": "https://hooks.slack.com/test"},
		NotifyOnSuccess: true,
		NotifyOnError:   true,
	}

	err := suite.service.SaveNotificationConfig(config)
	suite.NoError(err)
	originalID := config.ID

	// Update with new values
	newConfig := &NotificationConfig{
		Name:            "slack-notifications", // Same name
		Channel:         "slack",
		Enabled:         false,
		Config:          map[string]interface{}{"webhook_url": "https://hooks.slack.com/updated"},
		NotifyOnSuccess: false,
		NotifyOnError:   true,
	}

	err = suite.service.SaveNotificationConfig(newConfig)
	suite.NoError(err)
	suite.Equal(originalID, newConfig.ID)
}

// Test GetNotificationConfigs
func (suite *ServiceTestSuite) TestGetNotificationConfigs() {
	configs := []NotificationConfig{
		{
			Name:            "slack-notifications",
			Channel:         "slack",
			Enabled:         true,
			Config:          map[string]interface{}{"webhook_url": "https://hooks.slack.com/test"},
			NotifyOnSuccess: true,
			NotifyOnError:   true,
		},
		{
			Name:            "discord-notifications",
			Channel:         "discord",
			Enabled:         false,
			Config:          map[string]interface{}{"webhook_url": "https://discord.com/api/webhooks/test"},
			NotifyOnSuccess: true,
			NotifyOnError:   true,
		},
	}

	for i := range configs {
		err := suite.service.SaveNotificationConfig(&configs[i])
		suite.NoError(err)
	}

	// Get all configs
	retrieved, err := suite.service.GetNotificationConfigs()
	suite.NoError(err)
	suite.Len(retrieved, 2)
}

// Test GetEnabledNotificationConfigs
func (suite *ServiceTestSuite) TestGetEnabledNotificationConfigs() {
	// Create enabled config
	enabledConfig := NotificationConfig{
		Name:            "slack-notifications",
		Channel:         "slack",
		Enabled:         true,
		Config:          map[string]interface{}{"webhook_url": "https://hooks.slack.com/test"},
		NotifyOnSuccess: true,
		NotifyOnError:   true,
	}
	err := suite.service.SaveNotificationConfig(&enabledConfig)
	suite.NoError(err)

	// Create disabled config
	disabledConfig := NotificationConfig{
		Name:            "discord-notifications",
		Channel:         "discord",
		Enabled:         false,
		Config:          map[string]interface{}{"webhook_url": "https://discord.com/api/webhooks/test"},
		NotifyOnSuccess: true,
		NotifyOnError:   true,
	}
	err = suite.service.SaveNotificationConfig(&disabledConfig)
	suite.NoError(err)

	// Manually update the enabled field to ensure it's disabled
	// (due to GORM default:true behavior)
	err = suite.db.Model(&disabledConfig).Update("enabled", false).Error
	suite.NoError(err)

	// Get enabled configs only (GetEnabledNotificationConfigs already filters by enabled=true)
	retrieved, err := suite.service.GetEnabledNotificationConfigs()
	suite.NoError(err)
	// Should only get 1 config since GetEnabledNotificationConfigs filters by enabled=true
	suite.Len(retrieved, 1) // Only enabled configs
	suite.Equal("slack-notifications", retrieved[0].Name)
}

// Test GetNotificationConfigByName - Success
func (suite *ServiceTestSuite) TestGetNotificationConfigByName_Success() {
	config := &NotificationConfig{
		Name:            "test-notifications",
		Channel:         "slack",
		Enabled:         true,
		Config:          map[string]interface{}{"webhook_url": "https://hooks.slack.com/test"},
		NotifyOnSuccess: true,
		NotifyOnError:   true,
	}

	err := suite.service.SaveNotificationConfig(config)
	suite.NoError(err)

	retrieved, err := suite.service.GetNotificationConfigByName("test-notifications")
	suite.NoError(err)
	suite.Equal(config.Name, retrieved.Name)
	suite.Equal(config.Channel, retrieved.Channel)
}

// Test GetNotificationConfigByName - Not found
func (suite *ServiceTestSuite) TestGetNotificationConfigByName_NotFound() {
	retrieved, err := suite.service.GetNotificationConfigByName("non-existent")
	suite.Error(err)
	suite.Nil(retrieved)
}

// Test UpdateNotificationConfig
func (suite *ServiceTestSuite) TestUpdateNotificationConfig() {
	config := &NotificationConfig{
		Name:            "test-notifications",
		Channel:         "slack",
		Enabled:         true,
		Config:          map[string]interface{}{"webhook_url": "https://hooks.slack.com/test"},
		NotifyOnSuccess: true,
		NotifyOnError:   true,
	}

	err := suite.service.SaveNotificationConfig(config)
	suite.NoError(err)

	// Update config
	config.Enabled = false
	config.Config = map[string]interface{}{"webhook_url": "https://hooks.slack.com/updated"}

	err = suite.service.UpdateNotificationConfig(config)
	suite.NoError(err)

	// Verify update
	retrieved, err := suite.service.GetNotificationConfigByName("test-notifications")
	suite.NoError(err)
	suite.False(retrieved.Enabled)
	suite.Equal("https://hooks.slack.com/updated", retrieved.Config["webhook_url"])
}

// Test DeleteNotificationConfig
func (suite *ServiceTestSuite) TestDeleteNotificationConfig() {
	config := &NotificationConfig{
		Name:            "test-notifications",
		Channel:         "slack",
		Enabled:         true,
		Config:          map[string]interface{}{"webhook_url": "https://hooks.slack.com/test"},
		NotifyOnSuccess: true,
		NotifyOnError:   true,
	}

	err := suite.service.SaveNotificationConfig(config)
	suite.NoError(err)

	// Delete config
	err = suite.service.DeleteNotificationConfig("test-notifications")
	suite.NoError(err)

	// Verify deletion
	retrieved, err := suite.service.GetNotificationConfigByName("test-notifications")
	suite.Error(err)
	suite.Nil(retrieved)
}

// Additional tests for ServiceMySQLConfig validation

func TestServiceMySQLConfig_Validate(t *testing.T) {
	tests := []struct {
		name    string
		config  ServiceMySQLConfig
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid config",
			config: ServiceMySQLConfig{
				Host:     "localhost",
				Port:     "3306",
				User:     "root",
				Password: "password",
				Database: "testdb",
			},
			wantErr: false,
		},
		{
			name: "missing host",
			config: ServiceMySQLConfig{
				Port:     "3306",
				User:     "root",
				Password: "password",
				Database: "testdb",
			},
			wantErr: true,
			errMsg:  "host is required",
		},
		{
			name: "missing port",
			config: ServiceMySQLConfig{
				Host:     "localhost",
				User:     "root",
				Password: "password",
				Database: "testdb",
			},
			wantErr: true,
			errMsg:  "port is required",
		},
		{
			name: "missing user",
			config: ServiceMySQLConfig{
				Host:     "localhost",
				Port:     "3306",
				Password: "password",
				Database: "testdb",
			},
			wantErr: true,
			errMsg:  "user is required",
		},
		{
			name: "missing database",
			config: ServiceMySQLConfig{
				Host:     "localhost",
				Port:     "3306",
				User:     "root",
				Password: "password",
			},
			wantErr: true,
			errMsg:  "database is required",
		},
		{
			name: "empty password allowed",
			config: ServiceMySQLConfig{
				Host:     "localhost",
				Port:     "3306",
				User:     "root",
				Database: "testdb",
			},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.wantErr {
				assert.Error(t, err)
				assert.Contains(t, err.Error(), tt.errMsg)
			} else {
				assert.NoError(t, err)
			}
		})
	}
}

// Test AutoMigrate function
func TestAutoMigrate(t *testing.T) {
	db, err := gorm.Open(sqlite.Open(":memory:"), &gorm.Config{})
	assert.NoError(t, err)

	err = AutoMigrate(db)
	assert.NoError(t, err)

	// Verify tables were created
	assert.True(t, db.Migrator().HasTable(&TokenConfig{}))
	assert.True(t, db.Migrator().HasTable(&BackupHistory{}))
	assert.True(t, db.Migrator().HasTable(&BackupConfig{}))
	assert.True(t, db.Migrator().HasTable(&NotificationConfig{}))
}
