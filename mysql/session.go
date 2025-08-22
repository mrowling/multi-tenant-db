package mysql

import (
	"strings"
	"sync"
)

// SessionVariables holds session-specific variables
type SessionVariables struct {
	sessionVars map[string]interface{} // @@variables
	userVars    map[string]interface{} // @variables
	mu          sync.RWMutex
}

// NewSessionVariables creates a new session variables instance with defaults
func NewSessionVariables() *SessionVariables {
	sv := &SessionVariables{
		sessionVars: make(map[string]interface{}),
		userVars:    make(map[string]interface{}),
	}
	
	// Set default session variables
	sv.sessionVars["autocommit"] = 1
	sv.sessionVars["sql_mode"] = ""
	sv.sessionVars["character_set_client"] = "utf8mb4"
	sv.sessionVars["character_set_connection"] = "utf8mb4"
	sv.sessionVars["character_set_results"] = "utf8mb4"
	sv.sessionVars["collation_connection"] = "utf8mb4_general_ci"
	sv.sessionVars["time_zone"] = "SYSTEM"
	sv.sessionVars["tx_isolation"] = "REPEATABLE-READ"
	sv.sessionVars["version_comment"] = "Multitenant DB"
	
	return sv
}

// Set sets a session variable
func (sv *SessionVariables) Set(name string, value interface{}) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.sessionVars[strings.ToLower(name)] = value
}

// SetUser sets a user-defined variable
func (sv *SessionVariables) SetUser(name string, value interface{}) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.userVars[strings.ToLower(name)] = value
}

// Get gets a session variable
func (sv *SessionVariables) Get(name string) (interface{}, bool) {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	val, exists := sv.sessionVars[strings.ToLower(name)]
	return val, exists
}

// GetUser gets a user-defined variable
func (sv *SessionVariables) GetUser(name string) (interface{}, bool) {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	val, exists := sv.userVars[strings.ToLower(name)]
	return val, exists
}

// UnsetUser removes a user-defined variable
func (sv *SessionVariables) UnsetUser(name string) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	delete(sv.userVars, strings.ToLower(name))
}

// Unset removes a session variable
func (sv *SessionVariables) Unset(name string) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	delete(sv.sessionVars, strings.ToLower(name))
}

// GetAll returns all session variables
func (sv *SessionVariables) GetAll() map[string]interface{} {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	
	result := make(map[string]interface{})
	for k, v := range sv.sessionVars {
		result[k] = v
	}
	return result
}

// SessionManager manages sessions for connections
type SessionManager struct {
	sessions          map[uint32]*SessionVariables
	sessionMu         sync.RWMutex
	connectionCounter uint32
	connCounterMu     sync.Mutex
	
	// Current connection tracking
	currentConnMu sync.RWMutex
	currentConnID uint32
}

// NewSessionManager creates a new session manager
func NewSessionManager() *SessionManager {
	return &SessionManager{
		sessions: make(map[uint32]*SessionVariables),
	}
}

// GetOrCreateSession gets or creates a session for a connection
func (sm *SessionManager) GetOrCreateSession(connID uint32) *SessionVariables {
	sm.sessionMu.Lock()
	defer sm.sessionMu.Unlock()
	
	if session, exists := sm.sessions[connID]; exists {
		return session
	}
	
	session := NewSessionVariables()
	sm.sessions[connID] = session
	return session
}

// RemoveSession removes a session when connection closes
func (sm *SessionManager) RemoveSession(connID uint32) {
	sm.sessionMu.Lock()
	defer sm.sessionMu.Unlock()
	delete(sm.sessions, connID)
}

// GetNextConnectionID generates a unique connection ID
func (sm *SessionManager) GetNextConnectionID() uint32 {
	sm.connCounterMu.Lock()
	defer sm.connCounterMu.Unlock()
	sm.connectionCounter++
	return sm.connectionCounter
}

// SetCurrentConnection sets the current connection ID
func (sm *SessionManager) SetCurrentConnection(connID uint32) {
	sm.currentConnMu.Lock()
	defer sm.currentConnMu.Unlock()
	sm.currentConnID = connID
}

// GetCurrentConnection gets the current connection ID
func (sm *SessionManager) GetCurrentConnection() uint32 {
	sm.currentConnMu.RLock()
	defer sm.currentConnMu.RUnlock()
	return sm.currentConnID
}

// GetSession gets a session by connection ID
func (sm *SessionManager) GetSession(connID uint32) (*SessionVariables, bool) {
	sm.sessionMu.RLock()
	defer sm.sessionMu.RUnlock()
	session, exists := sm.sessions[connID]
	return session, exists
}
