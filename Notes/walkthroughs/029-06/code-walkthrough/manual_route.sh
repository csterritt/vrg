#!/usr/bin/env bash
# Issue #29 manual route: the real binary against disposable
# temporary fixtures — never a repository file.
#
# Generates three case directories under a mktemp directory and runs
# the pty sessions under a trap that removes the directory on exit
# or interruption. Each fixture keeps a '.orig' copy the pty driver
# edits and restores on disk — the file changing under the running
# session is what 'r' reloads.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
TMP="$(mktemp -d /tmp/vrg-029-fixture.XXXXXX)"

cleanup() {
    rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

# c1 — same-length replacement: 'needle one 10' on line 10 of 40;
# the driver rewrites it 'sizzle' between search and reload.
mkdir -p "$TMP/c1"
{
    for i in $(seq 1 40); do
        case "$i" in
            10) printf 'needle one %02d\n' "$i" ;;
            *)  printf 'pad one %02d\n' "$i" ;;
        esac
    done
} > "$TMP/c1/one.txt"
cp "$TMP/c1/one.txt" "$TMP/c1/one.txt.orig"

# c2 — trailing matched lines: 'needle two 05' and 'needle two 60' on
# lines 5 and 60 of 60; the driver truncates the file to 40 lines.
mkdir -p "$TMP/c2"
{
    for i in $(seq 1 60); do
        case "$i" in
            5|60) printf 'needle two %02d\n' "$i" ;;
            *)    printf 'pad two %02d\n' "$i" ;;
        esac
    done
} > "$TMP/c2/two.txt"
cp "$TMP/c2/two.txt" "$TMP/c2/two.txt.orig"

# c3 — revert: same one-stop shape as c1; the driver edits then
# restores the original bytes.
mkdir -p "$TMP/c3"
cp "$TMP/c1/one.txt" "$TMP/c3/revert.txt"
cp "$TMP/c3/revert.txt" "$TMP/c3/revert.txt.orig"

echo "fixture: <tmp> (c1/c2/c3 — one file each)"

python3 "$HERE/pty_stale.py" "$TMP"
