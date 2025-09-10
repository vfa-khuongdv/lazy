package main

import (
	"fmt"
	"log"

	"github.com/vfa-khuongdv/lazy"
	"github.com/vfa-khuongdv/lazy/pkg/backup"
	"github.com/vfa-khuongdv/lazy/pkg/notification"
	"golang.org/x/oauth2"
)

func main() {
	// Create MySQL configuration for storing backup configurations
	sqlConfig := lazy.NewMySQLConfig(
		"127.0.0.1",      // host
		"3306",           // port
		"root",           // user
		"root",           // password
		"golang_sync_db", // database name for storing configurations
	)

	// Create OAuth2 Configuration
	authConfig := &oauth2.Config{
		ClientID:     "your-client-id",
		ClientSecret: "your-client-secret",
		RedirectURL:  "http://localhost:8081/auth/google/callback",
	}

	// Create scheduler configurations
	schedules := []backup.SchedulerConfig{
		{
			Name:           "your-config-name", // Configuration name
			BackupMode:     "full",             // Backup Mode (full data or only schema)
			DatabaseConfig: sqlConfig,          // Database URL
			CronExpression: "0 * * * * *",      // Cron schedule (every 1 minutes)
		},
	}

	// Create notification configurations
	notifications := []notification.NotificationConfig{
		{
			Name:    "chatwork-team",
			Channel: string(notification.ChannelChatwork),
			Config: map[string]interface{}{
				"api_token": "your-chatwork-api-token",
				"room_id":   "307079269",
			},
			NotifyOnSuccess: true,
			NotifyOnError:   true,
			Enabled:         true,
		},
		{
			Name:    "discord-team",
			Channel: string(notification.ChannelDiscord),
			Config: map[string]interface{}{
				"webhook_url": "your-discord-webhook-url",
				"username":    "Database Backup Bot",
				"avatar_url":  "https://example.com/bot.png",
			},
			NotifyOnSuccess: true,
			NotifyOnError:   true,
			Enabled:         true,
		},
		{
			Name:    "slack-team",
			Channel: string(notification.ChannelSlack),
			Config: map[string]interface{}{
				"webhook_url": "Your-slack-webhook-url",
				"channel":     "#trading",
				"username":    "DB Backup Service",
			},
			NotifyOnSuccess: true,
			NotifyOnError:   true,
			Enabled:         true,
		},
	}

	lazyConfig := &lazy.Config{
		OAuthConfig:        authConfig,
		DatabaseConfig:     sqlConfig,
		SchedulerConfig:    schedules,
		NotificationConfig: notifications,
	}

	// create manager backup
	manager, err := lazy.NewBackupManager(lazyConfig)
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

	// Keep the program running
	select {}

}
