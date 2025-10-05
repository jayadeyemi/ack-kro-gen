#!/usr/bin/env bash
#
# go.sh - Convenience script for ack-kro-gen development and validation
#
# This script automates the build-run-validate cycle for the ACK KRO generator.
# It's used for agent-based validation testing, not traditional Go unit tests.
#
# USAGE:
#   ./go.sh                    # Run with defaults
#   OFFLINE=true ./go.sh       # Run in offline mode (no network)
#   CONCURRENCY=8 ./go.sh      # Run with 8 parallel services
#
# ENVIRONMENT VARIABLES:
#   BIN          - CLI binary path (default: ./ack-kro-gen)
#   GRAPHS       - Graph config file (default: graphs.yaml)
#   OUT          - Output directory (default: out)
#   CACHE        - Chart cache directory (default: .cache/charts)
#   CONCURRENCY  - Parallel services (default: 4)
#   LOG_LEVEL    - Log verbosity: debug|info (default: debug)
#   OFFLINE      - Offline mode, no network access (default: false)
#
# WORKFLOW:
#   1. Clean previous outputs from out/ack/
#   2. Display environment configuration
#   3. Build the CLI binary
#   4. Run the generator with configured settings
#   5. Validate outputs:
#      - Check file sizes and counts
#      - Peek at generated content
#      - Grep for KRO RGD structure markers
#
# VALIDATION:
#   Agent-based validation focuses on:
#   - Placeholder substitution correctness
#   - Deterministic resource ordering
#   - Schema reference generation accuracy
#   - Completeness of generated RGDs against chart contents
#   - Valid KRO ResourceGraphDefinition structure
#
# OUTPUT:
#   - Generated RGDs in out/ack/<service>-{crds,ctrl}.yaml
#   - Build and run logs in out/go.log
#
# EXAMPLES:
#   # Quick offline validation with cached charts
#   OFFLINE=true ./go.sh
#
#   # Full online run with maximum concurrency
#   CONCURRENCY=8 OFFLINE=false LOG_LEVEL=info ./go.sh
#
#   # Debug single service generation
#   LOG_LEVEL=debug CONCURRENCY=1 ./go.sh
#
set -euo pipefail

BIN="${BIN:-./ack-kro-gen}"
GRAPHS="${GRAPHS:-graphs.yaml}"
OUT="${OUT:-out}"
CACHE="${CACHE:-.cache/charts}"
CONCURRENCY="${CONCURRENCY:-4}"
LOG_LEVEL="${LOG_LEVEL:-debug}"
OFFLINE="${OFFLINE:-false}"

step() { printf "\n==> %s\n" "$*"; }

step "env"
printf "BIN=%s\nGRAPHS=%s\nOUT=%s\nCACHE=%s\nCONCURRENCY=%s\nLOG_LEVEL=%s\nOFFLINE=%s\n" \
  "$BIN" "$GRAPHS" "$OUT" "$CACHE" "$CONCURRENCY" "$LOG_LEVEL" "$OFFLINE"

step "clean output"
rm -rf "$OUT/ack"
mkdir -p "$OUT/ack"

step "go version"
go version

step "go mod tidy"
go mod tidy

step "build"
go build -o "$BIN" ./cmd/ack-kro-gen

step "list graphs"
grep -nE 'service:|version:|releaseName:|namespace:' "$GRAPHS" || true

step "warm cache check"
ls -lah "$CACHE" || mkdir -p "$CACHE"

step "run generator"
"$BIN" \
  --charts-cache "$CACHE" \
  --offline="$OFFLINE" \
  --graphs "$GRAPHS" \
  --out "$OUT" \
  --concurrency "$CONCURRENCY" \
  --log-level "$LOG_LEVEL" 2>&1 | tee "$OUT/go.log"

step "summarize outputs"
ls -lah "$OUT"/ack || true
wc -c "$OUT"/ack/* || true

step "peek files"
for f in "$OUT"/ack/*; do
  echo "--- $f (head)"
  head -n 20 "$f"
done

step "grep quick sanity"
grep -n 'apiVersion: kro.run/v1alpha1' "$OUT"/ack/* | sed 's/^/ok: /'
grep -n 'kind: ResourceGraphDefinition' "$OUT"/ack/* | sed 's/^/ok: /'

step "done"
