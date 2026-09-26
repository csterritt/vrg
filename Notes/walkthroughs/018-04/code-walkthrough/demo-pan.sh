#!/bin/bash
# Issue #18 manual check: horizontal panning in run-off-edge mode —
# the pan units, the visible-lines extent clamp with its paintable
# boundary, wrap-toggle retention, the file-change reset, and
# grapheme-safe clipping — on the real vrg binary in a real tmux PTY.
#
# Fixture at 80x24: 7-cell list, 4-cell gutter, 68-cell text area,
# 23 content rows.
#   a.txt (46 lines): line 1 "hit-a" (stop); line 2 = 300 cells
#     ("0123456789abcdefghij" + x's); line 3 = "ab" + 世 + 291 c's —
#     295 cells, 世 occupying cells 2-3; lines 4-46 ten-cell s-lines.
#   b.txt (30 lines): line 1 "hit-b" (stop); line 2 = "ab" + 世 +
#     76 x's + 世 — 82 cells, the final cluster at cells 80-81 so the
#     paintable-boundary maximum is 80; lines 3-30 ten-cell t-lines.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/pan"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

{ printf 'hit-a\n'
  printf '0123456789abcdefghij'; printf 'x%.0s' $(seq 1 280); printf '\n'
  printf 'ab世'; printf 'c%.0s' $(seq 1 291); printf '\n'
  for i in $(seq 4 46); do printf 's%09d\n' "$i"; done
} >"$W/fixture/a.txt"
{ printf 'hit-b\n'
  printf 'ab世'; printf 'x%.0s' $(seq 1 76); printf '世\n'
  for i in $(seq 3 30); do printf 't%09d\n' "$i"; done
} >"$W/fixture/b.txt"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"a.txt"}}}' \
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"hit-a\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"a.txt"},"binary_offset":null}}' \
'{"type":"begin","data":{"path":{"text":"b.txt"}}}' \
'{"type":"match","data":{"path":{"text":"b.txt"},"lines":{"text":"hit-b\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"b.txt"},"binary_offset":null}}' \
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

SES="vrg18-pan-$$"
cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"
tmux set-option -t "$SES" window-size manual

ESC=$(printf '\033')
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
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
wait_gutter() { # wait_gutter <pane line> <gutter digits>
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
keys() { for _ in $(seq 1 "$1"); do tmux send-keys -t "$SES" -l "$2"; done; }
nkeys() { for _ in $(seq 1 "$1"); do tmux send-keys -t "$SES" "$2"; done; }
wait_row() { # wait_row <pane line> <exact trimmed text>
	for _ in $(seq 1 600); do
		[ "$(text "$1")" = "$2" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for row $1 to be '$2' (got: $(text "$1" | cut -c1-30))"; return 1
}

# --- Startup: wrap mode on; w switches to run-off-edge, line 2 one row ---
wait_for "hit-a" || exit 1
check "startup: file panel" "$(row 1 | sed -n 's/.*── \([^ ]*\) .*/\1/p')" "a.txt"
tmux send-keys -t "$SES" w
wait_gutter 3 "2" || exit 1
FULL="$(printf '0123456789abcdefghij'; printf 'x%.0s' $(seq 1 48))"
wait_row 3 "$FULL" || exit 1
check "w: run-off-edge line 2 leads from cell 0" "$(text 3 | cut -c1-10)" "0123456789"
snap screen-01-runoff.txt

# --- , at offset 0 is a no-op ---
BEFORE="$(text 3)"
tmux send-keys -t "$SES" -l ','
sleep 0.3
check ", at offset 0 does nothing" "$(text 3)" "$BEFORE"

# --- . pans one column; the text shifts left by one ---
tmux send-keys -t "$SES" -l '.'
wait_text 3 1 "123456789abcd" || exit 1
check ". pans one column" "$(text 3 | cut -c1)" "1"

# --- .. twice more: offset 3 — the window opens inside line 3's 世
# (cells 2-3), so its trailing cell paints blank, never half a glyph ---
keys 2 '.'
wait_text 3 1 "3456789" || exit 1
check "offset 3: line 2 from cell 3" "$(text 3 | cut -c1)" "3"
check "half-clipped 世 paints blank" "$(text 4 | cut -c1-2)" " c"
snap screen-02-halfclip.txt

# --- > pans ten columns: offset 13 — 'd' (cell 13) leads ---
tmux send-keys -t "$SES" -l '>'
wait_text 3 1 "defghij" || exit 1
check "> pans ten columns" "$(text 3 | cut -c1)" "d"

# --- ] pans half the text width: 34 → offset 47, well into the x's ---
tmux send-keys -t "$SES" -l ']'
wait_text 3 1 "xxxx" || exit 1
check "] pans half the text width (34)" "$(text 3 | cut -c1)" "x"

# --- Eight more ] pans would reach 319 but clamp at 299: the widest
# visible line's last cell — and further pans do nothing ---
keys 8 ']'
wait_row 3 "x" || exit 1
check "maximum offset paints only the last cell" "$(text 3)" "x"
MAXROW="$(text 3)"
tmux send-keys -t "$SES" -l '.'
sleep 0.3
check "pans past the maximum do nothing" "$(text 3)" "$MAXROW"
snap screen-03-max.txt

# --- w w: the wrap layout installs (blank-gutter continuations), the
# offset stays dormant, and re-entry re-clamps it against an unchanged
# visible set — 299 survives ---
tmux send-keys -t "$SES" w
for _ in $(seq 1 600); do
	[ -z "$(gutter 4 | tr -d ' ')" ] && [ -n "$(text 4 | tr -d ' ')" ] && break
	sleep 0.05
done
check "w: wrapped continuation shows a blank gutter" "$(gutter 4)" "    "
snap screen-04-wrap.txt
tmux send-keys -t "$SES" w
wait_gutter 4 "3" || exit 1
check "w w: offset retained at the maximum" "$(text 3)" "x"
snap screen-05-retained.txt

# --- down x3 scrolls both long lines out of view: the visible-lines
# extent re-clamps the offset to 9, and scrolling back does not
# restore 299 ---
nkeys 3 Down
wait_gutter 2 "4" || exit 1
check "scroll into short lines re-clamps to 9" "$(text 2)" "4"
snap screen-06-reclamp.txt
nkeys 3 Up
wait_gutter 3 "2" || exit 1
check "scroll back does not restore: offset stays 9" "$(text 3 | cut -c1-11)" "9abcdefghij"
snap screen-07-norestore.txt

# --- n into b.txt resets the offset: line 2 paints from cell 0 ---
tmux send-keys -t "$SES" n
wait_for "hit-b" || exit 1
tmux send-keys -t "$SES" -l ',' # dismisses the file-change pop-up; a no-op pan
wait_text 3 1 "ab" || exit 1
check "n: file change starts at offset 0" "$(text 3 | cut -c1-2)" "ab"
snap screen-08-filechange.txt

# --- ]x3 would reach 102 but clamps at 80 — line 2 ends in 世 at
# cells 80-81, so the maximum is the cluster's start and it paints
# whole. Further pans do nothing ---
keys 3 ']'
wait_row 3 "世" || exit 1
check "maximum leaves the final 世 whole" "$(text 3)" "世"
tmux send-keys -t "$SES" -l '.'
sleep 0.3
check "pans past the boundary do nothing" "$(text 3)" "世"
snap screen-09-cluster-max.txt

tmux send-keys -t "$SES" q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "0"

[ "$FAIL" = 0 ] && echo "demo-pan: all checks passed" || exit 1
