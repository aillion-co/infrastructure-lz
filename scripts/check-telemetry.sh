#!/usr/bin/env bash
# Checks that Go files with I/O functions have OTEL instrumentation.
# Used as a pre-commit check and by Claude Code hooks.
set -euo pipefail

ERRORS=0

# Find all .go files that are not tests, generated, or vendor
while IFS= read -r -d '' file; do
    # Skip files that don't do I/O (models, types-only files)
    basename=$(basename "$file")
    if [[ "$basename" == *"_test.go" ]] || \
       [[ "$basename" == "*.pb.go" ]] || \
       [[ "$file" == *"/models/"* ]] || \
       [[ "$file" == *"/telemetry/"* ]]; then
        continue
    fi

    # Check if file has functions that take context.Context (I/O functions)
    if grep -qP 'func\s.*\bcontext\.Context\b' "$file" 2>/dev/null; then
        # Verify it imports telemetry package
        if ! grep -q 'telemetry' "$file" 2>/dev/null; then
            echo "WARN: $file has context.Context functions but does not import telemetry"
            ERRORS=$((ERRORS + 1))
        fi
    fi

    # Check for bare slog calls (should use slog.*Context variants)
    if grep -qP '\bslog\.(Info|Error|Warn|Debug)\(' "$file" 2>/dev/null; then
        if grep -qP 'func\s.*\bcontext\.Context\b' "$file" 2>/dev/null; then
            echo "WARN: $file uses bare slog calls in functions with context — use slog.*Context(ctx, ...) instead"
            ERRORS=$((ERRORS + 1))
        fi
    fi

done < <(find . -name '*.go' -not -path './vendor/*' -not -path './.git/*' -print0)

if [ "$ERRORS" -gt 0 ]; then
    echo ""
    echo "Found $ERRORS telemetry instrumentation warnings."
    echo "See CLAUDE.md 'Observability (MANDATORY)' section for requirements."
    exit 1
fi

echo "Telemetry check passed."
