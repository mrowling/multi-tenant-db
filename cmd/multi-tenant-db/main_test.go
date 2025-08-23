package main

import (
	"log"
	"os"
	"testing"

	"multitenant-db/cmd/multi-tenant-db/logger"
	"multitenant-db/cmd/multi-tenant-db/mysql"
)

func TestDatabaseManagerAdapter(t *testing.T) {
	// Setup
	testLogger := logger.Setup()
	mysqlHandler := mysql.NewHandler(testLogger)
	adapter := &DatabaseManagerAdapter{handler: mysqlHandler}

	// Test GetActiveDatabases
	activeDbs := adapter.GetActiveDatabases()
	if activeDbs == nil {
		t.Error("GetActiveDatabases should not return nil")
	}
	if len(activeDbs) == 0 {
		t.Error("Should have at least default database")
	}

	// Test GetOrCreateDatabase
	db, err := adapter.GetOrCreateDatabase("test_adapter")
	if err != nil {
		t.Errorf("Should be able to create database: %v", err)
	}
	if db == nil {
		t.Error("Created database should not be nil")
	}

	// Test ListDatabases
	databases := adapter.ListDatabases()
	if len(databases) == 0 {
		t.Error("Should list at least one database")
	}

	// Verify test database is in list
	found := false
	for _, dbName := range databases {
		if dbName == "test_adapter" {
			found = true
			break
		}
	}
	if !found {
		t.Error("Created database should be in list")
	}

	// Test DeleteDatabase
	err = adapter.DeleteDatabase("test_adapter")
	if err != nil {
		t.Errorf("Should be able to delete database: %v", err)
	}

	// Verify database is removed from list
	databases = adapter.ListDatabases()
	for _, dbName := range databases {
		if dbName == "test_adapter" {
			t.Error("Deleted database should not be in list")
		}
	}

	// Test DeleteDatabase with default (should fail)
	err = adapter.DeleteDatabase("default")
	if err == nil {
		t.Error("Should not be able to delete default database")
	}
}

func TestDatabaseManagerAdapter_GetActiveDatabases(t *testing.T) {
	testLogger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mysqlHandler := mysql.NewHandler(testLogger)
	adapter := &DatabaseManagerAdapter{handler: mysqlHandler}

	// Create some databases
	adapter.GetOrCreateDatabase("db1")
	adapter.GetOrCreateDatabase("db2")

	activeDbs := adapter.GetActiveDatabases()
	
	// Should have at least 3 databases (default + db1 + db2)
	if len(activeDbs) < 3 {
		t.Errorf("Expected at least 3 databases, got %d", len(activeDbs))
	}

	// Check that all values are true (indicating active)
	for idx, active := range activeDbs {
		if active != true {
			t.Errorf("Database %s should be active", idx)
		}
	}
}

func TestDatabaseManagerAdapter_ErrorHandling(t *testing.T) {
	testLogger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mysqlHandler := mysql.NewHandler(testLogger)
	adapter := &DatabaseManagerAdapter{handler: mysqlHandler}

	// Test deleting non-existent database
	err := adapter.DeleteDatabase("non_existent")
	if err == nil {
		t.Error("Should return error when deleting non-existent database")
	}

	// Test empty idx
	db, err := adapter.GetOrCreateDatabase("")
	if err != nil {
		t.Errorf("Should handle empty idx: %v", err)
	}
	if db == nil {
		t.Error("Should return default database for empty idx")
	}
}