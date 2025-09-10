package database

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// TokenConfig stores Google OAuth2 tokens for Drive API access
type TokenConfig struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	ClientID     string    `json:"client_id" gorm:"not null"`
	ClientSecret string    `json:"client_secret" gorm:"not null"`
	AccessToken  string    `json:"access_token" gorm:"not null"`
	RefreshToken string    `json:"refresh_token" gorm:"not null"`
	TokenType    string    `json:"token_type" gorm:"default:Bearer"`
	Expiry       time.Time `json:"expiry"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// BackupHistory keeps track of backup operations
type BackupHistory struct {
	ID          uint       `json:"id" gorm:"primarykey"`
	DatabaseURL string     `json:"database_url" gorm:"not null"`
	BackupType  string     `json:"backup_type" gorm:"not null"` // mysql, postgres, etc.
	FileName    string     `json:"file_name" gorm:"not null"`
	FileID      string     `json:"file_id"`   // Google Drive file ID
	FileSize    int64      `json:"file_size"` // File size in bytes
	Status      string     `json:"status"`    // success, failed, in_progress
	ErrorMsg    string     `json:"error_msg"` // Error message if failed
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at"`
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// BackupConfig stores backup configuration settings
type BackupConfig struct {
	ID           uint      `json:"id" gorm:"primarykey"`
	Name         string    `json:"name" gorm:"not null;unique"`
	BackupMode   string    `json:"backup_mode" gorm:"not null"` // full, schema, data
	DatabaseURL  string    `json:"database_url" gorm:"not null"`
	DatabaseType string    `json:"database_type" gorm:"not null"` // mysql, postgres, etc.
	CronSchedule string    `json:"cron_schedule" gorm:"not null"` // e.g., "0 2 * * *" (daily at 2 AM)
	Enabled      bool      `json:"enabled" gorm:"default:true"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// NotificationConfig stores notification channel configurations
type NotificationConfig struct {
	ID              uint                   `json:"id" gorm:"primarykey"`
	Name            string                 `json:"name" gorm:"not null;unique"`
	Channel         string                 `json:"channel" gorm:"not null"`
	Enabled         bool                   `json:"enabled" gorm:"default:false"`
	Config          map[string]interface{} `json:"config" gorm:"serializer:json"`
	NotifyOnSuccess bool                   `json:"notify_on_success" gorm:"default:false"`
	NotifyOnError   bool                   `json:"notify_on_error" gorm:"default:false"`
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
}

// Add table names
func (TokenConfig) TableName() string {
	return "dbu_token_configs"
}

func (BackupHistory) TableName() string {
	return "dbu_backup_histories"
}

func (BackupConfig) TableName() string {
	return "dbu_backup_configs"
}

func (NotificationConfig) TableName() string {
	return "dbu_notification_configs"
}

// ServiceMySQLConfig represents MySQL database configuration for the database service
type ServiceMySQLConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// Validate validates the MySQL configuration
func (c *ServiceMySQLConfig) Validate() error {
	if c.Host == "" {
		return fmt.Errorf("host is required")
	}
	if c.Port == "" {
		return fmt.Errorf("port is required")
	}
	if c.User == "" {
		return fmt.Errorf("user is required")
	}
	if c.Database == "" {
		return fmt.Errorf("database is required")
	}
	return nil
}

// AutoMigrate runs database migrations for all models
func AutoMigrate(db *gorm.DB) error {
	return db.AutoMigrate(
		&TokenConfig{},
		&BackupHistory{},
		&BackupConfig{},
		&NotificationConfig{},
	)
}
