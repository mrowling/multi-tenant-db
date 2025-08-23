package mysql

import (
	"database/sql"
	"fmt"
	"log"
	"sync"

	_ "github.com/mattn/go-sqlite3"
)

// DatabaseManager manages multiple SQLite databases, one per idx
type DatabaseManager struct {
	databases map[string]*sql.DB  // key is idx value, value is DB connection
	dbMu      sync.RWMutex
	logger    *log.Logger
}

// NewDatabaseManager creates a new database manager
func NewDatabaseManager(logger *log.Logger) *DatabaseManager {
	dm := &DatabaseManager{
		databases: make(map[string]*sql.DB),
		logger:    logger,
	}
	
	// Create default database (for when no idx is set)
	defaultDB, err := sql.Open("sqlite3", ":memory:")
	if err != nil {
		logger.Fatalf("Failed to create default SQLite database: %v", err)
	}
	dm.databases["default"] = defaultDB
	
	// Initialize sample data in default database
	dm.initSampleData("default")
	return dm
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
		dm.logger.Printf("Failed to create users table for idx %s: %v", idx, err)
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
		dm.logger.Printf("Failed to create products table for idx %s: %v", idx, err)
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
		dm.logger.Printf("Failed to insert sample users for idx %s: %v", idx, err)
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
		dm.logger.Printf("Failed to insert sample products for idx %s: %v", idx, err)
		return
	}
	
	dm.logger.Printf("Sample data initialized successfully for idx: %s", idx)
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
