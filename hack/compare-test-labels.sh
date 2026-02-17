#!/bin/bash
# Compare current test labels (IS state) with expected labels (WANT state)
# and identify tests that need label updates.
#
# Usage:
#   ./hack/compare-test-labels.sh
#
# Output shows tests where the current labels don't match the expected setup.

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

cd "$PROJECT_ROOT"

IS_FILE="/tmp/is_state.txt"
WANT_FILE="/tmp/want_state.txt"

echo "Collecting IS state (current labels from tests)..."

# Collect current state from all test packages
{
    go test -v ./test/e2e/... -count=1 -timeout=5m -print-labels 2>&1
    go test -v ./test/integration/... -count=1 -timeout=5m -print-labels 2>&1
    go test -v ./test/selfmonitor/... -count=1 -timeout=5m -print-labels 2>&1
} | grep "|" | awk '{
    # Extract: testcase | istio | experimental | fips
    split($0, parts, "|")
    gsub(/^[ \t]+|[ \t]+$/, "", parts[1])  # testcase
    gsub(/^[ \t]+|[ \t]+$/, "", parts[2])  # istio
    gsub(/^[ \t]+|[ \t]+$/, "", parts[3])  # experimental
    gsub(/^[ \t]+|[ \t]+$/, "", parts[4])  # fips
    print parts[1] "|" parts[2] "|" parts[3] "|" parts[4]
}' | sort -u > "$IS_FILE"

echo "Collecting WANT state (from testexecutions_uniq.md)..."

# Extract want state from testexecutions_uniq.md
# Format: | testcase | istio | experimental | fips |
awk -F'|' 'NR > 2 {
    gsub(/^[ \t]+|[ \t]+$/, "", $2)  # testcase
    gsub(/^[ \t]+|[ \t]+$/, "", $3)  # istio
    gsub(/^[ \t]+|[ \t]+$/, "", $4)  # experimental
    gsub(/^[ \t]+|[ \t]+$/, "", $5)  # fips
    if ($2 != "") {
        print $2 "|" $3 "|" $4 "|" $5
    }
}' "$PROJECT_ROOT/testexecutions_uniq.md" | sort -u > "$WANT_FILE"

echo ""
echo "=== COMPARISON RESULTS ==="
echo ""

# Find tests that need ISTIO label added
echo "--- Tests that need ISTIO label ADDED (want=yes, is=no) ---"
while IFS='|' read -r testcase want_istio want_exp want_fips; do
    is_line=$(grep "^${testcase}|" "$IS_FILE" 2>/dev/null || echo "")
    if [[ -n "$is_line" ]]; then
        is_istio=$(echo "$is_line" | cut -d'|' -f2)
        if [[ "$want_istio" == "yes" && "$is_istio" == "no" ]]; then
            echo "  $testcase"
        fi
    fi
done < "$WANT_FILE"

echo ""
echo "--- Tests that need ISTIO label REMOVED (want=no, is=yes) ---"
while IFS='|' read -r testcase want_istio want_exp want_fips; do
    is_line=$(grep "^${testcase}|" "$IS_FILE" 2>/dev/null || echo "")
    if [[ -n "$is_line" ]]; then
        is_istio=$(echo "$is_line" | cut -d'|' -f2)
        if [[ "$want_istio" == "no" && "$is_istio" == "yes" ]]; then
            echo "  $testcase"
        fi
    fi
done < "$WANT_FILE"

echo ""
echo "--- Tests that need EXPERIMENTAL label ADDED (want=yes, is=no) ---"
while IFS='|' read -r testcase want_istio want_exp want_fips; do
    is_line=$(grep "^${testcase}|" "$IS_FILE" 2>/dev/null || echo "")
    if [[ -n "$is_line" ]]; then
        is_exp=$(echo "$is_line" | cut -d'|' -f3)
        if [[ "$want_exp" == "yes" && "$is_exp" == "no" ]]; then
            echo "  $testcase"
        fi
    fi
done < "$WANT_FILE"

echo ""
echo "--- Tests that need EXPERIMENTAL label REMOVED (want=no, is=yes) ---"
while IFS='|' read -r testcase want_istio want_exp want_fips; do
    is_line=$(grep "^${testcase}|" "$IS_FILE" 2>/dev/null || echo "")
    if [[ -n "$is_line" ]]; then
        is_exp=$(echo "$is_line" | cut -d'|' -f3)
        if [[ "$want_exp" == "no" && "$is_exp" == "yes" ]]; then
            echo "  $testcase"
        fi
    fi
done < "$WANT_FILE"

echo ""
echo "--- Tests that need FIPS label ADDED (want=yes, is=no) ---"
while IFS='|' read -r testcase want_istio want_exp want_fips; do
    is_line=$(grep "^${testcase}|" "$IS_FILE" 2>/dev/null || echo "")
    if [[ -n "$is_line" ]]; then
        is_fips=$(echo "$is_line" | cut -d'|' -f4)
        if [[ "$want_fips" == "yes" && "$is_fips" == "no" ]]; then
            echo "  $testcase"
        fi
    fi
done < "$WANT_FILE"

echo ""
echo "--- Tests that need FIPS label REMOVED (want=no, is=yes) ---"
while IFS='|' read -r testcase want_istio want_exp want_fips; do
    # Check for exact match
    is_line=$(grep "^${testcase}|" "$IS_FILE" 2>/dev/null || echo "")
    if [[ -n "$is_line" ]]; then
        is_fips=$(echo "$is_line" | cut -d'|' -f4)
        if [[ "$want_fips" == "no" && "$is_fips" == "yes" ]]; then
            # Check if there's a subtest that matches the wanted config
            subtest_match=$(grep "^${testcase}/" "$IS_FILE" 2>/dev/null | while read -r subline; do
                sub_fips=$(echo "$subline" | cut -d'|' -f4)
                if [[ "$sub_fips" == "$want_fips" ]]; then
                    echo "found"
                    break
                fi
            done)
            if [[ -z "$subtest_match" ]]; then
                echo "  $testcase"
            fi
        fi
    fi
done < "$WANT_FILE"

echo ""
echo "--- Tests in WANT but not found in IS (missing tests?) ---"
while IFS='|' read -r testcase want_istio want_exp want_fips; do
    # Check for exact match first
    is_line=$(grep "^${testcase}|" "$IS_FILE" 2>/dev/null || echo "")
    if [[ -z "$is_line" ]]; then
        # Check for subtest match
        subtest_match=$(grep "^${testcase}/" "$IS_FILE" 2>/dev/null | while read -r subline; do
            sub_istio=$(echo "$subline" | cut -d'|' -f2)
            sub_exp=$(echo "$subline" | cut -d'|' -f3)
            sub_fips=$(echo "$subline" | cut -d'|' -f4)
            if [[ "$sub_istio" == "$want_istio" && "$sub_exp" == "$want_exp" && "$sub_fips" == "$want_fips" ]]; then
                echo "found"
                break
            fi
        done)
        if [[ -z "$subtest_match" ]]; then
            echo "  $testcase (want: istio=$want_istio exp=$want_exp fips=$want_fips)"
        fi
    fi
done < "$WANT_FILE"

echo ""
echo "=== SUMMARY ==="
echo "IS state file: $IS_FILE ($(wc -l < "$IS_FILE") tests)"
echo "WANT state file: $WANT_FILE ($(wc -l < "$WANT_FILE") tests)"
