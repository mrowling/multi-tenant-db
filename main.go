package main

import (
	"net/http"
	"time"

	"ephemeral-db/api"
	"ephemeral-db/logger"
	"ephemeral-db/mysql"
)

func main() {
	// Setup logger
	appLogger := logger.Setup()
	appLogger.Println("Starting Ephemeral DB server...")
	
	// Create MySQL protocol handler
	mysqlHandler := mysql.NewHandler(appLogger)
	
	// Start MySQL protocol server in a goroutine
	go mysql.StartServer(3306, mysqlHandler)
	
	// Create API handler
	apiHandler := api.NewHandler(appLogger)
	
	// Setup HTTP routes
	mux := apiHandler.SetupRoutes()
	
	// Wrap with logging middleware
	handler := apiHandler.LoggingMiddleware(mux)
	
	// HTTP Server configuration
	server := &http.Server{
		Addr:         ":8080",
		Handler:      handler,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	appLogger.Printf("HTTP server starting on port 8080")
	appLogger.Printf("MySQL protocol server starting on port 3306")
	
	appLogger.Printf("Available HTTP endpoints:")
	
	endpoints := []string{
		"http://localhost:8080/",
		"http://localhost:8080/health",
		"http://localhost:8080/api/info",
	}
	
	for _, endpoint := range endpoints {
		appLogger.Printf("  %s", endpoint)
	}
	
	appLogger.Printf("MySQL connection: mysql -h 127.0.0.1 -P 3306 -u root --protocol=TCP")
	
	// Start HTTP server
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		appLogger.Fatalf("HTTP server failed to start: %v", err)
	}
}
