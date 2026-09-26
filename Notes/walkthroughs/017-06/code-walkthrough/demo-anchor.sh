#!/bin/bash
# Issue #17 manual check: the logical anchor through rewrap, wrap
# toggle, and resize, the lossy EOF clamp, and responsiveness while a
# ~50 MB rewrap is in flight. Two tmux sessions drive the real vrg
# binary on real PTYs; `window-size manual` plus `resize-window`
# deliver real SIGWINCH resizes to the detached panes.
#
# Part A — big.txt (90 lines, so a two-digit gutter and a 9-cell list):
#   line 1     "hit00001" (the one stop)
#   lines 2-40 short t-lines
#   line 41    600 x's with "MARKER" at display cells 268..273 — at
#              text width 67 that offset is a wrap-row boundary
#   lines 42-89 short u-lines
#   line 90    a 300-cell last line
# At 80x24: list 9, gutter 4, text 67, content height 23. The long
# line is rendered rows 40..48; the MARKER row is 44. Total 102 rows,
# maxTop 79.
#
# Part B — huge.txt (~50 MB, ~530k lines): rapid resizes queue keyed
# layout requests; ctrl+c mid-rewrap must exit promptly with 130.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/anchor"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

LONG="$(printf 'x%.0s' $(seq 1 268))MARKER$(printf 'x%.0s' $(seq 1 326))"
LAST="$(printf 'z%.0s' $(seq 1 300))"

{ printf 'hit00001\n'
  for i in $(seq 2 40); do printf 't%06d\n' "$i"; done
  printf '%s\n' "$LONG"
  for i in $(seq 42 89); do printf 'u%06d\n' "$i"; done
  printf '%s\n' "$LAST"
} >"$W/fixture/big.txt"

cat >"$W/fakebin/rg" <<RG
#!/bin/sh
printf '%s\n' \\
'{"type":"begin","data":{"path":{"text":"big.txt"}}}' \\
'{"type":"match","data":{"path":{"text":"big.txt"},"lines":{"text":"hit00001\\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \\
'{"type":"end","data":{"path":{"text":"big.txt"},"binary_offset":null}}' \\
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
PATH="$W/fakebin:\$PATH" "$D/vrg" hit .
echo "\$?" >"$W/exit.txt"
sleep 30
EOF
chmod +x "$W/inner.sh"

SES="vrg17-anchor-$$"
cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"
tmux set-option -t "$SES" window-size manual

ESC=$(printf '\033')
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
# Pane line 1 is the filename rule; content rows are pane lines 2..24.
# Columns 1..9 are the file list, 10..13 the gutter, 14.. the text.
row() { screen | sed -n "${1}p" | plain; }
gutter() { row "$1" | cut -c10-13; }
text() { row "$1" | cut -c14- | sed 's/ *$//'; }
size() { tmux resize-window -t "$SES" -x "$1" -y "$2"; }

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 600); do
		screen | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1"; return 1
}
wait_gutter() { # wait_gutter <pane line> <gutter digits, e.g. 72 or ''>
	for _ in $(seq 1 600); do
		[ "$(gutter "$1" | tr -d ' ')" = "$2" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for gutter '$2' on pane line $1 (got: [$(gutter "$1")] $(text "$1" | cut -c1-20))"; return 1
}
wait_text() { # wait_text <pane line> <1-based col in text> <literal>
	local end=$(($2 + ${#3} - 1))
	for _ in $(seq 1 600); do
		[ "$(text "$1" | cut -c"$2-$end")" = "$3" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for '$3' at pane line $1 col $2 (got: $(text "$1" | cut -c1-30))"; return 1
}

FAIL=0
check() {
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}
snap() { screen | plain >"$W/$1"; }

# --- Startup: the visible line-1 stop, no scroll ---
wait_for "hit00001" || exit 1
check "startup: file panel" "$(row 1 | sed -n 's/.*── \([^ ]*\) .*/\1/p')" "big.txt"
check "startup: top line" "$(text 2)" "hit00001"

# --- Scroll partway into the wrapped long line: top = its MARKER row
# (rendered row 44, leading at display cell 268 of line 41) ---
for _ in $(seq 1 44); do tmux send-keys -t "$SES" Down; done
wait_text 2 1 "MARKER" || exit 1
check "scrolled: MARKER leads the top row" "$(text 2 | cut -c1-6)" "MARKER"
check "scrolled: continuation gutter" "$(gutter 2)" "    "
snap screen-01-marker-top-80.txt

# --- Narrow to 60: text width 47, so the row containing cell 268
# starts at cell 235 — MARKER sits at offset 33 of the top row ---
size 60 24
wait_text 2 34 "MARKER" || exit 1
echo "ok: narrow 60: MARKER still on the top row (at offset 33)"
snap screen-02-marker-top-60.txt

# --- Widen to 100 (text 87: the covering row starts at cell 261,
# MARKER at offset 7) then back to 80 (MARKER leads again) ---
size 100 24
wait_text 2 8 "MARKER" || exit 1
echo "ok: widen 100: MARKER still on the top row (at offset 7)"
size 80 24
wait_text 2 1 "MARKER" || exit 1
check "back to 80: MARKER leads the top row" "$(text 2 | cut -c1-6)" "MARKER"
snap screen-03-marker-top-back-80.txt

# --- w twice: run-off-edge shows line 41 as one clipped row (the
# retained column is invisible but kept); wrap restores it ---
tmux send-keys -t "$SES" w
wait_gutter 2 "41" || exit 1
check "w: run-off-edge top is line 41" "$(gutter 2)" "41  "
check "w: clipped row from cell 0" "$(text 2 | cut -c1-3)" "xxx"
snap screen-04-runoff.txt
tmux send-keys -t "$SES" w
wait_text 2 1 "MARKER" || exit 1
check "w back: MARKER leads the top row" "$(text 2 | cut -c1-6)" "MARKER"
check "w back: continuation gutter" "$(gutter 2)" "    "
snap screen-05-wrap-again.txt

# --- EOF clamp is lossy: scroll to the last page (top row 79, line
# 72); widening to 140 pulls the top up to line 70 — and the clamped
# position survives the round trip back ---
tmux send-keys -t "$SES" PageDown
tmux send-keys -t "$SES" PageDown
wait_gutter 2 "72" || exit 1
echo "ok: EOF: top line is 72 at 80 cols (last full page)"
snap screen-06-eof-80.txt
size 140 24
wait_gutter 2 "70" || exit 1
echo "ok: widen 140: top pulled up to line 70 (EOF clamp)"
snap screen-07-eof-140.txt
size 80 24
wait_gutter 2 "70" || exit 1
echo "ok: narrow back: line 70 stays at top — the old top is lost"
snap screen-08-eof-back-80.txt
size 140 24
wait_gutter 2 "70" || exit 1
echo "ok: widen again: line 70 still at top"

tmux send-keys -t "$SES" q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "part A exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "0"

# --- Part B: ~50 MB file, repeated resizes, ctrl+c mid-rewrap ---
BIG="$W/fixture/huge.txt"
{ printf 'hit'; printf 'a%.0s' $(seq 1 91); printf '\n'
  awk 'BEGIN{for(i=0;i<530000;i++) print "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"}'
} >"$BIG"
ls -l --block-size=M "$BIG" | awk '{print "fixture size:", $5}'

cat >"$W/fakebin/rg" <<RG
#!/bin/sh
printf '%s\n' \\
'{"type":"begin","data":{"path":{"text":"huge.txt"}}}' \\
'{"type":"match","data":{"path":{"text":"huge.txt"},"lines":{"text":"hit\\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \\
'{"type":"end","data":{"path":{"text":"huge.txt"},"binary_offset":null}}' \\
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"
rm -f "$W/exit.txt"

SES="vrg17-huge-$$"
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"
tmux set-option -t "$SES" window-size manual
wait_for "huge.txt" || exit 1
wait_for "hitaaa" || exit 1
snap screen-09-huge-80.txt

# A burst of alternating resizes — each issues a keyed layout request
# for the ~500k-line file while earlier requests go stale — then
# ctrl+c while a rewrap is in flight.
START=$(date +%s)
for i in $(seq 1 12); do
	tmux resize-window -t "$SES" -x 100 -y 30
	tmux resize-window -t "$SES" -x 80 -y 24
done
tmux send-keys -t "$SES" C-c
for _ in $(seq 1 300); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
ELAPSED=$(( $(date +%s) - START ))
check "ctrl+c mid-rewrap: exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "130"
[ "$ELAPSED" -lt 15 ] && echo "ok: resize burst + ctrl+c exited in under 15s" || { echo "FAIL: exit took ${ELAPSED}s"; FAIL=1; }

# Keep the 50 MB fixture out of the artifact set afterwards; every
# other generated file stays in this directory.
rm -f "$BIG"

[ "$FAIL" = 0 ] && echo "demo-anchor: all checks passed" || exit 1
