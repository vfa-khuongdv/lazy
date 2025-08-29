# Database Backup to Google Drive

A comprehensive Go package for automated database backups to Google Drive with OAuth2 authentication, automatic token refresh, and cron-based scheduling.

## Features

- 🔐 **OAuth2 Authentication**: Secure Google Drive API access with automatic token refresh
- 📊 **Database Support**: MySQL (with plans for PostgreSQL)
- ⏰ **Scheduled Backups**: Cron-based scheduling for automated backups
- 📁 **Google Drive Integration**: Automatic folder creation and file organization
- 📈 **Backup History**: Complete tracking of backup operations
- 🗃️ **Configuration Management**: SQLite-based storage for backup configurations
- 🔄 **Auto-Migration**: Database schema automatically migrates on startup
- 🔔 **Multi-Channel Notifications**: Support for Discord, Slack, and Chatwork notifications
- 📧 **Smart Alerts**: Configurable success/error notifications with rich formatting

## Installation

```bash
go get github.com/vfa-khuongdv/go-backup-drive
```

## Quick Start

### 1. Setup Google Drive API

1. Go to the [Google Cloud Console](https://console.cloud.google.com/)
2. Create a new project or select an existing one
3. Enable the Google Drive API
4. Create credentials (OAuth 2.0 Client ID)
5. Download the client credentials

### 2. Basic Usage

```go
package main

import (
    "log"

    backup "github.com/vfa-khuongdv/go-backup-drive"
)

func main() {
    // Configuration
    config := &backup.Config{
        ClientID:     "your-google-oauth-client-id.apps.googleusercontent.com",
        ClientSecret: "your-google-oauth-client-secret",
        RedirectURL:  "http://localhost:8080/oauth2callback", // Required for OAuth2
    }

    // Create backup manager
    manager, err := backup.NewBackupManager(config)
    if err != nil {
        log.Fatalf("Failed to create backup manager: %v", err)
    }
    defer manager.Close()

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
    err = manager.AddBackupConfig(
        "my-app-db",                           // Configuration name
        "user:pass@tcp(localhost:3306)/mydb",  // Database URL
        "mysql",                               // Database type
        "0 2 * * *",                          // Cron schedule (daily at 2 AM)
    )
    if err != nil {
        log.Fatalf("Failed to add backup config: %v", err)
    }

    // Keep the program running
    select {}
}
```

### 3. One-time Backup

```go
// Perform immediate backup
result, err := manager.BackupDatabase(
    "user:pass@tcp(localhost:3306)/mydb",
    "My Database Backups", // Google Drive folder name
)
if err != nil {
    log.Fatalf("Backup failed: %v", err)
}

log.Printf("Backup successful! File ID: %s", result.FileID)
log.Printf("Google Drive link: %s", result.WebViewLink)
```

## Configuration

### Config Structure

```go
type Config struct {
    // Required: Google OAuth2 credentials
    ClientID     string
    ClientSecret string

    // Required: MySQL database configuration for storing package metadata
    ConfigDatabase *backup.MySQLConfig

    // Optional: Temporary directory for backup files
    // Defaults to system temp directory
    TempDir string
}
```

### Database URL Formats

#### MySQL

```
user:password@tcp(host:port)/database
```

Example:

```
myuser:mypass@tcp(localhost:3306)/mydatabase
```

## API Reference

### BackupManager Methods

#### Authentication

- `GetAuthURL() string` - Get OAuth2 authorization URL
- `SetAuthCode(authCode string) error` - Exchange authorization code for tokens
- `GetTokenInfo() (*TokenInfo, error)` - Get current token information
- `ValidateToken() error` - Validate current token

#### Backup Configuration

- `AddBackupConfig(name, databaseURL, databaseType, cronSchedule string) error`
- `GetBackupConfigs() ([]BackupConfig, error)`
- `UpdateBackupConfig(name, cronSchedule string, enabled bool) error`
- `DeleteBackupConfig(name string) error`

#### Manual Backups

- `BackupNow(configName string) error` - Run configured backup immediately
- `BackupDatabase(databaseURL, folderName string) (*BackupResult, error)` - One-time backup

#### Monitoring

- `GetBackupHistory(limit, offset int) ([]BackupHistory, error)`
- `GetScheduledJobs() []JobInfo`
- `GetNextRunTimes(cronExpr string, count int) ([]time.Time, error)`

#### Google Drive

- `ListBackupFiles(folderName string, maxResults int64) ([]*File, error)`
- `DeleteBackupFile(fileID string) error`

#### Notifications

- `AddNotificationConfig(name, channel, config, notifyOnSuccess, notifyOnError) error`
- `GetNotificationConfigs() ([]NotificationConfig, error)`
- `UpdateNotificationConfig(name, enabled, notifyOnSuccess, notifyOnError) error`
- `DeleteNotificationConfig(name string) error`
- `TestNotification(configName string) error`

## Cron Schedule Format

The package uses cron expressions with support for seconds:

```
# ┌───────────── second (0 - 59)
# │ ┌───────────── minute (0 - 59)
# │ │ ┌───────────── hour (0 - 23)
# │ │ │ ┌───────────── day of the month (1 - 31)
# │ │ │ │ ┌───────────── month (1 - 12)
# │ │ │ │ │ ┌───────────── day of the week (0 - 6) (Sunday to Saturday)
# │ │ │ │ │ │
# │ │ │ │ │ │
# * * * * * *
```

### Examples:

- `0 2 * * *` - Daily at 2:00 AM
- `0 */6 * * *` - Every 6 hours
- `0 0 * * 1` - Every Monday at midnight
- `30 2 1 * *` - First day of month at 2:30 AM

## Database Schema

The package automatically creates and manages these tables:

### token_config

Stores Google OAuth2 tokens with automatic refresh capability.

### backup_configs

Stores backup job configurations and schedules.

### backup_histories

Tracks all backup operations with status, file information, and error messages.

## Error Handling

The package provides detailed error information:

```go
result, err := manager.BackupDatabase(databaseURL, folderName)
if err != nil {
    log.Printf("Backup failed: %v", err)
    // Handle specific error cases
    return
}
```

Common error scenarios:

- Invalid database connection
- Google Drive authentication issues
- Network connectivity problems
- Disk space limitations
- Invalid cron expressions

## Notifications

The package supports multiple notification channels to alert you when backups complete or fail.

### Supported Channels

- **Discord**: Rich embed notifications with colors and fields
- **Slack**: Formatted attachments with status colors
- **Chatwork**: Simple text messages with emojis

### Setting up Notifications

#### Discord Webhook

1. Go to your Discord server settings
2. Navigate to Integrations > Webhooks
3. Create a new webhook and copy the URL

```go
discordConfig := map[string]interface{}{
    "webhook_url": "https://discord.com/api/webhooks/123456789/abcdefgh",
    "username":    "Database Backup Bot",
    "avatar_url":  "https://example.com/bot.png",
}

manager.AddNotificationConfig(
    "discord-alerts",
    notification.ChannelDiscord,
    discordConfig,
    true, // Notify on success
    true, // Notify on error
)
```

#### Slack Webhook

1. Go to https://api.slack.com/apps
2. Create a new app and enable Incoming Webhooks
3. Create a webhook for your channel

```go
slackConfig := map[string]interface{}{
    "webhook_url": "https://hooks.slack.com/services/T00000000/B00000000/XXXXXXX",
    "channel":     "#database-alerts",
    "username":    "DB Backup Service",
    "icon_emoji":  ":database:",
}

manager.AddNotificationConfig(
    "slack-team",
    notification.ChannelSlack,
    slackConfig,
    true, // Notify on success
    true, // Notify on error
)
```

#### Chatwork API

1. Get your API token from Chatwork account settings
2. Find the room ID from the room URL

```go
chatworkConfig := map[string]interface{}{
    "api_token": "your-chatwork-api-token",
    "room_id":   "123456789",
}

manager.AddNotificationConfig(
    "chatwork-team",
    notification.ChannelChatwork,
    chatworkConfig,
    false, // Don't notify on success
    true,  // Only notify on errors
)
```

### Testing Notifications

```go
// Send a test message to verify configuration
if err := manager.TestNotification("discord-alerts"); err != nil {
    log.Printf("Test failed: %v", err)
}
```

### Managing Notifications

```go
// List all notification configurations
configs, err := manager.GetNotificationConfigs()

// Update notification preferences
manager.UpdateNotificationConfig(
    "slack-team",
    true,  // enabled
    false, // don't notify on success
    true,  // notify on error
)

// Remove notification configuration
manager.DeleteNotificationConfig("discord-alerts")
```

### Notification Message Examples

**Success Message:**

- ✅ Backup Completed: my-app-db
- Database Type: mysql
- File Name: myapp_backup_20231201_120000.sql
- File Size: 15.2 MB
- Duration: 45s
- Google Drive Link: [View File]

**Error Message:**

- ❌ Backup Failed: my-app-db
- Database Type: mysql
- Duration: 30s
- Error: Database connection failed: access denied

## Monitoring and Logging

### View Backup History

```go
history, err := manager.GetBackupHistory(20, 0) // Last 20 backups
for _, h := range history {
    log.Printf("Backup: %s - Status: %s", h.FileName, h.Status)
}
```

### Monitor Scheduled Jobs

```go
jobs := manager.GetScheduledJobs()
for _, job := range jobs {
    log.Printf("Job: %s - Next run: %s", job.Name, job.Next.Format(time.RFC3339))
}
```

## Security Considerations

1. **Credentials Storage**: Store OAuth2 credentials securely
2. **Database URLs**: Avoid hardcoding database passwords
3. **File Permissions**: Backup files have restricted permissions
4. **Token Storage**: OAuth tokens are encrypted in SQLite database
5. **Network Security**: Use TLS for all API communications

## Troubleshooting

### Common Issues

**Authentication Failed**

```bash
Error: failed to refresh token
```

Solution: Re-authenticate using `GetAuthURL()` and `SetAuthCode()`

**Database Connection Failed**

```bash
Error: database connection test failed
```

Solution: Verify database URL format and credentials

**Backup File Upload Failed**

```bash
Error: failed to upload to Google Drive
```

Solution: Check Google Drive API quotas and network connectivity

**Invalid Cron Expression**

```bash
Error: invalid cron expression
```

Solution: Verify cron format using online validators

### Enable Debug Logging

```go
import "log"

// Set log level for detailed debugging
log.SetFlags(log.LstdFlags | log.Lshortfile)
```

## Examples

See the `cmd/example/` directory for a comprehensive example with interactive menu.

Run the example:

```bash
go run cmd/example/main.go
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Add tests for new functionality
4. Ensure all tests pass
5. Submit a pull request

## Roadmap

- [ ] PostgreSQL support
- [ ] Microsoft SQL Server support
- [ ] Backup compression
- [ ] Backup encryption
- [ ] Backup retention policies
- [ ] Email notifications
- [ ] Webhook support
- [ ] Docker support
- [ ] Kubernetes operators

## License

MIT License - see LICENSE file for details.

## Support

For support and questions:

- Create an issue on GitHub
- Check existing issues for solutions
- Review the example code in `cmd/example/`

---

## Requirements

- Go 1.19 or higher
- MySQL client tools (`mysqldump`) for MySQL backups
- Google Cloud Project with Drive API enabled
- OAuth2 credentials for Google Drive access
