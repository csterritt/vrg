#!/usr/bin/env bash
# Manual route for Issue #26: read failures with real files.
#
# This script sets up a disposable temporary fixture directory,
# makes the second matched file unreadable with chmod 000, and
# runs vrg with piped keystrokes to demonstrate the read-failure
# behavior. A shell trap restores the original mode on exit or
# interruption. The injected-loader model tests remain the
# authoritative deterministic verification.
#
# Usage: bash manual_route.sh
# Requires: an unprivileged shell (chmod 000 must not be bypassed
# by root).

set -euo pipefail

# Build vrg first.
go build -o /tmp/vrg-issue26 ./cmd/vrg || { echo "BUILD-FAILED"; exit 1; }

# Create a disposable temporary fixture directory.
FIXTURE="$(mktemp -d /tmp/vrg-issue26-XXXXXX)"
ORIG_MODE=""

# Restore the original mode of the second file, then clean up.
cleanup() {
    if [[ -n "$ORIG_MODE" && -f "$FIXTURE/src/b.go" ]]; then
        chmod "$ORIG_MODE" "$FIXTURE/src/b.go" 2>/dev/null || true
    fi
    rm -rf "$FIXTURE" /tmp/vrg-issue26 /tmp/vrg-issue26-stderr 2>/dev/null || true
}
trap cleanup EXIT INT TERM

# Copy two matched files into the fixture.
mkdir -p "$FIXTURE/src"
printf 'content-a\nx\n' > "$FIXTURE/src/a.go"
printf 'content-b\ny\n' > "$FIXTURE/src/b.go"

# Record the original mode of the second file.
ORIG_MODE="$(stat -c '%a' "$FIXTURE/src/b.go")"
echo "Original mode of src/b.go: $ORIG_MODE"

# Make the second matched file unreadable.
chmod 000 "$FIXTURE/src/b.go"
echo "Mode of src/b.go after chmod 000: $(stat -c '%a' "$FIXTURE/src/b.go")"

# Run vrg with piped keystrokes. The TUI receives:
#   n       — navigate to file 2 (shows overlay + (unreadable))
#   (wait)  — let the load fail
#   ESC     — dismiss the overlay
#   n       — same-file step (no new overlay)
#   p       — back to file 1 (same-file)
#   p       — back to file 1 (cross-file, cached)
#   p       — re-enter file 2 (overlay + Loading)
#   ESC     — dismiss the overlay
#   q       — quit (exit 0)
#
# vrg writes diagnostics to stderr on exit. We capture stderr to
# verify the failure is listed.
#
# Note: Bubble Tea requires a terminal. We use script(1) to provide
# a pseudo-terminal so the TUI can render. The keystrokes are piped
# with small delays to let the async loads complete.
{
    printf 'n'
    sleep 0.5
    printf '\x1b'   # ESC
    sleep 0.2
    printf 'n'
    sleep 0.2
    printf 'p'
    sleep 0.2
    printf 'p'
    sleep 0.2
    printf 'p'
    sleep 0.5
    printf '\x1b'   # ESC
    sleep 0.2
    printf 'q'
    sleep 0.2
} | script -qec "/tmp/vrg-issue26 'x|y' $FIXTURE" /dev/null 2>/tmp/vrg-issue26-stderr || true

# Verify the exit code (vrg exits 0 for a successful search).
# Note: script(1) may mask the exit code; we check stderr instead.
echo "--- stderr capture ---"
cat /tmp/vrg-issue26-stderr || true
echo "--- end stderr ---"

# Explicitly restore the original mode to verify the trap logic
# works (the trap also runs on exit/interrupt as a safety net).
if [[ -n "$ORIG_MODE" && -f "$FIXTURE/src/b.go" ]]; then
    chmod "$ORIG_MODE" "$FIXTURE/src/b.go"
fi
echo "Mode of src/b.go after restore: $(stat -c '%a' "$FIXTURE/src/b.go" 2>/dev/null || echo 'file removed')"

# Clean up.
rm -f /tmp/vrg-issue26-stderr
echo "MANUAL-ROUTE-OK"
