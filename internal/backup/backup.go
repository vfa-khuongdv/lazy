package backup

import (
	"fmt"
	"strings"
)

// Backup interface defines the methods that all database backup implementations must provide
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

// BackupType represents the type of backup to perform
type BackupType string

const (
	BackupTypeFull   BackupType = "full"
	BackupTypeSchema BackupType = "schema"
)

// DatabaseType represents supported database types
type DatabaseType string

const (
	DatabaseTypeMySQL      DatabaseType = "mysql"
	DatabaseTypePostgreSQL DatabaseType = "postgresql"
)

// DatabaseConfig interface defines the contract for database configurations
type DatabaseConfig interface {
	GetType() DatabaseType
	Validate() error
	GetConnectionString() string
}

// MySQLConfig represents MySQL database configuration
type MySQLConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
}

// GetType returns the database type
func (c *MySQLConfig) GetType() DatabaseType {
	return DatabaseTypeMySQL
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

// PostgreSQLConfig represents PostgreSQL database configuration (for future use)
type PostgreSQLConfig struct {
	Host     string `json:"host"`
	Port     string `json:"port"`
	User     string `json:"user"`
	Password string `json:"password"`
	Database string `json:"database"`
	SSLMode  string `json:"ssl_mode,omitempty"`
}

// GetType returns the database type
func (c *PostgreSQLConfig) GetType() DatabaseType {
	return DatabaseTypePostgreSQL
}

// Validate validates the PostgreSQL configuration
func (c *PostgreSQLConfig) Validate() error {
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

// GetConnectionString returns the PostgreSQL connection string
func (c *PostgreSQLConfig) GetConnectionString() string {
	connStr := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s",
		c.Host, c.Port, c.User, c.Password, c.Database)
	if c.SSLMode != "" {
		connStr += fmt.Sprintf(" sslmode=%s", c.SSLMode)
	}
	return connStr
}

// NewBackupWithConfig creates a backup instance using the new DatabaseConfig interface
func NewBackupWithConfig(config DatabaseConfig) (Backup, error) {
	if err := config.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	switch config.GetType() {
	case DatabaseTypeMySQL:
		mysqlConfig := config.(*MySQLConfig)
		return NewMySQLBackup(mysqlConfig.ToSQLConfigure()), nil
	case DatabaseTypePostgreSQL:
		return nil, fmt.Errorf("PostgreSQL support is not implemented yet")
	default:
		return nil, fmt.Errorf("unsupported database type: %s", config.GetType())
	}
}

// NewBackup creates a backup instance based on the database type and configuration (backward compatibility)
func NewBackup(dbType string, config *SQLConfigure) (Backup, error) {
	switch strings.ToLower(dbType) {
	case "mysql":
		return NewMySQLBackup(config), nil
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbType)
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

	// Parse username:password@host:port/database or username:password@tcp(host:port)/database
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

	// Handle tcp() format if present
	if strings.HasPrefix(remaining, "tcp(") {
		remaining = strings.TrimPrefix(remaining, "tcp(")
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

	// Remove query parameters if present
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

// NewBackupFromURL creates a backup instance from a database URL (for backward compatibility)
func NewBackupFromURL(databaseURL string) (Backup, error) {
	url := strings.ToLower(databaseURL)

	if strings.Contains(url, "mysql") || strings.Contains(url, "tcp(") {
		config, err := ParseMySQLURL(databaseURL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse MySQL URL: %w", err)
		}
		return NewMySQLBackupWithConfig(config)
	}

	return nil, fmt.Errorf("unsupported database type in URL: %s", databaseURL)
}

// ParseDatabaseURL parses a database URL and returns the appropriate DatabaseConfig
func ParseDatabaseURL(databaseURL string) (DatabaseConfig, error) {
	url := strings.ToLower(databaseURL)

	if strings.Contains(url, "mysql") || strings.Contains(url, "tcp(") {
		return ParseMySQLURL(databaseURL)
	}

	if strings.Contains(url, "postgres") || strings.Contains(url, "postgresql") {
		return nil, fmt.Errorf("PostgreSQL URL parsing is not implemented yet")
	}

	return nil, fmt.Errorf("unsupported database type in URL: %s", databaseURL)
}

// BackupResult contains information about a completed backup operation
type BackupResult struct {
	FilePath     string        `json:"file_path"`
	FileName     string        `json:"file_name"`
	Size         int64         `json:"size"`
	BackupType   BackupType    `json:"backup_type"`
	DatabaseInfo *DatabaseInfo `json:"database_info"`
}
