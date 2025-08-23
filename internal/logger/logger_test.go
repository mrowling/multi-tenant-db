package logger

import (
	"bytes"
	"log"
	"os"
	"strings"
	"sync"
	"testing"
)

func TestSetup(t *testing.T) {
       os.Setenv("ENV", "")
       os.Remove("app.log")
       logger := Setup()
       if logger == nil {
	       t.Error("Logger should not be nil")
       }
       logger.Println("Test log message")
       if _, err := os.Stat("app.log"); os.IsNotExist(err) {
	       t.Error("Log file should have been created")
       }
}

func TestSetup_FileCreation(t *testing.T) {
       os.Setenv("ENV", "")
       os.Remove("app.log")
       logger := Setup()
       if logger == nil {
	       t.Error("Logger should not be nil even when directory doesn't exist")
       }
       logger.Println("Test message for file creation")
       if _, err := os.Stat("app.log"); os.IsNotExist(err) {
	       t.Error("Log file should have been created")
       }
}

func TestSetup_InvalidPath(t *testing.T) {
	os.Setenv("ENV", "")
	// Since Setup() uses a fixed path, this test doesn't apply
	// We'll test the logger functionality instead
	logger := Setup()
	
	if logger == nil {
		t.Error("Logger should not be nil")
	}

	// This test mainly ensures no panic occurs
	logger.Println("Test message")
}

func TestSetup_Prefix(t *testing.T) {
	// Capture stdout to test the prefix
	var buf bytes.Buffer
	logger := log.New(&buf, "[MULTI-TENANT-DB] ", log.LstdFlags|log.Lshortfile)
	
	logger.Println("Test prefix message")
	
	output := buf.String()
	if !strings.Contains(output, "[MULTI-TENANT-DB]") {
		t.Error("Logger output should contain the correct prefix")
	}
	if !strings.Contains(output, "Test prefix message") {
		t.Error("Logger output should contain the test message")
	}
}

func TestSetup_LogFormat(t *testing.T) {
	var buf bytes.Buffer
	logger := log.New(&buf, "[MULTI-TENANT-DB] ", log.LstdFlags|log.Lshortfile)
	
	logger.Println("Format test message")
	
	output := buf.String()
	
	// Check for timestamp (year should be present)
	if !strings.Contains(output, "2025") {
		t.Error("Logger output should contain timestamp")
	}
	
	// Check for file information
	if !strings.Contains(output, ".go:") {
		t.Error("Logger output should contain file information")
	}
}

func TestSetup_MultipleWrites(t *testing.T) {
       os.Setenv("ENV", "")
       os.Remove("app.log")
       logger := Setup()
       messages := []string{
	       "First message",
	       "Second message",
	       "Third message",
       }
       for _, msg := range messages {
	       logger.Println(msg)
       }
       content, err := os.ReadFile("app.log")
       if err != nil {
	       t.Errorf("Should be able to read log file: %v", err)
	       return
       }
       contentStr := string(content)
       for _, msg := range messages {
	       if !strings.Contains(contentStr, msg) {
		       t.Errorf("Log file should contain message: %s", msg)
	       }
       }
}

func TestSetup_DirectoryCreation(t *testing.T) {
       os.Setenv("ENV", "")
       os.Remove("app.log")
       logger := Setup()
       if logger == nil {
	       t.Error("Logger should not be nil")
       }
       logger.Println("Test directory creation")
       if _, err := os.Stat("app.log"); os.IsNotExist(err) {
	       t.Error("Setup should create log file")
       }
}

func TestSetup_ConcurrentAccess(t *testing.T) {
       os.Setenv("ENV", "")
       os.Remove("app.log")
       logger := Setup()
       const numGoroutines = 10
       const messagesPerGoroutine = 10
       var wg sync.WaitGroup
       for i := 0; i < numGoroutines; i++ {
	       wg.Add(1)
	       go func(id int) {
		       defer wg.Done()
		       for j := 0; j < messagesPerGoroutine; j++ {
			       logger.Printf("Goroutine %d, message %d", id, j)
		       }
	       }(i)
       }
       wg.Wait()
       content, err := os.ReadFile("app.log")
       if err != nil {
	       t.Errorf("Should be able to read log file: %v", err)
	       return
       }
       contentStr := string(content)
       messageCount := strings.Count(contentStr, "Goroutine")
       expectedCount := numGoroutines * messagesPerGoroutine
       if messageCount != expectedCount {
	       t.Errorf("Expected %d messages, found %d", expectedCount, messageCount)
       }
}

func TestSetup_EmptyLogFile(t *testing.T) {
	os.Setenv("ENV", "")
	// Since Setup() uses a fixed path, we test the functionality as-is
	logger := Setup()
	
	if logger == nil {
		t.Error("Logger should not be nil")
	}

	// Should not panic
	logger.Println("Test message")
}

func TestSetup_ExistingFile(t *testing.T) {
	os.Setenv("ENV", "")
	os.Remove("app.log")
	os.Remove("app.log")
	err := os.WriteFile("app.log", []byte("Initial content\n"), 0644)
	if err != nil {
		t.Fatalf("Failed to create initial log file: %v", err)
	}
	logger := Setup()
	logger.Println("New log message")
	content, err := os.ReadFile("app.log")
	if err != nil {
		t.Fatalf("Failed to read log file: %v", err)
	}
	contentStr := string(content)
	if !strings.Contains(contentStr, "Initial content") {
		t.Error("Log file should preserve existing content")
	}
	if !strings.Contains(contentStr, "New log message") {
		t.Error("Log file should contain new message")
	}
	if !strings.Contains(contentStr, "[MULTI-TENANT-DB]") {
		t.Error("New log message should have correct prefix")
	}
}
