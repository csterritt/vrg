#!/bin/bash
# Issue #20 manual check: hidden-content indicators in run-off-edge
# mode — "_" in a line's gutter when any of its text is hidden left,
# "*" when a match or marker on the line is entirely hidden left, and
# the reserved rightmost column's "*" on the current matched line when
# a match is entirely hidden right — on the real vrg binary in a real
# tmux PTY.
#
# Fixture at 80x24: 7-cell list, 4-cell gutter (2-digit number +
# indicator cell + separator), 68-cell text area, 23 content rows, one
# reserved indicator column (cell 80).
#   a.txt (12 lines):
#     line 1 = "hit" + 150 x's + "far"  — "hit" cells 0-2, "far" 153-155
#     line 2 = "hit" + 150 y's + "far"  — same match positions
#     line 3 = 150 z's + "far"          — "far" cells 150-152
#     line 4 = 5 x's + "mid" + 62 x's + "far"
#                                       — "mid" cells 5-7, "far" 70-72
#     lines 5-12 = nine-cell f-lines.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/indicators"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

X150="$(printf 'x%.0s' $(seq 1 150))"
Y150="$(printf 'y%.0s' $(seq 1 150))"
Z150="$(printf 'z%.0s' $(seq 1 150))"
X62="$(printf 'x%.0s' $(seq 1 62))"
L1="hit${X150}far"
L2="hit${Y150}far"
L3="${Z150}far"
L4="xxxxxmid${X62}far"

printf '%s\n' "$L1" "$L2" "$L3" "$L4" >"$W/fixture/a.txt"
for i in $(seq 5 12); do printf 'f%08d\n' "$i"; done >>"$W/fixture/a.txt"

cat >"$W/fakebin/rg" <<RG
#!/bin/sh
printf '%s\n' \\
'{"type":"begin","data":{"path":{"text":"a.txt"}}}' \\
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"$L1\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3},{"match":{"text":"far"},"start":153,"end":156}]}}' \\
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"$L2\n"},"line_number":2,"submatches":[{"match":{"text":"hit"},"start":0,"end":3},{"match":{"text":"far"},"start":153,"end":156}]}}' \\
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"$L3\n"},"line_number":3,"submatches":[{"match":{"text":"far"},"start":150,"end":153}]}}' \\
'{"type":"match","data":{"path":{"text":"a.txt"},"lines":{"text":"$L4\n"},"line_number":4,"submatches":[{"match":{"text":"mid"},"start":5,"end":8},{"match":{"text":"far"},"start":70,"end":73}]}}' \\
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

SES="vrg20-indicators-$$"
cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"
tmux set-option -t "$SES" window-size manual

ESC=$(printf '\033')
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
screen_e() { tmux capture-pane -p -e -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
# Pane line 1 is the filename rule; content rows are pane lines 2..24
# (source line N -> pane line N+1). Columns 1..7 file list, 8..11 the
# gutter (8-9 number, 10 the indicator cell, 11 separator), 12..79 the
# text area, 80 the reserved indicator column.
# capture-pane trims trailing blanks, so each row is padded back to
# the 80-cell frame before column probes.
row() { screen | sed -n "${1}p" | plain | awk '{printf "%-80s", $0}'; }
ind() { row "$1" | cut -c10; }
right() { row "$1" | cut -c80; }
text() { row "$1" | cut -c12-79 | sed 's/ *$//'; }

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
wait_col() { # wait_col <pane line> <1-based col> <expected char>
	for _ in $(seq 1 600); do
		[ "$(row "$1" | cut -c"$2")" = "$3" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for row $1 col $2 to be '$3'"; return 1
}
# underlined <pane line>: the row carries the current match's
# underline SGR (a 4 inside the parameter list).
underlined() { screen_e | sed -n "${1}p" | grep -q "${ESC}\[[0-9;]*\(4;\|4m\)"; }

FAIL=0
check() {
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}
snap() { screen | plain >"$W/$1"; }
key() { tmux send-keys -t "$SES" -l "$1"; }

X65="$(printf 'x%.0s' $(seq 1 65))"
X60="$(printf 'x%.0s' $(seq 1 60))"
X68="$(printf 'x%.0s' $(seq 1 68))"

# --- Startup lands in wrap mode: no indicators, and no reserved
# column — the wrapped text fills the frame's last cell ---
wait_for "hit" || exit 1
check "wrap: line 1's gutter indicator cell stays blank" "$(ind 2)" " "
check "wrap: no reserved column — text reaches the last cell" "$(right 2)" "x"
snap screen-01-wrap.txt

# --- w into run-off-edge at offset 0: nothing hides left, but the
# current line's second match "far" (cells 153-155) is entirely
# hidden right — the reserved column's "*" ---
key w
# A wrap row's first 68 cells match the run-off-edge text, so wait for
# the reserved column's star — proof the run-off-edge layout installed.
wait_col 2 80 "*" || exit 1
check "offset 0: current line 1 — 'far' hidden right -> reserved '*'" "$(right 2)" "*"
check "offset 0: line 3's 'far' hidden right but it is not current" "$(right 4)" " "
check "offset 0: nothing hidden left — the gutters stay blank" "$(ind 2)" " "
snap screen-02-offset0.txt

# --- > pans ten columns: every line's head hides left. Lines whose
# match is entirely hidden left upgrade "_" to "*"; the others keep
# "_" for their hidden text ---
key '>'
wait_row 2 "$X68" || exit 1
check "offset 10: line 1's 'hit' entirely hidden left -> '*'" "$(ind 2)" "*"
check "offset 10: line 1 still hides 'far' right -> both stars" "$(right 2)" "*"
check "offset 10: line 2's 'hit' hidden left -> '*'" "$(ind 3)" "*"
check "offset 10: line 3 hides only text left -> '_'" "$(ind 4)" "_"
check "offset 10: line 4's 'mid' (cells 5-7) hidden left -> '*'" "$(ind 5)" "*"
check "offset 10: line 5's nine cells all hidden left -> '_'" "$(ind 6)" "_"
check "offset 10: line 3's hidden-right 'far' draws nothing — not current" "$(right 4)" " "
snap screen-03-panned.txt

# --- n to line 2, 3, then 4: each reveal re-pans to the first
# submatch. Line 4's reveal lands the offset on "mid"'s cell 5 ---
key n
key n
key n
wait_row 5 "mid${X62}far" || exit 1
if underlined 5; then echo "ok: n x3 -> line 4 is the current matched line"; else echo "FAIL: line 4 not current"; FAIL=1; fi
check "offset 5: 'mid' and 'far' both fully painted — no stars" "$(right 5)" " "
check "offset 5: cells 0-4 hidden left -> '_'" "$(ind 5)" "_"

# --- . once: 'mid' loses its first cell — partially visible still
# counts as visible, so the gutter stays "_" ---
key .
wait_row 5 "id${X62}far" || exit 1
check "offset 6: 'mid' one cell hidden — partially visible -> '_'" "$(ind 5)" "_"

# --- . twice more: the last cell of 'mid' leaves the window —
# entirely hidden left now -> "*" ---
key .
key .
wait_row 5 "${X62}far" || exit 1
check "offset 8: 'mid' entirely hidden left -> '*'" "$(ind 5)" "*"
snap screen-04-mid-hidden.txt

# --- < pans back to offset 0: 'far' is entirely hidden right on the
# current line -> the reserved column's "*" ---
key '<'
wait_row 5 "xxxxxmid${X60}" || exit 1
check "offset 0: 'far' entirely hidden right on the current line -> '*'" "$(right 5)" "*"

# --- . three times: 'far' pokes one cell into the window at offset 3
# — partially visible -> the reserved cell clears ---
key .
key .
key .
wait_row 5 "xxmid${X62}f" || exit 1
check "offset 3: 'far' one cell painted — partially visible -> blank" "$(right 5)" " "
check "offset 3: 'mid' still fully painted — gutter '_'" "$(ind 5)" "_"
snap screen-05-far-partial.txt

# --- w back to wrap mode: every indicator disappears and the
# reserved column is gone — text occupies the last cell again ---
key w
wait_row 4 "$(printf 'x%.0s' $(seq 1 15))far" || exit 1
check "wrap again: no gutter indicator on any row" "$(ind 2)$(ind 3)$(ind 5)" "   "
check "wrap again: no reserved column — text at the last cell" "$(right 2)" "x"
snap screen-06-wrap.txt

key q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "0"

[ "$FAIL" = 0 ] && echo "demo-indicators: all checks passed" || exit 1
