package backup

import (
	"fmt"
	"log"
	"time"

	"github.com/vfa-khuongdv/go-backup-drive/internal/auth"
	"github.com/vfa-khuongdv/go-backup-drive/internal/backup"
	"github.com/vfa-khuongdv/go-backup-drive/internal/database"
	"github.com/vfa-khuongdv/go-backup-drive/internal/notification"
	"github.com/vfa-khuongdv/go-backup-drive/internal/scheduler"
	"github.com/vfa-khuongdv/go-backup-drive/pkg/gdrive"
)

// BackupManager is the main interface for the backup package
type BackupManager struct {
	dbService        *database.Service
	authService      *auth.Service
	driveService     *gdrive.Service
	schedulerService *scheduler.Service
	config           *Config
}

// Config holds the configuration for the backup manager
type Config struct {
	// Google OAuth2 credentials
	ClientID     string
	ClientSecret string
	RedirectURL  string

	// MySQL database configuration for storing package metadata
	DatabaseConfig *backup.MySQLConfig

	// Backup temporary directory (optional, uses system temp by default)
	TempDir string
}

// NewMySQLConfig creates a new MySQL configuration
func NewMySQLConfig(host, port, user, password, database string) *backup.MySQLConfig {
	return &backup.MySQLConfig{
		Host:     host,
		Port:     port,
		User:     user,
		Password: password,
		Database: database,
	}
}

// NewBackupManager creates a new backup manager instance
func NewBackupManager(config *Config) (*BackupManager, error) {
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.ClientID == "" || config.ClientSecret == "" || config.RedirectURL == "" {
		return nil, fmt.Errorf("google OAuth2 ClientID, ClientSecret, and RedirectURL are required")
	}

	// Initialize database service (uses MySQL for configuration storage)
	if config.DatabaseConfig == nil {
		return nil, fmt.Errorf("MySQL configuration database is required")
	}

	// Convert to database service config
	serviceConfig := &database.ServiceMySQLConfig{
		Host:     config.DatabaseConfig.Host,
		Port:     config.DatabaseConfig.Port,
		User:     config.DatabaseConfig.User,
		Password: config.DatabaseConfig.Password,
		Database: config.DatabaseConfig.Database,
	}

	dbService, err := database.NewService(serviceConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize database service: %w", err)
	}

	// Initialize auth service
	authService := auth.NewService(config.ClientID, config.ClientSecret, config.RedirectURL, dbService)

	// Initialize Google Drive service
	driveService := gdrive.NewService(authService)

	// Initialize scheduler service
	schedulerService := scheduler.NewService(dbService, driveService)

	manager := &BackupManager{
		dbService:        dbService,
		authService:      authService,
		driveService:     driveService,
		schedulerService: schedulerService,
		config:           config,
	}

	return manager, nil
}

// Initialize performs initial setup and starts the scheduler
func (bm *BackupManager) Initialize() error {
	log.Println("Initializing backup manager...")

	// Start the scheduler
	bm.schedulerService.Start()

	log.Println("Backup manager initialized successfully")
	return nil
}

// Close gracefully shuts down the backup manager
func (bm *BackupManager) Close() error {
	log.Println("Shutting down backup manager...")

	// Stop scheduler
	bm.schedulerService.Stop()

	// Close database connection
	if err := bm.dbService.Close(); err != nil {
		return fmt.Errorf("failed to close database service: %w", err)
	}

	log.Println("Backup manager shut down successfully")
	return nil
}

// Auth Methods

// GetAuthURL returns the OAuth2 authorization URL
func (bm *BackupManager) GetAuthURL() string {
	return bm.authService.GetAuthURL()
}

// SetAuthCode exchanges the authorization code for tokens
func (bm *BackupManager) SetAuthCode(authCode string) error {
	return bm.authService.ExchangeToken(authCode)
}

// GetTokenInfo returns information about the current token
func (bm *BackupManager) GetTokenInfo() (*auth.TokenInfo, error) {
	return bm.authService.GetTokenInfo()
}

// ValidateToken validates the current token by making a test API call
func (bm *BackupManager) ValidateToken() error {
	return bm.authService.ValidateToken()
}

// Backup Configuration Methods

// AddBackupMySQLConfig adds a new backup configuration using DatabaseConfig interface
func (bm *BackupManager) AddBackupMySQLConfig(name string, dbConfig backup.DatabaseConfig, cronSchedule string) error {
	// Validate cron expression
	if err := scheduler.ValidateCronExpression(cronSchedule); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("invalid database configuration: %w", err)
	}

	// Test database connection
	backupService, err := backup.NewBackupWithConfig(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to create backup service: %w", err)
	}

	if err := backupService.TestConnection(); err != nil {
		return fmt.Errorf("database connection test failed: %w", err)
	}

	config := &database.BackupConfig{
		Name:         name,
		DatabaseURL:  "mysql://" + dbConfig.GetConnectionString(),
		DatabaseType: string(dbConfig.GetType()),
		CronSchedule: cronSchedule,
		Enabled:      true,
	}

	if err := bm.dbService.SaveBackupConfig(config); err != nil {
		return fmt.Errorf("failed to save backup config: %w", err)
	}

	// Add to scheduler
	if err := bm.schedulerService.AddBackupJob(config); err != nil {
		return fmt.Errorf("failed to schedule backup job: %w", err)
	}

	log.Printf("Added backup configuration '%s' for %s database", name, dbConfig.GetType())
	return nil
}

// GetBackupConfigs returns all backup configurations
func (bm *BackupManager) GetBackupConfigs() ([]database.BackupConfig, error) {
	return bm.dbService.GetBackupConfigs()
}

// UpdateBackupConfig updates an existing backup configuration
func (bm *BackupManager) UpdateBackupConfig(name, cronSchedule string, enabled bool) error {
	config, err := bm.dbService.GetBackupConfigByName(name)
	if err != nil {
		return fmt.Errorf("backup config not found: %w", err)
	}

	// Validate new cron expression if provided
	if cronSchedule != "" && cronSchedule != config.CronSchedule {
		if err := scheduler.ValidateCronExpression(cronSchedule); err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
		config.CronSchedule = cronSchedule
	}

	config.Enabled = enabled

	// Update configuration
	if err := bm.dbService.UpdateBackupConfig(config); err != nil {
		return fmt.Errorf("failed to update backup config: %w", err)
	}

	// Update scheduler
	if enabled {
		if err := bm.schedulerService.AddBackupJob(config); err != nil {
			return fmt.Errorf("failed to reschedule backup job: %w", err)
		}
	} else {
		bm.schedulerService.RemoveBackupJob(name)
	}

	log.Printf("Updated backup configuration '%s'", name)
	return nil
}

// DeleteBackupConfig removes a backup configuration
func (bm *BackupManager) DeleteBackupConfig(name string) error {
	// Remove from scheduler
	bm.schedulerService.RemoveBackupJob(name)

	// Delete from database
	if err := bm.dbService.DeleteBackupConfig(name); err != nil {
		return fmt.Errorf("failed to delete backup config: %w", err)
	}

	log.Printf("Deleted backup configuration '%s'", name)
	return nil
}

// Manual Backup Methods

// BackupNow performs an immediate backup for the specified configuration
func (bm *BackupManager) BackupNow(configName string) error {
	return bm.schedulerService.ExecuteBackupNow(configName)
}

// BackupDatabaseWithConfig performs an immediate backup using DatabaseConfig
func (bm *BackupManager) BackupDatabase(dbConfig backup.DatabaseConfig, folderName string) (*BackupResult, error) {
	// Validate configuration
	if err := dbConfig.Validate(); err != nil {
		return nil, fmt.Errorf("invalid database configuration: %w", err)
	}

	// Create backup service
	backupService, err := backup.NewBackupWithConfig(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create backup service: %w", err)
	}

	// Test connection
	if err := backupService.TestConnection(); err != nil {
		return nil, fmt.Errorf("database connection test failed: %w", err)
	}

	// Get database info
	dbInfo, err := backupService.GetDatabaseInfo()
	if err != nil {
		log.Printf("Warning: failed to get database info: %v", err)
	}

	// Create backup
	tempDir := "/tmp/db-backups"
	backupPath, err := backupService.BackupSchema(tempDir)
	if err != nil {
		return nil, fmt.Errorf("backup failed: %w", err)
	}

	// Upload to Google Drive
	var uploadResult *gdrive.UploadResult
	if folderName != "" {
		folder, err := bm.driveService.GetOrCreateFolder(folderName)
		if err != nil {
			return nil, fmt.Errorf("failed to create/get folder: %w", err)
		}
		uploadResult, err = bm.driveService.UploadFile(backupPath, folder.Id)
		if err != nil {
			return nil, fmt.Errorf("failed to upload to Google Drive: %w", err)
		}
	} else {
		uploadResult, err = bm.driveService.UploadFile(backupPath)
		if err != nil {
			return nil, fmt.Errorf("failed to upload to Google Drive: %w", err)
		}
	}

	return &BackupResult{
		FileID:       uploadResult.FileID,
		FileName:     uploadResult.FileName,
		Size:         uploadResult.Size,
		WebViewLink:  uploadResult.WebViewLink,
		DatabaseInfo: dbInfo,
		UploadedAt:   time.Now(),
	}, nil
}

// ===== History and Monitoring Methods =====

// GetBackupHistory returns backup history with pagination
func (bm *BackupManager) GetBackupHistory(limit, offset int) ([]database.BackupHistory, error) {
	return bm.dbService.GetBackupHistory(limit, offset)
}

// GetScheduledJobs returns information about currently scheduled jobs
func (bm *BackupManager) GetScheduledJobs() []scheduler.JobInfo {
	return bm.schedulerService.GetScheduledJobs()
}

// GetNextRunTimes returns the next N run times for a cron expression
func (bm *BackupManager) GetNextRunTimes(cronExpr string, count int) ([]time.Time, error) {
	return scheduler.GetNextRunTimes(cronExpr, count)
}

// ===== Google Drive Methods =====

// ListBackupFiles lists backup files in Google Drive
func (bm *BackupManager) ListBackupFiles(folderName string, maxResults int64) ([]*gdrive.File, error) {
	query := "name contains '.sql' and trashed=false"

	if folderName != "" {
		folder, err := bm.driveService.FindFolder(folderName)
		if err != nil {
			return nil, fmt.Errorf("folder not found: %w", err)
		}
		query = fmt.Sprintf("%s and '%s' in parents", query, folder.Id)
	}

	files, err := bm.driveService.ListFiles(query, maxResults)
	if err != nil {
		return nil, fmt.Errorf("failed to list files: %w", err)
	}

	// Convert to our File type
	var result []*gdrive.File
	for _, file := range files {
		result = append(result, &gdrive.File{
			ID:          file.Id,
			Name:        file.Name,
			Size:        file.Size,
			CreatedTime: file.CreatedTime,
			WebViewLink: file.WebViewLink,
		})
	}

	return result, nil
}

// DeleteBackupFile deletes a backup file from Google Drive
func (bm *BackupManager) DeleteBackupFile(fileID string) error {
	return bm.driveService.DeleteFile(fileID)
}

// Notification Configuration Methods

// AddNotificationConfig adds a new notification channel configuration
func (bm *BackupManager) AddNotificationConfig(name string, channel notification.NotificationChannel, config map[string]interface{}, notifyOnSuccess, notifyOnError bool) error {
	// Create appropriate notifier to validate config
	var notifier notification.Notifier
	switch channel {
	case notification.ChannelChatwork:
		chatworkConfig, err := bm.parseChatworkConfig(config)
		if err != nil {
			return fmt.Errorf("invalid Chatwork config: %w", err)
		}
		notifier = notification.NewChatworkNotifier(*chatworkConfig)
	case notification.ChannelDiscord:
		discordConfig, err := bm.parseDiscordConfig(config)
		if err != nil {
			return fmt.Errorf("invalid Discord config: %w", err)
		}
		notifier = notification.NewDiscordNotifier(*discordConfig)
	case notification.ChannelSlack:
		slackConfig, err := bm.parseSlackConfig(config)
		if err != nil {
			return fmt.Errorf("invalid Slack config: %w", err)
		}
		notifier = notification.NewSlackNotifier(*slackConfig)
	default:
		return fmt.Errorf("unsupported notification channel: %s", channel)
	}

	// Validate configuration
	if err := notifier.ValidateConfig(config); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	// Save notification configuration
	notifConfig := &database.NotificationConfig{
		Name:            name,
		Channel:         string(channel),
		Enabled:         true,
		Config:          config,
		NotifyOnSuccess: notifyOnSuccess,
		NotifyOnError:   notifyOnError,
	}

	if err := bm.dbService.SaveNotificationConfig(notifConfig); err != nil {
		return fmt.Errorf("failed to save notification config: %w", err)
	}

	log.Printf("Added notification configuration '%s' for %s", name, channel)
	return nil
}

// GetNotificationConfigs returns all notification configurations
func (bm *BackupManager) GetNotificationConfigs() ([]database.NotificationConfig, error) {
	return bm.dbService.GetNotificationConfigs()
}

// UpdateNotificationConfig updates an existing notification configuration
func (bm *BackupManager) UpdateNotificationConfig(name string, enabled, notifyOnSuccess, notifyOnError bool) error {
	config, err := bm.dbService.GetNotificationConfigByName(name)
	if err != nil {
		return fmt.Errorf("notification config not found: %w", err)
	}

	config.Enabled = enabled
	config.NotifyOnSuccess = notifyOnSuccess
	config.NotifyOnError = notifyOnError

	if err := bm.dbService.UpdateNotificationConfig(config); err != nil {
		return fmt.Errorf("failed to update notification config: %w", err)
	}

	log.Printf("Updated notification configuration '%s'", name)
	return nil
}

// DeleteNotificationConfig removes a notification configuration
func (bm *BackupManager) DeleteNotificationConfig(name string) error {
	if err := bm.dbService.DeleteNotificationConfig(name); err != nil {
		return fmt.Errorf("failed to delete notification config: %w", err)
	}

	log.Printf("Deleted notification configuration '%s'", name)
	return nil
}

// TestNotification sends a test notification to a specific channel
func (bm *BackupManager) TestNotification(configName string) error {
	notifyManager := bm.schedulerService.GetNotificationManager()
	return notifyManager.TestNotification(configName)
}

// Helper methods for parsing notification configs
func (bm *BackupManager) parseChatworkConfig(config map[string]interface{}) (*notification.ChatworkConfig, error) {
	apiToken, ok := config["api_token"].(string)
	if !ok || apiToken == "" {
		return nil, fmt.Errorf("api_token is required")
	}

	roomID, ok := config["room_id"].(string)
	if !ok || roomID == "" {
		return nil, fmt.Errorf("room_id is required")
	}

	return &notification.ChatworkConfig{
		APIToken: apiToken,
		RoomID:   roomID,
	}, nil
}

func (bm *BackupManager) parseDiscordConfig(config map[string]interface{}) (*notification.DiscordConfig, error) {
	webhookURL, ok := config["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required")
	}

	result := &notification.DiscordConfig{
		WebhookURL: webhookURL,
	}

	if username, ok := config["username"].(string); ok {
		result.Username = username
	}

	if avatarURL, ok := config["avatar_url"].(string); ok {
		result.AvatarURL = avatarURL
	}

	return result, nil
}

func (bm *BackupManager) parseSlackConfig(config map[string]interface{}) (*notification.SlackConfig, error) {
	webhookURL, ok := config["webhook_url"].(string)
	if !ok || webhookURL == "" {
		return nil, fmt.Errorf("webhook_url is required")
	}

	result := &notification.SlackConfig{
		WebhookURL: webhookURL,
	}

	if channel, ok := config["channel"].(string); ok {
		result.Channel = channel
	}

	if username, ok := config["username"].(string); ok {
		result.Username = username
	}

	if iconEmoji, ok := config["icon_emoji"].(string); ok {
		result.IconEmoji = iconEmoji
	}

	if iconURL, ok := config["icon_url"].(string); ok {
		result.IconURL = iconURL
	}

	return result, nil
}

// BackupResult contains information about a completed backup
type BackupResult struct {
	FileID       string               `json:"file_id"`
	FileName     string               `json:"file_name"`
	Size         int64                `json:"size"`
	WebViewLink  string               `json:"web_view_link"`
	DatabaseInfo *backup.DatabaseInfo `json:"database_info,omitempty"`
	UploadedAt   time.Time            `json:"uploaded_at"`
}
