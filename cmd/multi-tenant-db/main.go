package main

import (
	"flag"
	"fmt"
	"net/http"
	"time"

	"multitenant-db/internal/api"
	"multitenant-db/internal/config"
	"multitenant-db/internal/logger"
	"multitenant-db/internal/mysql"
)

// DatabaseManagerAdapter adapts the mysql Handler's DatabaseManager for the API
type DatabaseManagerAdapter struct {
	handler *mysql.Handler
}

// GetActiveDatabases returns active databases as map[string]interface{}
func (adapter *DatabaseManagerAdapter) GetActiveDatabases() map[string]interface{} {
	// This method isn't used by the API, but we need it for the interface
	result := make(map[string]interface{})
	databases := adapter.handler.GetDatabaseManager().GetActiveDatabases()
	for idx := range databases {
		result[idx] = true
	}
	return result
}

// GetOrCreateDatabase creates a database for the given idx
func (adapter *DatabaseManagerAdapter) GetOrCreateDatabase(idx string) (interface{}, error) {
	return adapter.handler.GetDatabaseManager().GetOrCreateDatabase(idx)
}

// DeleteDatabase deletes a database for the given idx
func (adapter *DatabaseManagerAdapter) DeleteDatabase(idx string) error {
	return adapter.handler.GetDatabaseManager().DeleteDatabase(idx)
}

// ListDatabases returns a list of database indices
func (adapter *DatabaseManagerAdapter) ListDatabases() []string {
	return adapter.handler.GetDatabaseManager().ListDatabases()
}

func main() {
	// Parse command line flags
	var (
		dbType     = flag.String("default-db-type", "", "Default database type (sqlite or mysql)")
		dbPath     = flag.String("default-db-path", "", "SQLite database file path (for sqlite type)")
		dbHost     = flag.String("default-db-host", "", "MySQL host (for mysql type)")
		dbPort     = flag.Int("default-db-port", 3306, "MySQL port (for mysql type)")
		dbUser     = flag.String("default-db-user", "", "MySQL username (for mysql type)")
		dbPassword = flag.String("default-db-password", "", "MySQL password (for mysql type)")
		dbName     = flag.String("default-db-name", "", "MySQL database name (for mysql type)")
		dbSSLMode  = flag.String("default-db-ssl-mode", "", "MySQL SSL mode (for mysql type)")
		httpPort   = flag.Int("http-port", 8080, "HTTP server port")
		mysqlPort  = flag.Int("mysql-port", 3306, "MySQL protocol server port")
	)
	flag.Parse()

	// Setup logger
	appLogger := logger.Setup()
	appLogger.Println("Starting Multitenant DB server...")
	
	// Load configuration
	cfg := config.NewConfig()
	
	// Override from environment variables
	if err := cfg.LoadFromEnv(); err != nil {
		appLogger.Fatalf("Failed to load configuration from environment: %v", err)
	}
	
	// Override from command line flags
	if *httpPort != 8080 {
		cfg.HTTPPort = *httpPort
	}
	if *mysqlPort != 3306 {
		cfg.MySQLPort = *mysqlPort
	}
	
	// Configure default database from command line flags
	if *dbType != "" {
		cfg.DefaultDatabase = &config.DefaultDatabaseConfig{
			Type: config.DatabaseType(*dbType),
		}
		
		switch *dbType {
		case "sqlite":
			if *dbPath != "" {
				cfg.DefaultDatabase.SQLitePath = *dbPath
				cfg.DefaultDatabase.ConnectionString = *dbPath
			} else {
				cfg.DefaultDatabase.ConnectionString = ":memory:"
			}
			
		case "mysql":
			cfg.DefaultDatabase.MySQLHost = *dbHost
			cfg.DefaultDatabase.MySQLPort = *dbPort
			cfg.DefaultDatabase.MySQLUser = *dbUser
			cfg.DefaultDatabase.MySQLPassword = *dbPassword
			cfg.DefaultDatabase.MySQLDatabase = *dbName
			cfg.DefaultDatabase.MySQLSSLMode = *dbSSLMode
			
			// Build connection string
			connStr, err := cfg.DefaultDatabase.BuildMySQLConnectionString()
			if err != nil {
				appLogger.Fatalf("Failed to build MySQL connection string: %v", err)
			}
			cfg.DefaultDatabase.ConnectionString = connStr
		}
	}
	
	// Validate configuration
	if err := cfg.Validate(); err != nil {
		appLogger.Fatalf("Invalid configuration: %v", err)
	}
	
	// Log default database configuration if present
	if cfg.DefaultDatabase != nil {
		appLogger.Printf("Using configured default database: %s", cfg.DefaultDatabase.Type)
		if cfg.DefaultDatabase.Type == config.DatabaseTypeSQLite {
			appLogger.Printf("SQLite database: %s", cfg.DefaultDatabase.ConnectionString)
		} else if cfg.DefaultDatabase.Type == config.DatabaseTypeMySQL {
			appLogger.Printf("MySQL database: %s", cfg.DefaultDatabase.MySQLHost)
		}
	} else {
		appLogger.Printf("Using default in-memory SQLite database")
	}
	
	// Create MySQL protocol handler with configuration
	mysqlHandler := mysql.NewHandlerWithConfig(appLogger, cfg)
	
	// Start MySQL protocol server in a goroutine
	go mysql.StartServer(cfg.MySQLPort, mysqlHandler)
	
	// Create database manager adapter for API
	dbManagerAdapter := &DatabaseManagerAdapter{mysqlHandler}
	
	// Create API handler
	apiHandler := api.NewHandler(appLogger, dbManagerAdapter)
	
	// Setup HTTP routes
	mux := apiHandler.SetupRoutes()
	
	// Wrap with logging middleware
	handler := apiHandler.LoggingMiddleware(mux)
	
	// HTTP Server configuration
	server := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.HTTPPort),
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	appLogger.Printf("HTTP server starting on port %d", cfg.HTTPPort)
	appLogger.Printf("MySQL protocol server starting on port %d", cfg.MySQLPort)
	
	appLogger.Printf("Available HTTP endpoints:")
	
	endpoints := []string{
		fmt.Sprintf("http://localhost:%d/", cfg.HTTPPort),
		fmt.Sprintf("http://localhost:%d/health", cfg.HTTPPort),
		fmt.Sprintf("http://localhost:%d/api/info", cfg.HTTPPort),
	}
	
	for _, endpoint := range endpoints {
		appLogger.Printf("  %s", endpoint)
	}
	
	appLogger.Printf("MySQL connection: mysql -h 127.0.0.1 -P %d -u root --protocol=TCP", cfg.MySQLPort)
	
	// Start HTTP server
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		appLogger.Fatalf("HTTP server failed to start: %v", err)
	}
}
