#!/usr/bin/env bash
set -euo pipefail

BIN="${BIN:-./ack-kro-gen}"
GRAPHS="${GRAPHS:-graphs.yaml}"
OUT="${OUT:-out}"
CACHE="${CACHE:-.cache/charts}"
CONCURRENCY="${CONCURRENCY:-4}"
LOG_LEVEL="${LOG_LEVEL:-debug}"   # accepted, even if not all paths use it yet
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
