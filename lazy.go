package lazy

import (
	"fmt"
	"log"
	"time"

	"github.com/vfa-khuongdv/lazy/internal/auth"
	"github.com/vfa-khuongdv/lazy/internal/backup"
	"github.com/vfa-khuongdv/lazy/internal/database"
	"github.com/vfa-khuongdv/lazy/internal/notification"
	"github.com/vfa-khuongdv/lazy/internal/scheduler"
	"github.com/vfa-khuongdv/lazy/pkg/gdrive"
	"golang.org/x/oauth2"
)

type LazyManager struct {
	dbService        *database.Service
	authService      *auth.Service
	driveService     *gdrive.Service
	schedulerService *scheduler.Service
	config           *Config
}

type Config struct {
	// Google OAuth2 credentials
	OAuthConfig *oauth2.Config
	// MySQL database configuration for storing package metadata
	DatabaseConfig *backup.MySQLConfig
	// Notification config for send notifications
	NotificationConfig []notification.NotificationConfig
	// Scheduler configured to control backup intervals
	SchedulerConfig []backup.SchedulerConfig
	// Backup temporary directory (optional, uses system temp by default)
	TempDir string
}

type BackupResult struct {
	FileID       string               `json:"file_id"`
	FileName     string               `json:"file_name"`
	Size         int64                `json:"size"`
	WebViewLink  string               `json:"web_view_link"`
	DatabaseInfo *backup.DatabaseInfo `json:"database_info,omitempty"`
	UploadedAt   time.Time            `json:"uploaded_at"`
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

// NewMySQLConfig creates a new Scheduler configuration
func NewSchedulerConfig(name string, backupMode string, databaseConfig *backup.MySQLConfig, cronExpression string) *backup.SchedulerConfig {
	return &backup.SchedulerConfig{
		Name:           name,
		BackupMode:     backupMode,
		DatabaseConfig: databaseConfig,
		CronExpression: cronExpression,
	}
}

// NewBackupManager creates a new backup manager instance
func NewBackupManager(config *Config) (*LazyManager, error) {
	if config == nil {
		return nil, fmt.Errorf("configuration is required")
	}

	if config.DatabaseConfig == nil || config.OAuthConfig == nil {
		return nil, fmt.Errorf("MySQL configuration database and OAuth configuration are required")
	}

	if config.OAuthConfig.ClientID == "" || config.OAuthConfig.ClientSecret == "" || config.OAuthConfig.RedirectURL == "" {
		return nil, fmt.Errorf("OAuth configuration must include ClientID, ClientSecret, and RedirectURL")
	}

	if config.DatabaseConfig.Host == "" || config.DatabaseConfig.Port == "" || config.DatabaseConfig.User == "" || config.DatabaseConfig.Database == "" {
		return nil, fmt.Errorf("MySQL configuration must include Host, Port, User, and Database")
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
	authService := auth.NewService(config.OAuthConfig.ClientID, config.OAuthConfig.ClientSecret, config.OAuthConfig.RedirectURL, dbService)

	// Initialize Google Drive service
	driveService := gdrive.NewService(authService)

	// Initialize scheduler service
	schedulerService := scheduler.NewService(dbService, driveService)

	manager := &LazyManager{
		dbService:        dbService,
		authService:      authService,
		driveService:     driveService,
		schedulerService: schedulerService,
		config:           config,
	}

	return manager, nil
}

// Initialize performs initial setup and starts the scheduler
func (lm *LazyManager) Initialize() error {
	log.Println("Initializing backup manager...")

	// Sync notification configs
	if err := lm.SyncNotifications(); err != nil {
		return fmt.Errorf("failed to sync notification configs: %w", err)
	}
	// Sync scheduler configs
	if err := lm.SyncSchedulerConfig(); err != nil {
		return fmt.Errorf("failed to scheduler configs: %w", err)
	}

	// Start the scheduler
	lm.schedulerService.Start()

	log.Println("Backup manager initialized successfully")
	return nil
}

// Close gracefully shuts down the backup manager
func (lm *LazyManager) Close() error {
	log.Println("Shutting down backup manager...")

	// Stop scheduler
	lm.schedulerService.Stop()

	// Close database connection
	if err := lm.dbService.Close(); err != nil {
		return fmt.Errorf("failed to close database service: %w", err)
	}

	log.Println("Backup manager shut down successfully")
	return nil
}

// Auth Methods

// GetAuthURL returns the OAuth2 authorization URL
func (lm *LazyManager) GetAuthURL() string {
	return lm.authService.GetAuthURL()
}

// SetAuthCode exchanges the authorization code for tokens
func (lm *LazyManager) SetAuthCode(authCode string) error {
	return lm.authService.ExchangeToken(authCode)
}

// GetTokenInfo returns information about the current token
func (lm *LazyManager) GetTokenInfo() (*auth.TokenInfo, error) {
	return lm.authService.GetTokenInfo()
}

// ValidateToken validates the current token by making a test API call
func (lm *LazyManager) ValidateToken() error {
	return lm.authService.ValidateToken()
}

// Backup Configuration Methods

// AddBackupMySQLConfig adds a new backup configuration using DatabaseConfig interface
func (lm *LazyManager) AddBackupMySQLConfig(name string, mode string, dbConfig *backup.MySQLConfig, expression string) error {
	// Validate cron expression
	if err := scheduler.ValidateCronExpression(expression); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	// Validate database configuration
	if err := dbConfig.Validate(); err != nil {
		return fmt.Errorf("invalid database configuration: %w", err)
	}

	// Test database connection
	backupService, err := backup.NewMySQLBackupWithConfig(dbConfig)
	if err != nil {
		return fmt.Errorf("failed to create backup service: %w", err)
	}

	if err := backupService.TestConnection(); err != nil {
		return fmt.Errorf("database connection test failed: %w", err)
	}

	config := &database.BackupConfig{
		Name:         name,
		BackupMode:   mode,
		DatabaseURL:  "mysql://" + dbConfig.GetConnectionString(),
		DatabaseType: "mysql",
		CronSchedule: expression,
		Enabled:      true,
	}

	if err := lm.dbService.SaveBackupConfig(config); err != nil {
		return fmt.Errorf("failed to save backup config: %w", err)
	}

	log.Printf("Added backup configuration '%s' for mysql database", name)
	return nil
}

// UpdateBackupConfig updates an existing backup configuration
func (lm *LazyManager) UpdateBackupConfig(name, cronSchedule string, enabled bool) error {
	config, err := lm.dbService.GetBackupConfigByName(name)
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
	if err := lm.dbService.UpdateBackupConfig(config); err != nil {
		return fmt.Errorf("failed to update backup config: %w", err)
	}

	// Update scheduler
	if enabled {
		if err := lm.schedulerService.AddBackupJob(config); err != nil {
			return fmt.Errorf("failed to reschedule backup job: %w", err)
		}
	} else {
		lm.schedulerService.RemoveBackupJob(name)
	}

	log.Printf("Updated backup configuration '%s'", name)
	return nil
}

// DeleteBackupConfig removes a backup configuration
func (lm *LazyManager) DeleteBackupConfig(name string) error {
	// Remove from scheduler
	lm.schedulerService.RemoveBackupJob(name)

	// Delete from database
	if err := lm.dbService.DeleteBackupConfig(name); err != nil {
		return fmt.Errorf("failed to delete backup config: %w", err)
	}

	log.Printf("Deleted backup configuration '%s'", name)
	return nil
}

func (lm *LazyManager) DeleteAllBackupConfig() error {
	// Remove all jobs from scheduler
	err := lm.dbService.DeleteAllBackupConfig()
	if err != nil {
		return fmt.Errorf("failed to delete all backup configs: %w", err)
	}
	return nil
}

// Sync scheduler conffig
func (lm *LazyManager) SyncSchedulerConfig() error {
	// Remove all existing backup configs
	if err := lm.DeleteAllBackupConfig(); err != nil {
		return fmt.Errorf("failed to clear backup configs: %w", err)
	}
	// Add new configs
	for _, scheduler := range lm.config.SchedulerConfig {
		if err := lm.AddBackupMySQLConfig(scheduler.Name, scheduler.BackupMode, scheduler.DatabaseConfig, scheduler.CronExpression); err != nil {
			return fmt.Errorf("failed to add backup config scheduler")
		}
	}
	return nil
}

func (lm *LazyManager) SyncNotifications() error {
	// Clear existing configs
	if err := lm.dbService.DeleteAllNotificationConfig(); err != nil {
		return fmt.Errorf("failed to clear notification configs: %w", err)
	}
	// Add new configs
	for _, config := range lm.config.NotificationConfig {
		if err := lm.dbService.SaveNotificationConfig(&database.NotificationConfig{
			Name:            config.Name,
			Channel:         config.Channel,
			Config:          config.Config,
			NotifyOnSuccess: config.NotifyOnSuccess,
			NotifyOnError:   config.NotifyOnError,
			Enabled:         config.Enabled,
		}); err != nil {
			return fmt.Errorf("failed to save notification config: %w", err)
		}
	}
	return nil
}
