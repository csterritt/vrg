#!/usr/bin/env bash
# Issue #28 manual route: the real binary against disposable
# temporary fixtures — never a repository file.
#
# Generates five case directories under a mktemp directory and runs
# the pty sessions under a trap that removes the directory on exit
# or interruption. The "slow load" is VRG_TEST_LOAD_GATE — a file
# whose existence holds every load command — so the pty driver can
# hold a load deterministically, act while it is held, and release
# it by deleting the file.
set -euo pipefail

HERE="$(cd "$(dirname "$0")" && pwd)"
TMP="$(mktemp -d /tmp/vrg-028-fixture.XXXXXX)"

cleanup() {
    rm -rf "$TMP"
}
trap cleanup EXIT INT TERM

# c1 — the slow first file: 'needle deep 500' on line 500 of 600 and
# a second stop on line 560 for the first-n check.
mkdir -p "$TMP/c1"
{
    for i in $(seq 1 600); do
        case "$i" in
            500) printf 'needle deep %03d\n' "$i" ;;
            560) printf 'needle deep %03d\n' "$i" ;;
            *)   printf 'pad deep %03d\n' "$i" ;;
        esac
    done
} > "$TMP/c1/deep.txt"

# c2 — the visible startup target: 'needle shallow 03' on line 3 of
# 40, with a second stop on line 30.
mkdir -p "$TMP/c2"
{
    for i in $(seq 1 40); do
        case "$i" in
            3)  printf 'needle shallow %02d\n' "$i" ;;
            30) printf 'needle shallow %02d\n' "$i" ;;
            *)  printf 'pad shallow %02d\n' "$i" ;;
        esac
    done
} > "$TMP/c2/shallow.txt"

# c3 — reload + immediate n: 'needle two 10' and 'needle two 50' on
# lines 10 and 50 of 60.
mkdir -p "$TMP/c3"
{
    for i in $(seq 1 60); do
        case "$i" in
            10|50) printf 'needle two %02d\n' "$i" ;;
            *)     printf 'pad two %02d\n' "$i" ;;
        esac
    done
} > "$TMP/c3/two.txt"

# c4 — scrolled-off match + r + n p away-and-back: same two-stop
# shape as c3.
mkdir -p "$TMP/c4"
cp "$TMP/c3/two.txt" "$TMP/c4/away.txt"

# c5 — resize during the load: same deep shape as c1 (one stop is
# enough here).
mkdir -p "$TMP/c5"
cp "$TMP/c1/deep.txt" "$TMP/c5/resize.txt"

echo "fixture: <tmp> (c1/c2/c3/c4/c5 — one file each)"

python3 "$HERE/pty_loadreveal.py" "$TMP"
