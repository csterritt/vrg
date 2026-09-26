#!/bin/bash
# Issue #38 manual scenario — a search whose matches sit on a line far
# longer than the content panel's usable text width, driven on a real
# tmux PTY at 80x24. With the file list shown (allocated 10 cells for
# "long.txt"), the panel is 70 cells and the run-off-edge text width is
# 66 (panel minus the three-cell gutter minus the one reserved
# right-indicator column). After `w` leaves wrap mode the first needle
# at cell 140 is hidden right, so the reserved column carries `*`;
# eight `>` presses pan right 10 cells each to offset 80, revealing the
# match at the last text cell while the second needle at cell 176 stays
# hidden right. The harness then asserts the boundary contract Issue
# #38 pins: no rendered row exceeds the terminal width, panned content
# never bleeds into the file list's columns, and the reserved `*` sits
# at the panel's right edge. `left` hides the list — the panel widens
# to 80 cells, the text width re-measures to 76, and the same
# indicators re-derive against the new geometry; `right` restores the
# list. `q` quits with status 0.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-panel-width"
SES="vrg38-width-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

# Line 1 is 200 cells: 140 x's, "needle" (cells 140-145), 30 y's,
# "needle" (cells 176-181), 18 z's — both matches on one over-wide line.
LONG="$(printf 'x%.0s' $(seq 140))needle$(printf 'y%.0s' $(seq 30))needle$(printf 'z%.0s' $(seq 18))"
printf '%s\nshort\n' "$LONG" >"$W/fixture/long.txt"

cat >"$W/fakebin/rg" <<RG
#!/bin/sh
printf '%s\n' \\
'{"type":"begin","data":{"path":{"text":"long.txt"}}}' \\
'{"type":"match","data":{"path":{"text":"long.txt"},"lines":{"text":"$LONG\\n"},"line_number":1,"submatches":[{"match":{"text":"needle"},"start":140,"end":146},{"match":{"text":"needle"},"start":176,"end":182}]}}' \\
'{"type":"end","data":{"path":{"text":"long.txt"},"binary_offset":null}}' \\
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
PATH="$W/fakebin:\$PATH" "$D/vrg" needle . 2>"$W/replay.txt"
echo "\$?" >"$W/exit.txt"
sleep 60
EOF
chmod +x "$W/inner.sh"

cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"

pane() { tmux capture-pane -p -t "$SES" 2>/dev/null; }

wait_for() {
	for _ in $(seq 1 600); do
		pane | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1" >&2
	exit 1
}
wait_grep() {
	for _ in $(seq 1 600); do
		pane | grep -qE "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: /$1/" >&2
	exit 1
}
send() { tmux send-keys -t "$SES" -- "$1"; }
send_lit() { tmux send-keys -t "$SES" -l -- "$1"; }

# check <file> <list-shown:0|1> — every row fits the terminal; with the
# list shown no content row may paint into its first 10 columns; the
# needle row must carry the reserved `*` at the panel's right edge
# (cell 80 in run-off-edge either way).
check() {
	awk -v shown="$2" '
		NR > 1 && length($0) > 80 { print "OVERWIDE row " NR ": " length($0); bad=1 }
		shown == 1 && NR > 1 {
			s = substr($0, 1, 10); gsub(/ /, "", s)
			if (length(s) > 0) { print "BLEED row " NR ": [" substr($0,1,10) "]"; bad=1 }
		}
		/needle/ && substr($0, 80, 1) != "*" { print "NO-STAR row " NR ": col80=" substr($0,80,1); bad=1 }
		END { exit bad }
	' "$1" || { echo "CHECK FAILED"; cat "$1"; exit 1; }
}

wait_for "long.txt"                       # browse up, wrap mode on
send w                                    # run-off-edge: reserved column on
wait_grep '\*[[:space:]]*$'               # first needle hidden right -> `*` at edge
send_lit ">>>>>>>>"                       # offset 0 -> 80, ten cells per `>`
wait_for "needle"                         # offset 80: first needle paints at last text cell
pane >"$W/screen-panned.txt"
check "$W/screen-panned.txt" 1

send left                                 # hide the file list: panel = terminal width
wait_grep '^1_ '                          # gutter now at column 1, hidden-left `_` visible
pane >"$W/screen-hidden.txt"
check "$W/screen-hidden.txt" 0

send_lit ">"                              # pan tracks the re-measured width
wait_grep '^1_ x{50}needle'               # offset 90: needle lands 50 cells in
pane >"$W/screen-hidden-pan.txt"
check "$W/screen-hidden-pan.txt" 0

send right                                # show the list: panel shrinks back
wait_grep '^ {10}1_ '                     # gutter back at columns 11-13
pane >"$W/screen-shown.txt"
check "$W/screen-shown.txt" 1

send q
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- panned right, list shown (text width 66) ---"
grep -v '^[[:space:]]*$' "$W/screen-panned.txt"
echo "--- list hidden (text width 76) ---"
grep -v '^[[:space:]]*$' "$W/screen-hidden.txt"
echo "--- list hidden, panned once more ---"
grep -v '^[[:space:]]*$' "$W/screen-hidden-pan.txt"
echo "--- list shown again (text width 66) ---"
grep -v '^[[:space:]]*$' "$W/screen-shown.txt"
