#!/bin/bash
# =============================================================================
# Memtier Benchmark Integration Test Suite
# =============================================================================
# Runs real memtier_benchmark commands against a local Redis instance.
# Each test runs for 1 second to validate all parameter combinations.
#
# Prerequisites:
#   - Redis running on localhost:6379
#   - memtier_benchmark installed
#
# Usage:
#   ./scripts/test-memtier-integration.sh [--install-redis] [--verbose]
# =============================================================================

set -e

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Configuration
REDIS_HOST="${REDIS_HOST:-localhost}"
REDIS_PORT="${REDIS_PORT:-6379}"
TEST_DURATION=1  # seconds per test
VERBOSE=${VERBOSE:-false}
INSTALL_REDIS=false

# Counters
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0
SKIPPED_TESTS=0

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --install-redis)
            INSTALL_REDIS=true
            shift
            ;;
        --verbose|-v)
            VERBOSE=true
            shift
            ;;
        --host)
            REDIS_HOST="$2"
            shift 2
            ;;
        --port)
            REDIS_PORT="$2"
            shift 2
            ;;
        --help|-h)
            echo "Usage: $0 [OPTIONS]"
            echo ""
            echo "Options:"
            echo "  --install-redis    Install Redis via Docker if not running"
            echo "  --verbose, -v      Show memtier output"
            echo "  --host HOST        Redis host (default: localhost)"
            echo "  --port PORT        Redis port (default: 6379)"
            echo "  --help, -h         Show this help"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

# =============================================================================
# Helper Functions
# =============================================================================

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[PASS]${NC} $1"
}

log_fail() {
    echo -e "${RED}[FAIL]${NC} $1"
}

log_skip() {
    echo -e "${YELLOW}[SKIP]${NC} $1"
}

log_section() {
    echo ""
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
    echo -e "${BLUE}  $1${NC}"
    echo -e "${BLUE}━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━${NC}"
}

check_redis() {
    if redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" ping > /dev/null 2>&1; then
        return 0
    else
        return 1
    fi
}

install_redis_docker() {
    log_info "Starting Redis via Docker..."
    docker run -d --name redis-test -p 6379:6379 redis:7-alpine > /dev/null 2>&1 || true
    sleep 2
    if check_redis; then
        log_success "Redis started successfully"
    else
        log_fail "Failed to start Redis"
        exit 1
    fi
}

check_memtier() {
    if command -v memtier_benchmark &> /dev/null; then
        return 0
    else
        return 1
    fi
}

run_test() {
    local test_name="$1"
    shift
    local args=("$@")
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    # Check if -n (request count) is in the args - if so, don't add --test-time
    local use_test_time=true
    for arg in "${args[@]}"; do
        if [[ "$arg" == "-n" ]] || [[ "$arg" =~ ^-n[0-9]+ ]]; then
            use_test_time=false
            break
        fi
    done
    
    # Build command
    local cmd
    if [[ "$use_test_time" == "true" ]]; then
        cmd="memtier_benchmark -s $REDIS_HOST -p $REDIS_PORT --test-time=$TEST_DURATION --hide-histogram ${args[*]}"
    else
        cmd="memtier_benchmark -s $REDIS_HOST -p $REDIS_PORT --hide-histogram ${args[*]}"
    fi
    
    if [[ "$VERBOSE" == "true" ]]; then
        echo -e "  ${YELLOW}CMD:${NC} $cmd"
    fi
    
    # Run test
    local output
    local exit_code
    if output=$(eval "$cmd" 2>&1); then
        exit_code=0
    else
        exit_code=$?
    fi
    
    if [[ $exit_code -eq 0 ]]; then
        PASSED_TESTS=$((PASSED_TESTS + 1))
        log_success "$test_name"
        if [[ "$VERBOSE" == "true" ]]; then
            # Extract key metrics
            local ops_sec=$(echo "$output" | grep -E "Totals\s+" | awk '{print $2}')
            local latency=$(echo "$output" | grep -E "Totals\s+" | awk '{print $5}')
            if [[ -n "$ops_sec" ]]; then
                echo -e "       Ops/sec: $ops_sec, Avg Latency: ${latency}ms"
            fi
        fi
    else
        FAILED_TESTS=$((FAILED_TESTS + 1))
        log_fail "$test_name"
        if [[ "$VERBOSE" == "true" ]]; then
            echo "$output" | head -10
        fi
    fi
}

skip_test() {
    local test_name="$1"
    local reason="$2"
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    SKIPPED_TESTS=$((SKIPPED_TESTS + 1))
    log_skip "$test_name ($reason)"
}

# =============================================================================
# Pre-flight Checks
# =============================================================================

log_section "Pre-flight Checks"

# Check memtier_benchmark
if ! check_memtier; then
    log_fail "memtier_benchmark not found. Install with:"
    echo "  macOS:  brew install memtier_benchmark"
    echo "  Ubuntu: apt install memtier"
    exit 1
fi
log_success "memtier_benchmark found: $(which memtier_benchmark)"

# Check Redis
if ! check_redis; then
    if [[ "$INSTALL_REDIS" == "true" ]]; then
        install_redis_docker
    else
        log_fail "Redis not running on $REDIS_HOST:$REDIS_PORT"
        echo "  Start Redis or use --install-redis flag"
        exit 1
    fi
fi
log_success "Redis connected: $REDIS_HOST:$REDIS_PORT"

# Show versions
log_info "memtier version: $(memtier_benchmark --version 2>&1 | head -1)"
log_info "Redis version: $(redis-cli -h $REDIS_HOST -p $REDIS_PORT INFO server 2>/dev/null | grep redis_version | cut -d: -f2 | tr -d '\r')"

# Flush Redis for clean tests
log_info "Flushing Redis database..."
redis-cli -h "$REDIS_HOST" -p "$REDIS_PORT" FLUSHALL > /dev/null 2>&1

# =============================================================================
# CONNECTION TESTS (10 tests)
# =============================================================================
log_section "Connection Tests"

run_test "Basic connection" \
    -t 1 -c 1 --ratio=1:1 --key-pattern=R:R

run_test "Multiple threads (2)" \
    -t 2 -c 1 --ratio=1:1 --key-pattern=R:R

run_test "Multiple threads (4)" \
    -t 4 -c 1 --ratio=1:1 --key-pattern=R:R

run_test "Multiple clients (10)" \
    -t 1 -c 10 --ratio=1:1 --key-pattern=R:R

run_test "Multiple clients (50)" \
    -t 2 -c 50 --ratio=1:1 --key-pattern=R:R

run_test "High parallelism (4 threads, 100 clients)" \
    -t 4 -c 100 --ratio=1:1 --key-pattern=R:R

run_test "Maximum parallelism (8 threads, 200 clients)" \
    -t 8 -c 200 --ratio=1:1 --key-pattern=R:R

run_test "Single connection latency test" \
    -t 1 -c 1 --ratio=1:1 --key-pattern=R:R --pipeline=1

run_test "DB selection (db=1)" \
    -t 1 -c 1 --ratio=1:1 --key-pattern=R:R --select-db=1

run_test "DB selection (db=5)" \
    -t 1 -c 1 --ratio=1:1 --key-pattern=R:R --select-db=5

# =============================================================================
# KEY PATTERN TESTS (15 tests)
# =============================================================================
log_section "Key Pattern Tests"

run_test "Random key pattern (R)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R

run_test "Sequential key pattern (S)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=S:S

run_test "Gaussian key pattern (G)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=G:G

run_test "Parallel key pattern (P)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=P:P

run_test "Combined pattern (S:G)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=S:G

run_test "Combined pattern (R:R)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R

run_test "Key range (0-1000)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --key-minimum=1 --key-maximum=1000

run_test "Key range (1000-10000)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --key-minimum=1000 --key-maximum=10000

run_test "Large key range (0-1000000)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --key-minimum=1 --key-maximum=1000000

run_test "Key prefix (test:)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --key-prefix="test:"

run_test "Key prefix (app:v1:user:)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --key-prefix="app:v1:user:"

run_test "Gaussian with stddev (0.1)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=G:G --key-stddev=0.1

run_test "Gaussian with stddev (0.05)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=G:G --key-stddev=0.05

run_test "Gaussian with median" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=G:G --key-median=50000 --key-maximum=100000

run_test "Distinct client seed" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --distinct-client-seed

# =============================================================================
# DATA SIZE TESTS (12 tests)
# =============================================================================
log_section "Data Size Tests"

run_test "Small data (32 bytes)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -d 32

run_test "Medium data (256 bytes)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -d 256

run_test "Large data (1024 bytes)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -d 1024

run_test "Very large data (4096 bytes)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -d 4096

run_test "Huge data (16384 bytes)" \
    -t 1 -c 5 --ratio=1:1 --key-pattern=R:R -d 16384

run_test "Data size range (32-256)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --data-size-range=32-256

run_test "Data size range (64-1024)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --data-size-range=64-1024

run_test "Data size range with pattern (R)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --data-size-range=32-256 --data-size-pattern=R

run_test "Data size range with pattern (S)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --data-size-range=32-256 --data-size-pattern=S

run_test "Data size list (weighted)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --data-size-list="32:50,128:30,512:20"

run_test "Random data (-R)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -R -d 256

run_test "Data offset" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -d 256 --data-offset=100

# =============================================================================
# COMMAND RATIO TESTS (10 tests)
# =============================================================================
log_section "Command Ratio Tests"

run_test "Equal ratio (1:1)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R

run_test "Read heavy (9:1)" \
    -t 2 -c 10 --ratio=9:1 --key-pattern=R:R

run_test "Write heavy (1:9)" \
    -t 2 -c 10 --ratio=1:9 --key-pattern=R:R

run_test "GET only (1:0)" \
    -t 2 -c 10 --ratio=1:0 --key-pattern=R:R

run_test "SET only (0:1)" \
    -t 2 -c 10 --ratio=0:1 --key-pattern=R:R

run_test "80/20 read (4:1)" \
    -t 2 -c 10 --ratio=4:1 --key-pattern=R:R

run_test "70/30 read (7:3)" \
    -t 2 -c 10 --ratio=7:3 --key-pattern=R:R

run_test "95/5 read (19:1)" \
    -t 2 -c 10 --ratio=19:1 --key-pattern=R:R

run_test "99/1 read (99:1)" \
    -t 2 -c 10 --ratio=99:1 --key-pattern=R:R

run_test "Extreme write (1:99)" \
    -t 2 -c 10 --ratio=1:99 --key-pattern=R:R

# =============================================================================
# PIPELINE TESTS (8 tests)
# =============================================================================
log_section "Pipeline Tests"

run_test "No pipeline (1)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --pipeline=1

run_test "Small pipeline (5)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --pipeline=5

run_test "Medium pipeline (10)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --pipeline=10

run_test "Large pipeline (25)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --pipeline=25

run_test "Very large pipeline (50)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --pipeline=50

run_test "Aggressive pipeline (100)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --pipeline=100

run_test "Maximum pipeline (200)" \
    -t 2 -c 5 --ratio=1:1 --key-pattern=R:R --pipeline=200

run_test "Pipeline with high parallelism" \
    -t 4 -c 50 --ratio=1:1 --key-pattern=R:R --pipeline=20

# =============================================================================
# RATE LIMITING TESTS (6 tests)
# =============================================================================
log_section "Rate Limiting Tests"

run_test "Rate limit 1000 ops/sec" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --rate-limiting=1000

run_test "Rate limit 5000 ops/sec" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --rate-limiting=5000

run_test "Rate limit 10000 ops/sec" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --rate-limiting=10000

run_test "Rate limit 50000 ops/sec" \
    -t 4 -c 50 --ratio=1:1 --key-pattern=R:R --rate-limiting=50000

run_test "Rate limit with pipeline" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --rate-limiting=10000 --pipeline=10

run_test "Low rate limit (100 ops/sec)" \
    -t 1 -c 2 --ratio=1:1 --key-pattern=R:R --rate-limiting=100

# =============================================================================
# EXPIRY TESTS (6 tests)
# =============================================================================
log_section "Expiry Tests"

run_test "Fixed expiry (10s)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --expiry-range=10-10

run_test "Fixed expiry (60s)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --expiry-range=60-60

run_test "Expiry range (10-60s)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --expiry-range=10-60

run_test "Expiry range (1-300s)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --expiry-range=1-300

run_test "Short expiry (1-5s)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --expiry-range=1-5

run_test "Long expiry (300-600s)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --expiry-range=300-600

# =============================================================================
# CUSTOM COMMAND TESTS (10 tests)
# =============================================================================
log_section "Custom Command Tests"

run_test "INCR command" \
    -t 2 -c 10 --command="INCR __key__" --command-key-pattern=R

run_test "INCR with ratio" \
    -t 2 -c 10 --command="INCR __key__" --command-key-pattern=R --command-ratio=1

run_test "LPUSH command" \
    -t 2 -c 10 --command="LPUSH __key__ __data__" --command-key-pattern=R

run_test "RPUSH command" \
    -t 2 -c 10 --command="RPUSH __key__ __data__" --command-key-pattern=R

run_test "SADD command" \
    -t 2 -c 10 --command="SADD __key__ __data__" --command-key-pattern=R

run_test "HSET command" \
    -t 2 -c 10 --command="HSET __key__ field __data__" --command-key-pattern=R

run_test "ZADD command" \
    -t 2 -c 10 --command="ZADD __key__ 1 __data__" --command-key-pattern=R

run_test "SETEX command (with expiry)" \
    -t 2 -c 10 --command="SETEX __key__ 60 __data__" --command-key-pattern=R

run_test "PING command" \
    -t 2 -c 10 --command="PING" --command-key-pattern=R

run_test "Custom command with key pattern (S)" \
    -t 2 -c 10 --command="INCR __key__" --command-key-pattern=S

# =============================================================================
# MULTI-KEY TESTS (4 tests)
# =============================================================================
log_section "Multi-Key Tests"

run_test "MGET 5 keys" \
    -t 2 -c 10 --ratio=1:0 --key-pattern=R:R --multi-key-get=5

run_test "MGET 10 keys" \
    -t 2 -c 10 --ratio=1:0 --key-pattern=R:R --multi-key-get=10

run_test "MGET 25 keys" \
    -t 2 -c 10 --ratio=1:0 --key-pattern=R:R --multi-key-get=25

run_test "MGET 50 keys" \
    -t 2 -c 10 --ratio=1:0 --key-pattern=R:R --multi-key-get=50

# =============================================================================
# RUN COUNT / ITERATION TESTS (4 tests)
# =============================================================================
log_section "Run Count Tests"

run_test "Single run" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -x 1

run_test "Two runs" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -x 2

run_test "Three runs" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -x 3

run_test "Five runs (for statistics)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -x 5

# =============================================================================
# OUTPUT FORMAT TESTS (4 tests)
# =============================================================================
log_section "Output Format Tests"

run_test "JSON output" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --json-out-file=/dev/null

run_test "Hide histogram" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --hide-histogram

run_test "Print percentiles" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --print-percentiles=50,90,99,99.9

run_test "Full output (no hide)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R

# =============================================================================
# RANDOMIZATION TESTS (4 tests)
# =============================================================================
log_section "Randomization Tests"

run_test "Distinct client seed" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --distinct-client-seed

run_test "Randomize" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --randomize

run_test "Both randomization options" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --distinct-client-seed --randomize

run_test "No randomization" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R

# =============================================================================
# RECONNECT TESTS (3 tests)
# =============================================================================
log_section "Reconnect Tests"

run_test "Reconnect interval 1000" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --reconnect-interval=1000

run_test "Reconnect interval 5000" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --reconnect-interval=5000

run_test "Reconnect interval 10000" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --reconnect-interval=10000

# =============================================================================
# INTEGRATION / REAL-WORLD SCENARIO TESTS (10 tests)
# =============================================================================
log_section "Real-World Scenario Tests"

run_test "Cache workload (read heavy, small data)" \
    -t 4 -c 50 --ratio=9:1 --key-pattern=R:R -d 256 --pipeline=10 --key-maximum=100000

run_test "Session store workload" \
    -t 4 -c 50 --ratio=1:1 --key-pattern=R:R -d 1024 --expiry-range=1800-3600 --key-prefix="session:"

run_test "Rate limiter workload" \
    -t 2 -c 20 --command="INCR __key__" --command-key-pattern=R --key-prefix="ratelimit:"

run_test "Leaderboard workload" \
    -t 2 -c 20 --command="ZADD __key__ 1 __data__" --command-key-pattern=R --key-prefix="leaderboard:"

run_test "Queue workload" \
    -t 2 -c 20 --command="LPUSH __key__ __data__" --command-key-pattern=S --key-prefix="queue:"

run_test "Pub/Sub simulation (high throughput)" \
    -t 8 -c 100 --ratio=0:1 --key-pattern=P:P -d 128 --pipeline=50

run_test "Hot-key simulation (Gaussian)" \
    -t 4 -c 50 --ratio=4:1 --key-pattern=G:G -d 256 --key-stddev=0.05 --key-maximum=10000

run_test "Large value store" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -d 8192 --pipeline=5

run_test "High connection count" \
    -t 4 -c 200 --ratio=1:1 --key-pattern=R:R -d 64 --pipeline=1

run_test "Maximum throughput" \
    -t 8 -c 200 --ratio=1:1 --key-pattern=R:R -d 32 --pipeline=100

# =============================================================================
# PROTOCOL TESTS (3 tests)
# =============================================================================
log_section "Protocol Tests"

run_test "RESP2 protocol" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -P resp2

run_test "RESP3 protocol" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -P resp3

run_test "Default Redis protocol" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R

# =============================================================================
# REQUEST COUNT TESTS (4 tests)
# =============================================================================
log_section "Request Count Tests"

run_test "Request count 1000" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -n 1000

run_test "Request count 5000" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -n 5000

run_test "Request count 10000" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -n 10000

run_test "Request count with pipeline" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -n 5000 --pipeline=10

# =============================================================================
# ZIPF DISTRIBUTION TESTS (4 tests)
# =============================================================================
log_section "Zipf Distribution Tests"

run_test "Zipf pattern (default exponent)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=Z:Z --key-minimum=1 --key-maximum=10000

run_test "Zipf with exponent 1.0" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=Z:Z --key-zipf-exp=1.0 --key-minimum=1 --key-maximum=10000

run_test "Zipf with exponent 1.5" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=Z:Z --key-zipf-exp=1.5 --key-minimum=1 --key-maximum=10000

run_test "Zipf with exponent 2.0 (high concentration)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=Z:Z --key-zipf-exp=2.0 --key-minimum=1 --key-maximum=10000

# =============================================================================
# OUTPUT FILE TESTS (5 tests)
# =============================================================================
log_section "Output File Tests"

TMPDIR=$(mktemp -d)
trap "rm -rf $TMPDIR" EXIT

run_test "Output to file (-o)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -o "$TMPDIR/output.txt"

run_test "JSON output to file" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --json-out-file="$TMPDIR/results.json"

run_test "HDR histogram file prefix" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --hdr-file-prefix="$TMPDIR/hdr_"

run_test "Client stats file" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --client-stats="$TMPDIR/client_stats.csv"

run_test "Print all runs (with 2 iterations)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -x 2 --print-all-runs

# =============================================================================
# DEBUG AND CONFIG TESTS (2 tests)
# =============================================================================
log_section "Debug and Config Tests"

run_test "Show config before running" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R --show-config

run_test "Debug output (-D)" \
    -t 1 -c 1 --ratio=1:1 --key-pattern=R:R -D

# =============================================================================
# NETWORK TESTS (2 tests - IPv4/IPv6)
# =============================================================================
log_section "Network Tests"

run_test "Force IPv4 (-4)" \
    -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -4

# IPv6 test - only run if localhost resolves to IPv6
if ping6 -c 1 localhost > /dev/null 2>&1; then
    run_test "Force IPv6 (-6)" \
        -t 2 -c 10 --ratio=1:1 --key-pattern=R:R -6 -s ::1
else
    skip_test "Force IPv6 (-6)" "IPv6 not available on localhost"
fi

# =============================================================================
# DATA IMPORT TESTS (5 tests)
# =============================================================================
log_section "Data Import Tests"

# Create test data file for import tests
# memtier_benchmark data-import expects a CSV file with specific format:
# Header: dumpflags, time, exptime, nbytes, nsuffix, it_flags, clsid, nkey, key, data
# nbytes = data length + 2 (for CR/LF terminator)
# nkey = key length
DATA_FILE="$TMPDIR/test_data.csv"
log_info "Creating test data file for import tests..."

# Create a CSV data file with proper memtier format
{
    echo "dumpflags, time, exptime, nbytes, nsuffix, it_flags, clsid, nkey, key, data"
    # nbytes = 12 (10 chars + 2 for CRLF), nkey = 4 (key1)
    echo "0, 0, 0, 12, 0, 0, 0, 4, key1, xxxxxxxxxx"
    echo "0, 0, 0, 14, 0, 0, 0, 4, key2, aaaaaaaaaaaa"
    echo "0, 0, 0, 17, 0, 0, 0, 4, key3, testvalue12345"
    echo "0, 0, 0, 22, 0, 0, 0, 4, key4, longertestvalue12345"
    echo "0, 0, 0, 12, 0, 0, 0, 4, key5, 0123456789"
} > "$DATA_FILE"

# Test data import with the file (using generate-keys to create new keys from imported values)
run_test "Data import from file" \
    -t 2 -c 10 --data-import="$DATA_FILE" --generate-keys --key-prefix="import-"

# Data import with verification (reads data back to verify correctness)
run_test "Data import with verify" \
    -t 2 -c 10 --data-import="$DATA_FILE" --generate-keys --key-prefix="verify-" --data-verify

# Verify only (no new writes, just verification of previously imported data)
# First import, then verify-only
run_test "Data import before verify-only" \
    -t 1 -c 5 --data-import="$DATA_FILE" --generate-keys --key-prefix="vonly-"
run_test "Verify only mode" \
    -t 1 -c 5 --data-import="$DATA_FILE" --generate-keys --key-prefix="vonly-" --verify-only

# Test no-expiry flag with data import (ignores any expiry info in imported data)
run_test "Data import with no-expiry" \
    -t 2 -c 10 --data-import="$DATA_FILE" --generate-keys --key-prefix="noexp-" --no-expiry

# =============================================================================
# SUMMARY
# =============================================================================
log_section "Test Summary"

echo ""
echo "Total Tests:   $TOTAL_TESTS"
echo -e "Passed:        ${GREEN}$PASSED_TESTS${NC}"
echo -e "Failed:        ${RED}$FAILED_TESTS${NC}"
echo -e "Skipped:       ${YELLOW}$SKIPPED_TESTS${NC}"
echo ""

if [[ $FAILED_TESTS -eq 0 ]]; then
    echo -e "${GREEN}✅ All tests passed!${NC}"
    exit 0
else
    echo -e "${RED}❌ Some tests failed${NC}"
    exit 1
fi
