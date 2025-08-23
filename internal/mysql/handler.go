package mysql

import (
	"fmt"
	"log"
	"net"
	"strings"

	"multitenant-db/internal/config"

	"github.com/go-mysql-org/go-mysql/mysql"
	"github.com/go-mysql-org/go-mysql/server"
)

// Handler represents the MySQL protocol handler
type Handler struct {
	databaseManager *DatabaseManager
	sessionManager  *SessionManager
	queryHandlers   *QueryHandlers
	logger          *log.Logger
	config          *config.Config
}

// NewHandler creates a new MySQL protocol handler
func NewHandler(logger *log.Logger) *Handler {
	return NewHandlerWithConfig(logger, nil)
}

// NewHandlerWithConfig creates a new MySQL protocol handler with configuration
func NewHandlerWithConfig(logger *log.Logger, cfg *config.Config) *Handler {
	var defaultDBConfig *config.DefaultDatabaseConfig
	if cfg != nil && cfg.DefaultDatabase != nil {
		defaultDBConfig = cfg.DefaultDatabase
	}
	
	handler := &Handler{
		databaseManager: NewDatabaseManagerWithConfig(logger, defaultDBConfig),
		sessionManager:  NewSessionManager(),
		logger:          logger,
		config:          cfg, // Store config for authentication
	}
	
	handler.queryHandlers = NewQueryHandlers(handler)
	return handler
}

// GetDatabaseManager returns the database manager (for API access)
func (h *Handler) GetDatabaseManager() *DatabaseManager {
	return h.databaseManager
}

// logWithIdx formats a log message including the "idx" user variable if set
func (h *Handler) logWithIdx(format string, args ...interface{}) {
	connID := h.sessionManager.GetCurrentConnection()
	session := h.sessionManager.GetOrCreateSession(connID)
	
	var prefix string
	// Check for user-defined session variable @idx
	if idxVar, exists := session.GetUser("idx"); exists && idxVar != nil {
		prefix = fmt.Sprintf("[idx=%v] ", idxVar)
	}
	
	message := fmt.Sprintf(format, args...)
	h.logger.Printf("%s%s", prefix, message)
}

// UseDB implements the MySQL UseDB command
func (h *Handler) UseDB(dbName string) error {
	h.logWithIdx("Client switching to database: %s", dbName)
	// Accept any database name for simplicity
	return nil
}

// HandleQuery implements the MySQL Query command
func (h *Handler) HandleQuery(query string) (*mysql.Result, error) {
	h.logWithIdx("Executing query: %s", query)
	
	// Convert query to lowercase for easier parsing
	queryLower := strings.ToLower(strings.TrimSpace(query))
	
	// Use the query handlers for MySQL-specific commands
	switch {
	case strings.HasPrefix(queryLower, "show databases"):
		return h.queryHandlers.HandleShowDatabases()
	case strings.HasPrefix(queryLower, "show tables"):
		return h.queryHandlers.HandleShowTables()
	case strings.HasPrefix(queryLower, "show variables"):
		return h.queryHandlers.HandleShowVariables()
	case strings.HasPrefix(queryLower, "describe ") || strings.HasPrefix(queryLower, "desc "):
		return h.queryHandlers.HandleDescribe(query)
	case strings.HasPrefix(queryLower, "set ") && strings.Contains(queryLower, "@"):
		return h.queryHandlers.HandleSet(query)
	case strings.Contains(queryLower, "@") && strings.HasPrefix(queryLower, "select"):
		return h.queryHandlers.HandleSelectVariable(query)
	default:
		// Let SQLite handle everything else
		return h.executeSQLiteQuery(query)
	}
}

// executeSQLiteQuery executes a query directly against SQLite and converts results to MySQL format
func (h *Handler) executeSQLiteQuery(query string) (*mysql.Result, error) {
	// Get the database for the current session
	session := h.sessionManager.GetOrCreateSession(h.sessionManager.GetCurrentConnection())
	db, err := h.databaseManager.GetDatabaseForSession(session)
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

// HandleFieldList implements field list requests
func (h *Handler) HandleFieldList(table string, wildcard string) ([]*mysql.Field, error) {
	h.logWithIdx("Field list requested for table: %s", table)	
	
	session := h.sessionManager.GetOrCreateSession(h.sessionManager.GetCurrentConnection())
	db, err := h.databaseManager.GetDatabaseForSession(session)
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
	h.logWithIdx("Prepared statement: %s", query)
	// Return statement ID, parameter count, column count, context
	return 1, 0, nil, nil
}

// HandleStmtExecute implements prepared statement execution
func (h *Handler) HandleStmtExecute(context interface{}, query string, args []interface{}) (*mysql.Result, error) {
	h.logWithIdx("Executing prepared statement with args: %v", args)
	return h.HandleQuery(query)
}

// HandleStmtClose implements prepared statement cleanup
func (h *Handler) HandleStmtClose(context interface{}) error {
	h.logWithIdx("Closing prepared statement")
	return nil
}

// HandleOtherCommand handles other MySQL commands
func (h *Handler) HandleOtherCommand(cmd byte, data []byte) error {
	h.logWithIdx("Other command received: %d", cmd)
	return mysql.NewDefaultError(mysql.ER_UNKNOWN_ERROR, "command not supported")
}

// Close closes all database connections
func (h *Handler) Close() error {
	return h.databaseManager.Close()
}

// StartServer starts the MySQL protocol server
func StartServer(port int, handler *Handler) error {
	handler.logger.Printf("Starting MySQL server on port %d", port)
	
	listener, err := net.Listen("tcp", fmt.Sprintf(":%d", port))
	if err != nil {
		return fmt.Errorf("failed to listen on port %d: %v", port, err)
	}
	defer listener.Close()
	
	handler.logger.Printf("MySQL server listening on port %d", port)
	
	for {
		conn, err := listener.Accept()
		if err != nil {
			handler.logger.Printf("Failed to accept connection: %v", err)
			continue
		}
		
		go func() {
			defer conn.Close()

			// Get authentication credentials
			username := "root"
			password := ""
			if handler.config != nil && handler.config.Auth != nil {
				username = handler.config.Auth.Username
				password = handler.config.Auth.Password
			}

			// Create new MySQL connection with authentication
			mysqlConn, err := server.NewConn(conn, username, password, handler)
			if err != nil {
				handler.logger.Printf("Failed to create MySQL connection: %v", err)
				return
			}
			defer func() {
				if mysqlConn != nil {
					defer func() {
						if r := recover(); r != nil {
							handler.logger.Printf("Recovered from panic in mysqlConn.Close(): %v", r)
						}
					}()
					mysqlConn.Close()
				}
			}()
			
			// Get connection ID and set it for this handler instance
			connID := handler.sessionManager.GetNextConnectionID()
			handler.sessionManager.SetCurrentConnection(connID)
			
			// Create initial session
			session := handler.sessionManager.GetOrCreateSession(connID)
			_ = session // Use session to avoid unused variable warning
			
			handler.logger.Printf("New MySQL client connected [conn=%d] from %s", connID, conn.RemoteAddr())
			
			// Clean up session when connection closes
			defer func() {
				// Try to get idx context before removing session
				var idxContext string
				if session := handler.sessionManager.GetOrCreateSession(connID); session != nil {
					if idxVar, hasIdx := session.GetUser("idx"); hasIdx && idxVar != nil {
						idxContext = fmt.Sprintf("[idx=%v] ", idxVar)
					}
				}
				
				handler.sessionManager.RemoveSession(connID)
				handler.logger.Printf("%sMySQL client disconnected [conn=%d]: %s", idxContext, connID, conn.RemoteAddr())
			}()
			
			// Handle the connection
			for {
				if err := mysqlConn.HandleCommand(); err != nil {
					// For connection errors, we can try to get idx context
					if session := handler.sessionManager.GetOrCreateSession(connID); session != nil {
						if idxVar, hasIdx := session.GetUser("idx"); hasIdx && idxVar != nil {
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
