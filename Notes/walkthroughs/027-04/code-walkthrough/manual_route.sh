#!/usr/bin/env bash
# Issue #27 manual route: the real binary against a disposable
# temporary fixture — never a repository file.
#
# Copies the single-match fixture into a mktemp directory and runs
# the whole pty session under a trap that removes the directory on
# exit or interruption. The pty script itself performs the external
# edits the issue exercises — appending lines, deleting the file,
# restoring it — all against the disposable copy only.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
TMP="$(mktemp -d /tmp/vrg-027-fixture.XXXXXX)"

cleanup() {
    rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

cp "$HERE/fixture/solo.txt" "$TMP/"
echo "fixture: <tmp> (solo.txt — one match, one stop)"

python3 "$HERE/pty_reload.py" "$TMP"
