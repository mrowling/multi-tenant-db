package config

import (
	"os"
	"testing"
)

func TestNewConfig(t *testing.T) {
	cfg := NewConfig()
	if cfg.HTTPPort != 8080 {
		t.Errorf("Expected default HTTP port 8080, got %d", cfg.HTTPPort)
	}
	if cfg.MySQLPort != 3306 {
		t.Errorf("Expected default MySQL port 3306, got %d", cfg.MySQLPort)
	}
	if cfg.DefaultDatabase != nil {
		t.Error("Expected no default database configuration")
	}
}

func TestLoadFromEnv_HTTPPort(t *testing.T) {
	// Save original env var
	original := os.Getenv("HTTP_PORT")
	defer os.Setenv("HTTP_PORT", original)

	os.Setenv("HTTP_PORT", "9090")
	
	cfg := NewConfig()
	err := cfg.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}
	
	if cfg.HTTPPort != 9090 {
		t.Errorf("Expected HTTP port 9090, got %d", cfg.HTTPPort)
	}
}

func TestLoadFromEnv_MySQLPort(t *testing.T) {
	// Save original env var
	original := os.Getenv("MYSQL_PORT")
	defer os.Setenv("MYSQL_PORT", original)

	os.Setenv("MYSQL_PORT", "3307")
	
	cfg := NewConfig()
	err := cfg.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}
	
	if cfg.MySQLPort != 3307 {
		t.Errorf("Expected MySQL port 3307, got %d", cfg.MySQLPort)
	}
}

func TestLoadFromEnv_SQLiteDatabase(t *testing.T) {
	// Save original env vars
	originalType := os.Getenv("DEFAULT_DB_TYPE")
	originalPath := os.Getenv("DEFAULT_DB_SQLITE_PATH")
	defer func() {
		os.Setenv("DEFAULT_DB_TYPE", originalType)
		os.Setenv("DEFAULT_DB_SQLITE_PATH", originalPath)
	}()

	os.Setenv("DEFAULT_DB_TYPE", "sqlite")
	os.Setenv("DEFAULT_DB_SQLITE_PATH", "/tmp/test.db")
	
	cfg := NewConfig()
	err := cfg.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}
	
	if cfg.DefaultDatabase == nil {
		t.Fatal("Expected default database configuration")
	}
	
	if cfg.DefaultDatabase.Type != DatabaseTypeSQLite {
		t.Errorf("Expected SQLite database type, got %s", cfg.DefaultDatabase.Type)
	}
	
	if cfg.DefaultDatabase.SQLitePath != "/tmp/test.db" {
		t.Errorf("Expected SQLite path /tmp/test.db, got %s", cfg.DefaultDatabase.SQLitePath)
	}
	
	if cfg.DefaultDatabase.ConnectionString != "/tmp/test.db" {
		t.Errorf("Expected connection string /tmp/test.db, got %s", cfg.DefaultDatabase.ConnectionString)
	}
}

func TestLoadFromEnv_SQLiteInMemory(t *testing.T) {
	// Save original env var
	original := os.Getenv("DEFAULT_DB_TYPE")
	defer os.Setenv("DEFAULT_DB_TYPE", original)

	os.Setenv("DEFAULT_DB_TYPE", "sqlite")
	// Don't set path, should default to in-memory
	
	cfg := NewConfig()
	err := cfg.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}
	
	if cfg.DefaultDatabase == nil {
		t.Fatal("Expected default database configuration")
	}
	
	if cfg.DefaultDatabase.Type != DatabaseTypeSQLite {
		t.Errorf("Expected SQLite database type, got %s", cfg.DefaultDatabase.Type)
	}
	
	if cfg.DefaultDatabase.ConnectionString != ":memory:" {
		t.Errorf("Expected connection string :memory:, got %s", cfg.DefaultDatabase.ConnectionString)
	}
}

func TestLoadFromEnv_MySQLDatabase(t *testing.T) {
	// Save original env vars
	envVars := []string{
		"DEFAULT_DB_TYPE", "DEFAULT_DB_MYSQL_HOST", "DEFAULT_DB_MYSQL_PORT",
		"DEFAULT_DB_MYSQL_USER", "DEFAULT_DB_MYSQL_PASSWORD", "DEFAULT_DB_MYSQL_DATABASE",
	}
	originals := make(map[string]string)
	for _, env := range envVars {
		originals[env] = os.Getenv(env)
	}
	defer func() {
		for env, val := range originals {
			os.Setenv(env, val)
		}
	}()

	os.Setenv("DEFAULT_DB_TYPE", "mysql")
	os.Setenv("DEFAULT_DB_MYSQL_HOST", "db.example.com")
	os.Setenv("DEFAULT_DB_MYSQL_PORT", "3307")
	os.Setenv("DEFAULT_DB_MYSQL_USER", "testuser")
	os.Setenv("DEFAULT_DB_MYSQL_PASSWORD", "testpass")
	os.Setenv("DEFAULT_DB_MYSQL_DATABASE", "testdb")
	
	cfg := NewConfig()
	err := cfg.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}
	
	if cfg.DefaultDatabase == nil {
		t.Fatal("Expected default database configuration")
	}
	
	if cfg.DefaultDatabase.Type != DatabaseTypeMySQL {
		t.Errorf("Expected MySQL database type, got %s", cfg.DefaultDatabase.Type)
	}
	
	if cfg.DefaultDatabase.MySQLHost != "db.example.com" {
		t.Errorf("Expected MySQL host db.example.com, got %s", cfg.DefaultDatabase.MySQLHost)
	}
	
	if cfg.DefaultDatabase.MySQLPort != 3307 {
		t.Errorf("Expected MySQL port 3307, got %d", cfg.DefaultDatabase.MySQLPort)
	}
	
	if cfg.DefaultDatabase.MySQLUser != "testuser" {
		t.Errorf("Expected MySQL user testuser, got %s", cfg.DefaultDatabase.MySQLUser)
	}
	
	if cfg.DefaultDatabase.MySQLPassword != "testpass" {
		t.Errorf("Expected MySQL password testpass, got %s", cfg.DefaultDatabase.MySQLPassword)
	}
	
	if cfg.DefaultDatabase.MySQLDatabase != "testdb" {
		t.Errorf("Expected MySQL database testdb, got %s", cfg.DefaultDatabase.MySQLDatabase)
	}
}

func TestLoadFromEnv_DirectConnectionString(t *testing.T) {
	// Save original env var
	original := os.Getenv("DEFAULT_DB_CONNECTION_STRING")
	defer os.Setenv("DEFAULT_DB_CONNECTION_STRING", original)

	os.Setenv("DEFAULT_DB_CONNECTION_STRING", "user:pass@tcp(localhost:3306)/mydb")
	
	cfg := NewConfig()
	err := cfg.LoadFromEnv()
	if err != nil {
		t.Fatalf("LoadFromEnv failed: %v", err)
	}
	
	if cfg.DefaultDatabase == nil {
		t.Fatal("Expected default database configuration")
	}
	
	if cfg.DefaultDatabase.Type != DatabaseTypeMySQL {
		t.Errorf("Expected MySQL database type, got %s", cfg.DefaultDatabase.Type)
	}
	
	if cfg.DefaultDatabase.ConnectionString != "user:pass@tcp(localhost:3306)/mydb" {
		t.Errorf("Expected connection string user:pass@tcp(localhost:3306)/mydb, got %s", cfg.DefaultDatabase.ConnectionString)
	}
}

func TestBuildMySQLConnectionString(t *testing.T) {
	tests := []struct {
		name     string
		config   DefaultDatabaseConfig
		expected string
		hasError bool
	}{
		{
			name: "basic connection",
			config: DefaultDatabaseConfig{
				Type:      DatabaseTypeMySQL,
				MySQLUser: "user",
				MySQLHost: "localhost",
				MySQLPort: 3306,
			},
			expected: "user@tcp(localhost:3306)/?parseTime=true",
			hasError: false,
		},
		{
			name: "with password",
			config: DefaultDatabaseConfig{
				Type:          DatabaseTypeMySQL,
				MySQLUser:     "user",
				MySQLPassword: "pass",
				MySQLHost:     "localhost",
				MySQLPort:     3306,
			},
			expected: "user:pass@tcp(localhost:3306)/?parseTime=true",
			hasError: false,
		},
		{
			name: "with database",
			config: DefaultDatabaseConfig{
				Type:          DatabaseTypeMySQL,
				MySQLUser:     "user",
				MySQLPassword: "pass",
				MySQLHost:     "localhost",
				MySQLPort:     3306,
				MySQLDatabase: "mydb",
			},
			expected: "user:pass@tcp(localhost:3306)/mydb?parseTime=true",
			hasError: false,
		},
		{
			name: "with SSL mode",
			config: DefaultDatabaseConfig{
				Type:          DatabaseTypeMySQL,
				MySQLUser:     "user",
				MySQLHost:     "localhost",
				MySQLPort:     3306,
				MySQLSSLMode:  "required",
			},
			expected: "user@tcp(localhost:3306)/?tls=required&parseTime=true",
			hasError: false,
		},
		{
			name: "no user",
			config: DefaultDatabaseConfig{
				Type:      DatabaseTypeMySQL,
				MySQLHost: "localhost",
				MySQLPort: 3306,
			},
			expected: "",
			hasError: true,
		},
		{
			name: "wrong type",
			config: DefaultDatabaseConfig{
				Type: DatabaseTypeSQLite,
			},
			expected: "",
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := tt.config.BuildMySQLConnectionString()
			
			if tt.hasError {
				if err == nil {
					t.Errorf("Expected error, got none")
				}
				return
			}
			
			if err != nil {
				t.Errorf("Unexpected error: %v", err)
				return
			}
			
			if result != tt.expected {
				t.Errorf("Expected %s, got %s", tt.expected, result)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	tests := []struct {
		name     string
		config   Config
		hasError bool
	}{
		{
			name: "valid config",
			config: Config{
				HTTPPort:  8080,
				MySQLPort: 3306,
			},
			hasError: false,
		},
		{
			name: "invalid HTTP port",
			config: Config{
				HTTPPort:  0,
				MySQLPort: 3306,
			},
			hasError: true,
		},
		{
			name: "invalid MySQL port",
			config: Config{
				HTTPPort:  8080,
				MySQLPort: 70000,
			},
			hasError: true,
		},
		{
			name: "valid with SQLite default DB",
			config: Config{
				HTTPPort:  8080,
				MySQLPort: 3306,
				DefaultDatabase: &DefaultDatabaseConfig{
					Type:             DatabaseTypeSQLite,
					ConnectionString: ":memory:",
				},
			},
			hasError: false,
		},
		{
			name: "invalid SQLite config",
			config: Config{
				HTTPPort:  8080,
				MySQLPort: 3306,
				DefaultDatabase: &DefaultDatabaseConfig{
					Type: DatabaseTypeSQLite,
					// Missing connection string
				},
			},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			
			if tt.hasError && err == nil {
				t.Errorf("Expected error, got none")
			} else if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}

func TestDefaultDatabaseConfigValidate(t *testing.T) {
	tests := []struct {
		name     string
		config   DefaultDatabaseConfig
		hasError bool
	}{
		{
			name: "valid SQLite",
			config: DefaultDatabaseConfig{
				Type:             DatabaseTypeSQLite,
				ConnectionString: ":memory:",
			},
			hasError: false,
		},
		{
			name: "valid MySQL",
			config: DefaultDatabaseConfig{
				Type:      DatabaseTypeMySQL,
				MySQLUser: "user",
				MySQLHost: "localhost",
				MySQLPort: 3306,
			},
			hasError: false,
		},
		{
			name: "invalid SQLite - no connection string",
			config: DefaultDatabaseConfig{
				Type: DatabaseTypeSQLite,
			},
			hasError: true,
		},
		{
			name: "invalid MySQL - no user",
			config: DefaultDatabaseConfig{
				Type: DatabaseTypeMySQL,
			},
			hasError: true,
		},
		{
			name: "unsupported type",
			config: DefaultDatabaseConfig{
				Type: "postgres",
			},
			hasError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			
			if tt.hasError && err == nil {
				t.Errorf("Expected error, got none")
			} else if !tt.hasError && err != nil {
				t.Errorf("Unexpected error: %v", err)
			}
		})
	}
}
