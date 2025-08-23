---
applyTo: "**"
---
# Multi-Tenant Database Server - AI Coding Agent Instructions

## Architecture Overview

This is a **multi-tenant database server** that implements the MySQL wire protocol with per-tenant SQLite backends. The core pattern: clients set `SET @idx = 'tenant_id'` to route all subsequent queries to isolated tenant databases.

### Key Components & Data Flow
```
MySQL Client → Handler (Port 3306) → SessionManager → DatabaseManager → SQLite (per-tenant)
HTTP Client → API Handler (Port 8080) → DatabaseManagerAdapter → DatabaseManager
```

## Critical Session & Tenant Isolation Pattern

**THE MOST IMPORTANT CONCEPT**: Tenant isolation via session variables
- `SET @idx = 'customer123'` routes queries to `customer123`'s isolated SQLite database
- `internal/mysql/session.go` manages per-connection session state with `SessionVariables`
- `internal/mysql/database.go` creates databases on-demand using `GetDatabaseForSession(session)`
- Each MySQL connection gets unique ID via `GetNextConnectionID()` and isolated session

### Essential Session Flow
```go
// In Handler - session lookup pattern used everywhere
connID := h.sessionManager.GetCurrentConnection()
session := h.sessionManager.GetOrCreateSession(connID)
db, err := h.databaseManager.GetDatabaseForSession(session) // Uses @idx from session
```

## Development Workflow

### Working Directory & Commands
- **Always confirm location**: Use `pwd` to verify you're in the project root (should show `/path/to/multi-tenant-db`)
- **Don't suggest `cd` commands**: Assume the user is already in the correct directory

### Use Task for ALL operations (not make/go commands)
```bash
task test:unit         # Unit tests only - THIS IS THE DEFAULT
task test:integration  # Integration tests with real MySQL protocol
task coverage         # Coverage with lcov.info generation  
task vet              # Go vet (always run before commits)
task build            # Build to bin/multitenant-db
```

### Testing Patterns
- Tests use `log.New(os.Stdout, "[TEST] ", log.LstdFlags)` for consistent logging
- Session isolation tests: Create multiple `connID`s, set different `@idx` values, verify separate databases
- Use `handler.sessionManager.SetCurrentConnection(connID)` to switch context in tests

## Configuration System Patterns

### Dual Configuration Sources (Environment + Flags)
```go
// Pattern in main.go and config/config.go
cfg.LoadFromEnv()        // Load from ENV first
// Then override with command-line flags
if *flagValue != "" {
    cfg.Field = *flagValue
}
```

### Authentication Configuration
- `internal/config/config.go` has `AuthConfig` for MySQL protocol auth
- `AUTH_USERNAME`/`AUTH_PASSWORD` env vars or `--auth-username`/`--auth-password` flags
- Defaults to `root` with empty password if not configured

## MySQL Protocol Implementation

### Query Handler Pattern (internal/mysql/handlers.go)
```go
// All handlers follow this pattern
func (qh *QueryHandlers) HandleSomething() (*mysql.Result, error) {
    session := qh.handler.sessionManager.GetOrCreateSession(qh.handler.sessionManager.GetCurrentConnection())
    db, err := qh.handler.databaseManager.GetDatabaseForSession(session)
    // Execute SQLite query, convert to MySQL Result format
}
```

### Adding New MySQL Commands
1. Add method in `QueryHandlers` struct in `handlers.go`
2. Add routing logic in `Handler.HandleQuery()` in `handler.go`
3. Follow existing pattern: session → database → SQLite query → MySQL result conversion

## Adapter Pattern for HTTP API

**CRITICAL**: `cmd/multi-tenant-db/main.go` uses `DatabaseManagerAdapter` to bridge MySQL Handler and HTTP API
```go
// This adapter prevents HTTP API from importing mysql package directly
type DatabaseManagerAdapter struct {
    handler *mysql.Handler
}
```

## Logging Patterns

### Context-Aware Logging with Tenant IDs
```go
// Pattern used throughout - logs include [idx=tenant] prefix
h.logWithIdx("Message with tenant context")
// Outputs: [MULTITENANT-DB] [idx=customer123] Message with tenant context
```

## Thread Safety & Concurrency

- `SessionManager`: Uses `sync.RWMutex` for session map operations
- `DatabaseManager`: Uses `sync.RWMutex` for database map (multiple readers, single writer)
- Each MySQL connection runs in separate goroutine with isolated session state

## File Organization Rules

- `internal/mysql/`: MySQL protocol implementation (session, database, handlers)
- `internal/config/`: Configuration with env/flag loading and validation  
- `internal/api/`: HTTP REST API handlers
- `cmd/multi-tenant-db/`: Main application entry point with adapter pattern
- Tests: Use build tags for separation (`-short` for unit, `-tags=integration` for integration)

## Common Debugging Commands

```bash
# Run server with custom auth
./bin/multitenant-db --auth-username=myuser --auth-password=mypass

# Connect with MySQL client
mysql -h 127.0.0.1 -P 3306 -u myuser -p --protocol=TCP

# Test tenant isolation
mysql> SET @idx = 'test'; SHOW TABLES;
```

## Configuration Examples

### Environment Variables
```bash
export AUTH_USERNAME=admin
export AUTH_PASSWORD=secret
export DEFAULT_DB_TYPE=mysql
export DEFAULT_DB_MYSQL_HOST=db.example.com
```

### Command Line
```bash
./multitenant-db --auth-username=admin --default-db-type=sqlite --default-db-path=/tmp/default.db
```

## Task Completion Guidelines

When you have fully completed your task, write me a commit message.

Also propose three new ideas for my application, or on what you should do next.
