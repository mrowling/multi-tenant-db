package mysql

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	"multitenant-db/internal/config"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/mattn/go-sqlite3"
)

// DatabaseManager manages multiple SQLite databases, one per idx
type DatabaseManager struct {
	databases     map[string]*sql.DB  // key is idx value, value is DB connection
	dbMu          sync.RWMutex
	logger        *log.Logger
	defaultConfig *config.DefaultDatabaseConfig // Optional default database configuration
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager(logger *log.Logger) *DatabaseManager {
	return NewDatabaseManagerWithConfig(logger, nil)
}

// NewDatabaseManagerWithConfig creates a new database manager with optional default database configuration
func NewDatabaseManagerWithConfig(logger *log.Logger, defaultConfig *config.DefaultDatabaseConfig) *DatabaseManager {
	dm := &DatabaseManager{
		databases:     make(map[string]*sql.DB),
		logger:        logger,
		defaultConfig: defaultConfig,
	}
	
	// Create default database
	var defaultDB *sql.DB
	var err error
	
	if defaultConfig != nil {
		// Use configured default database
		defaultDB, err = dm.createConfiguredDatabase(defaultConfig)
		if err != nil {
			logger.Printf("Failed to create configured default database, falling back to in-memory SQLite: %v", err)
			defaultDB, err = sql.Open("sqlite3", ":memory:")
		}
	} else {
		// Create default in-memory SQLite database (existing behavior)
		defaultDB, err = sql.Open("sqlite3", ":memory:")
	}
	
	if err != nil {
		logger.Fatalf("Failed to create default database: %v", err)
	}
	
	dm.databases["default"] = defaultDB
	
	// Initialize sample data in default database
	dm.initSampleData("default")
	return dm
}

// createConfiguredDatabase creates a database connection using the provided configuration
func (dm *DatabaseManager) createConfiguredDatabase(dbConfig *config.DefaultDatabaseConfig) (*sql.DB, error) {
	switch dbConfig.Type {
	case config.DatabaseTypeSQLite:
		dm.logger.Printf("Creating SQLite default database: %s", dbConfig.ConnectionString)
		return sql.Open("sqlite3", dbConfig.ConnectionString)
		
	case config.DatabaseTypeMySQL:
		dm.logger.Printf("Creating MySQL default database connection to: %s", dbConfig.MySQLHost)
		return sql.Open("mysql", dbConfig.ConnectionString)
		
	default:
		return nil, fmt.Errorf("unsupported database type: %s", dbConfig.Type)
	}
}

// GetOrCreateDatabase gets or creates a database for the specified idx
func (dm *DatabaseManager) GetOrCreateDatabase(idx string) (*sql.DB, error) {
	dm.dbMu.Lock()
	defer dm.dbMu.Unlock()
	
	// If idx is empty, use default
	if idx == "" {
		idx = "default"
	}
	
	// Check if database already exists
	if db, exists := dm.databases[idx]; exists {
		return db, nil
	}
	
	// Create new in-memory database for this idx
	db, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		return nil, fmt.Errorf("failed to create database for idx %s: %v", idx, err)
	}
	
	dm.databases[idx] = db
	dm.logger.Printf("Created new database for idx: %s", idx)
	
	// Initialize with sample data
	dm.initSampleData(idx)
	
	return db, nil
}

// GetDatabaseForSession gets the database for a specific session
func (dm *DatabaseManager) GetDatabaseForSession(session *SessionVariables) (*sql.DB, error) {
	// Get idx from session (user-defined session variable @idx)
	var idx string
	if idxVar, exists := session.GetUser("idx"); exists && idxVar != nil {
		idx = fmt.Sprintf("%v", idxVar)
	}
	
	return dm.GetOrCreateDatabase(idx)
}

// Initialize with some sample data
func (dm *DatabaseManager) initSampleData(idx string) {
	db, exists := dm.databases[idx]
	if !exists {
		dm.logger.Printf("Database for idx %s not found, cannot initialize sample data", idx)
		return
	}
	
	// Determine if this is a MySQL or SQLite database
	isMySQL := dm.isDefaultDatabase(idx) && dm.defaultConfig != nil && dm.defaultConfig.Type == config.DatabaseTypeMySQL
	
	var createUsersTable, createProductsTable, insertUsers, insertProducts string
	
	if isMySQL {
		// MySQL syntax
		createUsersTable = `
			CREATE TABLE IF NOT EXISTS users (
				id INT AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				email VARCHAR(255),
				age INT
			)`
		
		createProductsTable = `
			CREATE TABLE IF NOT EXISTS products (
				id INT AUTO_INCREMENT PRIMARY KEY,
				name VARCHAR(255) NOT NULL,
				price DECIMAL(10,2),
				category VARCHAR(255)
			)`
		
		insertUsers = `
			INSERT IGNORE INTO users (name, email, age) VALUES 
			('Alice', 'alice@example.com', 30),
			('Bob', 'bob@example.com', 25),
			('Charlie', 'charlie@example.com', 35)`
		
		insertProducts = `
			INSERT IGNORE INTO products (name, price, category) VALUES 
			('Laptop', 999.99, 'electronics'),
			('Book', 19.99, 'education'),
			('Coffee', 4.99, 'beverages')`
	} else {
		// SQLite syntax
		createUsersTable = `
			CREATE TABLE IF NOT EXISTS users (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				email TEXT,
				age INTEGER
			)`
		
		createProductsTable = `
			CREATE TABLE IF NOT EXISTS products (
				id INTEGER PRIMARY KEY AUTOINCREMENT,
				name TEXT NOT NULL,
				price REAL,
				category TEXT
			)`
		
		insertUsers = `
			INSERT OR IGNORE INTO users (name, email, age) VALUES 
			('Alice', 'alice@example.com', 30),
			('Bob', 'bob@example.com', 25),
			('Charlie', 'charlie@example.com', 35)`
		
		insertProducts = `
			INSERT OR IGNORE INTO products (name, price, category) VALUES 
			('Laptop', 999.99, 'electronics'),
			('Book', 19.99, 'education'),
			('Coffee', 4.99, 'beverages')`
	}
	
	// Create users table
	_, err := db.Exec(createUsersTable)
	if err != nil {
		dm.logger.Printf("Failed to create users table for idx %s: %v", idx, err)
		return
	}
	
	// Create products table
	_, err = db.Exec(createProductsTable)
	if err != nil {
		dm.logger.Printf("Failed to create products table for idx %s: %v", idx, err)
		return
	}
	
	// Insert sample users
	_, err = db.Exec(insertUsers)
	if err != nil {
		dm.logger.Printf("Failed to insert sample users for idx %s: %v", idx, err)
		return
	}
	
	// Insert sample products
	_, err = db.Exec(insertProducts)
	if err != nil {
		dm.logger.Printf("Failed to insert sample products for idx %s: %v", idx, err)
		return
	}
	
	dm.logger.Printf("Sample data initialized successfully for idx: %s", idx)
}

// isDefaultDatabase checks if the given idx represents the default database
func (dm *DatabaseManager) isDefaultDatabase(idx string) bool {
	return idx == "" || idx == "default"
}

// Close closes all database connections
func (dm *DatabaseManager) Close() error {
	dm.dbMu.Lock()
	defer dm.dbMu.Unlock()
	
	for idx, db := range dm.databases {
		if err := db.Close(); err != nil {
			dm.logger.Printf("Error closing database for idx %s: %v", idx, err)
		}
	}
	return nil
}

// ListDatabases returns a list of all database indices
func (dm *DatabaseManager) ListDatabases() []string {
	dm.dbMu.RLock()
	defer dm.dbMu.RUnlock()
	
	var indices []string
	for idx := range dm.databases {
		indices = append(indices, idx)
	}
	return indices
}

// GetActiveDatabases returns a map of all active databases (for SHOW DATABASES)
func (dm *DatabaseManager) GetActiveDatabases() map[string]*sql.DB {
	dm.dbMu.RLock()
	defer dm.dbMu.RUnlock()
	
	// Return a copy of the map to avoid external modification
	result := make(map[string]*sql.DB)
	for idx, db := range dm.databases {
		result[idx] = db
	}
	return result
}

// DeleteDatabase removes a database for a specific idx
func (dm *DatabaseManager) DeleteDatabase(idx string) error {
	dm.dbMu.Lock()
	defer dm.dbMu.Unlock()
	
	// Don't allow deletion of default database
	if idx == "" || idx == "default" {
		return fmt.Errorf("cannot delete default database")
	}
	
	// Check if database exists
	db, exists := dm.databases[idx]
	if !exists {
		return fmt.Errorf("database for idx %s does not exist", idx)
	}
	
	// Close the database connection
	if err := db.Close(); err != nil {
		dm.logger.Printf("Error closing database for idx %s: %v", idx, err)
	}
	
	// Remove from map
	delete(dm.databases, idx)
	dm.logger.Printf("Database deleted for idx: %s", idx)
	
	return nil
}
