//go:build docker
// +build docker

package docker_test

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

const (
	defaultTimeout = 30 * time.Second
)

var (
	dbHost     = getEnv("MULTITENANT_DB_HOST", "localhost")
	dbPort     = getEnv("MULTITENANT_DB_PORT", "3306")
	httpPort   = getEnv("MULTITENANT_DB_HTTP_PORT", "8080")
	mysqlDSN   = fmt.Sprintf("testuser:testpass@tcp(%s:%s)/", dbHost, dbPort)
	httpBaseURL = fmt.Sprintf("http://%s:%s", dbHost, httpPort)
)

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func waitForService(t *testing.T, url string, timeout time.Duration) {
	t.Helper()
	client := &http.Client{Timeout: 5 * time.Second}
	deadline := time.Now().Add(timeout)
	
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil && resp.StatusCode == http.StatusOK {
			resp.Body.Close()
			return
		}
		if resp != nil {
			resp.Body.Close()
		}
		time.Sleep(1 * time.Second)
	}
	t.Fatalf("Service at %s did not become ready within %v", url, timeout)
}

func TestDockerMultiTenantDB_HealthCheck(t *testing.T) {
	waitForService(t, httpBaseURL+"/health", defaultTimeout)
	
	resp, err := http.Get(httpBaseURL + "/health")
	if err != nil {
		t.Fatalf("Health check failed: %v", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		t.Errorf("Expected status 200, got %d", resp.StatusCode)
	}
}

func TestDockerMultiTenantDB_MySQLProtocol(t *testing.T) {
	waitForService(t, httpBaseURL+"/health", defaultTimeout)
	
	// Test MySQL connection
	db, err := sql.Open("mysql", mysqlDSN)
	if err != nil {
		t.Fatalf("Failed to connect to MySQL: %v", err)
	}
	defer db.Close()
	
	// Test basic ping
	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping MySQL server: %v", err)
	}
	
	// Test tenant isolation
	t.Run("TenantIsolation", func(t *testing.T) {
		testTenantIsolation(t, db)
	})
	
	// Test concurrent connections
	t.Run("ConcurrentConnections", func(t *testing.T) {
		testConcurrentConnections(t)
	})
}

func testTenantIsolation(t *testing.T, db *sql.DB) {
	// Set up tenant 1
	_, err := db.Exec("SET @idx = 'tenant1'")
	if err != nil {
		t.Fatalf("Failed to set tenant1: %v", err)
	}
	
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users (id INT PRIMARY KEY, name VARCHAR(50))")
	if err != nil {
		t.Fatalf("Failed to create table for tenant1: %v", err)
	}
	
	_, err = db.Exec("INSERT INTO users (id, name) VALUES (1, 'Alice')")
	if err != nil {
		t.Fatalf("Failed to insert data for tenant1: %v", err)
	}
	
	// Switch to tenant 2
	_, err = db.Exec("SET @idx = 'tenant2'")
	if err != nil {
		t.Fatalf("Failed to set tenant2: %v", err)
	}
	
	_, err = db.Exec("CREATE TABLE IF NOT EXISTS users (id INT PRIMARY KEY, name VARCHAR(50))")
	if err != nil {
		t.Fatalf("Failed to create table for tenant2: %v", err)
	}
	
	_, err = db.Exec("INSERT INTO users (id, name) VALUES (1, 'Bob')")
	if err != nil {
		t.Fatalf("Failed to insert data for tenant2: %v", err)
	}
	
	// Verify tenant1 data
	_, err = db.Exec("SET @idx = 'tenant1'")
	if err != nil {
		t.Fatalf("Failed to switch back to tenant1: %v", err)
	}
	
	var name string
	err = db.QueryRow("SELECT name FROM users WHERE id = 1").Scan(&name)
	if err != nil {
		t.Fatalf("Failed to query tenant1 data: %v", err)
	}
	
	if name != "Alice" {
		t.Errorf("Expected Alice, got %s", name)
	}
	
	// Verify tenant2 data
	_, err = db.Exec("SET @idx = 'tenant2'")
	if err != nil {
		t.Fatalf("Failed to switch back to tenant2: %v", err)
	}
	
	err = db.QueryRow("SELECT name FROM users WHERE id = 1").Scan(&name)
	if err != nil {
		t.Fatalf("Failed to query tenant2 data: %v", err)
	}
	
	if name != "Bob" {
		t.Errorf("Expected Bob, got %s", name)
	}
}

func testConcurrentConnections(t *testing.T) {
	const numConnections = 10
	results := make(chan error, numConnections)
	
	for i := 0; i < numConnections; i++ {
		go func(tenantID int) {
			db, err := sql.Open("mysql", mysqlDSN)
			if err != nil {
				results <- fmt.Errorf("connection %d failed: %v", tenantID, err)
				return
			}
			defer db.Close()
			
			// Set unique tenant
			_, err = db.Exec(fmt.Sprintf("SET @idx = 'concurrent_tenant_%d'", tenantID))
			if err != nil {
				results <- fmt.Errorf("set tenant %d failed: %v", tenantID, err)
				return
			}
			
			// Create and query data
			_, err = db.Exec("CREATE TABLE IF NOT EXISTS test_table (id INT PRIMARY KEY, tenant_id INT)")
			if err != nil {
				results <- fmt.Errorf("create table tenant %d failed: %v", tenantID, err)
				return
			}
			
			_, err = db.Exec("INSERT INTO test_table (id, tenant_id) VALUES (1, ?)", tenantID)
			if err != nil {
				results <- fmt.Errorf("insert tenant %d failed: %v", tenantID, err)
				return
			}
			
			var retrievedTenantID int
			err = db.QueryRow("SELECT tenant_id FROM test_table WHERE id = 1").Scan(&retrievedTenantID)
			if err != nil {
				results <- fmt.Errorf("query tenant %d failed: %v", tenantID, err)
				return
			}
			
			if retrievedTenantID != tenantID {
				results <- fmt.Errorf("tenant %d: expected %d, got %d", tenantID, tenantID, retrievedTenantID)
				return
			}
			
			results <- nil
		}(i)
	}
	
	// Collect results
	for i := 0; i < numConnections; i++ {
		if err := <-results; err != nil {
			t.Error(err)
		}
	}
}

func TestDockerMultiTenantDB_HTTPApi(t *testing.T) {
	waitForService(t, httpBaseURL+"/health", defaultTimeout)
	
	client := &http.Client{Timeout: 10 * time.Second}
	
	// Test health endpoint
	t.Run("Health", func(t *testing.T) {
		resp, err := client.Get(httpBaseURL + "/health")
		if err != nil {
			t.Fatalf("Health request failed: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
	
	// Test query endpoint
	t.Run("QueryEndpoint", func(t *testing.T) {
		queryData := map[string]interface{}{
			"tenant_id": "http_test_tenant",
			"query":     "CREATE TABLE IF NOT EXISTS api_test (id INT PRIMARY KEY, data VARCHAR(100))",
		}
		
		jsonData, err := json.Marshal(queryData)
		if err != nil {
			t.Fatalf("Failed to marshal query data: %v", err)
		}
		
		resp, err := client.Post(httpBaseURL+"/query", "application/json", bytes.NewBuffer(jsonData))
		if err != nil {
			t.Fatalf("Query request failed: %v", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode != http.StatusOK {
			t.Errorf("Expected status 200, got %d", resp.StatusCode)
		}
	})
}
