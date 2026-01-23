#!/usr/bin/env bash
set -euo pipefail

MIN_COVERAGE="${1:-10}"
COVERAGE_FILE=".coverage/coverage.out"

if [ ! -f "$COVERAGE_FILE" ]; then
    echo "ERROR: Coverage file not found. Run 'make coverage' first."
    exit 1
fi

TOTAL=$(go tool cover -func="$COVERAGE_FILE" | grep total: | awk '{print $3}' | tr -d '%')

echo "Total coverage: ${TOTAL}%"
echo "Minimum required: ${MIN_COVERAGE}%"

if (( $(echo "$TOTAL < $MIN_COVERAGE" | bc -l) )); then
    echo "FAIL: Coverage ${TOTAL}% is below minimum ${MIN_COVERAGE}%"
    exit 1
fi

echo "PASS: Coverage meets threshold"
