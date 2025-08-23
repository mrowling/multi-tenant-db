# Multitenant DB - Project Structure

The Multitenant DB application is built with a modular architecture providing multi-tenant database isolation using per-idx SQLite databases with MySQL protocol compatibility.

## Directory Structure

```
multitenant-db/
├── main.go                  # Main application entry point
├── go.mod                   # Go module definition (multitenant-db)
├── go.sum                   # Dependency checksums
├── LICENSE                  # MIT License
├── README.md                # Main documentation
├── STRUCTURE.md             # This file - architecture overview
├── logs/                    # Application logs
│   └── app.log             # Persistent log file
├── api/                     # HTTP REST API package
│   └── handler.go          # HTTP handlers and database management endpoints
├── mysql/                   # MySQL protocol implementation package
│   ├── handler.go          # Main MySQL protocol handler and server
│   ├── session.go          # Session and variable management
│   ├── database.go         # Multi-tenant database manager
│   └── handlers.go         # Specific MySQL query handlers
└── logger/                  # Shared logging package
    └── logger.go           # Logger setup and configuration
```

## Architecture Overview

### Multi-Tenant Data Isolation
```
┌─────────────────────────────────────────────────────────────┐
│                    Multitenant DB Server                     │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────────┐  │
│  │ HTTP API    │  │ MySQL Proto │  │   Session Manager    │  │
│  │ Port 8080   │  │ Port 3306   │  │ ┌──────────────────┐ │  │
│  │             │  │             │  │ │ Conn1: idx=prod  │ │  │
│  │ - Database  │  │ - Wire Proto│  │ │ Conn2: idx=dev   │ │  │
│  │   Management│  │ - Session   │  │ │ Conn3: idx=test  │ │  │
│  │ - Monitoring│  │   Handling  │  │ └──────────────────┘ │  │
│  └─────────────┘  └─────────────┘  └──────────────────────┘  │
│                           │                                  │
│  ┌─────────────────────────────────────────────────────────┐ │
│  │              Database Manager                            │ │
│  │ ┌─────────────┐ ┌─────────────┐ ┌─────────────────────┐ │ │
│  │ │SQLite DB    │ │SQLite DB    │ │SQLite DB            │ │ │
│  │ │idx=default  │ │idx=prod     │ │idx=dev              │ │ │
│  │ │             │ │             │ │                     │ │ │
│  │ │users        │ │users        │ │users                │ │ │
│  │ │products     │ │products     │ │products             │ │ │
│  │ └─────────────┘ └─────────────┘ └─────────────────────┘ │ │
│  └─────────────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────────────┘
```

## Package Responsibilities

### `main.go` - Application Bootstrap
- Application entry point and configuration
- Database manager adapter for API integration
- Concurrent server startup (HTTP + MySQL)
- Graceful shutdown handling

### `api/` Package - HTTP REST API
- **Database Management Endpoints**:
  - `GET /api/databases` - List all tenant databases
  - `POST /api/databases/create` - Create new tenant database
  - `DELETE /api/databases/delete?idx=<id>` - Delete tenant database
- **Monitoring Endpoints**:
  - `GET /health` - Health check
  - `GET /api/info` - API information and connection details
- **JSON Response Handling**: Structured responses with timestamps and status

### `mysql/` Package - MySQL Protocol Implementation

#### `handler.go` - Main Protocol Handler
- **MySQL Wire Protocol**: Server implementation using go-mysql library
- **Connection Management**: Per-connection session tracking with unique IDs
- **Query Routing**: Directs queries to appropriate handlers based on type
- **Session Integration**: Links with SessionManager and DatabaseManager
- **Logging**: Context-aware logging with tenant idx prefixes

#### `session.go` - Session & Variable Management
- **SessionVariables**: Per-session state management
  - User variables (`@var`) - connection-scoped
  - Session variables (`@@var`) - session-scoped
  - Get/Set/Unset operations
- **SessionManager**: Multi-connection session orchestration
  - Connection ID generation and tracking
  - Session lifecycle management
  - Thread-safe session operations

#### `database.go` - Multi-Tenant Database Manager
- **DatabaseManager**: Per-tenant SQLite database management
  - Dynamic database creation based on idx values
  - Thread-safe database access with RWMutex
  - Automatic sample data initialization
  - Database lifecycle (create, access, delete)
- **Tenant Isolation**: Complete data separation per idx
- **Session Integration**: Automatic database selection based on session variables

#### `handlers.go` - MySQL Query Handlers
- **Query-Specific Handlers**:
  - `HandleShowDatabases()` - Lists all tenant databases
  - `HandleShowTables()` - Shows tables in current tenant database
  - `HandleDescribe()` - Table schema information
  - `HandleSet()` - Variable assignment (user and session variables)
  - `HandleSelectVariable()` - Variable retrieval
  - `HandleShowVariables()` - List all session variables

### `logger/` Package - Centralized Logging
- **Multi-Output Logging**: Console + file output
- **Structured Logging**: Timestamps, log levels, source file information
- **Application Prefix**: `[MULTITENANT-DB]` for easy log filtering

## Data Flow

### 1. MySQL Client Connection
```
Client → MySQL Protocol (Port 3306) → Handler.StartServer() → 
Session Creation → Connection ID Assignment → Ready for Queries
```

### 2. Tenant Context Setting
```
SET @idx = 'customer123' → SessionManager.SetUser() → 
Context Stored in Session → All Subsequent Queries Use This Context
```

### 3. Query Execution with Tenant Routing
```
SELECT * FROM users → Handler.HandleQuery() → 
Session Lookup (idx=customer123) → DatabaseManager.GetDatabaseForSession() → 
SQLite Query Execution → MySQL Result Format → Client Response
```

### 4. Database Management via API
```
POST /api/databases/create → DatabaseManager.GetOrCreateDatabase() → 
New SQLite Database Created → Sample Data Initialized → 
Available for MySQL Queries
```

## Concurrency & Thread Safety

### Thread-Safe Components
- **SessionManager**: Mutex protection for session map operations
- **DatabaseManager**: RWMutex for database map access (multiple readers, single writer)
- **MySQL Connections**: Each connection handled in separate goroutine

### Isolation Guarantees
- **Session Isolation**: Each MySQL connection has independent session state
- **Data Isolation**: Complete tenant data separation via separate SQLite databases
- **Variable Isolation**: Session and user variables scoped per connection

## Key Design Patterns

### 1. **Dependency Injection**
- Handler receives DatabaseManager and SessionManager instances
- QueryHandlers receive Handler reference for access to managers

### 2. **Adapter Pattern**
- DatabaseManagerAdapter bridges MySQL Handler and HTTP API interfaces
- Allows API to manage databases without direct MySQL package dependencies

### 3. **Factory Pattern**
- DatabaseManager creates SQLite databases on-demand
- SessionManager creates sessions per connection

### 4. **Strategy Pattern**
- Query routing based on query type and content
- Different handlers for different MySQL command types

## Extension Points

### Adding New MySQL Commands
1. Add handler method in `handlers.go`
2. Update query routing in `handler.go`
3. Implement MySQL protocol compliance

### Adding New API Endpoints
1. Add handler method in `api/handler.go`
2. Register route in `SetupRoutes()`
3. Define request/response structures

### Custom Business Logic
1. Extend query handlers with validation
2. Add tenant-specific business rules
3. Implement custom data transformations
