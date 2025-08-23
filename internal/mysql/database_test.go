package mysql

import (
	"fmt"
	"log"
	"os"
	"sync"
	"testing"
)

func TestNewDatabaseManager(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)

	if dm.databases == nil {
		t.Error("databases map should be initialized")
	}

	// Check that default database is created
	if _, exists := dm.databases["default"]; !exists {
		t.Error("default database should be created")
	}

	// Check that default database is accessible
	db, err := dm.GetOrCreateDatabase("default")
	if err != nil {
		t.Errorf("Should be able to access default database: %v", err)
	}
	if db == nil {
		t.Error("Default database should not be nil")
	}
}

func TestDatabaseManager_GetOrCreateDatabase(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)

	// Test creating a new database
	db1, err := dm.GetOrCreateDatabase("test1")
	if err != nil {
		t.Errorf("Should be able to create new database: %v", err)
	}
	if db1 == nil {
		t.Error("Created database should not be nil")
	}

	// Test getting the same database
	db2, err := dm.GetOrCreateDatabase("test1")
	if err != nil {
		t.Errorf("Should be able to get existing database: %v", err)
	}
	if db1 != db2 {
		t.Error("Should get the same database instance for the same idx")
	}

	// Test that empty idx returns default
	db3, err := dm.GetOrCreateDatabase("")
	if err != nil {
		t.Errorf("Should be able to get default database with empty idx: %v", err)
	}
	defaultDB, _ := dm.GetOrCreateDatabase("default")
	if db3 != defaultDB {
		t.Error("Empty idx should return default database")
	}
}

func TestDatabaseManager_GetDatabaseForSession(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)
	sm := NewSessionManager()

	// Test with no idx set (should use default)
	session := sm.GetOrCreateSession(1)
	db, err := dm.GetDatabaseForSession(session)
	if err != nil {
		t.Errorf("Should be able to get database for session: %v", err)
	}
	if db == nil {
		t.Error("Database should not be nil")
	}

	// Test with user variable idx set
	session.SetUser("idx", "user_test")
	db2, err := dm.GetDatabaseForSession(session)
	if err != nil {
		t.Errorf("Should be able to get database for session with user idx: %v", err)
	}

	// Verify different databases are returned for different idx values
	if db == db2 {
		t.Error("Different idx values should return different databases")
	}
}

func TestDatabaseManager_DeleteDatabase(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)

	// Create a database
	_, err := dm.GetOrCreateDatabase("to_delete")
	if err != nil {
		t.Errorf("Should be able to create database: %v", err)
	}

	// Verify it exists
	if _, exists := dm.databases["to_delete"]; !exists {
		t.Error("Database should exist before deletion")
	}

	// Delete the database
	err = dm.DeleteDatabase("to_delete")
	if err != nil {
		t.Errorf("Should be able to delete database: %v", err)
	}

	// Verify it no longer exists
	if _, exists := dm.databases["to_delete"]; exists {
		t.Error("Database should not exist after deletion")
	}

	// Test deleting non-existent database
	err = dm.DeleteDatabase("non_existent")
	if err == nil {
		t.Error("Should return error when deleting non-existent database")
	}

	// Test deleting default database (should fail)
	err = dm.DeleteDatabase("default")
	if err == nil {
		t.Error("Should not be able to delete default database")
	}
}

func TestDatabaseManager_ListDatabases(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)

	// Initially should have default database
	databases := dm.ListDatabases()
	if len(databases) != 1 {
		t.Errorf("Expected 1 database initially, got %d", len(databases))
	}
	if databases[0] != "default" {
		t.Errorf("Expected 'default' database, got '%s'", databases[0])
	}

	// Create additional databases
	dm.GetOrCreateDatabase("test1")
	dm.GetOrCreateDatabase("test2")

	databases = dm.ListDatabases()
	if len(databases) != 3 {
		t.Errorf("Expected 3 databases, got %d", len(databases))
	}

	// Check that all databases are in the list
	databaseSet := make(map[string]bool)
	for _, db := range databases {
		databaseSet[db] = true
	}

	expectedDatabases := []string{"default", "test1", "test2"}
	for _, expected := range expectedDatabases {
		if !databaseSet[expected] {
			t.Errorf("Expected database '%s' to be in list", expected)
		}
	}
}

func TestDatabaseManager_GetActiveDatabases(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)

	// Create some databases
	dm.GetOrCreateDatabase("active1")
	dm.GetOrCreateDatabase("active2")

	active := dm.GetActiveDatabases()
	if active == nil {
		t.Error("Active databases should not be nil")
	}

	// Should contain at least the databases we created plus default
	if len(active) < 3 {
		t.Errorf("Expected at least 3 active databases, got %d", len(active))
	}
}

func TestDatabaseManager_InitSampleData(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)

	// Create a fresh database specifically for this test
	db, err := dm.GetOrCreateDatabase("sample_data_test")
	if err != nil {
		t.Errorf("Should be able to create sample data test database: %v", err)
		return
	}

	// Check that users table has data
	var userCount int
	err = db.QueryRow("SELECT COUNT(*) FROM users").Scan(&userCount)
	if err != nil {
		t.Errorf("Should be able to query users table: %v", err)
		return
	}

	if userCount == 0 {
		t.Error("Users table should have sample data")
	}

	// Check that products table has data
	var productCount int
	err = db.QueryRow("SELECT COUNT(*) FROM products").Scan(&productCount)
	if err != nil {
		t.Errorf("Should be able to query products table: %v", err)
		return
	}

	if productCount == 0 {
		t.Error("Products table should have sample data")
	}
}

func TestDatabaseManager_QueryDatabase(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)

	// Create a test database
	db, err := dm.GetOrCreateDatabase("query_test")
	if err != nil {
		t.Errorf("Should be able to create test database: %v", err)
	}

	// Test a simple query on the sample data
	rows, err := db.Query("SELECT name FROM users WHERE id = 1")
	if err != nil {
		t.Errorf("Should be able to query sample data: %v", err)
	}
	defer rows.Close()

	var name string
	if rows.Next() {
		err = rows.Scan(&name)
		if err != nil {
			t.Errorf("Should be able to scan name: %v", err)
		}
	}

	if name != "Alice" {
		t.Errorf("Expected 'Alice', got '%s'", name)
	}
}

func TestDatabaseManager_Concurrency(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)
	var wg sync.WaitGroup

	// Test concurrent database creation and access
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			idx := fmt.Sprintf("concurrent_test_%d", i)

			// Create database
			db1, err := dm.GetOrCreateDatabase(idx)
			if err != nil {
				t.Errorf("Should be able to create database %s: %v", idx, err)
				return
			}

			// Get the same database again
			db2, err := dm.GetOrCreateDatabase(idx)
			if err != nil {
				t.Errorf("Should be able to get database %s: %v", idx, err)
				return
			}

			if db1 != db2 {
				t.Errorf("Should get same database instance for %s", idx)
			}

			// Test query on the database
			rows, err := db1.Query("SELECT COUNT(*) FROM users")
			if err != nil {
				t.Errorf("Should be able to query database %s: %v", idx, err)
				return
			}
			rows.Close()
		}(i)
	}

	wg.Wait()

	// Verify all databases were created
	databases := dm.ListDatabases()
	if len(databases) < 51 { // 50 + default
		t.Errorf("Expected at least 51 databases, got %d", len(databases))
	}
}

func TestDatabaseManager_ErrorHandling(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)

	// Test with invalid database operation (this is more of an integration test)
	db, err := dm.GetOrCreateDatabase("error_test")
	if err != nil {
		t.Errorf("Should be able to create database: %v", err)
	}

	// Try to execute invalid SQL
	_, err = db.Exec("INVALID SQL STATEMENT")
	if err == nil {
		t.Error("Should return error for invalid SQL")
	}

	// The database should still be accessible for valid queries
	rows, err := db.Query("SELECT COUNT(*) FROM users")
	if err != nil {
		t.Errorf("Should be able to run valid query after error: %v", err)
	}
	if rows != nil {
		rows.Close()
	}
}

// Helper function to check if a string is in a slice
func stringInSlice(a string, list []string) bool {
	for _, b := range list {
		if b == a {
			return true
		}
	}
	return false
}

func TestDatabaseManager_SpecialCharacters(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)

	// Test with special characters in idx
	specialIdx := "test-with-dashes_and_underscores.and.dots"
	db, err := dm.GetOrCreateDatabase(specialIdx)
	if err != nil {
		t.Errorf("Should be able to create database with special characters: %v", err)
	}
	if db == nil {
		t.Error("Database should not be nil")
	}

	// Verify it's in the list
	databases := dm.ListDatabases()
	if !stringInSlice(specialIdx, databases) {
		t.Error("Database with special characters should be in list")
	}
}

func TestDatabaseManager_CaseSensitivity(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	dm := NewDatabaseManager(logger)

	// Test case sensitivity
	db1, err := dm.GetOrCreateDatabase("CaseTest")
	if err != nil {
		t.Errorf("Should be able to create database: %v", err)
	}

	db2, err := dm.GetOrCreateDatabase("casetest")
	if err != nil {
		t.Errorf("Should be able to create database: %v", err)
	}

	// These should be different databases (case sensitive)
	if db1 == db2 {
		t.Error("Case-different idx values should create different databases")
	}

	databases := dm.ListDatabases()
	hasCaseTest := stringInSlice("CaseTest", databases)
	hasLowerCaseTest := stringInSlice("casetest", databases)

	if !hasCaseTest || !hasLowerCaseTest {
		t.Error("Both case variants should exist in database list")
	}
}
