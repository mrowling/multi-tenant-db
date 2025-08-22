package api

import (
	"encoding/json"
	"log"
	"net/http"
	"time"
)

// Response struct for JSON responses
type Response struct {
	Message   string    `json:"message"`
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// Handler represents the HTTP API handler
type Handler struct {
	logger *log.Logger
}

// NewHandler creates a new API handler
func NewHandler(logger *log.Logger) *Handler {
	return &Handler{
		logger: logger,
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

// Root endpoint
func (h *Handler) RootHandler(w http.ResponseWriter, r *http.Request) {
	response := Response{
		Message:   "Welcome to Ephemeral DB!",
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

// Info endpoint with API information
func (h *Handler) InfoHandler(w http.ResponseWriter, r *http.Request) {
	info := map[string]interface{}{
		"service":     "ephemeral-db",
		"version":     "1.0.0",
		"description": "A MySQL-compatible server with custom logic",
		"protocols": map[string]interface{}{
			"http": map[string]interface{}{
				"port": 8080,
				"endpoints": []string{
					"GET /",
					"GET /health",
					"GET /api/info",
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

// SetupRoutes configures the HTTP routes
func (h *Handler) SetupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	
	// Register routes
	mux.HandleFunc("/", h.RootHandler)
	mux.HandleFunc("/health", h.HealthHandler)
	mux.HandleFunc("/api/info", h.InfoHandler)
	
	return mux
}
