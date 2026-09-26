#!/bin/bash
# Issue #23 manual check: zero-width match markers on the real vrg
# binary in a real tmux PTY. `vrg '^' a.txt` paints an inverse cell at
# column 0 of every line — empty lines included — and `vrg '$' a.txt`
# paints one at each line's end-of-line position, including display
# column 3 of the CRLF-terminated "hit\r\n". `n` navigates between the
# marker stops, and run-off-edge panning past a marker sets the left
# gutter "*".
#
# Fixture at 80x24: 7-cell list, 3-cell gutter (1-digit number +
# indicator cell + separator at columns 8-10), 69-cell text area
# (columns 11-79), one reserved indicator column (cell 80).
#   a.txt (4 lines):
#     line 1 = "alpha"    — '^' marker at cell 0; '$' marker at cell 5
#     line 2 = ""         — both markers at cell 0 (marker-only line)
#     line 3 = "hit\r\n"  — '^' at cell 0; '$' on the removed CRLF maps
#                          to display column 3
#     line 4 = "omega"    — '^' at cell 0; '$' at cell 5
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/markers"
rm -rf "$W"
mkdir -p "$W/fixture"

printf 'alpha\n\nhit\r\nomega\n' >"$W/fixture/a.txt"

# run_vrg <pattern> <exitfile>: fresh inner.sh + tmux session.
SES="vrg23-markers-$$"
cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
run_vrg() {
	cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
"$D/vrg" "$1" a.txt
echo "\$?" >"$W/$2"
sleep 30
EOF
	chmod +x "$W/inner.sh"
	tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"
	tmux set-option -t "$SES" window-size manual
}

ESC=$(printf '\033')
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
screen_e() { tmux capture-pane -p -e -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
# Pane line 1 is the filename rule; content rows are pane lines 2..24
# (source line N -> pane line N+1). Columns 1..7 file list, 8..10 the
# gutter (8 number, 9 the indicator cell, 10 separator), 11..79 the
# text area, 80 the reserved indicator column.
# capture-pane trims trailing blanks, so each row is padded back to
# the 80-cell frame before column probes.
row() { screen | sed -n "${1}p" | plain | awk '{printf "%-80s", $0}'; }
row_e() { screen_e | sed -n "${1}p"; }
ind() { row "$1" | cut -c9; }
right() { row "$1" | cut -c80; }
text() { row "$1" | cut -c11-79 | sed 's/ *$//'; }

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
# A marker cell is a space painted in the inverse pair — tmux emits it
# as a contiguous SGR cluster containing 47 immediately before the
# space (e.g. ESC[30m ESC[47m SPACE); the current matched line's
# marker adds the underline attribute (a bare 4 in the same cluster).
sgr_spaces() { row_e "$1" | grep -oE "(${ESC}\[[0-9;]*m)+ "; }
marker() { sgr_spaces "$1" | grep -q 47; }
cur_marker() { sgr_spaces "$1" | grep 47 | grep -qE "\[4m|;4"; }
not_cur_marker() { marker "$1" && ! cur_marker "$1"; }
marker_after() { row_e "$1" | grep -qE "$2(${ESC}\[[0-9;]*m)+ "; }
wait_cur_marker() {
	for _ in $(seq 1 600); do
		cur_marker "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for the current match's marker on row $1"; return 1
}

FAIL=0
check() {
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}
probe() { # probe <desc> <predicate> [args...]
	local desc="$1"; shift
	if "$@"; then echo "ok: $desc"; else echo "FAIL: $desc"; FAIL=1; fi
}
snap() { screen | plain >"$W/$1"; }
key() { tmux send-keys -t "$SES" -l "$1"; }
quit() {
	key q
	for _ in $(seq 1 100); do [ -f "$W/$1" ] && break; sleep 0.05; done
	check "$2 exit status" "$(cat "$W/$1" 2>/dev/null)" "0"
	tmux kill-session -t "$SES" 2>/dev/null
	sleep 0.2
}

# ============ run A: vrg '^' a.txt — a marker at column 0 of every line
run_vrg '^' exit-caret.txt

# --- Startup lands in wrap mode: each row's first text cell is the
# marker — an inverse space — and the rest of the text sits unshifted
# behind it; the empty line paints the marker alone ---
wait_for "lpha" || exit 1
check "wrap '^': line 1's first cell is the marker, 'lpha' unshifted" "$(text 2)" " lpha"
probe "wrap '^': line 1's marker is the current match — inverse + underline" cur_marker 2
check "wrap '^': the empty line 2's marker cell paints alone" "$(text 3)" ""
probe "wrap '^': the empty line 2 carries a marker cell" marker 3
check "wrap '^': line 3 'it' follows the cell-0 marker unshifted" "$(text 4)" " it"
probe "wrap '^': line 3 paints its marker" marker 4
probe "wrap '^': line 4 paints its marker" marker 5
snap screen-01-caret-wrap.txt

# --- n walks the marker stops: each line's cell-0 marker is a
# navigable reveal target — including the empty line's ---
key n
wait_cur_marker 3 || exit 1
echo "ok: n -> line 2's marker is the current match"
key n
wait_cur_marker 4 || exit 1
echo "ok: n -> line 3's marker is the current match"
probe "n: line 1's marker is no longer the current match" 'not_cur_marker' 2
snap screen-02-caret-n.txt

# --- w into run-off-edge, > pans to the maximum offset 4 (the widest
# lines' paintable boundary): every marker at cell 0 is entirely
# hidden left, so every gutter upgrades to "*" — the marker-only line
# included ---
key w
key '>'
wait_row 2 "a" || exit 1
check "offset 4: line 1's marker hidden left -> '*'" "$(ind 2)" "*"
check "offset 4: the marker-only line 2's marker hidden left -> '*'" "$(ind 3)" "*"
check "offset 4: line 3's marker hidden left -> '*'" "$(ind 4)" "*"
check "offset 4: line 4's marker hidden left -> '*'" "$(ind 5)" "*"
check "offset 4: line 1 shows only its last cell" "$(text 2)" "a"
snap screen-03-caret-panned.txt
quit exit-caret.txt "vrg '^'"

# ============ run B: vrg '$' a.txt — a marker at each line's EOL
run_vrg '$' exit-dollar.txt

# --- Wrap mode: the marker cell sits one past each line's last
# character; "hit\r\n"'s terminator-only match lands on display
# column 3; the empty line's marker is its only cell ---
wait_for "alpha" || exit 1
check "wrap '$': line 1's text unchanged" "$(text 2)" "alpha"
probe "wrap '$': line 1's marker paints one cell past 'alpha'" marker_after 2 a
probe "wrap '$': line 1's marker is the current match — inverse + underline" cur_marker 2
probe "wrap '$': the empty line 2's marker is its only cell" marker 3
check "wrap '$': the empty line 2 shows nothing else" "$(text 3)" ""
check "wrap '$': line 3 displays 'hit' — the CRLF is undisplayed" "$(text 4)" "hit"
probe "wrap '$': the terminator-only marker lands on column 3, right after 'hit'" marker_after 4 t
probe "wrap '$': line 4's marker paints one cell past 'omega'" marker_after 5 a
snap screen-04-dollar-wrap.txt

# --- n moves the current match to the next line's marker ---
key n
wait_cur_marker 3 || exit 1
echo "ok: n -> line 2's marker is the current match"

# --- w + > pans to the maximum offset 5 (the end-of-line markers
# extend the widest lines' extent to 6 and are themselves paintable
# boundaries). Lines 1 and 4 keep their markers visible — painted at
# the window's first column — so their gutters show "_" for hidden
# text only; lines 2 and 3's markers are entirely hidden left -> "*" ---
key w
key '>'
wait_col 3 9 "*" || exit 1
check "offset 5: line 1's marker stays visible at the window's edge -> '_'" "$(ind 2)" "_"
probe "offset 5: line 1's marker still paints its cell" marker 2
check "offset 5: the marker-only line 2's marker hidden left -> '*'" "$(ind 3)" "*"
check "offset 5: line 3's terminator marker hidden left -> '*'" "$(ind 4)" "*"
check "offset 5: line 4's marker stays visible -> '_'" "$(ind 5)" "_"
snap screen-05-dollar-panned.txt
quit exit-dollar.txt "vrg '$'"

[ "$FAIL" = 0 ] && echo "demo-markers: all checks passed" || exit 1
