package api

import (
	"encoding/json"
	"log"
	"net/http"
	"strings"
	"time"
)

// DatabaseManager interface to avoid circular imports
type DatabaseManager interface {
	GetActiveDatabases() map[string]interface{}
	GetOrCreateDatabase(idx string) (interface{}, error)
	DeleteDatabase(idx string) error
	ListDatabases() []string
}

// Response struct for JSON responses
type Response struct {
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// DatabaseResponse struct for database operations
type DatabaseResponse struct {
	Databases []DatabaseInfo `json:"databases"`
	Status    string         `json:"status"`
	Timestamp time.Time      `json:"timestamp"`
}

// DatabaseInfo struct for database information
type DatabaseInfo struct {
	Name string `json:"name"`
	Idx  string `json:"idx"`
}

// CreateDatabaseRequest struct for database creation
type CreateDatabaseRequest struct {
	Idx string `json:"idx"`
}

// Handler represents the HTTP API handler
type Handler struct {
	logger *log.Logger
	dbManager DatabaseManager
}

// NewHandler creates a new API handler
func NewHandler(logger *log.Logger, dbManager DatabaseManager) *Handler {
	return &Handler{
		logger: logger,
		dbManager: dbManager,
	}
}

// Middleware for logging HTTP requests
func (h *Handler) LoggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		
		// Call the next handler
		next.ServeHTTP(w, r)
		
		// Log the request
		h.logger.Printf("%s %s %s %v", r.Method, r.URL.Path, r.RemoteAddr, time.Since(start))
	})
}

// HealthHandler godoc
// @Summary Health check
// @Description Returns server health status
// @Tags health
// @Produce json
// @Success 200 {object} Response
// @Router /health [get]
// Health check endpoint
func (h *Handler) HealthHandler(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Message:   "Server is healthy",
		Status:    "ok",
		Timestamp: time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	h.logger.Printf("Health check requested from %s", r.RemoteAddr)
}

// RootHandler godoc
// @Summary Root welcome
// @Description Welcome message for Multitenant DB API
// @Tags root
// @Produce json
// @Success 200 {object} Response
// @Router / [get]
// Root endpoint
func (h *Handler) RootHandler(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Message:   "Welcome to Multitenant DB!",
		Status:    "ok",
		Timestamp: time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(response); err != nil {
		h.logger.Printf("Error encoding response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	h.logger.Printf("Root endpoint accessed from %s", r.RemoteAddr)
}

// InfoHandler godoc
// @Summary API info
// @Description Returns API and protocol information
// @Tags info
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /api/info [get]
// Info endpoint with API information
func (h *Handler) InfoHandler(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"service":     "multitenant-db",
		"version":     "1.0.0",
		"description": "A MySQL-compatible multi-tenant database server with per-idx isolation",
		"protocols": map[string]interface{}{
			"http": map[string]interface{}{
				"port": 8080,
			       "endpoints": []string{
				       "GET /",
				       "GET /health",
				       "GET /api/info",
				       "GET /api/databases",
				       "POST /api/databases",
				       "DELETE /api/databases?idx=<idx>",
			       },
			},
			"mysql": map[string]interface{}{
				"port":       3306,
				"connection": "mysql -h 127.0.0.1 -P 3306 -u root --protocol=TCP",
				"features": []string{
					"SHOW TABLES",
					"SHOW DATABASES",
					"SELECT queries",
					"DESCRIBE tables",
					"Basic INSERT support",
					"Connection Attributes",
				},
				"connection_attributes": []string{
					"SET CONNECTION_ATTRIBUTE 'key'='value'",
					"SHOW CONNECTION_ATTRIBUTES",
					"CLEAR CONNECTION_ATTRIBUTES",
				},
			},
		},
		"sample_data": map[string]interface{}{
			"tables": []string{"users", "products"},
			"queries": []string{
				"SHOW TABLES",
				"SELECT * FROM users",
				"SELECT * FROM products",
				"DESCRIBE users",
				"SET CONNECTION_ATTRIBUTE 'app_name'='my_app'",
				"SHOW CONNECTION_ATTRIBUTES",
				"CLEAR CONNECTION_ATTRIBUTES",
			},
		},
		"timestamp": time.Now(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	
	if err := json.NewEncoder(w).Encode(info); err != nil {
		h.logger.Printf("Error encoding API info response: %v", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		return
	}
	
	h.logger.Printf("API info requested from %s", r.RemoteAddr)
}

// DatabasesHandler godoc
// @Summary Manage tenant databases
// @Description List, create, or delete tenant databases
// @Tags databases
// @Produce json
// @Param idx query string false "Tenant idx (for DELETE)"
// @Param request body CreateDatabaseRequest false "Create database request (for POST)"
// @Success 200 {object} DatabaseResponse "List/Delete success"
// @Success 201 {object} map[string]interface{} "Create success"
// @Failure 400 {object} map[string]interface{} "Bad request"
// @Failure 405 {object} map[string]interface{} "Method not allowed"
// @Failure 500 {object} map[string]interface{} "Internal error"
// @Router /api/databases [get]
// @Router /api/databases [post]
// @Router /api/databases [delete]
// DatabasesHandler handles GET, POST, DELETE for /api/databases
func (h *Handler) DatabasesHandler(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		databases := h.dbManager.ListDatabases()
		var dbInfos []DatabaseInfo
		for _, idx := range databases {
			var name string
			if idx == "" || idx == "default" {
				name = "multitenant_db"
			} else {
				name = "multitenant_db_idx_" + idx
			}
			dbInfos = append(dbInfos, DatabaseInfo{
				Name: name,
				Idx:  idx,
			})
		}
		response := DatabaseResponse{
			Databases: dbInfos,
			Status:    "ok",
			Timestamp: time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Printf("Error encoding databases response: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		h.logger.Printf("Databases listed for %s", r.RemoteAddr)
	case http.MethodPost:
		var req CreateDatabaseRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid JSON request", http.StatusBadRequest)
			return
		}
		if req.Idx == "" {
			http.Error(w, "idx field is required", http.StatusBadRequest)
			return
		}
		_, err := h.dbManager.GetOrCreateDatabase(req.Idx)
		if err != nil {
			h.logger.Printf("Error creating database for idx %s: %v", req.Idx, err)
			http.Error(w, "Failed to create database", http.StatusInternalServerError)
			return
		}
		var name string
		if req.Idx == "default" {
			name = "multitenant_db"
		} else {
			name = "multitenant_db_idx_" + req.Idx
		}
		response := map[string]interface{}{
			"message":   "Database created successfully",
			"status":    "ok",
			"database":  name,
			"idx":       req.Idx,
			"timestamp": time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Printf("Error encoding create database response: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		h.logger.Printf("Database created for idx %s from %s", req.Idx, r.RemoteAddr)
	case http.MethodDelete:
		idx := r.URL.Query().Get("idx")
		if idx == "" {
			http.Error(w, "idx query parameter is required", http.StatusBadRequest)
			return
		}
		if idx == "default" {
			http.Error(w, "Cannot delete default database", http.StatusBadRequest)
			return
		}
		err := h.dbManager.DeleteDatabase(idx)
		if err != nil {
			h.logger.Printf("Error deleting database for idx %s: %v", idx, err)
			http.Error(w, "Failed to delete database", http.StatusInternalServerError)
			return
		}
		response := map[string]interface{}{
			"message":   "Database deleted successfully",
			"status":    "ok",
			"idx":       idx,
			"timestamp": time.Now(),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		if err := json.NewEncoder(w).Encode(response); err != nil {
			h.logger.Printf("Error encoding delete database response: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}
		h.logger.Printf("Database deleted for idx %s from %s", idx, r.RemoteAddr)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// SetupRoutes configures the HTTP routes
func (h *Handler) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	
	// Register routes
	mux.HandleFunc("/", h.RootHandler)
	mux.HandleFunc("/health", h.HealthHandler)
	mux.HandleFunc("/api/info", h.InfoHandler)
	mux.HandleFunc("/api/databases", h.DatabasesHandler)
	
	// Query log routes - simplified paths
	mux.HandleFunc("/api/query-logs", h.ListQueryLogTenantsHandler)
	mux.HandleFunc("/api/query-logs/", h.handleQueryLogRoutes)
	
	return mux
}

// handleQueryLogRoutes handles query log related routes
func (h *Handler) handleQueryLogRoutes(w http.ResponseWriter, r *http.Request) {
	// Parse the path to extract tenant ID and action
	path := r.URL.Path[len("/api/query-logs/"):]
	
	if path == "" {
		// Handle /api/query-logs/ -> list tenants
		h.ListQueryLogTenantsHandler(w, r)
		return
	}
	
	// Split path to get tenant and action
	parts := strings.Split(path, "/")
	if len(parts) == 0 {
		http.NotFound(w, r)
		return
	}
	
	if len(parts) == 1 {
		// Handle /api/query-logs/{tenantId} -> get logs for tenant
		h.GetQueryLogsHandler(w, r)
		return
	}
	
	if len(parts) == 2 && parts[1] == "stats" {
		// Handle /api/query-logs/{tenantId}/stats -> get stats for tenant
		h.GetQueryLogStatsHandler(w, r)
		return
	}
	
	// If no specific endpoint matches, return 404
	http.NotFound(w, r)
}
