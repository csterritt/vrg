#!/usr/bin/env bash
# Issue #47 manual route: real read failures for hostile filenames,
# observed as single-line diagnostics in the overlay and in the
# stderr replay.
#
# Two disposable fixture directories are created under /tmp: session
# A's carries four files whose names embed ESC, an invalid UTF-8 byte,
# a newline, and a tab; session B's carries a clean file plus one
# newline-named file. pty_singleline.py drives the testhooks binary on
# a real pty, holding each load at VRG_TEST_LOAD_GATE, removing the
# fixture, and releasing the real os.ReadFile — no chmod denial, so no
# elevated-privilege or ACL environment can turn the "failure" into a
# successful read, and no permission bit needs restoring.
#
# The EXIT trap removes both disposable directories on every exit —
# success or failure — and prints the removal as cleanup evidence.
set -euo pipefail
cd "$(dirname "$0")"

BIN=./vrg-testhooks
if [ ! -x "$BIN" ]; then
    echo "build the testhooks binary first:" >&2
    echo "  go build -tags vrg_testhooks -o Notes/walkthroughs/047-04/code-walkthrough/vrg-testhooks ./cmd/vrg" >&2
    exit 1
fi

TMPA="$(mktemp -d /tmp/vrg-047-a.XXXXXX)"
TMPB="$(mktemp -d /tmp/vrg-047-b.XXXXXX)"
echo "fixture dirs (normalized): <tmp-a> $TMPA  <tmp-b> $TMPB"

cleanup() {
    rm -rf -- "$TMPA" "$TMPB"
    if [ -e "$TMPA" ] || [ -e "$TMPB" ]; then
        echo "CLEANUP FAIL: fixture dirs survive" >&2
        return 1
    fi
    echo "cleanup ok: <tmp-a> and <tmp-b> removed — no hostile filename survives"
}
trap cleanup EXIT

# Hostile names need raw bytes — create the fixtures in python so the
# invalid-UTF-8 name is exact regardless of locale.
python3 - "$TMPA" "$TMPB" <<'PY'
import os, sys
a, b = sys.argv[1], sys.argv[2]
for name in (b"a-esc\x1bfile.txt",   # ESC byte in the name
             b"b-inv\xfffile.txt",   # invalid UTF-8 byte
             b"c-nl\nfile.txt",      # embedded newline
             b"d-tab\tfile.txt"):    # embedded tab
    fd = os.open(os.path.join(os.fsencode(a), name),
                 os.O_WRONLY | os.O_CREAT, 0o644)
    os.write(fd, b"needle hit\n")
    os.close(fd)
for name, body in ((b"aaa.txt", b"needle aaa\n"),
                   (b"zz-nl\nfile.txt", b"needle zz\n")):
    fd = os.open(os.path.join(os.fsencode(b), name),
                 os.O_WRONLY | os.O_CREAT, 0o644)
    os.write(fd, body)
    os.close(fd)
PY

# Listing evidence — %r quoting, so no hostile byte reaches this
# transcript raw, and the directory is normalized for stability.
echo "fixtures created (byte-repr listing):"
python3 - "$TMPA" "$TMPB" <<'PY'
import os, sys
for label, d in (("<tmp-a>", sys.argv[1]), ("<tmp-b>", sys.argv[2])):
    print("  %s:" % label)
    for name in sorted(os.listdir(os.fsencode(d))):
        print("    %r" % name)
PY

python3 pty_singleline.py "$TMPA" "$TMPB"
echo "pty route exit: $?"
