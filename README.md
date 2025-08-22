# Multitenant DB - MySQL-Compatible Multi-Tenant Database Server

A Go application that implements the MySQL wire protocol with SQLite3 backends, providing complete database isolation per tenant using `idx` values. Each tenant gets their own isolated SQLite database while maintaining MySQL protocol compatibility.

## 🌟 Key Features

### Multi-Tenant Architecture
- **Per-Tenant Database Isolation**: Each `idx` value gets its own SQLite database
- **Dynamic Database Creation**: Databases are created on-demand when accessed
- **Session-Aware Routing**: Queries are automatically routed to the correct tenant database
- **RESTful Database Management**: Create, list, and delete tenant databases via HTTP API

### Protocol Support
- **MySQL Wire Protocol** (Port 3306) - Compatible with all MySQL clients
- **HTTP REST API** (Port 8080) - Database management and monitoring
- **Session Variables**: Support for both user (`@var`) and session (`@@var`) variables

## 🏗️ Architecture

```
┌─────────────────┐    ┌──────────────────────┐    ┌─────────────────┐
│   MySQL Client  │ ── │  Multitenant Server  │ ── │ SQLite DB (idx1)│
│                 │    │                      │    ├─────────────────┤
│ SET @idx='prod' │    │   Session Manager    │    │ SQLite DB (idx2)│
│ SELECT * FROM   │    │   Database Manager   │    ├─────────────────┤
│ users;          │    │   Query Router       │    │ SQLite DB (def) │
└─────────────────┘    └──────────────────────┘    └─────────────────┘
```

## 🚀 Quick Start

### 1. Build and Run
```bash
# Build the application
go build -o multitenant-db

# Run the server
./multitenant-db
```

### 2. Connect with MySQL Client
```bash
# Connect to the server
mysql -h 127.0.0.1 -P 3306 -u root --protocol=TCP
```

### 3. Set Your Tenant ID and Query
```sql
-- Set your tenant identifier
mysql> SET @idx = 'customer123';

-- Now all queries go to customer123's isolated database
mysql> SHOW TABLES;
mysql> SELECT * FROM users;
mysql> INSERT INTO users (name, email) VALUES ('John', 'john@customer123.com');
```

## 🏢 Multi-Tenant Usage

### Setting Tenant Context
```sql
-- Using user variables (recommended)
SET @idx = 'tenant_alpha';
SELECT * FROM users;  -- Queries tenant_alpha's database

-- Using session variables
SET @@idx = 'tenant_beta';
SELECT * FROM products;  -- Queries tenant_beta's database

-- Switch tenants dynamically
SET @idx = 'tenant_gamma';
INSERT INTO users (name, email) VALUES ('Alice', 'alice@gamma.com');
```

### Viewing Multi-Tenant Databases
```sql
-- Show all tenant databases
SHOW DATABASES;
```

Output:
```
+-----------------------------+
| Database                    |
+-----------------------------+
| information_schema          |
| mysql                       |
| performance_schema          |
| sys                         |
| multitenant_db              |  ← Default tenant
| multitenant_db_idx_prod     |  ← Production tenant
| multitenant_db_idx_dev      |  ← Development tenant
| multitenant_db_idx_test123  |  ← Test tenant
+-----------------------------+
```

## 🔌 HTTP API for Database Management

### List All Tenant Databases
```bash
curl http://localhost:8080/api/databases | jq
```

Response:
```json
{
  "databases": [
    {"name": "multitenant_db", "idx": "default"},
    {"name": "multitenant_db_idx_prod", "idx": "prod"},
    {"name": "multitenant_db_idx_dev", "idx": "dev"}
  ],
  "status": "ok",
  "timestamp": "2025-08-23T01:30:00Z"
}
```

### Create a New Tenant Database
```bash
curl -X POST http://localhost:8080/api/databases/create \
  -H "Content-Type: application/json" \
  -d '{"idx": "new_customer"}' | jq
```

### Delete a Tenant Database
```bash
curl -X DELETE "http://localhost:8080/api/databases/delete?idx=old_tenant" | jq
```

## 📊 Sample Data Structure

Each tenant database is initialized with sample tables:

### Users Table
| id | name    | email               | age |
|----|---------|---------------------|-----|
| 1  | Alice   | alice@example.com   | 30  |
| 2  | Bob     | bob@example.com     | 25  |
| 3  | Charlie | charlie@example.com | 35  |

### Products Table
| id | name   | price  | category    |
|----|--------|--------|-------------|
| 1  | Laptop | 999.99 | electronics |
| 2  | Book   | 19.99  | education   |
| 3  | Coffee | 4.99   | beverages   |

## 🔍 Supported MySQL Commands

- **Database Operations**: `SHOW DATABASES`, `SHOW TABLES`, `DESCRIBE table`
- **Data Queries**: `SELECT`, `INSERT`, `UPDATE`, `DELETE`
- **Variable Management**: `SET @var = value`, `SELECT @var`, `SET @@var = value`
- **Standard SQL**: All SQLite-compatible SQL commands

## 🌐 HTTP API Endpoints

### Core Endpoints
- `GET /` - Welcome message
- `GET /health` - Health check
- `GET /api/info` - API information and connection details

### Database Management
- `GET /api/databases` - List all tenant databases
- `POST /api/databases/create` - Create new tenant database
- `DELETE /api/databases/delete?idx=<tenant_id>` - Delete tenant database

## 💾 Session and Variable Management

### User Variables (`@var`)
```sql
SET @idx = 'my_tenant';           -- Set tenant context
SET @user_preference = 'dark';    -- Store user preferences
SELECT @idx, @user_preference;    -- Retrieve variables
```

### Session Variables (`@@var`)
```sql
SET @@idx = 'my_tenant';          -- Set tenant context (session scope)
SET @@custom_setting = 'value';   -- Custom session settings
SHOW VARIABLES;                   -- Show all session variables
```

### Variable Unset
```sql
SET @idx = NULL;                  -- Unset user variable
SET @@custom_setting = NULL;      -- Unset session variable
```

## 🔒 Security Considerations

⚠️ **Development/Demo Server**: This server is designed for development and demonstration purposes.

For production use, implement:
- **Authentication**: User management and password validation
- **Authorization**: Tenant access controls and permissions
- **Input Validation**: SQL injection prevention
- **Network Security**: TLS/SSL encryption, firewall rules
- **Rate Limiting**: Connection and query rate limits
- **Audit Logging**: Query and access logging

## 🏗️ Architecture Details

### Components
- **Session Manager**: Tracks connections and tenant contexts
- **Database Manager**: Creates and manages per-tenant SQLite databases
- **Query Router**: Routes queries to correct tenant database
- **HTTP API**: RESTful database management interface

### Concurrency
- **Thread-Safe**: All components use proper mutex locking
- **Per-Connection Sessions**: Each MySQL connection has isolated session state
- **Concurrent Access**: Multiple tenants can query simultaneously

### Storage
- **In-Memory SQLite**: Databases exist only while server runs
- **Per-Tenant Isolation**: Complete data separation between tenants
- **Auto-Initialization**: Sample data created for each new tenant

## 🛠️ Extending the Server

### Adding Custom Tables
```go
// In database.go - initSampleData()
_, err = db.Exec(`
    CREATE TABLE custom_table (
        id INTEGER PRIMARY KEY,
        tenant_data TEXT
    )
`)
```

### Adding New MySQL Commands
```go
// In handlers.go - add new handler method
func (qh *QueryHandlers) HandleCustomCommand(query string) (*mysql.Result, error) {
    // Implementation
}
```

### Custom Business Logic
```go
// Implement tenant-specific business rules in query handlers
// Add custom validation, data transformation, etc.
```

## 📝 Logging

Application logs include tenant context:
```
[MULTITENANT-DB] [idx=customer123] Executing query: SELECT * FROM users
[MULTITENANT-DB] [idx=prod] Database created for idx: prod
[MULTITENANT-DB] [idx=dev] Set user variable: @idx = dev
```

Logs are written to:
- Console output (with colors and formatting)
- `logs/app.log` file (persistent logging)

## 🎯 Use Cases

- **SaaS Applications**: Isolate customer data in multi-tenant applications
- **Development/Testing**: Separate environments per developer or test suite
- **Microservices**: Per-service database isolation
- **A/B Testing**: Separate data stores for different test variants
- **Customer Demos**: Isolated demo environments per prospect

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

The software is provided "AS IS", without warranty of any kind. No support obligations.
