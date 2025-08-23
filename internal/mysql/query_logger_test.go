package mysql

import (
	"fmt"
	"log"
	"os"
	"testing"
	"time"
)

func TestNewQueryLogger(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	ql := NewQueryLogger(logger, "") // Use in-memory for tests
	
	if ql == nil {
		t.Fatal("Expected non-nil QueryLogger")
	}
	
	if ql.logDatabases == nil {
		t.Fatal("Expected logDatabases map to be initialized")
	}
	
	if ql.logger != logger {
		t.Fatal("Expected logger to be set correctly")
	}
}

func TestQueryLoggerLogQuery(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	ql := NewQueryLogger(logger, "") // Use in-memory for tests
	
	// Test logging a successful query
	tenantID := "test_tenant_log_query"
	query := "SELECT * FROM users"
	connectionID := "conn_1"
	duration := 100 * time.Millisecond
	
	err := ql.LogQuery(tenantID, query, connectionID, duration, true, "")
	if err != nil {
		t.Fatalf("Failed to log query: %v", err)
	}
	
	// Test logging a failed query
	err = ql.LogQuery(tenantID, "INVALID SQL", connectionID, 50*time.Millisecond, false, "syntax error")
	if err != nil {
		t.Fatalf("Failed to log failed query: %v", err)
	}
}

func TestQueryLoggerGetQueryLogs(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	ql := NewQueryLogger(logger, "") // Use in-memory for tests
	
	tenantID := "test_tenant_get_logs"
	
	// Log some test queries
	testQueries := []struct {
		query        string
		connectionID string
		duration     time.Duration
		success      bool
		errorMsg     string
	}{
		{"SELECT * FROM users", "conn_1", 100 * time.Millisecond, true, ""},
		{"INSERT INTO users VALUES (1, 'test')", "conn_1", 50 * time.Millisecond, true, ""},
		{"INVALID SQL", "conn_2", 25 * time.Millisecond, false, "syntax error"},
	}
	
	for _, tq := range testQueries {
		err := ql.LogQuery(tenantID, tq.query, tq.connectionID, tq.duration, tq.success, tq.errorMsg)
		if err != nil {
			t.Fatalf("Failed to log query: %v", err)
		}
	}
	
	// Retrieve logs
	logs, err := ql.GetQueryLogs(tenantID, 10, 0, nil, nil)
	if err != nil {
		t.Fatalf("Failed to get query logs: %v", err)
	}
	
	if len(logs) != len(testQueries) {
		t.Fatalf("Expected %d logs, got %d", len(testQueries), len(logs))
	}
	
	// Verify first log (should be most recent due to ORDER BY executed_at DESC)
	firstLog := logs[0]
	logEntry, ok := firstLog.(QueryLogEntry)
	if !ok {
		t.Fatalf("Expected QueryLogEntry, got %T", firstLog)
	}
	
	if logEntry.TenantID != tenantID {
		t.Errorf("Expected tenant ID %s, got %s", tenantID, logEntry.TenantID)
	}
	
	if logEntry.Query != testQueries[2].query { // Last inserted query should be first in results
		t.Errorf("Expected query %s, got %s", testQueries[2].query, logEntry.Query)
	}
}

func TestQueryLoggerGetQueryLogsWithPagination(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	ql := NewQueryLogger(logger, "")
	
	tenantID := "pagination_test"
	
	// Log 5 test queries
	for i := 0; i < 5; i++ {
		query := fmt.Sprintf("SELECT %d", i)
		err := ql.LogQuery(tenantID, query, "conn_1", 10*time.Millisecond, true, "")
		if err != nil {
			t.Fatalf("Failed to log query %d: %v", i, err)
		}
	}
	
	// Test pagination - get first 2 logs
	logs, err := ql.GetQueryLogs(tenantID, 2, 0, nil, nil)
	if err != nil {
		t.Fatalf("Failed to get paginated logs: %v", err)
	}
	
	if len(logs) != 2 {
		t.Fatalf("Expected 2 logs, got %d", len(logs))
	}
	
	// Test pagination - get next 2 logs
	logs, err = ql.GetQueryLogs(tenantID, 2, 2, nil, nil)
	if err != nil {
		t.Fatalf("Failed to get second page of logs: %v", err)
	}
	
	if len(logs) != 2 {
		t.Fatalf("Expected 2 logs in second page, got %d", len(logs))
	}
}

func TestQueryLoggerGetQueryLogStats(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	ql := NewQueryLogger(logger, "")
	
	tenantID := "stats_test"
	
	// Log test queries with different success states
	testCases := []struct {
		success  bool
		duration time.Duration
	}{
		{true, 100 * time.Millisecond},
		{true, 200 * time.Millisecond},
		{false, 50 * time.Millisecond},
		{true, 150 * time.Millisecond},
	}
	
	for i, tc := range testCases {
		query := fmt.Sprintf("SELECT %d", i)
		errorMsg := ""
		if !tc.success {
			errorMsg = "test error"
		}
		err := ql.LogQuery(tenantID, query, "conn_1", tc.duration, tc.success, errorMsg)
		if err != nil {
			t.Fatalf("Failed to log query %d: %v", i, err)
		}
	}
	
	// Get stats
	stats, err := ql.GetQueryLogStats(tenantID)
	if err != nil {
		t.Fatalf("Failed to get query stats: %v", err)
	}
	
	// Verify stats
	if stats["tenant_id"] != tenantID {
		t.Errorf("Expected tenant_id %s, got %v", tenantID, stats["tenant_id"])
	}
	
	if stats["total_queries"] != int64(4) {
		t.Errorf("Expected total_queries 4, got %v", stats["total_queries"])
	}
	
	if stats["successful_queries"] != int64(3) {
		t.Errorf("Expected successful_queries 3, got %v", stats["successful_queries"])
	}
	
	if stats["failed_queries"] != int64(1) {
		t.Errorf("Expected failed_queries 1, got %v", stats["failed_queries"])
	}
	
	successRate := stats["success_rate"].(float64)
	expectedSuccessRate := 75.0 // 3/4 * 100
	if successRate != expectedSuccessRate {
		t.Errorf("Expected success_rate %.1f, got %.1f", expectedSuccessRate, successRate)
	}
}

func TestQueryLoggerListTenantLogs(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	ql := NewQueryLogger(logger, "")
	
	// Initially should be empty
	tenants := ql.ListTenantLogs()
	if len(tenants) != 0 {
		t.Errorf("Expected 0 tenants initially, got %d", len(tenants))
	}
	
	// Log queries for different tenants
	tenant1 := "tenant1"
	tenant2 := "tenant2"
	
	err := ql.LogQuery(tenant1, "SELECT 1", "conn_1", 10*time.Millisecond, true, "")
	if err != nil {
		t.Fatalf("Failed to log query for tenant1: %v", err)
	}
	
	err = ql.LogQuery(tenant2, "SELECT 2", "conn_2", 20*time.Millisecond, true, "")
	if err != nil {
		t.Fatalf("Failed to log query for tenant2: %v", err)
	}
	
	// Should now have 2 tenants
	tenants = ql.ListTenantLogs()
	if len(tenants) != 2 {
		t.Errorf("Expected 2 tenants, got %d", len(tenants))
	}
	
	// Check that both tenants are present
	tenantMap := make(map[string]bool)
	for _, tenant := range tenants {
		tenantMap[tenant] = true
	}
	
	if !tenantMap[tenant1] {
		t.Errorf("Expected tenant1 to be in the list")
	}
	
	if !tenantMap[tenant2] {
		t.Errorf("Expected tenant2 to be in the list")
	}
}

func TestQueryLoggerDefaultTenant(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	ql := NewQueryLogger(logger, "")
	
	// Log query with empty tenant ID (should use "default")
	err := ql.LogQuery("", "SELECT 1", "conn_1", 10*time.Millisecond, true, "")
	if err != nil {
		t.Fatalf("Failed to log query with empty tenant ID: %v", err)
	}
	
	// Retrieve logs using "default" tenant ID
	logs, err := ql.GetQueryLogs("default", 10, 0, nil, nil)
	if err != nil {
		t.Fatalf("Failed to get logs for default tenant: %v", err)
	}
	
	if len(logs) != 1 {
		t.Errorf("Expected 1 log for default tenant, got %d", len(logs))
	}
	
	logEntry := logs[0].(QueryLogEntry)
	if logEntry.TenantID != "default" {
		t.Errorf("Expected tenant_id 'default', got %s", logEntry.TenantID)
	}
}

func TestQueryLoggerNumericTenantID(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	ql := NewQueryLogger(logger, "") // Use in-memory for tests
	
	// Test numeric tenant IDs (as strings, since QueryLogger.LogQuery takes string tenantID)
	testCases := []struct {
		name     string
		tenantID string
		query    string
	}{
		{"numeric_123", "123", "SELECT * FROM users WHERE tenant = 123"},
		{"numeric_456", "456", "INSERT INTO products (name) VALUES ('test')"},
		{"numeric_789", "789", "UPDATE users SET age = 30 WHERE id = 1"},
	}
	
	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			// Log query with numeric tenant ID (as string)
			err := ql.LogQuery(tc.tenantID, tc.query, "conn_1", 10*time.Millisecond, true, "")
			if err != nil {
				t.Fatalf("Failed to log query for numeric tenant %s: %v", tc.tenantID, err)
			}
			
			// Retrieve logs for the numeric tenant
			logs, err := ql.GetQueryLogs(tc.tenantID, 10, 0, nil, nil)
			if err != nil {
				t.Fatalf("Failed to get logs for numeric tenant %s: %v", tc.tenantID, err)
			}
			
			if len(logs) != 1 {
				t.Errorf("Expected 1 log for tenant %s, got %d", tc.tenantID, len(logs))
			}
			
			logEntry := logs[0].(QueryLogEntry)
			if logEntry.TenantID != tc.tenantID {
				t.Errorf("Expected tenant_id %s, got %s", tc.tenantID, logEntry.TenantID)
			}
			
			if logEntry.Query != tc.query {
				t.Errorf("Expected query %s, got %s", tc.query, logEntry.Query)
			}
		})
	}
	
	// Test that different numeric tenants are isolated
	logs123, err := ql.GetQueryLogs("123", 10, 0, nil, nil)
	if err != nil {
		t.Fatalf("Failed to get logs for tenant 123: %v", err)
	}
	
	logs456, err := ql.GetQueryLogs("456", 10, 0, nil, nil)
	if err != nil {
		t.Fatalf("Failed to get logs for tenant 456: %v", err)
	}
	
	// Each tenant should have exactly 1 log
	if len(logs123) != 1 || len(logs456) != 1 {
		t.Errorf("Expected 1 log each for tenants 123 and 456, got %d and %d", len(logs123), len(logs456))
	}
	
	// Logs should be isolated (different queries)
	log123 := logs123[0].(QueryLogEntry)
	log456 := logs456[0].(QueryLogEntry)
	
	if log123.Query == log456.Query {
		t.Error("Logs for different numeric tenants should be isolated")
	}
}
