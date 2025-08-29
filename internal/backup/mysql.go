package backup

import (
	"database/sql"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

type SQLConfigure struct {
	Host     string
	Port     string
	User     string
	Password string
	Database string
}

type MySQLBackup struct {
	config *SQLConfigure
}

// NewMySQLBackup creates a new MySQL backup instance
func NewMySQLBackup(config *SQLConfigure) *MySQLBackup {
	return &MySQLBackup{
		config: config,
	}
}

// BackupSchema creates a SQL dump file of the database schema and data
func (m *MySQLBackup) BackupSchema(outputDir string) (string, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_backup_%s.sql", m.config.Database, timestamp)
	outputPath := filepath.Join(outputDir, filename)

	// Use mysqldump to create the backup
	connInfo := m.buildConnectionInfo()
	if err := m.runMySQLDump(connInfo, outputPath); err != nil {
		return "", fmt.Errorf("failed to run mysqldump: %w", err)
	}

	return outputPath, nil
}

// BackupSchemaOnly creates a SQL dump file of only the database schema (no data)
func (m *MySQLBackup) BackupSchemaOnly(outputDir string) (string, error) {
	// Create output directory if it doesn't exist
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return "", fmt.Errorf("failed to create output directory: %w", err)
	}

	// Generate filename with timestamp
	timestamp := time.Now().Format("20060102_150405")
	filename := fmt.Sprintf("%s_schema_%s.sql", m.config.Database, timestamp)
	outputPath := filepath.Join(outputDir, filename)

	// Use mysqldump with --no-data flag to backup schema only
	connInfo := m.buildConnectionInfo()
	if err := m.runMySQLDumpSchemaOnly(connInfo, outputPath); err != nil {
		return "", fmt.Errorf("failed to run mysqldump for schema: %w", err)
	}

	return outputPath, nil
}

// TestConnection tests the database connection
func (m *MySQLBackup) TestConnection() error {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		m.config.User, m.config.Password, m.config.Host, m.config.Port, m.config.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping database: %w", err)
	}

	return nil
}

// GetDatabaseInfo returns basic information about the database
func (m *MySQLBackup) GetDatabaseInfo() (*DatabaseInfo, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s",
		m.config.User, m.config.Password, m.config.Host, m.config.Port, m.config.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}
	defer db.Close()

	info := &DatabaseInfo{
		Type:     "mysql",
		Database: m.config.Database,
		Host:     m.config.Host,
		Port:     m.config.Port,
	}

	// Get MySQL version
	var version string
	if err := db.QueryRow("SELECT VERSION()").Scan(&version); err == nil {
		info.Version = version
	}

	// Get table count
	var tableCount int
	if err := db.QueryRow("SELECT COUNT(*) FROM information_schema.tables WHERE table_schema = ?", m.config.Database).Scan(&tableCount); err == nil {
		info.TableCount = tableCount
	}

	return info, nil
}

// ConnectionInfo holds parsed connection details
type ConnectionInfo struct {
	Username string
	Password string
	Host     string
	Port     string
	Database string
}

// DatabaseInfo contains information about the database
type DatabaseInfo struct {
	Type       string `json:"type"`
	Database   string `json:"database"`
	Host       string `json:"host"`
	Port       string `json:"port"`
	Version    string `json:"version,omitempty"`
	TableCount int    `json:"table_count,omitempty"`
}

// buildConnectionInfo creates ConnectionInfo from SQLConfigure
func (m *MySQLBackup) buildConnectionInfo() *ConnectionInfo {
	return &ConnectionInfo{
		Username: m.config.User,
		Password: m.config.Password,
		Host:     m.config.Host,
		Port:     m.config.Port,
		Database: m.config.Database,
	}
}

// runMySQLDump executes mysqldump command
func (m *MySQLBackup) runMySQLDump(connInfo *ConnectionInfo, outputPath string) error {
	args := []string{
		fmt.Sprintf("--user=%s", connInfo.Username),
		fmt.Sprintf("--password=%s", connInfo.Password),
		fmt.Sprintf("--host=%s", connInfo.Host),
		fmt.Sprintf("--port=%s", connInfo.Port),
		"--single-transaction",
		"--routines",
		"--triggers",
		connInfo.Database,
	}

	cmd := exec.Command("mysqldump", args...)

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	// Capture stderr for error reporting
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}

// runMySQLDumpSchemaOnly executes mysqldump command for schema only
func (m *MySQLBackup) runMySQLDumpSchemaOnly(connInfo *ConnectionInfo, outputPath string) error {
	args := []string{
		fmt.Sprintf("--user=%s", connInfo.Username),
		fmt.Sprintf("--password=%s", connInfo.Password),
		fmt.Sprintf("--host=%s", connInfo.Host),
		fmt.Sprintf("--port=%s", connInfo.Port),
		"--no-data",
		"--routines",
		"--triggers",
		connInfo.Database,
	}

	cmd := exec.Command("mysqldump", args...)

	// Create output file
	outFile, err := os.Create(outputPath)
	if err != nil {
		return fmt.Errorf("failed to create output file: %w", err)
	}
	defer outFile.Close()

	cmd.Stdout = outFile

	// Capture stderr for error reporting
	var stderr strings.Builder
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("mysqldump failed: %w, stderr: %s", err, stderr.String())
	}

	return nil
}
