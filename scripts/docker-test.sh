#!/bin/bash

set -e

# Enable Docker BuildKit for better caching
export DOCKER_BUILDKIT=1
export COMPOSE_DOCKER_CLI_BUILD=1

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
COMPOSE_FILE="docker-compose.test.yml"
PROJECT_NAME="multitenant-db-test"
TIMEOUT=300

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

check_if_rebuild_needed() {
    local image_name=$1
    local dockerfile=$2
    
    # Check if image exists
    if ! docker image inspect "$image_name" >/dev/null 2>&1; then
        return 0  # Need to build
    fi
    
    # Check if Dockerfile is newer than image
    local image_created=$(docker image inspect "$image_name" --format='{{.Created}}' | cut -d'T' -f1 | tr -d '-')
    local dockerfile_modified=$(stat -c %Y "$dockerfile" 2>/dev/null || echo 0)
    local today=$(date +%Y%m%d)
    
    return 1  # Don't need to rebuild
}

cleanup() {
    log_info "Cleaning up Docker resources..."
    docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" down -v --remove-orphans || true
    # Note: We're NOT running docker system prune to preserve build cache for faster subsequent runs
    # Run 'task test:docker:clean' manually if you need to free up disk space
}

wait_for_service() {
    local service=$1
    local max_attempts=30
    local attempt=1
    
    log_info "Waiting for $service to be healthy..."
    
    while [ $attempt -le $max_attempts ]; do
        if docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" ps "$service" | grep -q "healthy"; then
            log_success "$service is ready!"
            return 0
        fi
        
        log_info "Attempt $attempt/$max_attempts: $service not ready yet..."
        sleep 10
        attempt=$((attempt + 1))
    done
    
    log_error "$service failed to become healthy within timeout"
    return 1
}

run_test_suite() {
    local test_type=$1
    log_info "Running $test_type test suite..."
    
    case $test_type in
        "unit")
            docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" run --rm test-runner task test:unit
            ;;
        "integration") 
            docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" run --rm test-runner go test -tags=docker -v ./test/docker/...
            ;;
        "all")
            run_test_suite "unit"
            run_test_suite "integration"
            ;;
        *)
            log_error "Unknown test type: $test_type"
            return 1
            ;;
    esac
}

collect_logs() {
    log_info "Collecting logs from all services..."
    mkdir -p logs/docker-tests
    
    for service in multitenant-db mysql-client test-runner; do
        if docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" ps -q "$service" >/dev/null 2>&1; then
            log_info "Collecting logs for $service..."
            docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs "$service" > "logs/docker-tests/${service}.log" 2>&1 || true
        fi
    done
    
    log_success "Logs collected in logs/docker-tests/"
}

generate_coverage_report() {
    log_info "Generating coverage report..."
    docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" run --rm test-runner bash -c "
        task coverage
        cp coverage.out lcov.info /app/logs/docker-tests/ 2>/dev/null || true
    "
    log_success "Coverage report generated"
}

# Main execution
main() {
    local test_type=${1:-"all"}
    local cleanup_on_exit=${2:-"true"}
    
    log_info "Starting Docker-based test suite..."
    log_info "Test type: $test_type"
    log_info "Project: $PROJECT_NAME"
    
    # Set up cleanup trap
    if [ "$cleanup_on_exit" = "true" ]; then
        trap cleanup EXIT
    fi
    
    # Build and start services
    log_info "Building and starting services..."
    
    # Check if we need to rebuild images
    local need_rebuild=false
    
    if check_if_rebuild_needed "multitenant-db:latest" "Dockerfile"; then
        log_info "MultiTenant DB image needs rebuild"
        need_rebuild=true
    fi
    
    if check_if_rebuild_needed "test-runner:latest" "Dockerfile.test"; then
        log_info "Test runner image needs rebuild"
        need_rebuild=true
    fi
    
    if [ "$need_rebuild" = "true" ]; then
        log_info "Building Docker images..."
        docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" build --parallel
    else
        log_info "Using existing Docker images (no rebuild needed)"
    fi
    
    docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" up -d multitenant-db mysql-client
    
    # Wait for main service to be ready
    if ! wait_for_service "multitenant-db"; then
        collect_logs
        exit 1
    fi
    
    # Run tests based on type
    case $test_type in
        "debug")
            log_info "Starting debug mode with Adminer..."
            docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" --profile debug up -d adminer
            log_success "Debug services started:"
            log_success "  - Adminer: http://localhost:8081"
            log_success "  - MultiTenant DB: http://localhost:8080"
            log_success "  - MySQL Protocol: localhost:3306"
            log_info "Press Ctrl+C to stop debug mode"
            docker-compose -f "$COMPOSE_FILE" -p "$PROJECT_NAME" logs -f multitenant-db
            ;;
        "coverage")
            generate_coverage_report
            ;;
        *)
            if run_test_suite "$test_type"; then
                log_success "All tests passed!"
                collect_logs
                
                if [ "$test_type" = "all" ]; then
                    generate_coverage_report
                fi
            else
                log_error "Tests failed!"
                collect_logs
                exit 1
            fi
            ;;
    esac
}

# Help function
show_help() {
    echo "Docker-based test suite for MultiTenant DB"
    echo ""
    echo "Usage: $0 [TEST_TYPE] [CLEANUP]"
    echo ""
    echo "TEST_TYPE options:"
    echo "  unit        - Run unit tests only"
    echo "  integration - Run Docker integration tests"
    echo "  all         - Run all test suites (default)"
    echo "  coverage    - Generate coverage report"
    echo "  debug       - Start services for manual testing"
    echo ""
    echo "CLEANUP options:"
    echo "  true        - Clean up Docker resources on exit (default)"
    echo "  false       - Leave Docker resources running"
    echo ""
    echo "Examples:"
    echo "  $0                    # Run all tests with cleanup"
    echo "  $0 integration        # Run integration tests only"
    echo "  $0 debug false        # Start debug mode, don't cleanup"
}

# Parse command line arguments
if [ "$1" = "-h" ] || [ "$1" = "--help" ]; then
    show_help
    exit 0
fi

main "$@"
