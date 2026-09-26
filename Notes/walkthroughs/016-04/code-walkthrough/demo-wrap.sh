#!/bin/bash
# Issue #16 manual check: wrap mode and the w toggle. The fake rg
# reports two stops in one 40-line long.txt — line 1 ("hit00001") and
# the 500-cell line 19, whose match starts at display cell 450. At
# 80x24 the layout is: list width 10, gutter 4 (two-digit line numbers
# plus two spaces), so the text width is 66 in wrap mode and 65 in
# run-off-edge (the reserved indicator column). Lines 1..18 are
# rendered rows 0..17 and the long line wraps into rows 18..25; cell
# 450 sits in row 24, so startup shows the lead row and its first
# continuation rows at the bottom of the screen and n reveals the
# hidden match row at floor(23/3) = 7 — top row 17. The real vrg
# binary runs on a real tmux PTY; the script sends real keypresses and
# asserts gutters, alignment, clipping, tab stops, and the revealed
# match row.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/wrap"
SES="vrg16-wrap-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

# The 500-cell line: 450 x's, "hit" (cells 450..452), 47 x's.
LONG="$(printf 'x%.0s' $(seq 1 450))hit$(printf 'x%.0s' $(seq 1 47))"

{ printf 'hit00001\n'
  printf 'a\tb\tc\n'
  for i in $(seq 3 18); do printf 'x%06d\n' "$i"; done
  printf '%s\n' "$LONG"
  for i in $(seq 20 40); do printf 'x%06d\n' "$i"; done
} >"$W/fixture/long.txt"

cat >"$W/fakebin/rg" <<RG
#!/bin/sh
printf '%s\n' \\
'{"type":"begin","data":{"path":{"text":"long.txt"}}}' \\
'{"type":"match","data":{"path":{"text":"long.txt"},"lines":{"text":"hit00001\\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \\
'{"type":"match","data":{"path":{"text":"long.txt"},"lines":{"text":"${LONG}\\n"},"line_number":19,"submatches":[{"match":{"text":"hit"},"start":450,"end":453}]}}' \\
'{"type":"end","data":{"path":{"text":"long.txt"},"binary_offset":null}}' \\
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
PATH="$W/fakebin:\$PATH" "$D/vrg" hit .
echo "\$?" >"$W/exit.txt"
sleep 60
EOF
chmod +x "$W/inner.sh"

cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"

ESC=$(printf '\033')
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
screen_e() { tmux capture-pane -p -e -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
# Pane line 1 is the filename rule; content rows are pane lines 2..24.
# Columns 1..10 are the file list, 11..14 the gutter, 15.. the text.
row() { screen | sed -n "${1}p" | plain; }
row_e() { screen_e | sed -n "${1}p"; }
gutter() { row "$1" | cut -c11-14; }
text() { row "$1" | cut -c15-80 | sed 's/ *$//'; }

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 600); do
		screen | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1"; return 1
}
wait_row() { # wait_row <pane line> <literal>
	for _ in $(seq 1 600); do
		row "$1" | grep -qF "$2" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for $2 on pane line $1 (got: $(row "$1"))"; return 1
}

FAIL=0
check() { # check <desc> <got> <want>
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}
snap() { screen | plain >"$W/$1"; }
X66="$(printf 'x%.0s' $(seq 1 66))"
X65="$(printf 'x%.0s' $(seq 1 65))"

# Startup (wrap mode is the default): the first stop is the visible
# line-1 match, so no scroll. The bottom of the screen already shows
# the long line's lead row (rendered row 18, pane line 20) and its
# first continuation rows behind blank gutters.
wait_for "hit00001" || exit 1
check "startup: file panel" "$(row 1 | sed -n 's/.*── \([^ ]*\) .*/\1/p')" "long.txt"
check "startup: current match" "$(text 2)" "hit00001"
# Line 2's tabs expanded to the eight-column stops: a at text cell 0,
# b at 8, c at 16.
check "startup: tab stops" "$(row 3 | cut -c15-31)" "a       b       c"
# The 500-cell line's lead row (pane line 20): gutter "19  " then 66
# x's — text width is panel minus gutter, no reserved column.
check "startup: lead-row gutter" "$(gutter 20)" "19  "
check "startup: lead-row fill" "$(text 20)" "$X66"
# Continuation rows (pane lines 21..): blank gutter, text aligned
# where the lead row's text began.
check "startup: continuation gutter blank" "$(gutter 21)" "    "
check "startup: continuation aligned" "$(text 21)" "$X66"
check "startup: deeper continuation" "$(text 24)" "$X66"
snap screen-01-startup.txt

# n to the match at cell 450 of the 500-cell line: rendered row 24 is
# hidden below the window, so the reveal lands it at floor(23/3) = 7 —
# top row 17. Pane line 2 is line 18, the match row is a blank-gutter
# continuation row on pane line 9 with "hit" at columns 69..71.
tmux send-keys -t "$SES" n
wait_row 9 "hit" || exit 1
check "n: first content row" "$(text 2)" "x000018"
check "n: lead row of line 19" "$(gutter 3)" "19  "
check "n: match on its own row" "$(row 9 | cut -c69-71)" "hit"
check "n: match row gutter blank" "$(gutter 9)" "    "
row_e 9 | grep -q "${ESC}\[4m" && echo "ok: n: match underlined (current match)" || { echo "FAIL: match row not underlined"; FAIL=1; }
snap screen-02-n-reveal.txt

# w toggles run-off-edge: the reserved indicator column appears (text
# width 65), the 500-cell line becomes one clipped row — pane line 3
# is "19  " + 65 x's — and pane line 4 is line 20, not a continuation.
tmux send-keys -t "$SES" w
wait_row 4 "x000020" || exit 1
check "w: run-off-edge gutter" "$(gutter 3)" "19  "
check "w: clipped to reserved edge" "$(text 3)" "$X65"
check "w: next row is line 20" "$(text 4)" "x000020"
snap screen-03-runoff.txt

# w again restores wrap mode: the lead row fills 66 cells and the next
# row is a blank-gutter continuation again.
tmux send-keys -t "$SES" w
wait_row 4 "xxxx" || exit 1
check "w back: lead-row fill" "$(text 3)" "$X66"
check "w back: continuation gutter blank" "$(gutter 4)" "    "
check "w back: continuation aligned" "$(text 4)" "$X66"
snap screen-04-wrap-again.txt

tmux send-keys -t "$SES" q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "exit status" "$(cat "$W/exit.txt")" "0"
[ "$FAIL" = 0 ] && echo "demo-wrap: all checks passed" || exit 1
