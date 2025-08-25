# Multitenant DB - MySQL-Compatible Multi-Tenant Database Server

A Go application that implements the MySQL wire protocol with SQLite3 backends, providing complete database isolation per tenant using `idx` values. Each tenant gets their own isolated database while maintaining MySQL protocol compatibility.

## 🧰 Development Tasks

The project uses [Task](https://taskfile.dev/) for automation. Available tasks:


## 🌟 Key Features

### Multi-Tenant Architecture
- **Per-Tenant Database Isolation**: Each `idx` value gets its own SQLite database
- **Dynamic Database Creation**: Databases are created on-demand when accessed
- **Session-Aware Routing**: Queries are automatically routed to the correct tenant database
- **RESTful Database Management**: Create, list, and delete tenant databases via HTTP API
- **Query Auditing**: Query and review all queries executed per tenant via logging or API

### Protocol Support
- **MySQL Wire Protocol** (Port 3306) - Compatible with all MySQL clients
- **HTTP REST API** (Port 8080) - Database management and monitoring
- **Session Variables**: Support for session (`@var`) variables


## 🚀 Quick Start

### 1. Build and Run
```bash
task build
task run
```

### 2. Connect with MySQL Client
```bash
mysql -h 127.0.0.1 -P 3306 -u root --protocol=TCP
```

### 3. Set Your Tenant ID and Query
```sql
SET @idx = 'customer123';
SHOW TABLES;
SELECT * FROM users;
INSERT INTO users (name, email) VALUES ('John', 'john@customer123.com');
```

## 🏢 Multi-Tenant Usage

### Setting Tenant Context
```sql
SET @idx = 'tenant_alpha';
SELECT * FROM users;

SET @@idx = 'tenant_beta';
SELECT * FROM products;

SET @idx = 'tenant_gamma';
INSERT INTO users (name, email) VALUES ('Alice', 'alice@gamma.com');
```

### Viewing Multi-Tenant Databases
```sql
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

## 💾 Session and Variable Management

### Session Variables (`@var`)
```sql
SET @idx = 'my_tenant';
SET @user_preference = 'dark';
SELECT @idx, @user_preference;
```


### Variable Unset
```sql
SET @idx = NULL;
```

## 🔒 Security Considerations

⚠️ **Development/Demo Server**: This server is designed for development and demonstration purposes.

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

## 🗂️ Project Structure

Following the [Standard Go Project Layout](https://github.com/golang-standards/project-layout):


### Testing Structure

The project uses build tags to separate unit and integration tests:

```bash
task | grep test
* coverage:                      Run unit tests with coverage and generate lcov.info
* coverage-html:                 Run tests with coverage and open HTML report
* test:                          Run all tests (unit + integration)
* test-short:                    Run unit tests only (legacy alias)
* coverage:integration:          Run integration tests with coverage
* test:all:                      Run all tests (unit + integration)
* test:docker:                   Run comprehensive Docker-based test suite
* test:docker:build:             Pre-build Docker images for faster test runs
* test:docker:clean:             Clean up Docker test resources
* test:docker:coverage:          Generate coverage report in Docker environment
* test:docker:debug:             Start Docker services for manual testing and debugging
* test:docker:integration:       Run integration tests with Docker Compose
* test:docker:rebuild:           Force rebuild of Docker images
* test:docker:unit:              Run unit tests in Docker environment
* test:integration:              Run integration tests
* test:unit:                     Run unit tests only
```


## 📝 Logging

Application logs include tenant context:
```
[MULTI-TENANT-DB] [idx=customer123] Executing query: SELECT * FROM users
[MULTI-TENANT-DB] [idx=prod] Database created for idx: prod
[MULTI-TENANT-DB] [idx=dev] Set user variable: @idx = dev
```

## 🎯 Use Cases

- **SaaS Applications**: Isolate customer data in multi-tenant applications
- **Development/Testing**: Separate environments per developer or test suite
- **Microservices**: Per-service database isolation
- **A/B Testing**: Separate data stores for different test variants
- **Customer Demos**: Isolated demo environments per prospect

## 📄 License

MIT License - see [LICENSE](LICENSE) file for details.

The software is provided "AS IS", without warranty of any kind. No support obligations.

## 📖 API Documentation (Swagger/OpenAPI)

During development, the HTTP server serves interactive Swagger UI and OpenAPI docs at:

- [http://localhost:8080/swagger/index.html](http://localhost:8080/swagger/index.html)

The OpenAPI spec and Swagger files are located in:

- `api/swagger/swagger.json` (OpenAPI JSON)
- `api/swagger/swagger.yaml` (OpenAPI YAML)

To update the docs after editing API comments, use:

```bash
task swagger
```

> **Note:** Swagger UI is only available in development mode (when `ENV=development` or unset).
MIT License - see [LICENSE](LICENSE) file for details.

The software is provided "AS IS", without warranty of any kind. No support obligations.
