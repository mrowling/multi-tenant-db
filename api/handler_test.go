package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// MockDatabaseManager implements the DatabaseManager interface for testing
type MockDatabaseManager struct {
	databases map[string]interface{}
	deleted   map[string]bool
	mu        sync.RWMutex
}

func NewMockDatabaseManager() *MockDatabaseManager {
	return &MockDatabaseManager{
		databases: map[string]interface{}{
			"default": struct{}{},
			"test1":   struct{}{},
			"test2":   struct{}{},
		},
		deleted: make(map[string]bool),
	}
}

func (m *MockDatabaseManager) GetActiveDatabases() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]interface{})
	for k, v := range m.databases {
		if !m.deleted[k] {
			result[k] = v
		}
	}
	return result
}

func (m *MockDatabaseManager) GetOrCreateDatabase(idx string) (interface{}, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if idx == "" {
		idx = "default"
	}
	if idx == "error_test" {
		return nil, fmt.Errorf("simulated error")
	}
	m.databases[idx] = struct{}{}
	m.deleted[idx] = false
	return struct{}{}, nil
}

func (m *MockDatabaseManager) DeleteDatabase(idx string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if idx == "default" {
		return fmt.Errorf("cannot delete default database")
	}
	if idx == "error_test" {
		return fmt.Errorf("simulated delete error")
	}
	if _, exists := m.databases[idx]; !exists {
		return fmt.Errorf("database %s does not exist", idx)
	}
	m.deleted[idx] = true
	return nil
}

func (m *MockDatabaseManager) ListDatabases() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var result []string
	for k := range m.databases {
		if !m.deleted[k] {
			result = append(result, k)
		}
	}
	return result
}

func TestNewHandler(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	
	handler := NewHandler(logger, mockDB)
	
	if handler == nil {
		t.Error("Handler should not be nil")
	}
	if handler.logger != logger {
		t.Error("Handler should store the provided logger")
	}
	if handler.dbManager != mockDB {
		t.Error("Handler should store the provided database manager")
	}
}

func TestHandler_HealthHandler(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.HealthHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Health handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var response Response
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Should be able to unmarshal response: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response.Status)
	}
	if response.Message != "Server is healthy" {
		t.Errorf("Unexpected health message: %s", response.Message)
	}
}

func TestHandler_InfoHandler(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	req, err := http.NewRequest("GET", "/api/info", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.InfoHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Info handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	// Check that response contains expected information
	body := rr.Body.String()
	if !strings.Contains(body, "multitenant-db") {
		t.Error("Info response should contain 'multitenant-db'")
	}
	if !strings.Contains(body, "3306") {
		t.Error("Info response should contain MySQL port information")
	}
	if !strings.Contains(body, "8080") {
		t.Error("Info response should contain HTTP port information")
	}
}

func TestHandler_ListDatabasesHandler(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	req, err := http.NewRequest("GET", "/api/databases", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.ListDatabasesHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("List databases handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var response DatabaseResponse
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Should be able to unmarshal response: %v", err)
	}

	if response.Status != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response.Status)
	}

	if len(response.Databases) == 0 {
		t.Error("Should return some databases")
	}

	// Check that default database is included
	hasDefault := false
	for _, db := range response.Databases {
		if db.Idx == "default" {
			hasDefault = true
			break
		}
	}
	if !hasDefault {
		t.Error("Response should include default database")
	}
}

func TestHandler_CreateDatabaseHandler(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	// Test successful creation
	requestBody := CreateDatabaseRequest{Idx: "new_test_db"}
	jsonBody, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("POST", "/api/databases/create", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.CreateDatabaseHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusCreated {
		t.Errorf("Create database handler returned wrong status code: got %v want %v",
			status, http.StatusCreated)
	}

	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Should be able to unmarshal response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response["status"])
	}
	message := response["message"].(string)
	if !strings.Contains(message, "created successfully") {
		t.Error("Response message should indicate successful creation")
	}
}

func TestHandler_CreateDatabaseHandler_EmptyIdx(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	// Test with empty idx
	requestBody := CreateDatabaseRequest{Idx: ""}
	jsonBody, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("POST", "/api/databases/create", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.CreateDatabaseHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Create database handler should return bad request for empty idx: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandler_CreateDatabaseHandler_InvalidJSON(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	// Test with invalid JSON
	req, err := http.NewRequest("POST", "/api/databases/create", bytes.NewBuffer([]byte("invalid json")))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.CreateDatabaseHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Create database handler should return bad request for invalid JSON: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandler_CreateDatabaseHandler_DatabaseError(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	// Test with idx that triggers error in mock
	requestBody := CreateDatabaseRequest{Idx: "error_test"}
	jsonBody, _ := json.Marshal(requestBody)

	req, err := http.NewRequest("POST", "/api/databases/create", bytes.NewBuffer(jsonBody))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.CreateDatabaseHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Create database handler should return internal server error for database error: got %v want %v",
			status, http.StatusInternalServerError)
	}
}

func TestHandler_DeleteDatabaseHandler(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	// Test successful deletion
	req, err := http.NewRequest("DELETE", "/api/databases/delete?idx=test1", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.DeleteDatabaseHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Delete database handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	var response map[string]interface{}
	err = json.Unmarshal(rr.Body.Bytes(), &response)
	if err != nil {
		t.Errorf("Should be able to unmarshal response: %v", err)
	}

	if response["status"] != "ok" {
		t.Errorf("Expected status 'ok', got '%s'", response["status"])
	}
	message := response["message"].(string)
	if !strings.Contains(message, "deleted successfully") {
		t.Error("Response message should indicate successful deletion")
	}
}

func TestHandler_DeleteDatabaseHandler_MissingIdx(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	// Test without idx parameter
	req, err := http.NewRequest("DELETE", "/api/databases/delete", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.DeleteDatabaseHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Delete database handler should return bad request for missing idx: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandler_DeleteDatabaseHandler_DefaultDatabase(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	// Test trying to delete default database
	req, err := http.NewRequest("DELETE", "/api/databases/delete?idx=default", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.DeleteDatabaseHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusBadRequest {
		t.Errorf("Delete database handler should return bad request for default database: got %v want %v",
			status, http.StatusBadRequest)
	}
}

func TestHandler_DeleteDatabaseHandler_DatabaseError(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	// Test with idx that triggers error in mock
	req, err := http.NewRequest("DELETE", "/api/databases/delete?idx=error_test", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.DeleteDatabaseHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusInternalServerError {
		t.Errorf("Delete database handler should return internal server error for database error: got %v want %v",
			status, http.StatusInternalServerError)
	}
}

func TestHandler_RootHandler(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	req, err := http.NewRequest("GET", "/", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.RootHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusOK {
		t.Errorf("Root handler returned wrong status code: got %v want %v",
			status, http.StatusOK)
	}

	body := rr.Body.String()
	if !strings.Contains(body, "Multitenant DB") {
		t.Error("Root response should contain 'Multitenant DB'")
	}
	if !strings.Contains(body, "Welcome") {
		t.Error("Root response should contain 'Welcome'")
	}
}

func TestHandler_MethodNotAllowed(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	// Test GET to create endpoint (should be POST only)
	req, err := http.NewRequest("GET", "/api/databases/create", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	http.HandlerFunc(handler.CreateDatabaseHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("Create database handler should return method not allowed for GET: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}

	// Test POST to list endpoint (should be GET only)
	req, err = http.NewRequest("POST", "/api/databases", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr = httptest.NewRecorder()
	http.HandlerFunc(handler.ListDatabasesHandler).ServeHTTP(rr, req)

	if status := rr.Code; status != http.StatusMethodNotAllowed {
		t.Errorf("List databases handler should return method not allowed for POST: got %v want %v",
			status, http.StatusMethodNotAllowed)
	}
}

func TestHandler_ConcurrentRequests(t *testing.T) {
	logger := log.New(os.Stdout, "[TEST] ", log.LstdFlags)
	mockDB := NewMockDatabaseManager()
	handler := NewHandler(logger, mockDB)

	// Test concurrent requests to different endpoints
	numRequests := 10
	done := make(chan bool, numRequests*3)

	// Concurrent health checks
	for i := 0; i < numRequests; i++ {
		go func() {
			req, _ := http.NewRequest("GET", "/health", nil)
			rr := httptest.NewRecorder()
			http.HandlerFunc(handler.HealthHandler).ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("Concurrent health check failed with status %d", rr.Code)
			}
			done <- true
		}()
	}

	// Concurrent database lists
	for i := 0; i < numRequests; i++ {
		go func() {
			req, _ := http.NewRequest("GET", "/api/databases", nil)
			rr := httptest.NewRecorder()
			http.HandlerFunc(handler.ListDatabasesHandler).ServeHTTP(rr, req)
			if rr.Code != http.StatusOK {
				t.Errorf("Concurrent database list failed with status %d", rr.Code)
			}
			done <- true
		}()
	}

	// Concurrent database creations
	for i := 0; i < numRequests; i++ {
		go func(i int) {
			requestBody := CreateDatabaseRequest{Idx: fmt.Sprintf("concurrent_%d", i)}
			jsonBody, _ := json.Marshal(requestBody)
			req, _ := http.NewRequest("POST", "/api/databases/create", bytes.NewBuffer(jsonBody))
			req.Header.Set("Content-Type", "application/json")
			rr := httptest.NewRecorder()
			http.HandlerFunc(handler.CreateDatabaseHandler).ServeHTTP(rr, req)
			if rr.Code != http.StatusCreated {
				t.Errorf("Concurrent database creation failed with status %d", rr.Code)
			}
			done <- true
		}(i)
	}

	// Wait for all requests to complete
	for i := 0; i < numRequests*3; i++ {
		<-done
	}
}

func TestResponse_JSONSerialization(t *testing.T) {
	response := Response{
		Message:   "Test message",
		Status:    "success",
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Errorf("Should be able to marshal response: %v", err)
	}

	var unmarshaled Response
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Errorf("Should be able to unmarshal response: %v", err)
	}

	if unmarshaled.Message != response.Message {
		t.Errorf("Message should match after JSON round trip")
	}
	if unmarshaled.Status != response.Status {
		t.Errorf("Status should match after JSON round trip")
	}
}

func TestDatabaseResponse_JSONSerialization(t *testing.T) {
	response := DatabaseResponse{
		Databases: []DatabaseInfo{
			{Name: "test1", Idx: "test1"},
			{Name: "test2", Idx: "test2"},
		},
		Status:    "success",
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(response)
	if err != nil {
		t.Errorf("Should be able to marshal database response: %v", err)
	}

	var unmarshaled DatabaseResponse
	err = json.Unmarshal(jsonData, &unmarshaled)
	if err != nil {
		t.Errorf("Should be able to unmarshal database response: %v", err)
	}

	if len(unmarshaled.Databases) != len(response.Databases) {
		t.Errorf("Database count should match after JSON round trip")
	}
	if unmarshaled.Status != response.Status {
		t.Errorf("Status should match after JSON round trip")
	}
}
