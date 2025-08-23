package mysql

import (
	"database/sql"
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// QueryLogEntry represents a single query log entry
type QueryLogEntry struct {
	ID          int64     `json:"id"`
	TenantID    string    `json:"tenant_id"`
	Query       string    `json:"query"`
	ExecutedAt  time.Time `json:"executed_at"`
	Duration    int64     `json:"duration_ms"` // Duration in milliseconds
	Success     bool      `json:"success"`
	ErrorMsg    string    `json:"error_message,omitempty"`
	ConnectionID string   `json:"connection_id"`
}

// QueryLogger manages query logging for all tenants
type QueryLogger struct {
	logDatabases map[string]*sql.DB // key is tenant ID, value is log DB connection
	dbMu         sync.RWMutex
	logger       *log.Logger
	logDir       string // Directory for log databases, empty means use in-memory
	instanceID   int64  // Unique instance ID to avoid cross-test pollution
}

// NewQueryLogger creates a new query logger
func NewQueryLogger(logger *log.Logger, logDir string) *QueryLogger {
	return &QueryLogger{
		logDatabases: make(map[string]*sql.DB),
		logger:       logger,
		logDir:       logDir,
		instanceID:   rand.Int63(), // Random instance ID to avoid test interference
	}
}

// getOrCreateLogDatabase gets or creates a log database for the specified tenant
func (ql *QueryLogger) getOrCreateLogDatabase(tenantID string) (*sql.DB, error) {
	ql.dbMu.Lock()
	defer ql.dbMu.Unlock()

	// Use "default" for empty tenant ID
	if tenantID == "" {
		tenantID = "default"
	}

	// Check if log database already exists
	if db, exists := ql.logDatabases[tenantID]; exists {
		return db, nil
	}

	// Create new SQLite database for query logs
	// Use in-memory database if no logs directory is configured or in test mode
	var dbPath string
	if ql.logDir == "" {
		// For in-memory databases, use a unique shared cache per instance to avoid test interference
		dbPath = fmt.Sprintf("file:memdb_%d_%s?mode=memory&cache=shared&_fk=1", ql.instanceID, tenantID)
	} else {
		dbPath = fmt.Sprintf("%s/query_logs_%s.db", ql.logDir, tenantID)
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log database for tenant %s: %v", tenantID, err)
	}

	// Create the query_logs table
	createTableSQL := `
		CREATE TABLE IF NOT EXISTS query_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			tenant_id TEXT NOT NULL,
			query TEXT NOT NULL,
			executed_at DATETIME NOT NULL,
			duration_ms INTEGER NOT NULL,
			success BOOLEAN NOT NULL,
			error_message TEXT,
			connection_id TEXT NOT NULL,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP
		);
		
		CREATE INDEX IF NOT EXISTS idx_tenant_executed_at ON query_logs(tenant_id, executed_at);
		CREATE INDEX IF NOT EXISTS idx_connection_id ON query_logs(connection_id);
	`

	if _, err := db.Exec(createTableSQL); err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create query_logs table for tenant %s: %v", tenantID, err)
	}

	ql.logDatabases[tenantID] = db
	ql.logger.Printf("Created query log database for tenant: %s", tenantID)
	return db, nil
}

// LogQuery logs a query execution
func (ql *QueryLogger) LogQuery(tenantID, query, connectionID string, duration time.Duration, success bool, errorMsg string) error {
	// Normalize tenant ID (empty becomes "default")
	if tenantID == "" {
		tenantID = "default"
	}
	
	db, err := ql.getOrCreateLogDatabase(tenantID)
	if err != nil {
		return fmt.Errorf("failed to get log database: %v", err)
	}

	insertSQL := `
		INSERT INTO query_logs (tenant_id, query, executed_at, duration_ms, success, error_message, connection_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`

	executedAt := time.Now()
	durationMs := duration.Nanoseconds() / 1000000 // Convert to milliseconds

	_, err = db.Exec(insertSQL, tenantID, query, executedAt, durationMs, success, errorMsg, connectionID)
	if err != nil {
		return fmt.Errorf("failed to insert query log: %v", err)
	}

	return nil
}

// GetQueryLogs retrieves query logs for a tenant with optional filters
func (ql *QueryLogger) GetQueryLogs(tenantID string, limit int, offset int, startTime, endTime *time.Time) ([]interface{}, error) {
	db, err := ql.getOrCreateLogDatabase(tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get log database: %v", err)
	}

	// Build the query with optional time filters
	querySQL := `
		SELECT id, tenant_id, query, executed_at, duration_ms, success, 
		       COALESCE(error_message, '') as error_message, connection_id
		FROM query_logs 
		WHERE tenant_id = ?
	`
	args := []interface{}{tenantID}

	if startTime != nil {
		querySQL += " AND executed_at >= ?"
		args = append(args, *startTime)
	}

	if endTime != nil {
		querySQL += " AND executed_at <= ?"
		args = append(args, *endTime)
	}

	querySQL += " ORDER BY executed_at DESC"

	if limit > 0 {
		querySQL += " LIMIT ?"
		args = append(args, limit)
	}

	if offset > 0 {
		querySQL += " OFFSET ?"
		args = append(args, offset)
	}

	rows, err := db.Query(querySQL, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %v", err)
	}
	defer rows.Close()

	var logs []interface{}
	for rows.Next() {
		var entry QueryLogEntry
		var executedAtStr string

		err := rows.Scan(
			&entry.ID,
			&entry.TenantID,
			&entry.Query,
			&executedAtStr,
			&entry.Duration,
			&entry.Success,
			&entry.ErrorMsg,
			&entry.ConnectionID,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan log entry: %v", err)
		}

		// Parse the timestamp
		entry.ExecutedAt, err = time.Parse("2006-01-02 15:04:05", executedAtStr)
		if err != nil {
			// Try with timezone format
			entry.ExecutedAt, err = time.Parse(time.RFC3339, executedAtStr)
			if err != nil {
				ql.logger.Printf("Warning: failed to parse timestamp %s: %v", executedAtStr, err)
				entry.ExecutedAt = time.Now() // Fallback
			}
		}

		logs = append(logs, entry)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating over logs: %v", err)
	}

	return logs, nil
}

// GetQueryLogStats returns statistics for a tenant's query logs
func (ql *QueryLogger) GetQueryLogStats(tenantID string) (map[string]interface{}, error) {
	db, err := ql.getOrCreateLogDatabase(tenantID)
	if err != nil {
		return nil, fmt.Errorf("failed to get log database: %v", err)
	}

	statsSQL := `
		SELECT 
			COUNT(*) as total_queries,
			COUNT(CASE WHEN success = 1 THEN 1 END) as successful_queries,
			COUNT(CASE WHEN success = 0 THEN 1 END) as failed_queries,
			COALESCE(AVG(duration_ms), 0) as avg_duration_ms,
			COALESCE(MAX(duration_ms), 0) as max_duration_ms,
			COALESCE(MIN(duration_ms), 0) as min_duration_ms
		FROM query_logs 
		WHERE tenant_id = ?
	`

	var stats struct {
		TotalQueries      int64
		SuccessfulQueries int64
		FailedQueries     int64
		AvgDuration       float64
		MaxDuration       int64
		MinDuration       int64
	}

	err = db.QueryRow(statsSQL, tenantID).Scan(
		&stats.TotalQueries,
		&stats.SuccessfulQueries,
		&stats.FailedQueries,
		&stats.AvgDuration,
		&stats.MaxDuration,
		&stats.MinDuration,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to get query stats: %v", err)
	}

	// Calculate success rate safely
	var successRate float64
	if stats.TotalQueries > 0 {
		successRate = float64(stats.SuccessfulQueries) / float64(stats.TotalQueries) * 100
	}

	result := map[string]interface{}{
		"tenant_id":          tenantID,
		"total_queries":      stats.TotalQueries,
		"successful_queries": stats.SuccessfulQueries,
		"failed_queries":     stats.FailedQueries,
		"success_rate":       successRate,
		"avg_duration_ms":    stats.AvgDuration,
		"max_duration_ms":    stats.MaxDuration,
		"min_duration_ms":    stats.MinDuration,
	}

	return result, nil
}

// ListTenantLogs returns a list of all tenants that have query logs
func (ql *QueryLogger) ListTenantLogs() []string {
	ql.dbMu.RLock()
	defer ql.dbMu.RUnlock()

	tenants := make([]string, 0, len(ql.logDatabases))
	for tenantID := range ql.logDatabases {
		tenants = append(tenants, tenantID)
	}

	return tenants
}

// Close closes all log database connections
func (ql *QueryLogger) Close() error {
	ql.dbMu.Lock()
	defer ql.dbMu.Unlock()

	for tenantID, db := range ql.logDatabases {
		if err := db.Close(); err != nil {
			ql.logger.Printf("Error closing log database for tenant %s: %v", tenantID, err)
		}
	}

	ql.logDatabases = make(map[string]*sql.DB)
	return nil
}
