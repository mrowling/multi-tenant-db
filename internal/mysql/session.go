package mysql

import (
	"strings"
	"sync"
)

// SessionVariables holds session-specific variables
type SessionVariables struct {
	userVars map[string]interface{} // @variables (user-defined session variables)
	mu       sync.RWMutex
}

// NewSessionVariables creates a new session variables instance
func NewSessionVariables() *SessionVariables {
	return &SessionVariables{
		userVars: make(map[string]interface{}),
	}
}

// SetUser sets a user-defined variable
func (sv *SessionVariables) SetUser(name string, value interface{}) {
	sv.mu.Lock()
	defer sv.mu.Unlock()
	sv.userVars[strings.ToLower(name)] = value
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

// GetAllUser returns all user-defined variables
func (sv *SessionVariables) GetAllUser() map[string]interface{} {
	sv.mu.RLock()
	defer sv.mu.RUnlock()
	
	result := make(map[string]interface{})
	for k, v := range sv.userVars {
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
