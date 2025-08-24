# Docker-Based Test Suite

This directory contains a comprehensive Docker-based test suite for the MultiTenant Database Server. The test suite uses Docker Compose to create isolated test environments and provides multiple test scenarios.

## Overview

The Docker test suite provides:

- **Isolated Testing Environment**: Complete isolation using Docker containers
- **Multi-Service Testing**: Tests the full stack including MySQL protocol and HTTP API
- **Concurrent Testing**: Multi-tenant isolation under concurrent load
- **Integration Testing**: End-to-end testing with real MySQL clients
- **Debug Mode**: Interactive testing and debugging capabilities

## Quick Start

### Prerequisites

- Docker and Docker Compose installed
- Task runner installed (`go install github.com/go-task/task/v3/cmd/task@latest`)

### Running Tests

```bash
# Run all test suites
task test:docker

# Run specific test types
task test:docker:unit        # Unit tests in Docker
task test:docker:integration # Integration tests
task test:docker:coverage    # Coverage report

# Debug mode (starts services for manual testing)
task test:docker:debug

# Clean up Docker resources
task test:docker:clean
```

## Test Components

### 1. Docker Compose Services

#### Core Services
- **multitenant-db**: The main application under test
- **mysql-client**: MySQL client for protocol testing
- **test-runner**: Go test execution environment

#### Testing Services
- **adminer**: Database inspector (debug profile only)

### 2. Test Suites

#### Integration Tests (`test/docker/docker_integration_test.go`)
- Health check validation
- MySQL protocol testing
- Multi-tenant isolation verification
- Concurrent connection testing
- HTTP API testing
- Load testing scenarios

#### Cleanup Tests (`test/docker/cleanup_test.go`)
- Resource cleanup validation
- Connection pool exhaustion testing
- Long-running transaction handling
- Concurrent operation cleanup

## Configuration

### Environment Variables

The test suite uses these environment variables:

```bash
MULTITENANT_DB_HOST=multitenant-db    # Application host
MULTITENANT_DB_PORT=3306              # MySQL protocol port
MULTITENANT_DB_HTTP_PORT=8080         # HTTP API port
MYSQL_CLIENT_HOST=mysql-client        # MySQL client host
```

### Test Application Configuration

The application under test uses:

```bash
AUTH_USERNAME=testuser     # MySQL authentication username
AUTH_PASSWORD=testpass     # MySQL authentication password
LOG_LEVEL=debug           # Enhanced logging for tests
```

## Test Scenarios

### 1. Basic Functionality
- Health endpoint verification
- MySQL protocol connection
- Basic SQL operations
- Authentication testing

### 2. Multi-Tenant Isolation
- Tenant data separation
- Concurrent tenant operations
- Cross-tenant data verification
- Session isolation testing

### 3. Performance & Concurrency
- Concurrent connection handling
- Connection pool management
- Resource utilization
- Multi-tenant stress testing

### 4. Reliability & Cleanup
- Graceful connection handling
- Resource cleanup verification
- Error recovery testing
- Long-running operation handling

## Debug Mode

Debug mode starts the following services:

- **Application**: http://localhost:8080 (HTTP API)
- **MySQL Protocol**: localhost:3306 (MySQL client access)
- **Adminer**: http://localhost:8081 (Database inspector)

### Manual Testing Examples

```bash
# Start debug mode
task test:docker:debug

# Connect with MySQL client (in another terminal)
mysql -h 127.0.0.1 -P 3306 -u testuser -p --protocol=TCP
# Password: testpass

# Test tenant isolation
mysql> SET @idx = 'debug_tenant_1';
mysql> CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(50));
mysql> INSERT INTO users VALUES (1, 'Alice');

mysql> SET @idx = 'debug_tenant_2';
mysql> CREATE TABLE users (id INT PRIMARY KEY, name VARCHAR(50));
mysql> INSERT INTO users VALUES (1, 'Bob');

mysql> SET @idx = 'debug_tenant_1';
mysql> SELECT * FROM users; -- Should show Alice

# Test HTTP API
curl http://localhost:8080/health
curl -X POST http://localhost:8080/query \
  -H "Content-Type: application/json" \
  -d '{"tenant_id": "api_test", "query": "CREATE TABLE test (id INT)"}'
```

## File Structure

```
test/
├── docker/
│   ├── docker_integration_test.go  # Main integration tests
│   └── cleanup_test.go             # Cleanup and reliability tests

docker-compose.test.yml             # Docker Compose configuration
Dockerfile.test                     # Test runner Docker image
scripts/
└── docker-test.sh                  # Test execution script
```

## Troubleshooting

### Common Issues

1. **Service not ready**: Increase health check timeout in docker-compose.test.yml
2. **Port conflicts**: Change port mappings if 3306/8080/8081 are in use
3. **Resource limits**: Adjust Docker resource limits for load testing
4. **Test timeouts**: Increase test timeouts in test files

### Debugging

```bash
# Check service logs
docker-compose -f docker-compose.test.yml -p multitenant-db-test logs multitenant-db

# Check service health
docker-compose -f docker-compose.test.yml -p multitenant-db-test ps

# Manual service interaction
docker-compose -f docker-compose.test.yml -p multitenant-db-test exec multitenant-db /bin/sh

# View test runner environment
docker-compose -f docker-compose.test.yml -p multitenant-db-test run --rm test-runner env
```

### Log Collection

Test logs are automatically collected in `logs/docker-tests/`:
- `multitenant-db.log`: Application logs
- `mysql-client.log`: MySQL client logs  
- `test-runner.log`: Test execution logs
- `coverage.out`: Coverage data
- `lcov.info`: Coverage in LCOV format

## Integration with CI/CD

The Docker test suite is designed for CI/CD integration:

```yaml
# Example GitHub Actions workflow
- name: Run Docker Test Suite
  run: task test:docker

- name: Upload Coverage
  uses: codecov/codecov-action@v1
  with:
    file: ./lcov.info
```

## Performance Expectations

### Build Performance
- **First Run**: ~5-8 minutes (downloads base images, installs dependencies)
- **Subsequent Runs**: ~1-2 minutes (using Docker layer cache)
- **Code-only Changes**: ~30-60 seconds (only rebuilds changed layers)
- **No Changes**: ~10-20 seconds (pure cache hit)

### Integration Test Targets
- **Success Rate**: > 95%
- **Concurrent Connections**: Up to 50 simultaneous MySQL connections
- **Response Time**: < 100ms for basic operations
- **Test Duration**: < 5 minutes total

### Resource Usage
- **Memory**: < 512MB per service
- **CPU**: < 2 cores total
- **Connections**: Up to 50 concurrent MySQL connections

## Best Practices

1. **Preserve build cache**: Use `task test:docker:clean` only when you need to free disk space
2. **Monitor resources**: Check Docker resource usage during tests  
3. **Isolate tests**: Each test should clean up its own data
4. **Use timeouts**: Set appropriate timeouts for all operations
5. **Log everything**: Enable debug logging for troubleshooting
6. **Layer optimization**: The Dockerfiles are optimized for caching - avoid changing go.mod/go.sum frequently
