package main

import (
	"net/http"
	"time"

	"multitenant-db/api"
	"multitenant-db/logger"
	"multitenant-db/mysql"
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
	// Setup logger
	appLogger := logger.Setup()
	appLogger.Println("Starting Multitenant DB server...")
	
	// Create MySQL protocol handler
	mysqlHandler := mysql.NewHandler(appLogger)
	
	// Start MySQL protocol server in a goroutine
	go mysql.StartServer(3306, mysqlHandler)
	
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
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	appLogger.Printf("HTTP server starting on port 8080")
	appLogger.Printf("MySQL protocol server starting on port 3306")
	
	appLogger.Printf("Available HTTP endpoints:")
	
	endpoints := []string{
		"http://localhost:8080/",
		"http://localhost:8080/health",
		"http://localhost:8080/api/info",
	}
	
	for _, endpoint := range endpoints {
		appLogger.Printf("  %s", endpoint)
	}
	
	appLogger.Printf("MySQL connection: mysql -h 127.0.0.1 -P 3306 -u root --protocol=TCP")
	
	// Start HTTP server
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		appLogger.Fatalf("HTTP server failed to start: %v", err)
	}
}
