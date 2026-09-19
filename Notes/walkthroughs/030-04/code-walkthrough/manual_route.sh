#!/usr/bin/env bash
# Issue #30 manual route: the real binary against disposable
# temporary fixtures — never a repository file.
#
# Generates one case directory under a mktemp directory and runs the
# pty session under a trap that removes the directory on exit or
# interruption. The fixture is the issue's own UTF-16 LE recipe —
# printf '\xff\xfeh\0i\0\n\0' — a file rg transcodes to match 'hi'
# while vrg's FileBuffer marks it unsupported.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
TMP="$(mktemp -d /tmp/vrg-030-fixture.XXXXXX)"

cleanup() {
    rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

# The single UTF-16 LE file: BOM FF FE then 'h\0i\0\n\0'. rg's
# default BOM detection transcodes it, so 'hi' still matches and the
# file stays a retained cursor stop; the raw bytes classify it
# unsupported for display.
printf '\xff\xfeh\0i\0\n\0' > "$TMP/u16.txt"

echo "fixture: <tmp> (u16.txt — UTF-16 LE \"hi\\n\")"

python3 "$HERE/pty_unsupported.py" "$TMP"
