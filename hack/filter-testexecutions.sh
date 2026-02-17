#!/bin/bash
# Filter testexecutions_uniq.md by istio, experimental, and fips settings
#
# Usage:
#   ./hack/filter-testexecutions.sh [--istio yes|no] [--experimental yes|no] [--fips yes|no]
#
# Examples:
#   ./hack/filter-testexecutions.sh --istio yes                    # All tests requiring Istio
#   ./hack/filter-testexecutions.sh --fips no                      # All tests without FIPS
#   ./hack/filter-testexecutions.sh --istio no --experimental yes  # Non-Istio experimental tests
#   ./hack/filter-testexecutions.sh                                # Show all tests (no filter)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

INPUT_FILE="$PROJECT_ROOT/testexecutions_uniq.md"

# Default: no filter (empty means match any)
ISTIO_FILTER=""
EXPERIMENTAL_FILTER=""
FIPS_FILTER=""

# Parse arguments
while [[ $# -gt 0 ]]; do
    case $1 in
        --istio)
            ISTIO_FILTER="$2"
            shift 2
            ;;
        --experimental)
            EXPERIMENTAL_FILTER="$2"
            shift 2
            ;;
        --fips)
            FIPS_FILTER="$2"
            shift 2
            ;;
        -h|--help)
            echo "Usage: $0 [--istio yes|no] [--experimental yes|no] [--fips yes|no]"
            echo ""
            echo "Filter testexecutions_uniq.md by cluster setup requirements."
            echo ""
            echo "Options:"
            echo "  --istio yes|no         Filter by Istio requirement"
            echo "  --experimental yes|no  Filter by experimental features"
            echo "  --fips yes|no          Filter by FIPS mode"
            echo "  -h, --help             Show this help"
            echo ""
            echo "Examples:"
            echo "  $0 --istio yes                     # Tests requiring Istio"
            echo "  $0 --fips no                       # Tests without FIPS"
            echo "  $0 --istio no --experimental yes   # Non-Istio experimental tests"
            exit 0
            ;;
        *)
            echo "Unknown option: $1"
            exit 1
            ;;
    esac
done

if [[ ! -f "$INPUT_FILE" ]]; then
    echo "Error: $INPUT_FILE not found"
    exit 1
fi

# Print header
echo "| testcase | istio | experimental | fips |"
echo "|----------|-------|--------------|------|"

# Process file and filter
count=0
while IFS= read -r line; do
    # Skip header lines (first two lines)
    if [[ "$line" =~ ^[\|\ ]*testcase || "$line" =~ ^[\|\ ]*-+ ]]; then
        continue
    fi

    # Skip empty lines
    if [[ -z "$line" || "$line" =~ ^[[:space:]]*$ ]]; then
        continue
    fi

    # Extract fields (remove leading/trailing pipes and spaces)
    # Format: | testcase | istio | experimental | fips |
    testcase=$(echo "$line" | awk -F'|' '{gsub(/^[ \t]+|[ \t]+$/, "", $2); print $2}')
    istio=$(echo "$line" | awk -F'|' '{gsub(/^[ \t]+|[ \t]+$/, "", $3); print $3}')
    experimental=$(echo "$line" | awk -F'|' '{gsub(/^[ \t]+|[ \t]+$/, "", $4); print $4}')
    fips=$(echo "$line" | awk -F'|' '{gsub(/^[ \t]+|[ \t]+$/, "", $5); print $5}')

    # Skip if testcase is empty
    if [[ -z "$testcase" ]]; then
        continue
    fi

    # Apply filters
    if [[ -n "$ISTIO_FILTER" && "$istio" != "$ISTIO_FILTER" ]]; then
        continue
    fi

    if [[ -n "$EXPERIMENTAL_FILTER" && "$experimental" != "$EXPERIMENTAL_FILTER" ]]; then
        continue
    fi

    if [[ -n "$FIPS_FILTER" && "$fips" != "$FIPS_FILTER" ]]; then
        continue
    fi

    # Print matching row
    echo "$line"
    count=$((count + 1))
done < "$INPUT_FILE"

# Print summary
echo ""
echo "---"
echo "Total matching tests: $count"

if [[ -n "$ISTIO_FILTER" || -n "$EXPERIMENTAL_FILTER" || -n "$FIPS_FILTER" ]]; then
    echo "Filters applied:"
    [[ -n "$ISTIO_FILTER" ]] && echo "  - istio: $ISTIO_FILTER"
    [[ -n "$EXPERIMENTAL_FILTER" ]] && echo "  - experimental: $EXPERIMENTAL_FILTER"
    [[ -n "$FIPS_FILTER" ]] && echo "  - fips: $FIPS_FILTER"
fi

exit 0
