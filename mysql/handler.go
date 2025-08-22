package mysql

import (
	"database/sql"
	"fmt"
	"log"
	"net"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/server"
	_ "github.com/mattn/go-sqlite3"
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
	sv.sessionVars["version_comment"] = "Ephemeral DB"
	
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

// logWithApp formats a log message including the "idx" session variable if set
func (h *Handler) logWithApp(format string, args ...interface{}) {
	connID := h.GetCurrentConnection()
	session := h.GetOrCreateSession(connID)
	
	var prefix string
	// Check for user variable @idx first
	if idxVar, exists := session.GetUser("idx"); exists && idxVar != nil {
		prefix = fmt.Sprintf("[idx=%v] ", idxVar)
	} else if idxVar, exists := session.Get("idx"); exists && idxVar != nil {
		// Fall back to session variable @@idx
		prefix = fmt.Sprintf("[idx=%v] ", idxVar)
	}
	
	message := fmt.Sprintf(format, args...)
	h.logger.Printf("%s%s", prefix, message)
}

// Handler represents the MySQL protocol handler
type Handler struct {
	// SQLite database connections - one per idx
	databases map[string]*sql.DB  // key is idx value, value is DB connection
	dbMu      sync.RWMutex
	logger    *log.Logger
	
	// Session variables per connection
	sessions map[uint32]*SessionVariables
	sessionMu sync.RWMutex
	connectionCounter uint32
	connCounterMu sync.Mutex
	
	// Current connection tracking
	currentConnMu sync.RWMutex
	currentConnID uint32
}

// NewHandler creates a new MySQL protocol handler
func NewHandler(logger *log.Logger) *Handler {
	handler := &Handler{
		databases: make(map[string]*sql.DB),
		logger:    logger,
		sessions:  make(map[uint32]*SessionVariables),
	}
	
	// Create default database (for when no idx is set)
	defaultDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		logger.Fatalf("Failed to create default SQLite database: %v", err)
	}
	handler.databases["default"] = defaultDB
	
	// Initialize sample data in default database
	handler.initSampleData("default")
	return handler
}

// GetOrCreateSession gets or creates a session for a connection
func (h *Handler) GetOrCreateSession(connID uint32) *SessionVariables {
	h.sessionMu.Lock()
	defer h.sessionMu.Unlock()
	
	if session, exists := h.sessions[connID]; exists {
		return session
	}
	
	session := NewSessionVariables()
	h.sessions[connID] = session
	return session
}

// RemoveSession removes a session when connection closes
func (h *Handler) RemoveSession(connID uint32) {
	h.sessionMu.Lock()
	defer h.sessionMu.Unlock()
	delete(h.sessions, connID)
}

// GetOrCreateDatabase gets or creates a database for the specified idx
func (h *Handler) GetOrCreateDatabase(idx string) (*sql.DB, error) {
	h.dbMu.Lock()
	defer h.dbMu.Unlock()
	
	// If idx is empty, use default
	if idx == "" {
		idx = "default"
	}
	
	// Check if database already exists
	if db, exists := h.databases[idx]; exists {
		return db, nil
	}
	
	// Create new in-memory database for this idx
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to create database for idx %s: %v", idx, err)
	}
	
	h.databases[idx] = db
	h.logger.Printf("Created new database for idx: %s", idx)
	
	// Initialize with sample data
	h.initSampleData(idx)
	
	return db, nil
}

// GetCurrentDatabase gets the database for the current connection's idx
func (h *Handler) GetCurrentDatabase() (*sql.DB, error) {
	connID := h.GetCurrentConnection()
	session := h.GetOrCreateSession(connID)
	
	// Get idx from session (try user variable first, then session variable)
	var idx string
	if idxVar, exists := session.GetUser("idx"); exists && idxVar != nil {
		idx = fmt.Sprintf("%v", idxVar)
	} else if idxVar, exists := session.Get("idx"); exists && idxVar != nil {
		idx = fmt.Sprintf("%v", idxVar)
	}
	
	return h.GetOrCreateDatabase(idx)
}
func (h *Handler) GetNextConnectionID() uint32 {
	h.connCounterMu.Lock()
	defer h.connCounterMu.Unlock()
	h.connectionCounter++
	return h.connectionCounter
}

// SetCurrentConnection sets the current connection ID for this handler instance
func (h *Handler) SetCurrentConnection(connID uint32) {
	h.currentConnMu.Lock()
	defer h.currentConnMu.Unlock()
	h.currentConnID = connID
}

// GetCurrentConnection gets the current connection ID
func (h *Handler) GetCurrentConnection() uint32 {
	h.currentConnMu.RLock()
	defer h.currentConnMu.RUnlock()
	return h.currentConnID
}

// Initialize with some sample data
func (h *Handler) initSampleData(idx string) {
	db, exists := h.databases[idx]
	if !exists {
		h.logger.Printf("Database for idx %s not found, cannot initialize sample data", idx)
		return
	}
	
	// Create users table
	_, err := db.Exec(`
		CREATE TABLE users (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			email TEXT,
			age INTEGER
		)
	`)
	if err != nil {
		h.logger.Printf("Failed to create users table for idx %s: %v", idx, err)
		return
	}
	
	// Create products table
	_, err = db.Exec(`
		CREATE TABLE products (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			price REAL,
			category TEXT
		)
	`)
	if err != nil {
		h.logger.Printf("Failed to create products table for idx %s: %v", idx, err)
		return
	}
	
	// Insert sample users
	_, err = db.Exec(`
		INSERT INTO users (name, email, age) VALUES 
		('Alice', 'alice@example.com', 30),
		('Bob', 'bob@example.com', 25),
		('Charlie', 'charlie@example.com', 35)
	`)
	if err != nil {
		h.logger.Printf("Failed to insert sample users for idx %s: %v", idx, err)
		return
	}
	
	// Insert sample products
	_, err = db.Exec(`
		INSERT INTO products (name, price, category) VALUES 
		('Laptop', 999.99, 'electronics'),
		('Book', 19.99, 'education'),
		('Coffee', 4.99, 'beverages')
	`)
	if err != nil {
		h.logger.Printf("Failed to insert sample products for idx %s: %v", idx, err)
		return
	}
	
	h.logger.Printf("Sample data initialized successfully for idx: %s", idx)
}

// UseDB implements the MySQL UseDB command
func (h *Handler) UseDB(dbName string) error {
	h.logWithApp("Client switching to database: %s", dbName)
	// Accept any database name for simplicity
	return nil
}

// HandleQuery implements the MySQL Query command
func (h *Handler) HandleQuery(query string) (*mysql.Result, error) {
	h.logWithApp("Executing query: %s", query)
	
	// Convert query to lowercase for easier parsing
	queryLower := strings.ToLower(strings.TrimSpace(query))
	
	// Only intercept the absolute minimum MySQL-specific commands
	// that SQLite genuinely cannot handle
	switch {
	case strings.HasPrefix(queryLower, "show databases"):
		return h.handleShowDatabases()
	case strings.HasPrefix(queryLower, "show variables"):
		return h.handleShowVariables()
	case strings.HasPrefix(queryLower, "set ") && (strings.Contains(queryLower, "@") || strings.Contains(queryLower, "@@")):
		return h.handleSet(query)
	case strings.Contains(queryLower, "@@") || (strings.Contains(queryLower, "@") && strings.HasPrefix(queryLower, "select")):
		return h.handleSelectVariable(query)
	default:
		// Let SQLite handle everything else - including SHOW TABLES, DESCRIBE, etc.
		// SQLite has its own ways to handle these via system tables
		return h.executeSQLiteQuery(query)
	}
}

// Handle SHOW TABLES command
func (h *Handler) handleShowTables() (*mysql.Result, error) {
	db, err := h.GetCurrentDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %v", err)
	}
	
	rows, err := db.Query("SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%'")
	if err != nil {
		return nil, fmt.Errorf("failed to get tables: %v", err)
	}
	defer rows.Close()
	
	names := []string{"Tables_in_ephemeral_db"}
	var values [][]interface{}
	
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, fmt.Errorf("failed to scan table name: %v", err)
		}
		values = append(values, []interface{}{tableName})
	}
	
	resultset, err := mysql.BuildSimpleTextResultset(names, values)
	if err != nil {
		return nil, err
	}
	
	return mysql.NewResult(resultset), nil
}

// Handle SHOW DATABASES command
func (h *Handler) handleShowDatabases() (*mysql.Result, error) {
	names := []string{"Database"}
	values := [][]interface{}{
		{"ephemeral_db"},
		{"information_schema"},
	}
	
	resultset, err := mysql.BuildSimpleTextResultset(names, values)
	if err != nil {
		return nil, err
	}
	
	return mysql.NewResult(resultset), nil
}

// Handle DESCRIBE queries
func (h *Handler) handleDescribe(query string) (*mysql.Result, error) {
	db, err := h.GetCurrentDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %v", err)
	}
	
	queryLower := strings.ToLower(query)
	
	// Extract table name from DESCRIBE statement
	var tableName string
	if strings.Contains(queryLower, "users") {
		tableName = "users"
	} else if strings.Contains(queryLower, "products") {
		tableName = "products"
	} else {
		// Try to extract table name more generically
		parts := strings.Fields(queryLower)
		if len(parts) >= 2 {
			tableName = parts[1]
		} else {
			return nil, fmt.Errorf("could not determine table name from query")
		}
	}
	
	// Get table schema from SQLite
	rows, err := db.Query("PRAGMA table_info(" + tableName + ")")
	if err != nil {
		return nil, fmt.Errorf("table %s not found or error getting schema: %v", tableName, err)
	}
	defer rows.Close()
	
	names := []string{"Field", "Type", "Null", "Key", "Default", "Extra"}
	var values [][]interface{}
	
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull bool
		var defaultValue interface{}
		var pk bool
		
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("failed to scan column info: %v", err)
		}
		
		// Convert SQLite types to MySQL-like types
		var mysqlType string
		switch strings.ToLower(dataType) {
		case "integer":
			mysqlType = "int(11)"
		case "text":
			mysqlType = "varchar(255)"
		case "real":
			mysqlType = "decimal(10,2)"
		default:
			mysqlType = dataType
		}
		
		nullStr := "YES"
		if notNull {
			nullStr = "NO"
		}
		
		keyStr := ""
		if pk {
			keyStr = "PRI"
		}
		
		extraStr := ""
		if pk && strings.ToLower(dataType) == "integer" {
			extraStr = "auto_increment"
		}
		
		values = append(values, []interface{}{
			name, mysqlType, nullStr, keyStr, defaultValue, extraStr,
		})
	}
	
	if len(values) == 0 {
		return nil, fmt.Errorf("table %s not found", tableName)
	}

	resultset, err := mysql.BuildSimpleTextResultset(names, values)
	if err != nil {
		return nil, err
	}

	return mysql.NewResult(resultset), nil
}

// Handle SET commands for session variables and user-defined variables
func (h *Handler) handleSet(query string) (*mysql.Result, error) {
	// Get current session using the actual connection ID
	connID := h.GetCurrentConnection()
	session := h.GetOrCreateSession(connID)
	
	// Parse SET statement - support both session variables and user-defined variables
	// Patterns to match:
	// SET @@variable = value
	// SET @variable = value  
	// SET variable = value
	// SET @variable := value
	setRegex := regexp.MustCompile(`(?i)set\s+(?:session\s+)?(@{0,2})(\w+)\s*(:?=)\s*(.+)`)
	matches := setRegex.FindStringSubmatch(query)
	
	if len(matches) != 5 {
		return nil, fmt.Errorf("invalid SET syntax: %s", query)
	}
	
	prefix := matches[1]
	varName := strings.ToLower(matches[2])
	varValue := strings.Trim(matches[4], "\"'`")
	
	// Convert value based on variable type
	var value interface{}
	if strings.ToLower(varValue) == "null" {
		value = nil
	} else if strings.ToLower(varValue) == "true" || varValue == "1" {
		value = 1
	} else if strings.ToLower(varValue) == "false" || varValue == "0" {
		value = 0
	} else if intVal, err := strconv.Atoi(varValue); err == nil {
		value = intVal
	} else {
		value = varValue
	}
	
	// Determine if it's a user variable (@) or session variable (@@)
	if prefix == "@" {
		if value == nil {
			session.UnsetUser(varName)
			h.logWithApp("Unset user variable: @%s", varName)
		} else {
			session.SetUser(varName, value)
			h.logWithApp("Set user variable: @%s = %v", varName, value)
		}
	} else {
		if value == nil {
			session.Unset(varName)
			h.logWithApp("Unset session variable: %s", varName)
		} else {
			session.Set(varName, value)
			h.logWithApp("Set session variable: %s = %v", varName, value)
		}
	}
	
	// Return OK result
	result := mysql.NewResult(nil)
	result.AffectedRows = 0
	return result, nil
}

// Handle SELECT @@variable and @variable queries
func (h *Handler) handleSelectVariable(query string) (*mysql.Result, error) {
	connID := h.GetCurrentConnection()
	session := h.GetOrCreateSession(connID)
	
	// Parse variable reference - support both session variables (@@) and user variables (@)
	varRegex := regexp.MustCompile(`(@{1,2})(?:session\.)?(\w+)`)
	matches := varRegex.FindAllStringSubmatch(query, -1)
	
	if len(matches) == 0 {
		return nil, fmt.Errorf("no variables found in query: %s", query)
	}
	
	var names []string
	var values [][]interface{}
	
	// Handle single variable
	if len(matches) == 1 {
		prefix := matches[0][1]
		varName := strings.ToLower(matches[0][2])
		
		var value interface{}
		var exists bool
		
		if prefix == "@" {
			// User-defined variable
			value, exists = session.GetUser(varName)
			if !exists {
				value = nil // MySQL returns NULL for undefined user variables
			}
			names = []string{"@" + varName}
		} else {
			// Session variable
			value, exists = session.Get(varName)
			if !exists {
				return nil, fmt.Errorf("unknown session variable: %s", varName)
			}
			names = []string{"@@" + varName}
		}
		
		values = [][]interface{}{{value}}
	} else {
		// Handle multiple variables
		row := make([]interface{}, len(matches))
		for i, match := range matches {
			prefix := match[1]
			varName := strings.ToLower(match[2])
			
			var value interface{}
			if prefix == "@" {
				// User-defined variable
				value, _ = session.GetUser(varName)
				if value == nil {
					value = nil // MySQL returns NULL for undefined user variables
				}
				names = append(names, "@"+varName)
			} else {
				// Session variable
				value, _ = session.Get(varName)
				names = append(names, "@@"+varName)
			}
			
			row[i] = value
		}
		values = [][]interface{}{row}
	}
	
	resultset, err := mysql.BuildSimpleTextResultset(names, values)
	if err != nil {
		return nil, err
	}
	
	return mysql.NewResult(resultset), nil
}

// Handle SHOW VARIABLES command
func (h *Handler) handleShowVariables() (*mysql.Result, error) {
	connID := h.GetCurrentConnection()
	session := h.GetOrCreateSession(connID)
	
	names := []string{"Variable_name", "Value"}
	var values [][]interface{}
	
	allVars := session.GetAll()
	for varName, varValue := range allVars {
		values = append(values, []interface{}{varName, varValue})
	}
	
	resultset, err := mysql.BuildSimpleTextResultset(names, values)
	if err != nil {
		return nil, err
	}
	
	return mysql.NewResult(resultset), nil
}// HandleFieldList implements field list requests
func (h *Handler) HandleFieldList(table string, wildcard string) ([]*mysql.Field, error) {
	h.logWithApp("Field list requested for table: %s", table)	
	
	db, err := h.GetCurrentDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %v", err)
	}
	
	// Get table schema from SQLite
	rows, err := db.Query("PRAGMA table_info(" + table + ")")
	if err != nil {
		return nil, fmt.Errorf("table %s not found: %v", table, err)
	}
	defer rows.Close()
	
	var fields []*mysql.Field
	
	for rows.Next() {
		var cid int
		var name, dataType string
		var notNull bool
		var defaultValue interface{}
		var pk bool
		
		if err := rows.Scan(&cid, &name, &dataType, &notNull, &defaultValue, &pk); err != nil {
			return nil, fmt.Errorf("failed to scan column info: %v", err)
		}
		
		// Convert SQLite types to MySQL field types
		var fieldType byte
		switch strings.ToLower(dataType) {
		case "integer":
			fieldType = mysql.MYSQL_TYPE_LONG
		case "text":
			fieldType = mysql.MYSQL_TYPE_VAR_STRING
		case "real":
			fieldType = mysql.MYSQL_TYPE_DECIMAL
		default:
			fieldType = mysql.MYSQL_TYPE_VAR_STRING
		}
		
		fields = append(fields, &mysql.Field{
			Name: []byte(name),
			Type: fieldType,
		})
	}
	
	if len(fields) == 0 {
		return nil, fmt.Errorf("table %s not found or has no columns", table)
	}
	
	return fields, nil
}

// HandleStmtPrepare implements prepared statement preparation
func (h *Handler) HandleStmtPrepare(query string) (int, int, interface{}, error) {
	h.logWithApp("Prepared statement: %s", query)
	// Return statement ID, parameter count, column count, context
	return 1, 0, nil, nil
}

// HandleStmtExecute implements prepared statement execution
func (h *Handler) HandleStmtExecute(context interface{}, query string, args []interface{}) (*mysql.Result, error) {
	h.logWithApp("Executing prepared statement with args: %v", args)
	return h.HandleQuery(query)
}

// HandleStmtClose implements prepared statement cleanup
func (h *Handler) HandleStmtClose(context interface{}) error {
	h.logWithApp("Closing prepared statement")
	return nil
}

// HandleOtherCommand handles other MySQL commands
func (h *Handler) HandleOtherCommand(cmd byte, data []byte) error {
	h.logWithApp("Other command received: %d", cmd)
	return mysql.NewDefaultError(mysql.ER_UNKNOWN_ERROR, "command not supported")
}

// Close closes all database connections
func (h *Handler) Close() error {
	h.dbMu.Lock()
	defer h.dbMu.Unlock()
	
	for idx, db := range h.databases {
		if err := db.Close(); err != nil {
			h.logger.Printf("Error closing database for idx %s: %v", idx, err)
		}
	}
	return nil
}

// executeSQLiteQuery executes a query directly against SQLite and converts results to MySQL format
func (h *Handler) executeSQLiteQuery(query string) (*mysql.Result, error) {
	// Get the database for the current idx
	db, err := h.GetCurrentDatabase()
	if err != nil {
		return nil, fmt.Errorf("failed to get database: %v", err)
	}
	
	// First try as a query (SELECT, WITH, etc.) - anything that returns rows
	rows, err := db.Query(query)
	if err == nil {
		defer rows.Close()
		
		// Get column names
		columns, err := rows.Columns()
		if err != nil {
			return nil, fmt.Errorf("failed to get columns: %v", err)
		}
		
		// Prepare result data
		var values [][]interface{}
		
		for rows.Next() {
			// Create a slice of interface{} to hold each column value
			columnValues := make([]interface{}, len(columns))
			columnPointers := make([]interface{}, len(columns))
			
			for i := range columnValues {
				columnPointers[i] = &columnValues[i]
			}
			
			if err := rows.Scan(columnPointers...); err != nil {
				return nil, fmt.Errorf("failed to scan row: %v", err)
			}
			
			// Convert []byte to string for text columns
			row := make([]interface{}, len(columns))
			for i, val := range columnValues {
				if b, ok := val.([]byte); ok {
					row[i] = string(b)
				} else {
					row[i] = val
				}
			}
			
			values = append(values, row)
		}
		
		if err = rows.Err(); err != nil {
			return nil, fmt.Errorf("rows iteration error: %v", err)
		}
		
		// Build MySQL result
		resultset, err := mysql.BuildSimpleTextResultset(columns, values)
		if err != nil {
			return nil, fmt.Errorf("failed to build resultset: %v", err)
		}
		
		return mysql.NewResult(resultset), nil
	}
	
	// If Query() failed, try as Exec() - for INSERT, UPDATE, DELETE, DDL, etc.
	result, err := db.Exec(query)
	if err != nil {
		return nil, fmt.Errorf("SQLite error: %v", err)
	}
	
	mysqlResult := mysql.NewResult(nil)
	
	// Get affected rows
	if affected, err := result.RowsAffected(); err == nil {
		mysqlResult.AffectedRows = uint64(affected)
	}
	
	// Get last insert ID (useful for INSERT statements)
	if lastID, err := result.LastInsertId(); err == nil && lastID > 0 {
		mysqlResult.InsertId = uint64(lastID)
	}
	
	return mysqlResult, nil
}

// StartServer starts the MySQL protocol server
func StartServer(handler *Handler) {
	// Create server
	l, err := net.Listen("tcp", ":3306")
	if err != nil {
		handler.logger.Fatalf("Failed to listen on MySQL port 3306: %v", err)
	}
	
	handler.logger.Printf("MySQL protocol server starting on port 3306")
	
	for {
		conn, err := l.Accept()
		if err != nil {
			handler.logger.Printf("Failed to accept connection: %v", err)
			continue
		}
		
		// Handle each connection in a goroutine
		go func() {
			defer conn.Close()
			
			// Generate unique connection ID
			connID := handler.GetNextConnectionID()
			
			// Set current connection ID in handler
			handler.SetCurrentConnection(connID)
			
			// Create MySQL connection directly with the handler
			mysqlConn, err := server.NewConn(conn, "root", "", handler)
			if err != nil {
				handler.logger.Printf("Failed to create MySQL connection: %v", err)
				return
			}
			
			handler.logger.Printf("New MySQL client connected [conn=%d] from %s", connID, conn.RemoteAddr())
			
			// Clean up session when connection closes
			defer func() {
				// Try to get idx context before removing session
				var idxContext string
				if session, exists := handler.sessions[connID]; exists {
					if idxVar, hasIdx := session.GetUser("idx"); hasIdx && idxVar != nil {
						idxContext = fmt.Sprintf("[idx=%v] ", idxVar)
					} else if idxVar, hasIdx := session.Get("idx"); hasIdx && idxVar != nil {
						idxContext = fmt.Sprintf("[idx=%v] ", idxVar)
					}
				}
				
				handler.RemoveSession(connID)
				handler.logger.Printf("%sMySQL client disconnected [conn=%d]: %s", idxContext, connID, conn.RemoteAddr())
			}()
			
			// Handle the connection
			for {
				if err := mysqlConn.HandleCommand(); err != nil {
					// For connection errors, we can try to get idx context
					if session, exists := handler.sessions[connID]; exists {
						if idxVar, hasIdx := session.GetUser("idx"); hasIdx && idxVar != nil {
							handler.logger.Printf("[idx=%v] MySQL connection error [conn=%d]: %v", idxVar, connID, err)
						} else if idxVar, hasIdx := session.Get("idx"); hasIdx && idxVar != nil {
							handler.logger.Printf("[idx=%v] MySQL connection error [conn=%d]: %v", idxVar, connID, err)
						} else {
							handler.logger.Printf("MySQL connection error [conn=%d]: %v", connID, err)
						}
					} else {
						handler.logger.Printf("MySQL connection error [conn=%d]: %v", connID, err)
					}
					break
				}
			}
		}()
	}
}
