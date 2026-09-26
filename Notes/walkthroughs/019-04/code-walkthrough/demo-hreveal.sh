#!/bin/bash
# Issue #19 manual check: minimal horizontal reveal in run-off-edge
# mode — a hidden first-submatch start cell moves the horizontal
# offset by the minimum columns that paint it — on the real vrg
# binary in a real tmux PTY.
#
# Fixture at 80x24: 7-cell list, 4-cell gutter, 68-cell text area,
# 23 content rows, one reserved indicator column.
#   a.txt (12 lines):
#     line 1 = "xxxxxhit"              — match "hit" at cells 5-7
#     line 2 = 300 x's + "hit"         — match at cells 300-302
#     line 3 = 280 x's + "hit"         — match at cells 280-282
#     line 4 = 300 x's + "世"          — match at cells 300-301
#     lines 5-12 ten-cell f-lines.
# Text width 68: the line-2 reveal lands at offset 300 + 1 - 68 = 233
# ('h' on the last text column); the line-4 世 reveal lands at
# 300 + 2 - 68 = 234 so both cells of the glyph paint.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/hreveal"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

{ printf 'xxxxxhit\n'
  printf 'x%.0s' $(seq 1 300); printf 'hit\n'
  printf 'x%.0s' $(seq 1 280); printf 'hit\n'
  printf 'x%.0s' $(seq 1 300); printf '世\n'
  for i in $(seq 5 12); do printf 'f%08d\n' "$i"; done
} >"$W/fixture/a.txt"

L2="$(printf 'x%.0s' $(seq 1 300))hit"
L3="$(printf 'x%.0s' $(seq 1 280))hit"
L4="$(printf 'x%.0s' $(seq 1 300))世"
cat >"$W/fakebin/rg" <<RG
#!/bin/sh
printf '%s\n' \\
'{"type":"begin","data":{"path":{"text":"a.txt"}}}' \\
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"xxxxxhit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":5,"end":8}]}}' \\
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"$L2\n"},"line_number":2,"submatches":[{"match":{"text":"hit"},"start":300,"end":303}]}}' \\
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"$L3\n"},"line_number":3,"submatches":[{"match":{"text":"hit"},"start":280,"end":283}]}}' \\
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"$L4\n"},"line_number":4,"submatches":[{"match":{"text":"世"},"start":300,"end":303}]}}' \\
'{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}' \\
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

SES="vrg19-hreveal-$$"
cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"
tmux set-option -t "$SES" window-size manual

ESC=$(printf '\033')
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
screen_e() { tmux capture-pane -p -e -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
# Pane line 1 is the filename rule; content rows are pane lines 2..24.
# Columns 1..7 are the file list, 8..11 the gutter, 12..79 the text
# (column 80 is the reserved indicator cell).
row() { screen | sed -n "${1}p" | plain; }
gutter() { row "$1" | cut -c8-11; }
text() { row "$1" | cut -c12- | sed 's/ *$//'; }

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 600); do
		screen | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1"; return 1
}
wait_row() { # wait_row <pane line> <exact trimmed text>
	for _ in $(seq 1 600); do
		[ "$(text "$1")" = "$2" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for row $1 to be '$2' (got: $(text "$1" | cut -c1-30))"; return 1
}
# underlined <pane line>: the row carries the current match's
# underline SGR (a 4 inside the parameter list).
underlined() { screen_e | sed -n "${1}p" | grep -q "${ESC}\[[0-9;]*\(4;\|4m\)"; }

FAIL=0
check() {
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}
snap() { screen | plain >"$W/$1"; }

X67="$(printf 'x%.0s' $(seq 1 67))h"
X66="$(printf 'x%.0s' $(seq 1 66))"

# --- Startup lands in wrap mode; w switches to run-off-edge with the
# offset at 0: line 2 leads from cell 0 ---
wait_for "hit" || exit 1
tmux send-keys -t "$SES" w
wait_row 3 "$(printf 'x%.0s' $(seq 1 68))" || exit 1
check "w: run-off-edge, offset 0 — line 2 leads from cell 0" "$(text 3 | cut -c1-10)" "xxxxxxxxxx"
snap screen-01-runoff.txt

# --- n to the line-2 match at cell 300: the reveal moves the offset
# to 300 + 1 - 68 = 233 — the minimum that paints the start cell,
# landing 'h' on the last text column ---
tmux send-keys -t "$SES" n
wait_row 3 "$X67" || exit 1
check "n: far match reveals right — start cell on the last text column" "$(text 3)" "$X67"
underlined 3 || { echo "FAIL: line 2 not the current match"; FAIL=1; }
snap screen-02-n-far.txt

# --- p back to the line-1 match at cell 5: hidden left of the
# window, so the offset moves to exactly the target column ---
tmux send-keys -t "$SES" p
wait_row 2 "hit" || exit 1
check "p: back to cell 5 — offset lands on the target column" "$(text 2)" "hit"
snap screen-03-p-back.txt

# --- n again to line 2: offset back to 233 ---
tmux send-keys -t "$SES" n
wait_row 3 "$X67" || exit 1
check "n: far match again — offset 233" "$(text 3)" "$X67"

# --- n to the line-3 match at cell 280: inside the [233,301) window,
# already painted — the offset does not move while the current-line
# underline does ---
tmux send-keys -t "$SES" n
for _ in $(seq 1 100); do underlined 4 && break; sleep 0.05; done
check "n: visible match — row 2 of the window is unchanged" "$(text 3)" "$X67"
if underlined 4 && ! underlined 3; then
	echo "ok: underline moved to line 3 — the cursor advanced without scrolling"
else
	echo "FAIL: underline did not move to line 3"; FAIL=1
fi
snap screen-04-nomove.txt

# --- n to the line-4 match on 世 at cells 300-301: the cluster-width
# rule moves the offset to 300 + 2 - 68 = 234 so both cells of the
# first glyph paint at the right edge ---
tmux send-keys -t "$SES" n
wait_row 5 "$X66世" || exit 1
check "n: CJK match — both cells of 世 painted at the right edge" "$(text 5)" "$X66世"
snap screen-05-cjk.txt

tmux send-keys -t "$SES" q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "0"

[ "$FAIL" = 0 ] && echo "demo-hreveal: all checks passed" || exit 1
