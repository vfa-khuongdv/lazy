package database

import (
	"errors"
	"fmt"

	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

type Service struct {
	db *gorm.DB
}

// NewService creates a new database service using ServiceMySQLConfig
func NewService(config *ServiceMySQLConfig) (*Service, error) {
	if config == nil {
		return nil, fmt.Errorf("MySQL configuration is required")
	}

	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid MySQL configuration: %w", err)
	}

	// Build MySQL DSN (Data Source Name)
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		config.User, config.Password, config.Host, config.Port, config.Database)

	// Connect to MySQL database
	db, err := gorm.Open(mysql.Open(dsn), &gorm.Config{})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to MySQL database: %w", err)
	}

	service := &Service{db: db}

	// Auto-migrate models
	if err := AutoMigrate(db); err != nil {
		return nil, fmt.Errorf("failed to migrate database: %w", err)
	}

	return service, nil
}

// GetDB returns the database instance
func (s *Service) GetDB() *gorm.DB {
	return s.db
}

// SaveTokenConfig saves or updates token configuration
func (s *Service) SaveTokenConfig(config *TokenConfig) error {
	// Try to find existing config first
	var existing TokenConfig
	if err := s.db.First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new record
			return s.db.Create(config).Error
		}
		return err
	}

	// Update existing record
	config.ID = existing.ID
	return s.db.Save(config).Error
}

// GetTokenConfig retrieves the token configuration
func (s *Service) GetTokenConfig() (*TokenConfig, error) {
	var config TokenConfig
	if err := s.db.First(&config).Error; err != nil {
		return nil, err
	}
	return &config, nil
}

// SaveBackupHistory saves backup history record
func (s *Service) SaveBackupHistory(history *BackupHistory) error {
	return s.db.Create(history).Error
}

// UpdateBackupHistory updates backup history record
func (s *Service) UpdateBackupHistory(history *BackupHistory) error {
	return s.db.Save(history).Error
}

// GetBackupHistory retrieves backup history with pagination
func (s *Service) GetBackupHistory(limit, offset int) ([]BackupHistory, error) {
	var history []BackupHistory
	err := s.db.Order("created_at DESC").Limit(limit).Offset(offset).Find(&history).Error
	return history, err
}

// SaveBackupConfig saves backup configuration
func (s *Service) SaveBackupConfig(config *BackupConfig) error {
	// create or update
	var existing BackupConfig
	if err := s.db.Where("name = ?", config.Name).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new record
			return s.db.Create(config).Error
		}
		return err
	}

	// Update existing record
	config.ID = existing.ID
	config.CreatedAt = existing.CreatedAt // preserve original CreatedAt
	return s.db.Save(config).Error
}

// GetBackupConfigs retrieves all backup configurations
func (s *Service) GetBackupConfigs() ([]BackupConfig, error) {
	var configs []BackupConfig
	err := s.db.Where("enabled = ?", true).Find(&configs).Error
	return configs, err
}

// GetBackupConfigByName retrieves backup configuration by name
func (s *Service) GetBackupConfigByName(name string) (*BackupConfig, error) {
	var config BackupConfig
	err := s.db.Where("name = ?", name).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// UpdateBackupConfig updates backup configuration
func (s *Service) UpdateBackupConfig(config *BackupConfig) error {
	return s.db.Save(config).Error
}

// DeleteBackupConfig deletes backup configuration by name
func (s *Service) DeleteBackupConfig(name string) error {
	return s.db.Where("name = ?", name).Delete(&BackupConfig{}).Error
}

// Close closes the database connection
func (s *Service) Close() error {
	sqlDB, err := s.db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

// SaveNotificationConfig saves notification configuration
func (s *Service) SaveNotificationConfig(config *NotificationConfig) error {
	// Create or update
	var existing NotificationConfig
	if err := s.db.Where("name = ?", config.Name).First(&existing).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			// Create new record
			return s.db.Create(config).Error
		}
		return err
	}

	// Update existing record
	config.ID = existing.ID
	config.CreatedAt = existing.CreatedAt // preserve original CreatedAt
	return s.db.Save(config).Error
}

// GetNotificationConfigs retrieves all notification configurations
func (s *Service) GetNotificationConfigs() ([]NotificationConfig, error) {
	var configs []NotificationConfig
	err := s.db.Find(&configs).Error
	return configs, err
}

// GetEnabledNotificationConfigs retrieves enabled notification configurations
func (s *Service) GetEnabledNotificationConfigs() ([]NotificationConfig, error) {
	var configs []NotificationConfig
	err := s.db.Where("enabled = ?", true).Find(&configs).Error
	return configs, err
}

// GetNotificationConfigByName retrieves notification configuration by name
func (s *Service) GetNotificationConfigByName(name string) (*NotificationConfig, error) {
	var config NotificationConfig
	err := s.db.Where("name = ?", name).First(&config).Error
	if err != nil {
		return nil, err
	}
	return &config, nil
}

// UpdateNotificationConfig updates notification configuration
func (s *Service) UpdateNotificationConfig(config *NotificationConfig) error {
	return s.db.Save(config).Error
}

// DeleteNotificationConfig deletes notification configuration by name
func (s *Service) DeleteNotificationConfig(name string) error {
	return s.db.Where("name = ?", name).Delete(&NotificationConfig{}).Error
}
