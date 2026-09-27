#!/usr/bin/env bash
#
# scripts/verify.sh — the permanent Issue #50 verification entry point.
#
# Runs the full post-audit gate set in order from a clean checkout and
# fails on the first non-zero step:
#
#   1.  go build ./...
#   2.  go vet ./...
#   3.  go build -tags vrg_testhooks ./cmd/vrg
#   4.  go vet -tags vrg_testhooks ./cmd/vrg
#   5.  go test ./... -count=1
#   6.  CGO_ENABLED=1 go test -race ./... -count=1
#   7.  go test ./cmd/vrg -count=3
#   8.  go mod verify
#   9.  go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./...
#   10. go mod tidy -diff
#
# Prerequisites: the Go toolchain on PATH and a CGO-capable C toolchain
# for the race gate. Gate 9's pinned govulncheck invocation additionally
# requires network access to fetch the tool and the vulnerability
# database, or pre-populated module/tool and vulnerability-database
# caches. An offline clean machine without those caches does not satisfy
# the gate's environmental prerequisites: the script reports that as an
# ENVIRONMENT failure (exit 75), not a repository regression.
#
set -euo pipefail

cd "$(dirname "$0")/.."

TOTAL=10
step=0

gate() {
    step=$((step + 1))
    desc="$1"
    shift
    printf '\n=== gate %d/%d: %s ===\n' "$step" "$TOTAL" "$desc"
    "$@"
    printf '%s\n' "--- gate $step OK: $desc ---"
}

gate "go build ./..." go build ./...
gate "go vet ./..." go vet ./...
gate "go build -tags vrg_testhooks ./cmd/vrg" \
    go build -tags vrg_testhooks ./cmd/vrg
gate "go vet -tags vrg_testhooks ./cmd/vrg" \
    go vet -tags vrg_testhooks ./cmd/vrg
gate "go test ./... -count=1" go test ./... -count=1
gate "CGO_ENABLED=1 go test -race ./... -count=1" \
    env CGO_ENABLED=1 go test -race ./... -count=1
gate "go test ./cmd/vrg -count=3" go test ./cmd/vrg -count=3
gate "go mod verify" go mod verify

step=$((step + 1))
printf '\n=== gate %d/%d: %s ===\n' "$step" "$TOTAL" \
    "go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./..."
if out="$(go run golang.org/x/vuln/cmd/govulncheck@v1.5.0 ./... 2>&1)"; then
    printf '%s\n' "$out"
    printf '%s\n' "--- gate $step OK: govulncheck ---"
else
    rc=$?
    printf '%s\n' "$out"
    # A failure to fetch the tool or the vulnerability database is an
    # unmet environmental prerequisite, not a repository regression.
    if printf '%s' "$out" | grep -qiE 'dial tcp|no such host|i/o timeout|connection refused|network is unreachable|temporary failure|TLS handshake|proxyconnect|tls:|fetch|download|vulndb|lookup.*disabled|module lookup'; then
        printf '%s\n' "ENVIRONMENT FAILURE: govulncheck could not fetch the pinned tool or the vulnerability database — this gate requires network access or warm caches, so this is an unmet prerequisite, not a repository regression" >&2
        exit 75
    fi
    printf '%s\n' "gate $step FAILED (exit $rc): govulncheck reported reachable vulnerabilities or a scan error" >&2
    exit "$rc"
fi

gate "go mod tidy -diff" go mod tidy -diff

printf '\n=== verify.sh: all %d gates green ===\n' "$TOTAL"
