#!/usr/bin/env bash

# E2E Test Local Execution Script
# This script runs e2e tests locally with dynamic cluster configuration

set -e
set -o pipefail

# Script configuration
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "${SCRIPT_DIR}/.." && pwd)"

# Test configuration (can be overridden via environment variables)
MANAGER_IMAGE="${MANAGER_IMAGE:-europe-docker.pkg.dev/kyma-project/dev/telemetry-manager:latest}"
INSTALL_ISTIO="${INSTALL_ISTIO:-false}"
OPERATE_IN_FIPS_MODE="${OPERATE_IN_FIPS_MODE:-true}"
ENABLE_EXPERIMENTAL="${ENABLE_EXPERIMENTAL:-false}"
CUSTOM_LABELS_ANNOTATIONS="${CUSTOM_LABELS_ANNOTATIONS:-false}"

# Test execution parameters
TEST_PATH="${TEST_PATH:-./test/e2e/logs/agent/...}"
TEST_LABELS="${TEST_LABELS:-log-agent}"
TEST_VERBOSE="${TEST_VERBOSE:-true}"

# Color for errors only
RED='\033[0;31m'
NC='\033[0m'

# Error helper
error() {
    echo -e "${RED}Error: $1${NC}" >&2
    exit 1
}

# Check Docker is running
check_docker() {
    docker info &>/dev/null || error "Docker is not running"
}

# Build local manager image
build_local_image() {
    echo "Building local manager image..."

    local timestamp=$(date +%Y%m%d-%H%M%S)
    local git_sha=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    local tag="e2e-local-${timestamp}-${git_sha}"

    export MANAGER_IMAGE="telemetry-manager:${tag}"

    echo "Building: $MANAGER_IMAGE"
    cd "$PROJECT_ROOT"
    make docker-build IMG="$MANAGER_IMAGE"

    echo "Image built successfully"
}

# Provision k3d cluster
provision_cluster() {
    echo "Provisioning k3d cluster..."

    # Check if cluster already exists
    if k3d cluster list | grep -q "k3d-kyma"; then
        if [ "$FORCE_RECREATE" = "true" ]; then
            echo "Deleting existing cluster..."
            k3d cluster delete kyma || true
        else
            echo "Using existing cluster"
            return 0
        fi
    fi

    echo "Creating new k3d cluster..."
    cd "$PROJECT_ROOT"
    make provision-k3d

    echo "Waiting for cluster to be ready..."
    kubectl wait --for=condition=Ready nodes --all --timeout=120s

    echo "Cluster ready"
}

# Run tests
run_tests() {
    echo ""
    echo "Configuration:"
    echo "  Image:        $MANAGER_IMAGE"
    echo "  Istio:        $INSTALL_ISTIO"
    echo "  FIPS:         $OPERATE_IN_FIPS_MODE"
    echo "  Experimental: $ENABLE_EXPERIMENTAL"
    echo "  Path:         $TEST_PATH"
    echo "  Labels:       $TEST_LABELS"
    echo ""

    # Export environment variables for tests
    export MANAGER_IMAGE
    export INSTALL_ISTIO
    export OPERATE_IN_FIPS_MODE
    export ENABLE_EXPERIMENTAL
    export CUSTOM_LABELS_ANNOTATIONS

    cd "$PROJECT_ROOT"

    # Build test arguments
    local test_args="-timeout 30m"
    [ "$TEST_VERBOSE" = "true" ] && test_args="$test_args -v"
    [ -n "$TEST_LABELS" ] && test_args="$test_args -labels=\"$TEST_LABELS\""

    echo "Running: go test $TEST_PATH $test_args"
    echo ""

    if go test "$TEST_PATH" $test_args; then
        echo ""
        echo "Tests passed successfully"
        return 0
    else
        echo ""
        echo -e "${RED}Tests failed${NC}"
        return 1
    fi
}

# Print usage information
print_usage() {
    cat << EOF
Usage: $0 [OPTIONS]

Run e2e tests locally with dynamic cluster configuration.

OPTIONS:
    -h, --help                  Show this help message
    -i, --image IMAGE           Manager image to use (default: latest)
    --build                     Build manager image locally with timestamped tag
    --istio                     Install Istio before tests
    --no-fips                   Disable FIPS mode
    --experimental              Enable experimental mode
    --custom-labels             Enable custom labels/annotations
    -p, --path PATH             Test path (default: ./test/e2e/logs/agent/...)
    -l, --labels LABELS         Test labels filter (e.g., "log-agent and istio")
    --no-verbose                Disable verbose test output
    --skip-provision            Skip cluster provisioning
    --force-recreate            Delete and recreate existing cluster

EXAMPLES:
    # Build image locally and run default tests
    $0 --build

    # Build and run specific tests with Istio
    $0 --build --istio -p "./test/e2e/logs/gateway/..." -l "log-gateway and istio"

    # Run all istio integration tests
    $0 --istio --build -p "./test/integration/istio/..." -l "istio"

    # Run istio tests without fluent-bit (OTEL only)
    $0 --istio --build -p "./test/integration/istio/..." -l "istio and not fluent-bit"

    # Run istio fluent-bit tests without FIPS
    $0 --istio --no-fips --build -p "./test/integration/istio/..." -l "istio and fluent-bit"

    # Run metrics tests
    $0 -p "./test/e2e/metrics/agent/..." -l "metric-agent-a"

    # Use existing cluster with custom image
    $0 --skip-provision -i europe-docker.pkg.dev/kyma-project/dev/telemetry-manager:pr-123

    # Force recreate cluster
    $0 --force-recreate --build

ENVIRONMENT VARIABLES:
    MANAGER_IMAGE               Telemetry manager image
    INSTALL_ISTIO               Install Istio (true/false)
    OPERATE_IN_FIPS_MODE        FIPS mode (true/false)
    ENABLE_EXPERIMENTAL         Experimental mode (true/false)
    CUSTOM_LABELS_ANNOTATIONS   Custom labels/annotations (true/false)
    TEST_PATH                   Path to test files
    TEST_LABELS                 Test labels filter expression
    TEST_VERBOSE                Verbose output (true/false)

EOF
}

# Parse command line arguments
parse_args() {
    SKIP_PROVISION=false
    BUILD_IMAGE=false
    FORCE_RECREATE=false

    while [[ $# -gt 0 ]]; do
        case $1 in
            -h|--help)
                print_usage
                exit 0
                ;;
            -i|--image)
                MANAGER_IMAGE="$2"
                shift 2
                ;;
            --build|--build-image)
                BUILD_IMAGE=true
                shift
                ;;
            --istio)
                INSTALL_ISTIO="true"
                shift
                ;;
            --no-fips)
                OPERATE_IN_FIPS_MODE="false"
                shift
                ;;
            --experimental)
                ENABLE_EXPERIMENTAL="true"
                shift
                ;;
            --custom-labels)
                CUSTOM_LABELS_ANNOTATIONS="true"
                shift
                ;;
            -p|--path)
                TEST_PATH="$2"
                shift 2
                ;;
            -l|--labels)
                TEST_LABELS="$2"
                shift 2
                ;;
            --no-verbose)
                TEST_VERBOSE="false"
                shift
                ;;
            --skip-provision)
                SKIP_PROVISION=true
                shift
                ;;
            --force-recreate)
                FORCE_RECREATE=true
                shift
                ;;
            *)
                error "Unknown option: $1. Use --help for usage information."
                ;;
        esac
    done
}

# Main execution
main() {
    echo "E2E Test Local Execution"
    echo ""

    parse_args "$@"
    check_docker

    [ "$BUILD_IMAGE" = "true" ] && build_local_image
    [ "$SKIP_PROVISION" = "false" ] && provision_cluster

    run_tests
}

main "$@"
