package api

import (
	"encoding/json"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"
)

// QueryLogEntry represents a query log entry for API responses
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

// QueryLogResponse represents the response for query log requests
type QueryLogResponse struct {
	Logs      []QueryLogEntry `json:"logs"`
	Total     int             `json:"total"`
	Page      int             `json:"page"`
	PageSize  int             `json:"page_size"`
	Status    string          `json:"status"`
	Timestamp time.Time       `json:"timestamp"`
}

// QueryLogStatsResponse represents the response for query log statistics
type QueryLogStatsResponse struct {
	Stats     map[string]interface{} `json:"stats"`
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
}

// TenantsResponse represents the response for listing tenants with logs
type TenantsResponse struct {
	Tenants   []string  `json:"tenants"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// QueryLogger interface for API access
type QueryLogger interface {
	GetQueryLogs(tenantID string, limit int, offset int, startTime, endTime *time.Time) ([]interface{}, error)
	GetQueryLogStats(tenantID string) (map[string]interface{}, error)
	ListTenantLogs() []string
}

// GetQueryLogsHandler godoc
// @Summary Get query logs for a tenant
// @Description Retrieve query logs for a specific tenant with optional pagination and time filtering
// @Tags query-logs
// @Produce json
// @Param tenant_id path string true "Tenant ID"
// @Param page query int false "Page number (default: 1)"
// @Param page_size query int false "Page size (default: 50, max: 1000)"
// @Param start_time query string false "Start time filter (RFC3339 format)"
// @Param end_time query string false "End time filter (RFC3339 format)"
// @Success 200 {object} QueryLogResponse
// @Failure 400 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/tenants/{tenant_id}/query-logs [get]
func (h *Handler) GetQueryLogsHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from URL path
	path := r.URL.Path[len("/api/query-logs/"):]
	parts := strings.Split(path, "/")
	
	if len(parts) == 0 || parts[0] == "" {
		h.sendErrorResponse(w, "Tenant ID is required", http.StatusBadRequest)
		return
	}
	
	tenantID := parts[0]

	// Parse query parameters
	page := 1
	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		if p, err := strconv.Atoi(pageStr); err == nil && p > 0 {
			page = p
		}
	}

	pageSize := 50
	if pageSizeStr := r.URL.Query().Get("page_size"); pageSizeStr != "" {
		if ps, err := strconv.Atoi(pageSizeStr); err == nil && ps > 0 && ps <= 1000 {
			pageSize = ps
		}
	}

	// Parse time filters
	var startTime, endTime *time.Time
	if startTimeStr := r.URL.Query().Get("start_time"); startTimeStr != "" {
		if st, err := time.Parse(time.RFC3339, startTimeStr); err == nil {
			startTime = &st
		} else {
			h.sendErrorResponse(w, "Invalid start_time format. Use RFC3339 format.", http.StatusBadRequest)
			return
		}
	}

	if endTimeStr := r.URL.Query().Get("end_time"); endTimeStr != "" {
		if et, err := time.Parse(time.RFC3339, endTimeStr); err == nil {
			endTime = &et
		} else {
			h.sendErrorResponse(w, "Invalid end_time format. Use RFC3339 format.", http.StatusBadRequest)
			return
		}
	}

	// Get query logger interface
	queryLoggerProvider, ok := h.dbManager.(interface{ GetQueryLogger() interface{} })
	if !ok {
		h.sendErrorResponse(w, "Query logging not supported", http.StatusInternalServerError)
		return
	}
	
	queryLogger, ok := queryLoggerProvider.GetQueryLogger().(interface {
		GetQueryLogs(tenantID string, limit int, offset int, startTime, endTime *time.Time) ([]interface{}, error)
	})
	if !ok {
		h.sendErrorResponse(w, "Query logging not available", http.StatusInternalServerError)
		return
	}

	// Calculate offset
	offset := (page - 1) * pageSize

	// Get logs
	logs, err := queryLogger.GetQueryLogs(tenantID, pageSize, offset, startTime, endTime)
	if err != nil {
		h.logger.Printf("Error getting query logs for tenant %s: %v", tenantID, err)
		h.sendErrorResponse(w, "Failed to retrieve query logs", http.StatusInternalServerError)
		return
	}

	// Convert to API format
	apiLogs := make([]QueryLogEntry, len(logs))
	for i, logInterface := range logs {
		// Use reflection to convert the struct
		logValue := reflect.ValueOf(logInterface)
		if logValue.Kind() == reflect.Struct {
			apiLogs[i] = QueryLogEntry{
				ID:           logValue.FieldByName("ID").Int(),
				TenantID:     logValue.FieldByName("TenantID").String(),
				Query:        logValue.FieldByName("Query").String(),
				ExecutedAt:   logValue.FieldByName("ExecutedAt").Interface().(time.Time),
				Duration:     logValue.FieldByName("Duration").Int(),
				Success:      logValue.FieldByName("Success").Bool(),
				ErrorMsg:     logValue.FieldByName("ErrorMsg").String(),
				ConnectionID: logValue.FieldByName("ConnectionID").String(),
			}
		} else {
			h.logger.Printf("Warning: unexpected log entry type at index %d", i)
		}
	}

	response := QueryLogResponse{
		Logs:      apiLogs,
		Total:     len(apiLogs),
		Page:      page,
		PageSize:  pageSize,
		Status:    "ok",
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Printf("Error encoding query logs response: %v", err)
		h.sendErrorResponse(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.logger.Printf("Query logs retrieved for tenant %s (page %d, size %d)", tenantID, page, pageSize)
}

// GetQueryLogStatsHandler godoc
// @Summary Get query log statistics for a tenant
// @Description Retrieve query execution statistics for a specific tenant
// @Tags query-logs
// @Produce json
// @Param tenant_id path string true "Tenant ID"
// @Success 200 {object} QueryLogStatsResponse
// @Failure 400 {object} Response
// @Failure 500 {object} Response
// @Router /api/v1/tenants/{tenant_id}/query-stats [get]
func (h *Handler) GetQueryLogStatsHandler(w http.ResponseWriter, r *http.Request) {
	// Get tenant ID from URL path
	path := r.URL.Path[len("/api/query-logs/"):]
	parts := strings.Split(path, "/")
	
	if len(parts) < 2 || parts[0] == "" {
		h.sendErrorResponse(w, "Tenant ID is required", http.StatusBadRequest)
		return
	}
	
	tenantID := parts[0]

	// Get query logger interface
	queryLoggerProvider, ok := h.dbManager.(interface{ GetQueryLogger() interface{} })
	if !ok {
		h.sendErrorResponse(w, "Query logging not supported", http.StatusInternalServerError)
		return
	}
	
	queryLogger, ok := queryLoggerProvider.GetQueryLogger().(interface {
		GetQueryLogStats(tenantID string) (map[string]interface{}, error)
	})
	if !ok {
		h.sendErrorResponse(w, "Query logging not available", http.StatusInternalServerError)
		return
	}

	// Get stats
	stats, err := queryLogger.GetQueryLogStats(tenantID)
	if err != nil {
		h.logger.Printf("Error getting query stats for tenant %s: %v", tenantID, err)
		h.sendErrorResponse(w, "Failed to retrieve query statistics", http.StatusInternalServerError)
		return
	}

	response := QueryLogStatsResponse{
		Stats:     stats,
		Status:    "ok",
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Printf("Error encoding query stats response: %v", err)
		h.sendErrorResponse(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.logger.Printf("Query stats retrieved for tenant %s", tenantID)
}

// ListQueryLogTenantsHandler godoc
// @Summary List tenants with query logs
// @Description Get a list of all tenants that have query logs
// @Tags query-logs
// @Produce json
// @Success 200 {object} TenantsResponse
// @Failure 500 {object} Response
// @Router /api/v1/query-logs/tenants [get]
func (h *Handler) ListQueryLogTenantsHandler(w http.ResponseWriter, r *http.Request) {
	// Get query logger interface
	queryLoggerProvider, ok := h.dbManager.(interface{ GetQueryLogger() interface{} })
	if !ok {
		h.sendErrorResponse(w, "Query logging not supported", http.StatusInternalServerError)
		return
	}
	
	queryLogger, ok := queryLoggerProvider.GetQueryLogger().(interface {
		ListTenantLogs() []string
	})
	if !ok {
		h.sendErrorResponse(w, "Query logging not available", http.StatusInternalServerError)
		return
	}

	// Get tenant list
	tenants := queryLogger.ListTenantLogs()

	response := TenantsResponse{
		Tenants:   tenants,
		Status:    "ok",
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Printf("Error encoding tenants response: %v", err)
		h.sendErrorResponse(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}

	h.logger.Printf("Query log tenants list retrieved")
}

// sendErrorResponse is a helper method to send error responses
func (h *Handler) sendErrorResponse(w http.ResponseWriter, message string, statusCode int) {
	response := Response{
		Message:   message,
		Status:    "error",
		Timestamp: time.Now(),
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Printf("Error encoding error response: %v", err)
	}
}
