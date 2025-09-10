package scheduler

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"github.com/vfa-khuongdv/lazy/internal/database"
	"github.com/vfa-khuongdv/lazy/pkg/backup"
	"github.com/vfa-khuongdv/lazy/pkg/gdrive"
	"github.com/vfa-khuongdv/lazy/pkg/notification"
)

// Service handles scheduled backup operations
type Service struct {
	cron          *cron.Cron
	dbService     *database.Service
	driveService  *gdrive.Service
	notifyManager *notification.Manager
	tempDir       string
	mutex         sync.RWMutex
	jobs          map[string]cron.EntryID
}

// NewService creates a new scheduler service
func NewService(dbService *database.Service, driveService *gdrive.Service) *Service {
	// Create temporary directory for backups
	tempDir := filepath.Join(os.TempDir(), "db-backups")
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Printf("Failed to create temp directory: %v", err)
	}

	// Create notification manager
	notifyManager := notification.NewManager(dbService)

	return &Service{
		cron:          cron.New(cron.WithSeconds()),
		dbService:     dbService,
		driveService:  driveService,
		notifyManager: notifyManager,
		tempDir:       tempDir,
		jobs:          make(map[string]cron.EntryID),
	}
}

// Start starts the scheduler
func (s *Service) Start() {
	s.cron.Start()
	log.Println("Scheduler started")

	// Load existing backup configurations and schedule them
	s.loadAndScheduleConfigs()
}

// Stop stops the scheduler
func (s *Service) Stop() {
	ctx := s.cron.Stop()
	<-ctx.Done()
	log.Println("Scheduler stopped")
}

// AddBackupJob adds a new scheduled backup job
func (s *Service) AddBackupJob(config *database.BackupConfig) error {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if !config.Enabled {
		return fmt.Errorf("backup config '%s' is disabled", config.Name)
	}

	// Remove existing job if it exists
	if entryID, exists := s.jobs[config.Name]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, config.Name)
	}

	// Add new job
	entryID, err := s.cron.AddFunc(config.CronSchedule, func() {
		s.executeBackup(config)
	})

	if err != nil {
		return fmt.Errorf("failed to add cron job: %w", err)
	}

	s.jobs[config.Name] = entryID
	log.Printf("Added scheduled backup job '%s' with schedule '%s'", config.Name, config.CronSchedule)

	return nil
}

// RemoveBackupJob removes a scheduled backup job
func (s *Service) RemoveBackupJob(configName string) {
	s.mutex.Lock()
	defer s.mutex.Unlock()

	if entryID, exists := s.jobs[configName]; exists {
		s.cron.Remove(entryID)
		delete(s.jobs, configName)
		log.Printf("Removed scheduled backup job '%s'", configName)
	}
}

// GetScheduledJobs returns information about currently scheduled jobs
func (s *Service) GetScheduledJobs() []JobInfo {
	s.mutex.RLock()
	defer s.mutex.RUnlock()

	var jobs []JobInfo
	for name, entryID := range s.jobs {
		entry := s.cron.Entry(entryID)
		jobs = append(jobs, JobInfo{
			Name:     name,
			EntryID:  entryID,
			Next:     entry.Next,
			Previous: entry.Prev,
		})
	}

	return jobs
}

// ExecuteBackupNow executes a backup job immediately
func (s *Service) ExecuteBackupNow(configName string) error {
	config, err := s.dbService.GetBackupConfigByName(configName)
	if err != nil {
		return fmt.Errorf("failed to get backup config: %w", err)
	}

	go s.executeBackup(config)
	return nil
}

// loadAndScheduleConfigs loads backup configurations and schedules them
func (s *Service) loadAndScheduleConfigs() {
	configs, err := s.dbService.GetBackupConfigs()
	if err != nil {
		log.Printf("Failed to load backup configurations: %v", err)
		return
	}

	for _, config := range configs {
		if err := s.AddBackupJob(&config); err != nil {
			log.Printf("Failed to schedule backup job '%s': %v", config.Name, err)
		}
	}

	log.Printf("Loaded and scheduled %d backup jobs", len(configs))
}

// executeBackup performs the actual backup operation
func (s *Service) executeBackup(config *database.BackupConfig) {
	log.Printf("Starting backup job '%s'", config.Name)

	history := &database.BackupHistory{
		BackupType: config.DatabaseType,
		FileName:   "", // Will be set after backup creation
		Status:     "in_progress",
		StartedAt:  time.Now(),
	}

	if err := s.dbService.SaveBackupHistory(history); err != nil {
		log.Printf("Failed to save backup history: %v", err)
		return
	}

	// Create backup instance
	backupService, err := backup.NewBackupFromURL(config.DatabaseURL)
	if err != nil {
		s.updateBackupHistory(history, "failed", "", "", 0, fmt.Sprintf("Failed to create backup service: %v", err))
		return
	}

	// Test database connection first
	if err := backupService.TestConnection(); err != nil {
		s.updateBackupHistory(history, "failed", "", "", 0, fmt.Sprintf("Database connection failed: %v", err))
		return
	}

	// Perform backup

	var backupPath string

	switch config.BackupMode {
	case "full":
		backupPath, err = backupService.BackupSchema(s.tempDir)
		if err != nil {
			s.updateBackupHistory(history, "failed", "", "", 0, fmt.Sprintf("Backup failed: %v", err))
			return
		}
	case "schema":
		backupPath, err = backupService.BackupSchemaOnly(s.tempDir)
		if err != nil {
			s.updateBackupHistory(history, "failed", "", "", 0, fmt.Sprintf("Backup failed: %v", err))
			return
		}
	}

	// Get file size
	fileInfo, err := os.Stat(backupPath)
	if err != nil {
		s.updateBackupHistory(history, "failed", filepath.Base(backupPath), "", 0, fmt.Sprintf("Failed to get file info: %v", err))
		return
	}

	// Create or get backup folder in Google Drive
	folderName := fmt.Sprintf("DB Backups - %s", config.Name)
	folder, err := s.driveService.GetOrCreateFolder(folderName)
	if err != nil {
		s.updateBackupHistory(history, "failed", filepath.Base(backupPath), "", fileInfo.Size(), fmt.Sprintf("Failed to create Drive folder: %v", err))
		s.cleanupTempFile(backupPath)
		return
	}

	// Upload to Google Drive
	uploadResult, err := s.driveService.UploadFile(backupPath, folder.Id)
	if err != nil {
		s.updateBackupHistory(history, "failed", filepath.Base(backupPath), "", fileInfo.Size(), fmt.Sprintf("Failed to upload to Drive: %v", err))
		s.cleanupTempFile(backupPath)
		return
	}

	// Update backup history with success
	s.updateBackupHistory(history, "success", uploadResult.FileName, uploadResult.FileID, uploadResult.Size, "")

	// Send success notification
	completedAt := time.Now()
	notificationData := &notification.BackupNotificationData{
		ConfigName:   config.Name,
		DatabaseType: config.DatabaseType,
		BackupSize:   uploadResult.Size,
		Duration:     completedAt.Sub(history.StartedAt),
		FileName:     uploadResult.FileName,
		FileID:       uploadResult.FileID,
		WebViewLink:  uploadResult.WebViewLink,
		StartedAt:    history.StartedAt,
		CompletedAt:  completedAt,
	}
	s.notifyManager.SendBackupSuccessNotification(notificationData)

	// Clean up temporary file
	s.cleanupTempFile(backupPath)

	log.Printf("Backup job '%s' completed successfully. File ID: %s", config.Name, uploadResult.FileID)
}

// updateBackupHistory updates the backup history record
func (s *Service) updateBackupHistory(history *database.BackupHistory, status, fileName, fileID string, fileSize int64, errorMsg string) {
	now := time.Now()
	history.Status = status
	history.FileName = fileName
	history.FileID = fileID
	history.FileSize = fileSize
	history.ErrorMsg = errorMsg
	history.CompletedAt = &now

	// Send error notification if backup failed
	if status == "failed" {
		s.sendBackupErrorNotification(history, errorMsg)
	}

	if err := s.dbService.UpdateBackupHistory(history); err != nil {
		log.Printf("Failed to update backup history: %v", err)
	}
}

// sendBackupErrorNotification sends error notification for failed backups
func (s *Service) sendBackupErrorNotification(history *database.BackupHistory, errorMsg string) {
	// Extract config name from the database URL or use a default
	configName := s.extractConfigNameFromHistory(history)

	notificationData := &notification.BackupNotificationData{
		ConfigName:   configName,
		DatabaseType: history.BackupType,
		ErrorMessage: errorMsg,
		StartedAt:    history.StartedAt,
		CompletedAt:  *history.CompletedAt,
	}

	s.notifyManager.SendBackupErrorNotification(notificationData)
}

// extractConfigNameFromHistory tries to extract config name from backup history
func (s *Service) extractConfigNameFromHistory(history *database.BackupHistory) string {
	// Try to find a matching backup config
	configs, err := s.dbService.GetBackupConfigs()
	if err != nil {
		return "Unknown Config"
	}

	for _, config := range configs {
		if config.DatabaseURL == history.DatabaseURL {
			return config.Name
		}
	}

	return "Unknown Config"
}

// GetNotificationManager returns the notification manager instance
func (s *Service) GetNotificationManager() *notification.Manager {
	return s.notifyManager
}

// cleanupTempFile removes the temporary backup file
func (s *Service) cleanupTempFile(filePath string) {
	if err := os.Remove(filePath); err != nil {
		log.Printf("Failed to remove temporary file %s: %v", filePath, err)
	}
}

// ValidateCronExpression validates a cron expression
func ValidateCronExpression(expr string) error {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	_, err := parser.Parse(expr)
	return err
}

// GetNextRunTimes returns the next N run times for a cron expression
func GetNextRunTimes(cronExpr string, count int) ([]time.Time, error) {
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow | cron.Descriptor)
	schedule, err := parser.Parse(cronExpr)
	if err != nil {
		return nil, err
	}

	var times []time.Time
	now := time.Now()

	for i := 0; i < count; i++ {
		now = schedule.Next(now)
		times = append(times, now)
	}

	return times, nil
}
