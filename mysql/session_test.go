package mysql

import (
	"fmt"
	"sync"
	"testing"
)

func TestNewSessionVariables(t *testing.T) {
	sv := NewSessionVariables()

	// Test that session variables are initialized
	if sv.sessionVars == nil {
		t.Error("sessionVars map should be initialized")
	}
	if sv.userVars == nil {
		t.Error("userVars map should be initialized")
	}

	// Test default session variables
	expectedDefaults := map[string]interface{}{
		"autocommit":               1,
		"sql_mode":                 "",
		"character_set_client":     "utf8mb4",
		"character_set_connection": "utf8mb4",
		"character_set_results":    "utf8mb4",
		"collation_connection":     "utf8mb4_general_ci",
		"time_zone":                "SYSTEM",
		"tx_isolation":             "REPEATABLE-READ",
		"version_comment":          "Multitenant DB",
	}

	for key, expected := range expectedDefaults {
		value, exists := sv.Get(key)
		if !exists {
			t.Errorf("Default session variable %s should exist", key)
		}
		if value != expected {
			t.Errorf("Default session variable %s should be %v, got %v", key, expected, value)
		}
	}
}

func TestSessionVariables_Set_Get(t *testing.T) {
	sv := NewSessionVariables()

	// Test setting and getting session variables
	sv.Set("test_var", "test_value")
	value, exists := sv.Get("test_var")
	if !exists {
		t.Error("test_var should exist after setting")
	}
	if value != "test_value" {
		t.Errorf("Expected 'test_value', got %v", value)
	}

	// Test case insensitivity
	sv.Set("TEST_VAR", "new_value")
	value, exists = sv.Get("test_var")
	if !exists {
		t.Error("test_var should exist (case insensitive)")
	}
	if value != "new_value" {
		t.Errorf("Expected 'new_value', got %v", value)
	}
}

func TestSessionVariables_SetUser_GetUser(t *testing.T) {
	sv := NewSessionVariables()

	// Test setting and getting user variables
	sv.SetUser("user_var", "user_value")
	value, exists := sv.GetUser("user_var")
	if !exists {
		t.Error("user_var should exist after setting")
	}
	if value != "user_value" {
		t.Errorf("Expected 'user_value', got %v", value)
	}

	// Test case insensitivity
	sv.SetUser("USER_VAR", "new_user_value")
	value, exists = sv.GetUser("user_var")
	if !exists {
		t.Error("user_var should exist (case insensitive)")
	}
	if value != "new_user_value" {
		t.Errorf("Expected 'new_user_value', got %v", value)
	}
}

func TestSessionVariables_Unset(t *testing.T) {
	sv := NewSessionVariables()

	// Set a session variable then unset it
	sv.Set("temp_var", "temp_value")
	_, exists := sv.Get("temp_var")
	if !exists {
		t.Error("temp_var should exist after setting")
	}

	sv.Unset("temp_var")
	_, exists = sv.Get("temp_var")
	if exists {
		t.Error("temp_var should not exist after unsetting")
	}

	// Test unsetting a default variable
	sv.Unset("autocommit")
	_, exists = sv.Get("autocommit")
	if exists {
		t.Error("autocommit should not exist after unsetting")
	}
}

func TestSessionVariables_UnsetUser(t *testing.T) {
	sv := NewSessionVariables()

	// Set a user variable then unset it
	sv.SetUser("temp_user_var", "temp_user_value")
	_, exists := sv.GetUser("temp_user_var")
	if !exists {
		t.Error("temp_user_var should exist after setting")
	}

	sv.UnsetUser("temp_user_var")
	_, exists = sv.GetUser("temp_user_var")
	if exists {
		t.Error("temp_user_var should not exist after unsetting")
	}
}

func TestSessionVariables_GetAll(t *testing.T) {
	sv := NewSessionVariables()

	// Add a custom variable
	sv.Set("custom_var", "custom_value")

	all := sv.GetAll()
	
	// Should contain all default variables plus the custom one
	if len(all) < 10 { // We have 9 defaults + 1 custom
		t.Errorf("Expected at least 10 variables, got %d", len(all))
	}

	// Check that custom variable is included
	if all["custom_var"] != "custom_value" {
		t.Error("custom_var should be included in GetAll()")
	}

	// Check that a default variable is included
	if all["version_comment"] != "Multitenant DB" {
		t.Error("version_comment should be included in GetAll()")
	}
}

func TestSessionVariables_Concurrency(t *testing.T) {
	sv := NewSessionVariables()
	var wg sync.WaitGroup
	
	// Test concurrent access
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			key := fmt.Sprintf("var%d", i)
			value := fmt.Sprintf("value%d", i)
			
			// Set session variable
			sv.Set(key, value)
			
			// Set user variable
			sv.SetUser(key, value)
			
			// Get session variable
			if v, exists := sv.Get(key); !exists || v != value {
				t.Errorf("Concurrent session variable access failed for %s", key)
			}
			
			// Get user variable
			if v, exists := sv.GetUser(key); !exists || v != value {
				t.Errorf("Concurrent user variable access failed for %s", key)
			}
		}(i)
	}
	
	wg.Wait()
}

func TestNewSessionManager(t *testing.T) {
	sm := NewSessionManager()

	if sm.sessions == nil {
		t.Error("sessions map should be initialized")
	}
	if sm.connectionCounter != 0 {
		t.Error("connectionCounter should start at 0")
	}
}

func TestSessionManager_GetOrCreateSession(t *testing.T) {
	sm := NewSessionManager()

	// Test creating a new session
	session1 := sm.GetOrCreateSession(1)
	if session1 == nil {
		t.Error("Session should not be nil")
	}

	// Test getting the same session
	session2 := sm.GetOrCreateSession(1)
	if session1 != session2 {
		t.Error("Should get the same session instance for the same connection ID")
	}

	// Test creating a different session
	session3 := sm.GetOrCreateSession(2)
	if session1 == session3 {
		t.Error("Should get different session instances for different connection IDs")
	}
}

func TestSessionManager_RemoveSession(t *testing.T) {
	sm := NewSessionManager()

	// Create a session
	sm.GetOrCreateSession(1)
	_, exists := sm.GetSession(1)
	if !exists {
		t.Error("Session should exist after creation")
	}

	// Remove the session
	sm.RemoveSession(1)
	_, exists = sm.GetSession(1)
	if exists {
		t.Error("Session should not exist after removal")
	}
}

func TestSessionManager_GetNextConnectionID(t *testing.T) {
	sm := NewSessionManager()

	// Test that connection IDs are incremented
	id1 := sm.GetNextConnectionID()
	id2 := sm.GetNextConnectionID()
	id3 := sm.GetNextConnectionID()

	if id1 != 1 {
		t.Errorf("First connection ID should be 1, got %d", id1)
	}
	if id2 != 2 {
		t.Errorf("Second connection ID should be 2, got %d", id2)
	}
	if id3 != 3 {
		t.Errorf("Third connection ID should be 3, got %d", id3)
	}
}

func TestSessionManager_CurrentConnection(t *testing.T) {
	sm := NewSessionManager()

	// Test setting and getting current connection
	sm.SetCurrentConnection(42)
	current := sm.GetCurrentConnection()
	if current != 42 {
		t.Errorf("Expected current connection to be 42, got %d", current)
	}

	// Test changing current connection
	sm.SetCurrentConnection(100)
	current = sm.GetCurrentConnection()
	if current != 100 {
		t.Errorf("Expected current connection to be 100, got %d", current)
	}
}

func TestSessionManager_GetSession(t *testing.T) {
	sm := NewSessionManager()

	// Test getting non-existent session
	_, exists := sm.GetSession(999)
	if exists {
		t.Error("Non-existent session should not exist")
	}

	// Create a session and test getting it
	originalSession := sm.GetOrCreateSession(123)
	retrievedSession, exists := sm.GetSession(123)
	if !exists {
		t.Error("Session should exist after creation")
	}
	if originalSession != retrievedSession {
		t.Error("Retrieved session should be the same as original")
	}
}

func TestSessionManager_Concurrency(t *testing.T) {
	sm := NewSessionManager()
	var wg sync.WaitGroup
	
	// Test concurrent session operations
	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			connID := uint32(i)
			
			// Create session
			session := sm.GetOrCreateSession(connID)
			if session == nil {
				t.Errorf("Session should not be nil for connection %d", connID)
			}
			
			// Set current connection
			sm.SetCurrentConnection(connID)
			
			// Get session
			retrievedSession, exists := sm.GetSession(connID)
			if !exists {
				t.Errorf("Session should exist for connection %d", connID)
			}
			if session != retrievedSession {
				t.Errorf("Retrieved session should match original for connection %d", connID)
			}
		}(i)
	}
	
	wg.Wait()
}
