package logger

import (
	"io"
	"log"
	"os"
)

// Setup creates and configures the application logger
func Setup() *log.Logger {
	env := os.Getenv("ENV")
	if env == "PROD" || env == "prod" {
		// Production: log only to stdout
		logger := log.New(os.Stdout, "[MULTI-TENANT-DB] ", log.Ldate|log.Ltime|log.Lshortfile)
		log.SetOutput(os.Stdout)
		log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
		log.SetPrefix("[MULTI-TENANT-DB] ")
		return logger
	}

	// Non-production: log to file in current working directory and stdout
	logFile, err := os.OpenFile("app.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Failed to open log file: %v", err)
	}
	multiWriter := io.MultiWriter(os.Stdout, logFile)
	logger := log.New(multiWriter, "[MULTI-TENANT-DB] ", log.Ldate|log.Ltime|log.Lshortfile)
	log.SetOutput(multiWriter)
	log.SetFlags(log.Ldate | log.Ltime | log.Lshortfile)
	log.SetPrefix("[MULTI-TENANT-DB] ")
	return logger
}
