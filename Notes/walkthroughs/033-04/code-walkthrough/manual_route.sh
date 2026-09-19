#!/usr/bin/env bash
# Issue #33 walkthrough driver: disposable fixture, then the pty
# sessions that exercise the too-small gate on a real terminal size
# change. Never touches repository files — the whole tree is mktemp.
set -euo pipefail
HERE="$(cd "$(dirname "$0")" && pwd)"

TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT INT TERM

# Two files with 'needle' matches: enough for the browse view and a
# scrollable help session — no overlays needed on this route.
cat > "$TMP/aaa-one.txt" <<'TXT'
needle a one
filler a2
filler a3
filler a4
filler a5
filler a6
filler a7
filler a8
TXT
cat > "$TMP/bbb-two.txt" <<'TXT'
needle b one
filler b2
filler b3
filler b4
TXT

printf 'fixture: %s (aaa-one.txt, bbb-two.txt — %s matches)\n' \
  "$TMP" "'needle'"
python3 "$HERE/pty_toosmall.py" "$TMP"
