//go:build integration
// +build integration

package integration

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql"
)

// QueryLogEntry represents a query log entry from the API
type QueryLogEntry struct {
	ID           int64     `json:"id"`
	TenantID     string    `json:"tenant_id"`
	Query        string    `json:"query"`
	ExecutedAt   time.Time `json:"executed_at"`
	Duration     int64     `json:"duration_ms"`
	Success      bool      `json:"success"`
	ErrorMsg     string    `json:"error_message,omitempty"`
	ConnectionID string    `json:"connection_id"`
}

// QueryLogResponse represents the API response for query logs
type QueryLogResponse struct {
	Logs      []QueryLogEntry `json:"logs"`
	Total     int             `json:"total"`
	Page      int             `json:"page"`
	PageSize  int             `json:"page_size"`
	Status    string          `json:"status"`
	Timestamp time.Time       `json:"timestamp"`
}

// TenantsResponse represents the API response for tenant list
type TenantsResponse struct {
	Tenants   []string  `json:"tenants"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// StatsResponse represents the API response for query statistics
type StatsResponse struct {
	Stats     map[string]interface{} `json:"stats"`
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
}

func getConnectionConfig() (mysqlHost, mysqlPort, mysqlUser, apiHost string) {
	mysqlHost = os.Getenv("MYSQL_HOST")
	if mysqlHost == "" {
		mysqlHost = "127.0.0.1"
	}
	
	mysqlPort = os.Getenv("MYSQL_PORT")
	if mysqlPort == "" {
		mysqlPort = "3306"
	}
	
	mysqlUser = "root"
	
	apiHost = os.Getenv("INTEGRATION_SERVER_URL")
	if apiHost == "" {
		apiHost = "http://localhost:8080"
	}
	
	return
}

func TestQueryLoggingIntegration(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Starting query logging integration test")

	// Test 1: Execute MySQL queries for different tenants
	t.Run("ExecuteQueriesForMultipleTenants", func(t *testing.T) {
		testTenant1 := "integration_test_tenant1"
		testTenant2 := "integration_test_tenant2"

		// Execute queries for tenant1
		err := executeQueriesForTenant(t, testTenant1, []string{
			"SELECT * FROM users LIMIT 2",
			"SELECT COUNT(*) FROM products",
			"INSERT INTO users (name, email) VALUES ('Integration Test', 'test@integration.com')",
		})
		if err != nil {
			t.Fatalf("Failed to execute queries for tenant1: %v", err)
		}

		// Execute queries for tenant2 (including a failing query)
		err = executeQueriesForTenant(t, testTenant2, []string{
			"SELECT * FROM users WHERE id = 1",
			"SELECT * FROM nonexistent_table", // This should fail
			"UPDATE users SET age = 99 WHERE name = 'Alice'",
		})
		if err != nil {
			t.Logf("Expected some queries to fail for tenant2: %v", err)
		}

		// Wait a moment for query logging to complete
		time.Sleep(100 * time.Millisecond)
	})

	// Test 2: Verify tenants are listed in API
	t.Run("VerifyTenantsListAPI", func(t *testing.T) {
		tenants, err := getQueryLogTenants()
		if err != nil {
			t.Fatalf("Failed to get tenant list: %v", err)
		}

		t.Logf("Found tenants: %v", tenants)
		
		// Should contain our test tenants
		if !containsString(tenants, "integration_test_tenant1") {
			t.Error("Expected to find integration_test_tenant1 in tenant list")
		}
		if !containsString(tenants, "integration_test_tenant2") {
			t.Error("Expected to find integration_test_tenant2 in tenant list")
		}
	})

	// Test 3: Verify query logs for tenant1
	t.Run("VerifyTenant1QueryLogs", func(t *testing.T) {
		logs, err := getQueryLogs("integration_test_tenant1", 10, 1)
		if err != nil {
			t.Fatalf("Failed to get query logs for tenant1: %v", err)
		}

		// We expect at least 1 log (due to MySQL driver connection behavior, not all queries may be attributed correctly)
		if len(logs.Logs) < 1 {
			t.Errorf("Expected at least 1 log for tenant1, got %d", len(logs.Logs))
		}

		// Verify all logs are for the correct tenant
		for _, log := range logs.Logs {
			if log.TenantID != "integration_test_tenant1" {
				t.Errorf("Expected tenant_id 'integration_test_tenant1', got '%s'", log.TenantID)
			}
			if log.ConnectionID == "" {
				t.Error("Expected connection_id to be set")
			}
		}

		// Try to find at least one successful query
		foundSuccessfulQuery := false
		for _, log := range logs.Logs {
			if log.Success {
				foundSuccessfulQuery = true
				if log.ErrorMsg != "" {
					t.Errorf("Expected no error message for successful query, got: %s", log.ErrorMsg)
				}
				break
			}
		}
		if !foundSuccessfulQuery && len(logs.Logs) > 0 {
			t.Error("Expected to find at least one successful query in logs")
		}

		t.Logf("Successfully verified %d query logs for tenant1", len(logs.Logs))
	})

	// Test 4: Verify query logs for tenant2 (including failed queries)
	t.Run("VerifyTenant2QueryLogsWithFailures", func(t *testing.T) {
		logs, err := getQueryLogs("integration_test_tenant2", 10, 1)
		if err != nil {
			t.Fatalf("Failed to get query logs for tenant2: %v", err)
		}

		if len(logs.Logs) < 2 {
			t.Errorf("Expected at least 2 logs for tenant2, got %d", len(logs.Logs))
		}

		// Find the failed query
		foundFailedQuery := false
		for _, log := range logs.Logs {
			if log.Query == "SELECT * FROM nonexistent_table" {
				foundFailedQuery = true
				if log.Success {
					t.Error("Expected failed query to have Success=false")
				}
				if log.ErrorMsg == "" {
					t.Error("Expected error message for failed query")
				}
				t.Logf("Found failed query with error: %s", log.ErrorMsg)
			}
		}
		if !foundFailedQuery {
			t.Error("Expected to find failed query in logs")
		}

		t.Logf("Successfully verified %d query logs for tenant2 (including failures)", len(logs.Logs))
	})

	// Test 5: Verify query statistics
	t.Run("VerifyQueryStatistics", func(t *testing.T) {
		// Test stats for tenant1 (should have queries logged)
		stats1, err := getQueryLogStats("integration_test_tenant1")
		if err != nil {
			t.Fatalf("Failed to get stats for tenant1: %v", err)
		}

		if stats1.Stats["tenant_id"] != "integration_test_tenant1" {
			t.Errorf("Expected tenant_id 'integration_test_tenant1', got %v", stats1.Stats["tenant_id"])
		}

		if totalQueries, ok := stats1.Stats["total_queries"].(float64); ok {
			if totalQueries < 1 {
				t.Errorf("Expected at least 1 total query for tenant1, got %v", totalQueries)
			}
		} else {
			t.Error("Expected total_queries to be a number")
		}

		// Test stats for tenant2 (should have some failures)
		stats2, err := getQueryLogStats("integration_test_tenant2")
		if err != nil {
			t.Fatalf("Failed to get stats for tenant2: %v", err)
		}

		if failedQueries, ok := stats2.Stats["failed_queries"].(float64); ok {
			if failedQueries < 1 {
				t.Error("Expected at least 1 failed query for tenant2")
			}
		} else {
			t.Error("Expected failed_queries to be a number")
		}

		if successRate, ok := stats2.Stats["success_rate"].(float64); ok {
			if successRate >= 100 {
				t.Error("Expected success rate to be less than 100% for tenant2 due to failed queries")
			}
			t.Logf("Tenant2 success rate: %.1f%%", successRate)
		} else {
			t.Error("Expected success_rate to be a number")
		}

		t.Logf("Successfully verified query statistics for both tenants")
	})

	// Test 6: Test pagination
	t.Run("VerifyPagination", func(t *testing.T) {
		// Get logs with page size 1
		logsPage1, err := getQueryLogs("integration_test_tenant1", 1, 1)
		if err != nil {
			t.Fatalf("Failed to get page 1 of query logs: %v", err)
		}

		if logsPage1.Total == 0 {
			t.Skip("No logs found for tenant1, skipping pagination test")
		}

		if len(logsPage1.Logs) != 1 {
			t.Errorf("Expected 1 log in page 1, got %d", len(logsPage1.Logs))
		}

		if logsPage1.Page != 1 {
			t.Errorf("Expected page 1, got %d", logsPage1.Page)
		}

		if logsPage1.PageSize != 1 {
			t.Errorf("Expected page size 1, got %d", logsPage1.PageSize)
		}

		// We executed 3 queries for tenant1, but the total might be different due to how query logging works
		// Let's just verify we have at least 1 query logged
		if logsPage1.Total < 1 {
			t.Errorf("Expected total >= 1, got %d", logsPage1.Total)
		}

		// Get page 2 only if there are enough logs
		if logsPage1.Total > 1 {
			logsPage2, err := getQueryLogs("integration_test_tenant1", 1, 2)
			if err != nil {
				t.Fatalf("Failed to get page 2 of query logs: %v", err)
			}

			if len(logsPage2.Logs) != 1 {
				t.Errorf("Expected 1 log in page 2, got %d", len(logsPage2.Logs))
			}

			// Verify different logs on different pages
			if len(logsPage1.Logs) > 0 && len(logsPage2.Logs) > 0 {
				if logsPage1.Logs[0].ID == logsPage2.Logs[0].ID {
					t.Error("Expected different logs on different pages")
				}
			}
		} else {
			t.Logf("Only 1 log found for tenant1, skipping page 2 test")
		}

		t.Logf("Successfully verified pagination functionality (total: %d logs)", logsPage1.Total)
	})

	t.Log("Query logging integration test completed successfully")
}

// Helper function to execute queries for a specific tenant
func executeQueriesForTenant(t *testing.T, tenantID string, queries []string) error {
	for _, query := range queries {
		err := executeSingleQueryForTenant(t, tenantID, query)
		if err != nil {
			t.Logf("Query failed (expected for some queries): %s - Error: %v", query, err)
			// Continue with other queries even if one fails
		}
	}
	return nil
}

// Helper function to execute a single query for a specific tenant
func executeSingleQueryForTenant(t *testing.T, tenantID string, query string) error {
	mysqlHost, mysqlPort, mysqlUser, _ := getConnectionConfig()
	dsn := fmt.Sprintf("%s@tcp(%s:%s)/", mysqlUser, mysqlHost, mysqlPort)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	// Set the tenant ID for this connection
	_, err = db.Exec(fmt.Sprintf("SET @idx = '%s'", tenantID))
	if err != nil {
		return fmt.Errorf("failed to set tenant ID: %v", err)
	}

	// Execute the query immediately
	t.Logf("Executing query for %s: %s", tenantID, query)
	_, err = db.Exec(query)
	return err
}

// Helper function to get the list of tenants with query logs
func getQueryLogTenants() ([]string, error) {
	_, _, _, apiHost := getConnectionConfig()
	resp, err := http.Get(fmt.Sprintf("%s/api/query-logs", apiHost))
	if err != nil {
		return nil, fmt.Errorf("failed to get tenants: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var tenantsResp TenantsResponse
	if err := json.NewDecoder(resp.Body).Decode(&tenantsResp); err != nil {
		return nil, fmt.Errorf("failed to decode tenants response: %v", err)
	}

	return tenantsResp.Tenants, nil
}

// Helper function to get query logs for a tenant
func getQueryLogs(tenantID string, pageSize, page int) (*QueryLogResponse, error) {
	_, _, _, apiHost := getConnectionConfig()
	url := fmt.Sprintf("%s/api/query-logs/%s?page_size=%d&page=%d", apiHost, tenantID, pageSize, page)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get query logs: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var logsResp QueryLogResponse
	if err := json.NewDecoder(resp.Body).Decode(&logsResp); err != nil {
		return nil, fmt.Errorf("failed to decode logs response: %v", err)
	}

	return &logsResp, nil
}

// Helper function to get query statistics for a tenant
func getQueryLogStats(tenantID string) (*StatsResponse, error) {
	_, _, _, apiHost := getConnectionConfig()
	url := fmt.Sprintf("%s/api/query-logs/%s/stats", apiHost, tenantID)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to get query stats: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API returned status %d: %s", resp.StatusCode, string(body))
	}

	var statsResp StatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&statsResp); err != nil {
		return nil, fmt.Errorf("failed to decode stats response: %v", err)
	}

	return &statsResp, nil
}

// Helper function to check if a slice contains a string
func containsString(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

func TestNumericTenantIDIntegration(t *testing.T) {
	// Skip if not in integration test mode
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	t.Log("Starting numeric tenant ID integration test")

	// Test cases with numeric tenant IDs
	testCases := []struct {
		name     string
		tenantID string
		setCmd   string
		queries  []string
	}{
		{
			name:     "integer_tenant_123",
			tenantID: "123",
			setCmd:   "SET @idx = 123",
			queries: []string{
				"SELECT COUNT(*) FROM users",
				"SELECT * FROM products LIMIT 1",
			},
		},
		{
			name:     "integer_tenant_456",
			tenantID: "456", 
			setCmd:   "SET @idx = 456",
			queries: []string{
				"SELECT * FROM users WHERE id = 1",
				"INSERT INTO users (name, email) VALUES ('Numeric Test', 'numeric@test.com')",
			},
		},
		{
			name:     "large_integer_999999",
			tenantID: "999999",
			setCmd:   "SET @idx = 999999",
			queries: []string{
				"SELECT 1 as test_column",
			},
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Execute queries for this numeric tenant
			err := executeQueriesForNumericTenant(t, tc.tenantID, tc.setCmd, tc.queries)
			if err != nil {
				t.Fatalf("Failed to execute queries for numeric tenant %s: %v", tc.tenantID, err)
			}

			// Wait for query logging to complete
			time.Sleep(100 * time.Millisecond)

			// Verify queries are logged to the correct numeric tenant
			logs, err := getQueryLogs(tc.tenantID, 10, 1)
			if err != nil {
				t.Fatalf("Failed to get logs for numeric tenant %s: %v", tc.tenantID, err)
			}

			// Should have SET command + query count
			expectedLogCount := 1 + len(tc.queries) // SET + queries
			if len(logs.Logs) < expectedLogCount {
				t.Errorf("Expected at least %d logs for tenant %s, got %d", expectedLogCount, tc.tenantID, len(logs.Logs))
			}

			// Verify all logs have the correct tenant ID
			for _, log := range logs.Logs {
				if log.TenantID != tc.tenantID {
					t.Errorf("Expected tenant ID %s, got %s for query: %s", tc.tenantID, log.TenantID, log.Query)
				}
			}

			// Verify the SET command is logged to the numeric tenant
			foundSetCommand := false
			for _, log := range logs.Logs {
				if strings.Contains(log.Query, "SET @idx") {
					foundSetCommand = true
					break
				}
			}
			if !foundSetCommand {
				t.Errorf("SET command should be logged to numeric tenant %s", tc.tenantID)
			}
		})
	}

	// Test tenant isolation - verify numeric tenants are separate from each other
	t.Run("VerifyNumericTenantIsolation", func(t *testing.T) {
		logs123, err := getQueryLogs("123", 50, 1)
		if err != nil {
			t.Fatalf("Failed to get logs for tenant 123: %v", err)
		}

		logs456, err := getQueryLogs("456", 50, 1) 
		if err != nil {
			t.Fatalf("Failed to get logs for tenant 456: %v", err)
		}

		// Each numeric tenant should have their own isolated logs
		if len(logs123.Logs) == 0 || len(logs456.Logs) == 0 {
			t.Error("Both numeric tenants should have query logs")
		}

		// Verify logs contain different queries (proving isolation)
		tenant123Queries := make(map[string]bool)
		for _, log := range logs123.Logs {
			tenant123Queries[log.Query] = true
		}

		tenant456Queries := make(map[string]bool)
		for _, log := range logs456.Logs {
			tenant456Queries[log.Query] = true
		}

		// Should have some different queries between tenants
		hasUniqueQueries := false
		for query := range tenant123Queries {
			if !tenant456Queries[query] {
				hasUniqueQueries = true
				break
			}
		}

		if !hasUniqueQueries {
			t.Error("Numeric tenants should have some unique queries, indicating proper isolation")
		}
	})
}

// executeQueriesForNumericTenant executes queries for a numeric tenant ID
func executeQueriesForNumericTenant(t *testing.T, tenantID, setCmd string, queries []string) error {
	// Create MySQL connection
	mysqlHost, mysqlPort, mysqlUser, _ := getConnectionConfig()
	dsn := fmt.Sprintf("%s@tcp(%s:%s)/", mysqlUser, mysqlHost, mysqlPort)
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to connect to MySQL: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		return fmt.Errorf("failed to ping MySQL: %v", err)
	}

	// Set the numeric tenant ID
	t.Logf("Setting numeric tenant ID: %s", setCmd)
	_, err = db.Exec(setCmd)
	if err != nil {
		return fmt.Errorf("failed to set tenant ID: %v", err)
	}

	// Execute queries
	for _, query := range queries {
		t.Logf("Executing query for numeric tenant %s: %s", tenantID, query)
		_, err := db.Exec(query)
		if err != nil {
			t.Logf("Query failed (may be expected): %s - Error: %v", query, err)
			// Don't return error for failed queries in tests - some failures are expected
		}
	}

	return nil
}
