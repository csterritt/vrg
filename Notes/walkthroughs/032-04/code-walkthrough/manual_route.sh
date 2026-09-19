#!/usr/bin/env bash
# Issue #32 manual route: the real binary against disposable
# temporary fixtures — never a repository file.
#
# Generates one case directory under a mktemp directory and runs the
# pty sessions under a trap that removes the directory on exit or
# interruption:
#
#   <tmp>/aaa-readable.txt — 'needle a one' (line 1); the startup file
#   <tmp>/bbb-blocked.txt  — 'needle b one' (line 1); chmod 000 after
#       the index is built so its loads fail
#   <tmp>/bin/rg           — a fake ripgrep that exits 3 with no
#       output, for the fatal-overlay-only sessions
#   <tmp>/gate             — the VRG_TEST_LOAD_GATE file; while it
#       exists every file-load worker holds before the read
#   <tmp>/vrg-stderr.log   — the child's stderr
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
TMP="$(mktemp -d /tmp/vrg-032-fixture.XXXXXX)"

cleanup() {
    chmod 644 "$TMP/bbb-blocked.txt" 2>/dev/null || true
    rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

printf 'needle a one\nplain\n' > "$TMP/aaa-readable.txt"
printf 'needle b one\nplain\n' > "$TMP/bbb-blocked.txt"

mkdir "$TMP/bin"
printf '#!/bin/sh\nexit 3\n' > "$TMP/bin/rg"
chmod +x "$TMP/bin/rg"

echo "fixture: <tmp> (aaa-readable.txt, bbb-blocked.txt — 'needle' matches; bin/rg exits 3)"

python3 "$HERE/pty_precedence.py" "$TMP"
