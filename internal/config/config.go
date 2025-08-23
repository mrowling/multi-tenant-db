package config

import (
	"fmt"
	"net/url"
	"os"
	"strconv"
	"strings"
)

// DatabaseType represents the type of default database
type DatabaseType string

const (
	DatabaseTypeSQLite DatabaseType = "sqlite"
	DatabaseTypeMySQL  DatabaseType = "mysql"
)

// DefaultDatabaseConfig holds configuration for the default database
type DefaultDatabaseConfig struct {
	Type             DatabaseType `json:"type"`
	ConnectionString string       `json:"connection_string"`
	SQLitePath       string       `json:"sqlite_path,omitempty"`       // Path for SQLite file (optional)
	MySQLHost        string       `json:"mysql_host,omitempty"`        // MySQL host
	MySQLPort        int          `json:"mysql_port,omitempty"`        // MySQL port
	MySQLUser        string       `json:"mysql_user,omitempty"`        // MySQL username
	MySQLPassword    string       `json:"mysql_password,omitempty"`    // MySQL password
	MySQLDatabase    string       `json:"mysql_database,omitempty"`    // MySQL database name
	MySQLSSLMode     string       `json:"mysql_ssl_mode,omitempty"`    // MySQL SSL mode
}

// AuthConfig holds authentication configuration for MySQL protocol connections
type AuthConfig struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// Config holds the application configuration
type Config struct {
	DefaultDatabase *DefaultDatabaseConfig `json:"default_database,omitempty"`
	Auth            *AuthConfig            `json:"auth,omitempty"`
	HTTPPort        int                    `json:"http_port"`
	MySQLPort       int                    `json:"mysql_port"`
}

// NewConfig creates a new configuration with default values
func NewConfig() *Config {
	return &Config{
		HTTPPort:  8080,
		MySQLPort: 3306,
	}
}

// LoadFromEnv loads configuration from environment variables
func (c *Config) LoadFromEnv() error {
	// HTTP Port
	if port := os.Getenv("HTTP_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.HTTPPort = p
		}
	}

	// MySQL Port
	if port := os.Getenv("MYSQL_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			c.MySQLPort = p
		}
	}

	// Authentication Configuration
	if username := os.Getenv("AUTH_USERNAME"); username != "" {
		c.Auth = &AuthConfig{
			Username: username,
			Password: os.Getenv("AUTH_PASSWORD"),
		}
	} else if os.Getenv("AUTH_PASSWORD") != "" {
		// If only password is provided, use default username
		c.Auth = &AuthConfig{
			Username: "root",
			Password: os.Getenv("AUTH_PASSWORD"),
		}
	}

	// Default Database Configuration
	if dbType := os.Getenv("DEFAULT_DB_TYPE"); dbType != "" {
		c.DefaultDatabase = &DefaultDatabaseConfig{
			Type: DatabaseType(strings.ToLower(dbType)),
		}

		switch c.DefaultDatabase.Type {
		case DatabaseTypeSQLite:
			if path := os.Getenv("DEFAULT_DB_SQLITE_PATH"); path != "" {
				c.DefaultDatabase.SQLitePath = path
				c.DefaultDatabase.ConnectionString = path
			} else {
				// If no path specified, use in-memory
				c.DefaultDatabase.ConnectionString = ":memory:"
			}

		case DatabaseTypeMySQL:
			// Load MySQL configuration from environment
			host := os.Getenv("DEFAULT_DB_MYSQL_HOST")
			if host == "" {
				host = "localhost"
			}
			c.DefaultDatabase.MySQLHost = host

			port := 3306
			if portStr := os.Getenv("DEFAULT_DB_MYSQL_PORT"); portStr != "" {
				if p, err := strconv.Atoi(portStr); err == nil {
					port = p
				}
			}
			c.DefaultDatabase.MySQLPort = port

			c.DefaultDatabase.MySQLUser = os.Getenv("DEFAULT_DB_MYSQL_USER")
			c.DefaultDatabase.MySQLPassword = os.Getenv("DEFAULT_DB_MYSQL_PASSWORD")
			c.DefaultDatabase.MySQLDatabase = os.Getenv("DEFAULT_DB_MYSQL_DATABASE")
			c.DefaultDatabase.MySQLSSLMode = os.Getenv("DEFAULT_DB_MYSQL_SSL_MODE")

			// Build connection string
			connStr, err := c.DefaultDatabase.BuildMySQLConnectionString()
			if err != nil {
				return fmt.Errorf("failed to build MySQL connection string: %v", err)
			}
			c.DefaultDatabase.ConnectionString = connStr

		default:
			return fmt.Errorf("unsupported default database type: %s", dbType)
		}
	}

	// Also support direct connection string override
	if connStr := os.Getenv("DEFAULT_DB_CONNECTION_STRING"); connStr != "" {
		if c.DefaultDatabase == nil {
			c.DefaultDatabase = &DefaultDatabaseConfig{}
		}
		c.DefaultDatabase.ConnectionString = connStr
		
		// Try to detect type from connection string
		if strings.HasPrefix(connStr, "mysql://") || strings.Contains(connStr, "@tcp(") {
			c.DefaultDatabase.Type = DatabaseTypeMySQL
		} else {
			c.DefaultDatabase.Type = DatabaseTypeSQLite
		}
	}

	return nil
}

// BuildMySQLConnectionString builds a MySQL connection string from the configuration
func (dbc *DefaultDatabaseConfig) BuildMySQLConnectionString() (string, error) {
	if dbc.Type != DatabaseTypeMySQL {
		return "", fmt.Errorf("not a MySQL configuration")
	}

	// If a full connection string is already provided, use it
	if dbc.ConnectionString != "" && (strings.HasPrefix(dbc.ConnectionString, "mysql://") || strings.Contains(dbc.ConnectionString, "@tcp(")) {
		return dbc.ConnectionString, nil
	}

	// Build connection string from components
	if dbc.MySQLUser == "" {
		return "", fmt.Errorf("MySQL user is required")
	}

	host := dbc.MySQLHost
	if host == "" {
		host = "localhost"
	}

	port := dbc.MySQLPort
	if port == 0 {
		port = 3306
	}

	// Build DSN in the format: user:password@tcp(host:port)/database?params
	dsn := fmt.Sprintf("%s", dbc.MySQLUser)
	
	if dbc.MySQLPassword != "" {
		dsn += ":" + dbc.MySQLPassword
	}
	
	dsn += fmt.Sprintf("@tcp(%s:%d)", host, port)
	
	if dbc.MySQLDatabase != "" {
		dsn += "/" + dbc.MySQLDatabase
	} else {
		dsn += "/"
	}

	// Add SSL mode if specified
	params := []string{}
	if dbc.MySQLSSLMode != "" {
		params = append(params, "tls="+url.QueryEscape(dbc.MySQLSSLMode))
	}

	// Add parseTime for better time handling
	params = append(params, "parseTime=true")

	if len(params) > 0 {
		dsn += "?" + strings.Join(params, "&")
	}

	return dsn, nil
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.HTTPPort <= 0 || c.HTTPPort > 65535 {
		return fmt.Errorf("invalid HTTP port: %d", c.HTTPPort)
	}

	if c.MySQLPort <= 0 || c.MySQLPort > 65535 {
		return fmt.Errorf("invalid MySQL port: %d", c.MySQLPort)
	}

	if c.DefaultDatabase != nil {
		if err := c.DefaultDatabase.Validate(); err != nil {
			return fmt.Errorf("invalid default database configuration: %v", err)
		}
	}

	if c.Auth != nil {
		if err := c.Auth.Validate(); err != nil {
			return fmt.Errorf("invalid authentication configuration: %v", err)
		}
	}

	return nil
}

// Validate validates the default database configuration
func (dbc *DefaultDatabaseConfig) Validate() error {
	switch dbc.Type {
	case DatabaseTypeSQLite:
		if dbc.ConnectionString == "" {
			return fmt.Errorf("SQLite connection string is required")
		}
	case DatabaseTypeMySQL:
		if dbc.MySQLUser == "" && !strings.Contains(dbc.ConnectionString, "@") {
			return fmt.Errorf("MySQL user is required")
		}
		if dbc.MySQLHost == "" && !strings.Contains(dbc.ConnectionString, "@tcp(") {
			dbc.MySQLHost = "localhost" // Set default
		}
		if dbc.MySQLPort == 0 && !strings.Contains(dbc.ConnectionString, "@tcp(") {
			dbc.MySQLPort = 3306 // Set default
		}
	default:
		return fmt.Errorf("unsupported database type: %s", dbc.Type)
	}

	return nil
}

// Validate validates the authentication configuration
func (ac *AuthConfig) Validate() error {
	if ac.Username == "" {
		return fmt.Errorf("username is required")
	}
	// Password can be empty (for development/testing)
	return nil
}
