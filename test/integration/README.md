# Integration Tests

This directory contains integration tests that verify the complete functionality of the multi-tenant database server, including:

- MySQL protocol communication
- Query logging functionality  
- Multi-tenant isolation
- REST API endpoints
- End-to-end data flow

## Prerequisites

1. **Build the application**:
   ```bash
   task build
   ```

2. **Start the server** (in a separate terminal):
   ```bash
   ./bin/multitenant-db
   ```
   
   The server will start on:
   - MySQL protocol: `localhost:3306`
   - HTTP API: `localhost:8080`

3. **MySQL client** (optional, for manual testing):
   ```bash
   mysql -h 127.0.0.1 -P 3306 -u root --protocol=TCP
   ```

## Running Integration Tests

### Option 1: With server already running
```bash
# Run integration tests (assumes server is already running)
task test:integration
```

### Option 2: Automated server management
```bash
# Starts server, runs tests, stops server automatically
task test:integration:full
```

### Option 3: Manual go test command
```bash
# Direct go test command (assumes server is running)
go test -tags=integration -v ./test/integration/...
```

## Test Coverage

The integration test `TestQueryLoggingIntegration` covers:

### 1. Multi-Tenant Query Execution
- Executes MySQL queries for multiple tenants (`integration_test_tenant1`, `integration_test_tenant2`)
- Tests both successful and failing queries
- Verifies tenant isolation

### 2. API Endpoint Testing
- **GET `/api/query-logs`**: Lists all tenants with query logs
- **GET `/api/query-logs/{tenant}`**: Retrieves query logs for a specific tenant
- **GET `/api/query-logs/{tenant}/stats`**: Gets query statistics for a tenant

### 3. Query Log Verification
- Confirms queries are logged with correct metadata:
  - Tenant ID
  - Query text
  - Execution time
  - Success/failure status
  - Error messages (for failed queries)
  - Connection ID

### 4. Statistics Validation
- Total query counts
- Success/failure rates
- Execution time statistics (min, max, average)
- Per-tenant statistics isolation

### 5. Pagination Testing
- Verifies page size limits
- Tests page navigation
- Confirms total count accuracy

## Expected Test Data

The integration test will create the following test data:

### Tenant: `integration_test_tenant1`
- 3+ successful queries (SELECT, COUNT, INSERT)
- High success rate (should be 100%)

### Tenant: `integration_test_tenant2`  
- 2+ queries including 1 intentional failure
- Mixed success rate (less than 100%)
- Error message logging for failed queries

## Troubleshooting

### Server Not Running
```
Error: dial tcp 127.0.0.1:3306: connect: connection refused
```
**Solution**: Make sure the server is running on port 3306

### API Not Responding
```
Error: dial tcp 127.0.0.1:8080: connect: connection refused  
```
**Solution**: Make sure the server is running and the API is available on port 8080

### Permission Denied
```
Error: listen tcp :3306: bind: permission denied
```
**Solution**: Run with appropriate permissions or use different ports

### Database Conflicts
Integration tests use unique tenant names (`integration_test_tenant1`, `integration_test_tenant2`) to avoid conflicts with existing data.

## Manual Testing

You can also manually test the functionality:

1. **Connect and execute queries**:
   ```sql
   mysql -h 127.0.0.1 -P 3306 -u root --protocol=TCP
   
   SET @idx = 'manual_test';
   SELECT * FROM users;
   INSERT INTO test_table VALUES (1, 'test');
   ```

2. **Check API responses**:
   ```bash
   # List tenants
   curl "http://localhost:8080/api/query-logs"
   
   # Get logs for a tenant
   curl "http://localhost:8080/api/query-logs/manual_test"
   
   # Get statistics
   curl "http://localhost:8080/api/query-logs/manual_test/stats"
   ```

## CI/CD Integration

For automated testing in CI/CD pipelines:

```yaml
# Example GitHub Actions step
- name: Run Integration Tests
  run: |
    task build
    task test:integration:full
```

The `test:integration:full` task handles server lifecycle automatically, making it suitable for CI environments.
