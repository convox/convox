#!/usr/bin/env bash
# Verify SECTION markers are balanced and every Go const in
# prometheus_queries.go falls within a SECTION block.
#
# Run from repo root: ./scripts/check-section-markers.sh
#
# Wired into CI as a parity guardrail. Fails when a const is added outside any
# SECTION (which would silently break the section-ownership scheme used to
# avoid merge conflicts when multiple workstreams touch this file).

set -euo pipefail

GO_FILE="provider/k8s/prometheus_queries.go"
YAML_FILE="examples/gpu-llm/grafana/promql-source-of-truth.yaml"

if [[ ! -f "$GO_FILE" ]]; then
  echo "FAIL: $GO_FILE not found (run from repo root)" >&2
  exit 1
fi

# 1. Section markers in Go file are balanced.
go_begin=$(grep -c '^	// ===== SECTION:' "$GO_FILE" || true)
go_end=$(grep -c '^	// ===== END SECTION' "$GO_FILE" || true)

if [[ "$go_begin" != "$go_end" ]]; then
  echo "FAIL: $GO_FILE has $go_begin SECTION begins and $go_end SECTION ends — must match." >&2
  exit 1
fi
if [[ "$go_begin" -lt 1 ]]; then
  echo "FAIL: $GO_FILE has zero SECTION markers — at least one required." >&2
  exit 1
fi

# 2. Every line declaring a Go const inside the const ( ... ) block must fall
#    within a SECTION...END SECTION pair. Check via awk state machine.
out=$(awk '
  /^const \(/ { in_const = 1; in_section = 0; next }
  /^\)/        { in_const = 0; next }
  !in_const   { next }
  /^	\/\/ ===== SECTION:/ { in_section = 1; next }
  /^	\/\/ ===== END SECTION/ { in_section = 0; next }
  # const declarations look like: \tConstName = `...`  OR  \tConstName uintN = N
  /^	[A-Z][A-Za-z0-9_]+[ \t]*=/ {
    if (!in_section) {
      printf "%s:%d: const %s declared outside SECTION block\n", FILENAME, NR, $1
      err = 1
    }
  }
  END { exit err }
' "$GO_FILE")

if [[ -n "$out" ]]; then
  echo "$out" >&2
  echo "FAIL: const declared outside SECTION block (above)." >&2
  exit 1
fi

# 3. YAML manifest section markers balanced (cosmetic — YAML doesn't
#    syntactically require this, but keeps the human-readable mirror clean).
if [[ -f "$YAML_FILE" ]]; then
  yaml_begin=$(grep -c '^  # ===== SECTION:' "$YAML_FILE" || true)
  yaml_end=$(grep -c '^  # ===== END SECTION' "$YAML_FILE" || true)
  if [[ "$yaml_begin" != "$yaml_end" ]]; then
    echo "FAIL: $YAML_FILE has $yaml_begin SECTION begins and $yaml_end SECTION ends — must match." >&2
    exit 1
  fi
fi

echo "OK: SECTION markers balanced ($go_begin/$go_end Go, ${yaml_begin:-0}/${yaml_end:-0} YAML); all Go consts inside SECTION blocks."
