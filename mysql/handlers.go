package mysql

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/go-mysql-org/go-mysql/mysql"
)

// QueryHandlers contains all the specific query handlers
type QueryHandlers struct {
	handler *Handler
}

// NewQueryHandlers creates a new query handlers instance
func NewQueryHandlers(handler *Handler) *QueryHandlers {
	return &QueryHandlers{
		handler: handler,
	}
}

// HandleShowTables handles SHOW TABLES command
func (qh *QueryHandlers) HandleShowTables() (*mysql.Result, error) {
	session := qh.handler.sessionManager.GetOrCreateSession(qh.handler.sessionManager.GetCurrentConnection())
	db, err := qh.handler.databaseManager.GetDatabaseForSession(session)
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

// HandleShowDatabases handles SHOW DATABASES command
func (qh *QueryHandlers) HandleShowDatabases() (*mysql.Result, error) {
	names := []string{"Database"}
	var values [][]interface{}
	
	// Always include standard MySQL databases
	values = append(values, []interface{}{"information_schema"})
	values = append(values, []interface{}{"mysql"})
	values = append(values, []interface{}{"performance_schema"})
	values = append(values, []interface{}{"sys"})
	
	// Get all active databases from the database manager
	activeDatabases := qh.handler.databaseManager.GetActiveDatabases()
	
	// Add each active database with its idx identifier
	for idx := range activeDatabases {
		var dbName string
		if idx == "" || idx == "default" {
			dbName = "ephemeral_db"
		} else {
			dbName = fmt.Sprintf("ephemeral_db_idx_%s", idx)
		}
		values = append(values, []interface{}{dbName})
	}
	
	resultset, err := mysql.BuildSimpleTextResultset(names, values)
	if err != nil {
		return nil, err
	}
	
	return mysql.NewResult(resultset), nil
}

// HandleDescribe handles DESCRIBE queries
func (qh *QueryHandlers) HandleDescribe(query string) (*mysql.Result, error) {
	session := qh.handler.sessionManager.GetOrCreateSession(qh.handler.sessionManager.GetCurrentConnection())
	db, err := qh.handler.databaseManager.GetDatabaseForSession(session)
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

// HandleSet handles SET commands for session variables and user-defined variables
func (qh *QueryHandlers) HandleSet(query string) (*mysql.Result, error) {
	// Get current session using the actual connection ID
	connID := qh.handler.sessionManager.GetCurrentConnection()
	session := qh.handler.sessionManager.GetOrCreateSession(connID)
	
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
			qh.handler.logWithIdx("Unset user variable: @%s", varName)
		} else {
			session.SetUser(varName, value)
			qh.handler.logWithIdx("Set user variable: @%s = %v", varName, value)
		}
	} else {
		if value == nil {
			session.Unset(varName)
			qh.handler.logWithIdx("Unset session variable: %s", varName)
		} else {
			session.Set(varName, value)
			qh.handler.logWithIdx("Set session variable: %s = %v", varName, value)
		}
	}
	
	// Return OK result
	result := mysql.NewResult(nil)
	result.AffectedRows = 0
	return result, nil
}

// HandleSelectVariable handles SELECT @@variable and @variable queries
func (qh *QueryHandlers) HandleSelectVariable(query string) (*mysql.Result, error) {
	connID := qh.handler.sessionManager.GetCurrentConnection()
	session := qh.handler.sessionManager.GetOrCreateSession(connID)
	
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

// HandleShowVariables handles SHOW VARIABLES command
func (qh *QueryHandlers) HandleShowVariables() (*mysql.Result, error) {
	connID := qh.handler.sessionManager.GetCurrentConnection()
	session := qh.handler.sessionManager.GetOrCreateSession(connID)
	
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
}
