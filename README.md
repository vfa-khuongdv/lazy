# Lazy - Database Backup Service

A Go-based automated database backup service that backs up MySQL databases to Google Drive with multi-channel notifications support.

## Features

- **Automated MySQL Backups**: Schedule regular database backups using cron expressions
- **Google Drive Integration**: Automatically upload backups to Google Drive with OAuth2 authentication
- **Multi-Channel Notifications**: Send backup status notifications via Slack, Discord, and Chatwork
- **Flexible Backup Modes**: Support for full backups (schema + data) or schema-only backups
- **Web Interface**: RESTful API for managing backup configurations
- **Backup History**: Track all backup operations with detailed logs
- **Configuration Management**: Store and manage multiple backup configurations

## Architecture

```
├── cmd/example/           # Example application entry point
├── internal/
│   ├── auth/             # OAuth2 authentication service
│   ├── backup/           # Database backup implementations
│   ├── database/         # Database models and service layer
│   ├── notification/     # Multi-channel notification system
│   └── scheduler/        # Cron-based job scheduling
├── pkg/
│   └── gdrive/          # Google Drive API integration
└── lazy.go              # Main package interface
```

## Prerequisites

- Go 1.25+
- MySQL 5.7+ or 8.0+
- `mysqldump` utility (usually comes with MySQL client)
- Google Cloud Project with Drive API enabled
- OAuth2 credentials for Google Drive access

## Installation and Usage via go get

You can use this package directly in your Go project by running the following command:

```bash
go get github.com/vfa-khuongdv/lazy
```

After installation, you can import and use the functions from the package as shown below:

```go
import "github.com/vfa-khuongdv/lazy"

func main() {
    // Use functions from the lazy package
}
```

3. Set up Google OAuth2 credentials:
   - Create a project in Google Cloud Console
   - Enable Google Drive API
   - Create OAuth2 credentials (Web application)
   - Configure redirect URL: `http://localhost:8081/auth/google/callback`

## Configuration

### Basic Usage

```go
package main

import (
    "github.com/vfa-khuongdv/lazy"
    "github.com/vfa-khuongdv/lazy/pkg/backup"
    "github.com/vfa-khuongdv/lazy/pkg/notification"
    "golang.org/x/oauth2"
)

func main() {
    // MySQL configuration for storing backup metadata
    sqlConfig := lazy.NewMySQLConfig(
        "localhost",     // host
        "3306",         // port
        "root",         // user
        "password",     // password
        "backup_db",    // database name
    )

    // OAuth2 configuration
    authConfig := &oauth2.Config{
        ClientID:     "your-client-id",
        ClientSecret: "your-client-secret",
        RedirectURL:  "http://localhost:8081/auth/google/callback",
    }

    // Backup schedules
    schedules := []backup.SchedulerConfig{
        {
            Name:           "daily-backup",
            BackupMode:     "full",
            DatabaseConfig: sqlConfig,
            CronExpression: "0 2 * * *", // Daily at 2 AM
        },
    }

    // Notification configurations
    notifications := []notification.NotificationConfig{
        {
            Name:    "slack-alerts",
            Channel: "slack",
            Config: map[string]interface{}{
                "webhook_url": "your-slack-webhook-url",
                "channel":     "#backups",
            },
            NotifyOnSuccess: true,
            NotifyOnError:   true,
            Enabled:         true,
        },
    }

    config := &lazy.Config{
        OAuthConfig:        authConfig,
        DatabaseConfig:     sqlConfig,
        SchedulerConfig:    schedules,
        NotificationConfig: notifications,
    }

    manager, err := lazy.NewBackupManager(config)
    if err != nil {
        log.Fatal(err)
    }
    defer manager.Close()

    // Initialize and start
    if err := manager.Initialize(); err != nil {
        log.Fatal(err)
    }

    // First-time authentication
    if tokenInfo, _ := manager.GetTokenInfo(); !tokenInfo.HasToken {
        authURL := manager.GetAuthURL()
        fmt.Printf("Visit: %s\n", authURL)
        
        var authCode string
        fmt.Print("Enter authorization code: ")
        fmt.Scanln(&authCode)
        
        if err := manager.SetAuthCode(authCode); err != nil {
            log.Fatal(err)
        }
    }

    // Keep running
    select {}
}
```

### Notification Channels

#### Slack
```go
{
    Name:    "slack-team",
    Channel: "slack",
    Config: map[string]interface{}{
        "webhook_url": "https://hooks.slack.com/services/...",
        "channel":     "#backups",
        "username":    "Backup Bot",
    },
    NotifyOnSuccess: true,
    NotifyOnError:   true,
    Enabled:         true,
}
```

#### Discord
```go
{
    Name:    "discord-team",
    Channel: "discord",
    Config: map[string]interface{}{
        "webhook_url": "https://discord.com/api/webhooks/...",
        "username":    "Database Backup Bot",
        "avatar_url":  "https://example.com/bot.png",
    },
    NotifyOnSuccess: true,
    NotifyOnError:   true,
    Enabled:         true,
}
```

#### Chatwork
```go
{
    Name:    "chatwork-team",
    Channel: "chatwork",
    Config: map[string]interface{}{
        "api_token": "your-chatwork-api-token",
        "room_id":   "room-id",
    },
    NotifyOnSuccess: true,
    NotifyOnError:   true,
    Enabled:         true,
}
```

## Backup Modes

- **full**: Complete backup including schema and data
- **schema**: Schema-only backup (structure without data)

## Cron Expression Examples

- `0 2 * * *` - Daily at 2:00 AM
- `0 */6 * * *` - Every 6 hours
- `0 0 * * 0` - Weekly on Sunday at midnight
- `0 0 1 * *` - Monthly on the 1st at midnight

## API Endpoints

The service provides RESTful endpoints for managing configurations:

- `GET /api/backups` - List backup configurations
- `POST /api/backups` - Create backup configuration
- `PUT /api/backups/{name}` - Update backup configuration
- `DELETE /api/backups/{name}` - Delete backup configuration
- `GET /api/history` - Get backup history
- `POST /api/test-notification/{name}` - Test notification channel

## Development

### Running Tests
```bash
make test
```

### Building
```bash
make build
```

### Running Example
```bash
make run-example
```

### Code Quality
```bash
make lint
make fmt
```

## Database Schema

The service uses the following tables:
- `dbu_token_configs` - OAuth2 tokens
- `dbu_backup_configs` - Backup configurations
- `dbu_backup_histories` - Backup operation logs
- `dbu_notification_configs` - Notification channel configurations

## Security Considerations

- Store sensitive credentials (OAuth2 secrets, API tokens) securely
- Use environment variables for production deployments
- Regularly rotate API tokens and credentials
- Ensure proper database access controls
- Use HTTPS for webhook URLs

## Troubleshooting

### Common Issues

1. **mysqldump not found**: Ensure MySQL client tools are installed
2. **Authentication failed**: Verify OAuth2 credentials and redirect URL
3. **Database connection failed**: Check MySQL connection parameters
4. **Notification delivery failed**: Verify webhook URLs and API tokens

### Logs

The service provides detailed logging for:
- Backup operations
- Authentication events
- Notification delivery
- Scheduler activities

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Run `make test` and `make lint`
6. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Support

For issues and questions:
- Create an issue on GitHub
- Check the troubleshooting section
- Review the example implementation in `cmd/example/`