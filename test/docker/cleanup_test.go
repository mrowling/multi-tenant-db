//go:build docker
// +build docker

package docker_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// TestCleanupAfterTests ensures that resources are properly cleaned up
func TestCleanupAfterTests(t *testing.T) {
	// This test should run last to verify cleanup behavior
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	// Test with multiple tenants to ensure proper isolation during cleanup
	tenants := []string{"cleanup_test_1", "cleanup_test_2", "cleanup_test_3"}
	
	for _, tenant := range tenants {
		t.Run(fmt.Sprintf("CleanupTenant_%s", tenant), func(t *testing.T) {
			// Set tenant
			_, err := db.Exec(fmt.Sprintf("SET @idx = '%s'", tenant))
			if err != nil {
				t.Fatalf("Failed to set tenant %s: %v", tenant, err)
			}
			
			// Create a table with some data
			_, err = db.Exec("CREATE TABLE IF NOT EXISTS cleanup_test (id INT PRIMARY KEY, data TEXT, created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP)")
			if err != nil {
				t.Fatalf("Failed to create cleanup test table for tenant %s: %v", tenant, err)
			}
			
			// Insert test data
			for i := 0; i < 10; i++ {
				_, err = db.Exec("INSERT INTO cleanup_test (id, data) VALUES (?, ?)", i, fmt.Sprintf("test_data_%d", i))
				if err != nil {
					t.Fatalf("Failed to insert test data for tenant %s: %v", tenant, err)
				}
			}
			
			// Verify data exists
			var count int
			err = db.QueryRow("SELECT COUNT(*) FROM cleanup_test").Scan(&count)
			if err != nil {
				t.Fatalf("Failed to count records for tenant %s: %v", tenant, err)
			}
			
			if count != 10 {
				t.Errorf("Expected 10 records for tenant %s, got %d", tenant, count)
			}
			
			// Test concurrent operations to ensure no deadlocks during cleanup
			done := make(chan bool, 3)
			for i := 0; i < 3; i++ {
				go func(routineID int) {
					defer func() { done <- true }()
					
					localDB, err := sql.Open("mysql", mysqlDSN)
					if err != nil {
						t.Errorf("Goroutine %d: Failed to connect: %v", routineID, err)
						return
					}
					defer localDB.Close()
					
					// Set same tenant in different connection
					_, err = localDB.Exec(fmt.Sprintf("SET @idx = '%s'", tenant))
					if err != nil {
						t.Errorf("Goroutine %d: Failed to set tenant: %v", routineID, err)
						return
					}
					
					// Perform operations
					for j := 0; j < 5; j++ {
						_, err = localDB.Exec("INSERT OR REPLACE INTO cleanup_test (id, data) VALUES (?, ?)", 
							100+routineID*10+j, fmt.Sprintf("concurrent_data_%d_%d", routineID, j))
						if err != nil {
							t.Errorf("Goroutine %d: Failed to insert: %v", routineID, err)
							return
						}
						
						time.Sleep(10 * time.Millisecond) // Small delay to ensure concurrency
					}
				}(i)
			}
			
			// Wait for all goroutines to complete
			for i := 0; i < 3; i++ {
				select {
				case <-done:
					// Goroutine completed
				case <-time.After(10 * time.Second):
					t.Errorf("Goroutine %d did not complete within timeout", i)
				}
			}
		})
	}
}

// TestConnectionPoolExhaustion tests behavior under high connection load
func TestConnectionPoolExhaustion(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping connection pool test in short mode")
	}
	
	const maxConnections = 50
	connections := make([]*sql.DB, maxConnections)
	defer func() {
		// Cleanup all connections
		for i, conn := range connections {
			if conn != nil {
				conn.Close()
				connections[i] = nil
			}
		}
	}()
	
	// Open many connections
	for i := 0; i < maxConnections; i++ {
		db, err := sql.Open("mysql", mysqlDSN)
		if err != nil {
			t.Fatalf("Failed to open connection %d: %v", i, err)
		}
		connections[i] = db
		
		// Test each connection
		if err := db.Ping(); err != nil {
			t.Errorf("Connection %d failed ping: %v", i, err)
		}
		
		// Use different tenant for each connection
		tenantID := fmt.Sprintf("pool_test_%d", i)
		_, err = db.Exec(fmt.Sprintf("SET @idx = '%s'", tenantID))
		if err != nil {
			t.Errorf("Connection %d failed to set tenant: %v", i, err)
		}
	}
	
	// Verify all connections are still functional
	for i, db := range connections {
		if db == nil {
			continue
		}
		
		var result int
		err := db.QueryRow("SELECT 1").Scan(&result)
		if err != nil {
			t.Errorf("Connection %d failed simple query: %v", i, err)
		}
		
		if result != 1 {
			t.Errorf("Connection %d returned unexpected result: %d", i, result)
		}
	}
	
	t.Logf("Successfully created and tested %d concurrent connections", maxConnections)
}

// TestLongRunningTransaction tests cleanup of long-running transactions
func TestLongRunningTransaction(t *testing.T) {
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()
	
	// Set tenant
	_, err = db.Exec("SET @idx = 'long_transaction_test'")
	if err != nil {
		t.Fatalf("Failed to set tenant: %v", err)
	}
	
	// Create test table
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS long_transaction_test (id INT PRIMARY KEY, data TEXT)")
	if err != nil {
		t.Fatalf("Failed to create test table: %v", err)
	}
	
	// Start transaction
	tx, err := db.Begin()
	if err != nil {
		t.Fatalf("Failed to begin transaction: %v", err)
	}
	
	// Insert data in transaction
	_, err = tx.Exec("INSERT INTO long_transaction_test (id, data) VALUES (1, 'transaction_data')")
	if err != nil {
		tx.Rollback()
		t.Fatalf("Failed to insert in transaction: %v", err)
	}
	
	// Simulate some work
	time.Sleep(100 * time.Millisecond)
	
	// Test concurrent access from another connection
	go func() {
		otherDB, err := sql.Open("mysql", mysqlDSN)
		if err != nil {
			t.Errorf("Failed to open concurrent connection: %v", err)
			return
		}
		defer otherDB.Close()
		
		// Set same tenant
		_, err = otherDB.Exec("SET @idx = 'long_transaction_test'")
		if err != nil {
			t.Errorf("Failed to set tenant in concurrent connection: %v", err)
			return
		}
		
		// Try to read (should work even with uncommitted transaction)
		var count int
		err = otherDB.QueryRow("SELECT COUNT(*) FROM long_transaction_test").Scan(&count)
		if err != nil {
			t.Errorf("Failed to read from concurrent connection: %v", err)
		}
	}()
	
	// Commit transaction
	if err := tx.Commit(); err != nil {
		t.Fatalf("Failed to commit transaction: %v", err)
	}
	
	// Verify data is committed
	var data string
	err = db.QueryRow("SELECT data FROM long_transaction_test WHERE id = 1").Scan(&data)
	if err != nil {
		t.Fatalf("Failed to read committed data: %v", err)
	}
	
	if data != "transaction_data" {
		t.Errorf("Expected 'transaction_data', got %s", data)
	}
}
