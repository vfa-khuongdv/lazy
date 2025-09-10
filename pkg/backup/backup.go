package backup

import (
	"fmt"
	"strings"
)

// Backup defines methods for performing MySQL backups
type Backup interface {
	// BackupSchema creates a full backup (schema + data) and returns the file path
	BackupSchema(outputDir string) (string, error)

	// BackupSchemaOnly creates a schema-only backup and returns the file path
	BackupSchemaOnly(outputDir string) (string, error)

	// TestConnection tests the database connection
	TestConnection() error

	// GetDatabaseInfo returns information about the database
	GetDatabaseInfo() (*DatabaseInfo, error)
}

// BackupMode represents the type of backup to perform
type BackupMode string

const (
	FullBackup   BackupMode = "full"   // Full backup (schema + data)
	SchemaBackup BackupMode = "schema" // Schema-only backup
)

// MySQLConfig represents MySQL database configuration
type MySQLConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

type SchedulerConfig struct {
	Name           string       `json:"name,omitempty"`
	BackupMode     string       `json:"backup_mode,omitempty"`
	DatabaseConfig *MySQLConfig `json:"database_config,omitempty"`
	CronExpression string       `json:"cron_expression,omitempty"`
}

// Validate validates the MySQL configuration
func (c *MySQLConfig) Validate() error {
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

// GetConnectionString returns the MySQL connection string
func (c *MySQLConfig) GetConnectionString() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%s)/%s", c.User, c.Password, c.Host, c.Port, c.Database)
}

// ToSQLConfigure converts MySQLConfig to SQLConfigure for backward compatibility
func (c *MySQLConfig) ToSQLConfigure() *SQLConfigure {
	return &SQLConfigure{
		Host:     c.Host,
		Port:     c.Port,
		User:     c.User,
		Password: c.Password,
		Database: c.Database,
	}
}

// NewMySQLBackupWithConfig creates a MySQL backup instance using MySQLConfig
func NewMySQLBackupWithConfig(config *MySQLConfig) (Backup, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid MySQL configuration: %w", err)
	}
	return NewMySQLBackup(config.ToSQLConfigure()), nil
}

// ParseMySQLURL parses a MySQL connection URL into MySQLConfig
// Expected format: mysql://username:password@host:port/database or username:password@tcp(host:port)/database
func ParseMySQLURL(databaseURL string) (*MySQLConfig, error) {
	url := strings.TrimPrefix(databaseURL, "mysql://")

	parts := strings.Split(url, "@")
	if len(parts) != 2 {
		return nil, fmt.Errorf("invalid connection URL format")
	}

	// Extract username:password
	userPass := parts[0]
	userPassParts := strings.Split(userPass, ":")
	if len(userPassParts) != 2 {
		return nil, fmt.Errorf("invalid username:password format")
	}

	// Extract host:port/database
	remaining := parts[1]

	// Handle tcp() format
	if after, ok := strings.CutPrefix(remaining, "tcp("); ok {
		remaining = after
		parenIndex := strings.Index(remaining, ")")
		if parenIndex == -1 {
			return nil, fmt.Errorf("missing closing parenthesis in connection URL")
		}
		hostPort := remaining[:parenIndex]
		dbPart := remaining[parenIndex+1:]

		hostPortParts := strings.Split(hostPort, ":")
		if len(hostPortParts) != 2 {
			return nil, fmt.Errorf("invalid host:port format")
		}

		database := strings.TrimPrefix(dbPart, "/")
		if queryIndex := strings.Index(database, "?"); queryIndex != -1 {
			database = database[:queryIndex]
		}

		return &MySQLConfig{
			Host:     hostPortParts[0],
			Port:     hostPortParts[1],
			User:     userPassParts[0],
			Password: userPassParts[1],
			Database: database,
		}, nil
	}

	// Handle standard host:port/database format
	slashIndex := strings.Index(remaining, "/")
	if slashIndex == -1 {
		return nil, fmt.Errorf("missing database name")
	}

	hostPort := remaining[:slashIndex]
	database := remaining[slashIndex+1:]

	if queryIndex := strings.Index(database, "?"); queryIndex != -1 {
		database = database[:queryIndex]
	}

	hostPortParts := strings.Split(hostPort, ":")
	if len(hostPortParts) != 2 {
		return nil, fmt.Errorf("invalid host:port format")
	}

	return &MySQLConfig{
		Host:     hostPortParts[0],
		Port:     hostPortParts[1],
		User:     userPassParts[0],
		Password: userPassParts[1],
		Database: database,
	}, nil
}

// NewBackupFromURL creates a MySQL backup instance from a connection URL
func NewBackupFromURL(databaseURL string) (Backup, error) {
	config, err := ParseMySQLURL(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("failed to parse MySQL URL: %w", err)
	}
	return NewMySQLBackupWithConfig(config)
}
