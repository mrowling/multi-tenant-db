# Multi-Tenant Database Server

This directory contains the main application entry point for the multi-tenant database server.

## Structure

- `main.go` - Main application entry point and server startup logic
- `main_test.go` - Tests for the main application adapter and initialization

## Building

From the project root:

```bash
# Using task
task build

# Using go directly
go build -o bin/multitenant-db ./cmd/multi-tenant-db

# Release build
task build-release
```

## Running

```bash
# Using task
task run

# Or directly
task dev

# Or from binary
./bin/multitenant-db
```

## Features

- HTTP REST API server on port 8080
- MySQL protocol server on port 3306
- Multi-tenant database isolation
- SQLite backend with per-tenant databases
- Session variable management
- Connection pooling
