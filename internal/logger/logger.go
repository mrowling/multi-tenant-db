package logger

import (
	"io"
	"log"
	"os"
)

// Setup creates and configures the application logger
func Setup() *log.Logger {
	// Create logs directory if it doesn't exist
	if err := os.MkdirAll("logs", 0755); err != nil {
		log.Fatalf("Failed to create logs directory: %v", err)
	}
	
	// Create or open log file
	logFile, err := os.OpenFile("logs/app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	
	// Create a multi-writer that writes to both file and stdout
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	
	// Create logger that writes to both file and console
	logger := log.New(multiWriter, "[MULTI-TENANT-DB] ", log.Ldate|log.Ltime|log.Lshortfile)
	
	// Also configure the default logger to use the same format and multi-writer
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetPrefix("[MULTI-TENANT-DB] ")
	
	return logger
}
