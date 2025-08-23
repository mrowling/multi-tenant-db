package mysql

import (
	"log"
	"os"
	"testing"

	"multitenant-db/internal/config"
)

func TestNewDatabaseManagerWithConfig_SQLite(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	
	cfg := &config.DefaultDatabaseConfig{
		Type:             config.DatabaseTypeSQLite,
		ConnectionString: ":memory:",
	}
	
	dm := NewDatabaseManagerWithConfig(logger, cfg)
	
	if dm == nil {
		t.Fatal("DatabaseManager should not be nil")
	}
	
	// Check that default database exists
	if _, exists := dm.databases["default"]; !exists {
		t.Error("Default database should be created")
	}
	
	// Check that we can get the default database
	db, err := dm.GetOrCreateDatabase("default")
	if err != nil {
		t.Errorf("Should be able to get default database: %v", err)
	}
	if db == nil {
		t.Error("Default database should not be nil")
	}
}

func TestNewDatabaseManagerWithConfig_NilConfig(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	
	dm := NewDatabaseManagerWithConfig(logger, nil)
	
	if dm == nil {
		t.Fatal("DatabaseManager should not be nil")
	}
	
	// Should still create a default in-memory SQLite database
	if _, exists := dm.databases["default"]; !exists {
		t.Error("Default database should be created")
	}
}

func TestCreateConfiguredDatabase_SQLite(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)
	
	cfg := &config.DefaultDatabaseConfig{
		Type:             config.DatabaseTypeSQLite,
		ConnectionString: ":memory:",
	}
	
	db, err := dm.createConfiguredDatabase(cfg)
	if err != nil {
		t.Fatalf("Failed to create SQLite database: %v", err)
	}
	if db == nil {
		t.Error("Database should not be nil")
	}
	
	defer db.Close()
}

func TestCreateConfiguredDatabase_UnsupportedType(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)
	
	cfg := &config.DefaultDatabaseConfig{
		Type: "postgres", // Unsupported
	}
	
	_, err := dm.createConfiguredDatabase(cfg)
	if err == nil {
		t.Error("Should fail for unsupported database type")
	}
}

func TestIsDefaultDatabase(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)
	
	tests := []struct {
		idx      string
		expected bool
	}{
		{"", true},
		{"default", true},
		{"test1", false},
		{"customer123", false},
	}
	
	for _, tt := range tests {
		t.Run(tt.idx, func(t *testing.T) {
			result := dm.isDefaultDatabase(tt.idx)
			if result != tt.expected {
				t.Errorf("isDefaultDatabase(%q) = %v, want %v", tt.idx, result, tt.expected)
			}
		})
	}
}

func TestInitSampleData_SQLite(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	
	// Test with SQLite configuration (non-default database)
	dm := NewDatabaseManager(logger)
	
	// Create a new non-default database (should use SQLite)
	db, err := dm.GetOrCreateDatabase("test_sqlite")
	if err != nil {
		t.Fatalf("Failed to create test database: %v", err)
	}
	
	// Check that tables were created
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		t.Fatalf("Failed to query tables: %v", err)
	}
	defer rows.Close()
	
	tables := []string{}
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			t.Fatalf("Failed to scan table name: %v", err)
		}
		tables = append(tables, tableName)
	}
	
	expectedTables := []string{"users", "products"}
	if len(tables) != len(expectedTables) {
		t.Errorf("Expected %d tables, got %d", len(expectedTables), len(tables))
	}
	
	// Check that sample data exists
	var userCount int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		t.Fatalf("Failed to count users: %v", err)
	}
	if userCount != 3 {
		t.Errorf("Expected 3 users, got %d", userCount)
	}
	
	var productCount int
	err = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&productCount)
	if err != nil {
		t.Fatalf("Failed to count products: %v", err)
	}
	if productCount != 3 {
		t.Errorf("Expected 3 products, got %d", productCount)
	}
}
