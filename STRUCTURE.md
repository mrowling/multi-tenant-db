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

## Security Considerations

### Current Implementation (Development)
- No authentication required
- All tenants accessible
- No input validation beyond basic SQL parsing

### Production Recommendations
- Add JWT or API key authentication
- Implement tenant access controls
- Add SQL injection prevention
- Rate limiting per tenant/connection
- Audit logging for all operations
- JSON response handling
- Request/response middleware
- API routing and handlers

**Key Types:**
- `Handler`: Main API handler struct
- `Response`: Standard JSON response structure

**Key Functions:**
- `NewHandler()`: Creates API handler
- `SetupRoutes()`: Configures HTTP routes
- `LoggingMiddleware()`: HTTP request logging
- `RootHandler()`, `HealthHandler()`, `InfoHandler()`: Endpoint handlers

### `mysql/` Package
- MySQL wire protocol implementation
- SQL query parsing and handling
- In-memory data storage
- MySQL client connection management

**Key Types:**
- `Handler`: MySQL protocol handler struct

**Key Functions:**
- `NewHandler()`: Creates MySQL handler
- `StartServer()`: Starts MySQL protocol server
- `HandleQuery()`: Processes SQL queries
- Various `handle*()` methods for specific SQL commands

### `logger/` Package
- Centralized logging configuration
- File and console output setup
- Shared logger instance creation

**Key Functions:**
- `Setup()`: Creates and configures application logger

## Benefits of This Structure

1. **Separation of Concerns**: Each package has a single, well-defined responsibility
2. **Modularity**: Components can be developed and tested independently
3. **Reusability**: Packages can be imported and used in other projects
4. **Maintainability**: Easier to locate and modify specific functionality
5. **Testability**: Each package can have its own unit tests
6. **Scalability**: Easy to add new packages or extend existing ones

## Usage

The refactored application works exactly the same as before:

```bash
# Build and run
go build -o ephemeral-db main.go
./ephemeral-db

# Or run directly
go run main.go
```

## Adding New Features

### Adding HTTP Endpoints
1. Add handler methods to `api/handler.go`
2. Register routes in `SetupRoutes()`

### Adding MySQL Commands
1. Add query handling logic to `mysql/handler.go`
2. Extend the `HandleQuery()` method with new cases

### Adding New Packages
1. Create new directory under project root
2. Create package files with appropriate `package` declaration
3. Import and use in `main.go` or other packages

## Testing Structure

Each package can have its own test files:

```
api/
├── handler.go
└── handler_test.go

mysql/
├── handler.go
└── handler_test.go

logger/
├── logger.go
└── logger_test.go
```

This modular structure makes the codebase more professional and easier to work with as the project grows.
