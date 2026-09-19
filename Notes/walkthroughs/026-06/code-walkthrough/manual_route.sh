#!/usr/bin/env bash
# Issue #26 manual route: the real binary against a disposable
# temporary fixture — never a repository file.
#
# Copies the two matched fixture files into a mktemp directory,
# records the second file's original mode, and runs the whole pty
# session under a trap that restores the mode and removes the
# directory on exit or interruption. The pty script chmods 000 the
# second file only after the index is built — the search must read it
# while readable so its stops exist; the load failures are purely a
# load-time event.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
TMP="$(mktemp -d /tmp/vrg-026-fixture.XXXXXX)"
ORIG_MODE=""

cleanup() {
    if [ -n "$ORIG_MODE" ] && [ -e "$TMP/bbb-blocked.txt" ]; then
        chmod "$ORIG_MODE" "$TMP/bbb-blocked.txt" 2>/dev/null || true
    fi
    rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

cp "$HERE/fixture/aaa-readable.txt" "$HERE/fixture/bbb-blocked.txt" "$TMP/"
ORIG_MODE="$(stat -c %a "$TMP/bbb-blocked.txt")"
echo "fixture: <tmp> (bbb-blocked.txt mode $ORIG_MODE recorded)"

python3 "$HERE/pty_readfail.py" "$TMP"

# The pty script restores the mode itself; verify nothing is left
# unreadable before the trap removes the directory.
chmod "$ORIG_MODE" "$TMP/bbb-blocked.txt"
if find "$TMP" -type f ! -perm -u+r | grep -q .; then
    echo "FAIL: an unreadable file remains in $TMP"
    exit 1
fi
echo "permissions restored — no file left altered"
