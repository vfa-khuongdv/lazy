package main

import (
	"fmt"
	"log"

	"github.com/vfa-khuongdv/go-backup-drive"
	"github.com/vfa-khuongdv/go-backup-drive/internal/notification"
)

func main() {

	// Create MySQL configuration for storing backup configurations
	sqlConfig := backup.NewMySQLConfig(
		"127.0.0.1",      // host
		"3306",           // port
		"root",           // user
		"root",           // password
		"golang_sync_db", // database name for storing configurations
	)

	config := &backup.Config{
		ClientID:       "566118089952-r91h6t8an47ds3nc2f0vqk95h6k8udbe.apps.googleusercontent.com",
		ClientSecret:   "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",
		RedirectURL:    "http://localhost:8081/auth/google/callback",
		DatabaseConfig: sqlConfig, // MySQL database for storing configurations
	}

	// create manager backup
	manager, err := backup.NewBackupManager(config)
	if err != nil {
		log.Fatalf("Failed to create backup manager: %v", err)
	}

	// Ensure cleanup on exit
	defer func() {
		if err := manager.Close(); err != nil {
			log.Printf("Error closing backup manager: %v", err)
		}
	}()

	// Initialize and start scheduler
	if err := manager.Initialize(); err != nil {
		log.Fatalf("Failed to initialize: %v", err)
	}

	// Authenticate with Google Drive (first time only)
	if tokenInfo, _ := manager.GetTokenInfo(); !tokenInfo.HasToken {
		authURL := manager.GetAuthURL()
		log.Printf("Visit: %s", authURL)

		// Get authorization code from user
		var authCode string
		fmt.Print("Enter authorization code: ")
		fmt.Scanln(&authCode)

		if err := manager.SetAuthCode(authCode); err != nil {
			log.Fatalf("Authentication failed: %v", err)
		}
	}

	// Add a backup configuration
	err = manager.AddBackupMySQLConfig(
		"golang_sync_db_dev", // Configuration name
		sqlConfig,            // Database URL
		"0 * * * * *",        // Cron schedule (every 1 minutes)
	)
	if err != nil {
		log.Fatalf("Failed to add backup config: %v", err)
	}

	// Add notify to chatwork
	chatworkConfig := map[string]interface{}{
		"api_token": "d34014530752cf959b0a4690a741b606",
		"room_id":   "307079269",
	}

	manager.AddNotificationConfig(
		"chatwork-team",
		notification.ChannelChatwork,
		chatworkConfig,
		true, // Only notify on success
		true, // Only notify on errors
	)

	// Keep the program running
	select {}

}
