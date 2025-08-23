//go:build integration
// +build integration

package integration_test

import (
	"net/http"
	"testing"
	"time"

	"multitenant-db/internal/api"
	"multitenant-db/internal/logger"
	"multitenant-db/internal/mysql"
)

func TestFullSystemIntegration(t *testing.T) {
	// Setup full system
	testLogger := logger.Setup()
	mysqlHandler := mysql.NewHandler(testLogger)
	
	// Create adapter for API
	adapter := &DatabaseManagerAdapter{
		handler: mysqlHandler,
	}
	
	// Setup API handler
	apiHandler := api.NewHandler(testLogger, adapter)
	
	// Setup HTTP routes
	mux := apiHandler.SetupRoutes()
	
	// Wrap with logging middleware
	handler := apiHandler.LoggingMiddleware(mux)
	
	// Start HTTP server in background
	server := &http.Server{
		Addr:    ":8081", // Use different port for integration tests
		Handler: handler,
	}
	
	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			t.Errorf("Server failed to start: %v", err)
		}
	}()
	
	// Give server time to start
	time.Sleep(100 * time.Millisecond)
	
	// Test actual HTTP requests
	resp, err := http.Get("http://localhost:8081/health")
	if err != nil {
		t.Fatalf("Failed to make health check request: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
	
	// Clean up
	server.Close()
}

func TestDatabasePersistence(t *testing.T) {
	// Test that databases persist within the same handler session
	testLogger := logger.Setup()
	
	// Create handler instance
	handler := mysql.NewHandler(testLogger)
	
	// Create a database
	db, err := handler.GetDatabaseManager().GetOrCreateDatabase("persistence_test")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	
	// Verify database exists
	if db == nil {
		t.Fatal("Database should not be nil")
	}
	
	// Verify database persists in the same handler
	databases := handler.GetDatabaseManager().ListDatabases()
	found := false
	for _, dbName := range databases {
		if dbName == "persistence_test" {
			found = true
			break
		}
	}
	
	if !found {
		t.Error("Database should persist within the same handler session")
	}
	
	// Test database operations
	dbInstance, err := handler.GetDatabaseManager().GetOrCreateDatabase("persistence_test")
	if err != nil {
		t.Fatalf("Failed to retrieve existing database: %v", err)
	}
	
	if dbInstance == nil {
		t.Fatal("Retrieved database should not be nil")
	}
	
	// Clean up
	handler.GetDatabaseManager().DeleteDatabase("persistence_test")
}

// DatabaseManagerAdapter adapts the mysql Handler's DatabaseManager for the API
// This is duplicated from main.go for integration tests
type DatabaseManagerAdapter struct {
	handler *mysql.Handler
}

func (adapter *DatabaseManagerAdapter) GetActiveDatabases() map[string]interface{} {
	result := make(map[string]interface{})
	databases := adapter.handler.GetDatabaseManager().GetActiveDatabases()
	for k, v := range databases {
		result[k] = v
	}
	return result
}

func (adapter *DatabaseManagerAdapter) GetOrCreateDatabase(idx string) (interface{}, error) {
	return adapter.handler.GetDatabaseManager().GetOrCreateDatabase(idx)
}

func (adapter *DatabaseManagerAdapter) CreateDatabase(idx string) error {
	_, err := adapter.handler.GetDatabaseManager().GetOrCreateDatabase(idx)
	return err
}

func (adapter *DatabaseManagerAdapter) DeleteDatabase(idx string) error {
	return adapter.handler.GetDatabaseManager().DeleteDatabase(idx)
}

func (adapter *DatabaseManagerAdapter) ListDatabases() []string {
	return adapter.handler.GetDatabaseManager().ListDatabases()
}
