#!/usr/bin/env bash
# Issue #31 manual route: the real binary against disposable
# temporary fixtures — never a repository file.
#
# Generates one case directory under a mktemp directory and runs the
# pty sessions under a trap that removes the directory on exit or
# interruption. Two plain-text files matching 'foo' give the browse
# session two cursor stops; the second session searches a pattern
# nothing matches for the no-results route.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
TMP="$(mktemp -d /tmp/vrg-031-fixture.XXXXXX)"

cleanup() {
    rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

printf 'foo one\nfoo two\n' > "$TMP/aa.txt"
printf 'foo three\n'       > "$TMP/bb.txt"

echo "fixture: <tmp> (aa.txt, bb.txt — 'foo' matches)"

python3 "$HERE/pty_help.py" "$TMP"
