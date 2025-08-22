# Ephemeral DB - MySQL Protocol Server with SQLite Backend

A Go application that implements the MySQL wire protocol with SQLite3 as the backend database, allowing MySQL clients to connect and interact with a real SQL database through the MySQL protocol.

## Features

- **HTTP API Server** (Port 8080)
  - Health checks
  - API information
  - JSON responses

- **MySQL Protocol Server** (Port 3306)
  - Compatible with MySQL clients
  - SQLite3 backend database
  - Full SQL query support

## Backend Database

- **SQLite3**: Real SQL database engine with full query support
- **In-Memory**: Database exists only while the server is running
- **Full SQL**: Supports complex queries, joins, transactions, etc.
- **Auto-Schema**: Sample tables created automatically on startup

## Supported MySQL Commands

- `SHOW DATABASES`
- `SHOW TABLES`
- `SELECT * FROM users`
- `SELECT * FROM products`
- `DESCRIBE users`
- `DESCRIBE products`
- `INSERT INTO users (name, email, age) VALUES ('John', 'john@example.com', 25)`
- `UPDATE users SET age = 26 WHERE name = 'John'`
- `DELETE FROM users WHERE name = 'John'`
- Any standard SQL query (SELECT, INSERT, UPDATE, DELETE, CREATE TABLE, etc.)

## Sample Data

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

## Running the Server

```bash
# Build the application
go build -o ephemeral-db main.go

# Run the server
./ephemeral-db
```

## Connecting with MySQL Clients

⚠️ **Important**: Always use TCP connections, not Unix sockets. This server uses TCP protocol on port 3306, similar to AWS RDS and other network-based MySQL services.

### MySQL CLI
```bash
# Recommended: Force TCP connection
mysql -h 127.0.0.1 -P 3306 -u root --protocol=TCP

# Alternative: Use localhost (may try socket first)
mysql -h localhost -P 3306 -u root
```

### Example Queries
```sql
-- Show available databases
SHOW DATABASES;

-- Show tables in the current database
SHOW TABLES;

-- Query the users table
SELECT * FROM users;

-- Query the products table
SELECT * FROM products;

-- Describe table structure
DESCRIBE users;
DESCRIBE products;
```

### MySQL Workbench
- Host: 127.0.0.1 (or localhost)
- Port: 3306
- Username: root
- Password: (leave empty)
- Connection Method: Standard (TCP/IP)

## HTTP API Endpoints

- `GET http://localhost:8080/` - Welcome message
- `GET http://localhost:8080/health` - Health check
- `GET http://localhost:8080/api/info` - API information and MySQL connection details

## Logs

Application logs are written to:
- Console output
- `logs/app.log` file

## Architecture

The application runs two servers concurrently:

1. **HTTP Server** (goroutine in main thread)
2. **MySQL Protocol Server** (background goroutine)

Each MySQL client connection is handled in its own goroutine for concurrent access.

### Network Protocol Notes

- **TCP/IP Only**: This server uses TCP connections on port 3306, not Unix sockets
- **RDS Compatible**: Connection method matches AWS RDS and other cloud MySQL services
- **Cross-platform**: TCP connections work consistently across different operating systems
- **Network Security**: Can be secured with firewalls, VPNs, and network ACLs like production databases

## Extending the Server

To add your custom logic:

1. **Add new tables** in `initSampleData()`
2. **Extend query parsing** in `handleSelect()`, `handleInsert()`, etc.
3. **Add new MySQL commands** in `HandleQuery()`
4. **Implement business logic** in the query handlers

## Security Note

⚠️ **This is a development/demo server**. For production use:
- Add authentication
- Implement proper SQL parsing
- Add input validation
- Secure network access
- Add rate limiting
