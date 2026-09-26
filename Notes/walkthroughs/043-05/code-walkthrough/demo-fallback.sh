#!/bin/bash
# Issue #43 manual check: a standalone combining mark — a zero-width
# grapheme cluster with no base — paints the recorded fallback cell:
# U+25CC DOTTED CIRCLE plus the mark's own bytes, one real terminal
# cell, highlighted exactly when matched. Driven on a real tmux PTY
# at 80x24; capture-pane gives the composed screen.
#
# Fixture f.txt:
#   line 1: "a" \x01 U+0301 "x" — the mark's standalone cluster follows
#           the ^A escape; display "a^A◌́x", match on the mark's bytes.
#   line 2: U+0301 "lead" — a standalone mark at line start, also a
#           match; display "◌́lead".
#   line 3: "cafe" U+0301 — an ordinary base-plus-mark cluster, a match
#           on "fe◌́" for contrast; display "cafe◌́" unchanged.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-fallback"
rm -rf "$W"
mkdir -p "$W/fakebin" "$W/work"

printf 'a\x01\xcc\x81x\n\xcc\x81lead\ncafe\xcc\x81\n' >"$W/work/f.txt"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
# The raw mark bytes are 0xcc 0x81 (U+0301): line 1 submatch [2,4),
# line 2 [0,2); line 3 matches "fe◌́" at [3,7) inside the base cluster.
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"a\u0001́x\n"},"line_number":1,"submatches":[{"match":{"text":"́"},"start":2,"end":4}]}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"́lead\n"},"line_number":2,"submatches":[{"match":{"text":"́"},"start":0,"end":2}]}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"café\n"},"line_number":3,"submatches":[{"match":{"text":"fé"},"start":3,"end":7}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

fail=0
SES="vrg43-$$"
cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
check() { # check <label> <0|1>
	if [ "$2" -eq 0 ]; then echo "ok: $1"; else echo "FAIL: $1"; fail=1; fi
}

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/work"
PATH="$W/fakebin:\$PATH" "$D/vrg" "◌́|fe◌́" .
echo "\$?" >"$W/exit.txt"
sleep 60
EOF
chmod +x "$W/inner.sh"

tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"
pane() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
panee() { tmux capture-pane -e -p -t "$SES" 2>/dev/null; }
wait_for() {
	for _ in $(seq 1 600); do
		pane | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "timed out waiting for: $1" >&2
	return 1
}
settle() { sleep 0.3; }

wait_for "lead" || exit 1
settle

# --- Phase 1: the fallback paints, mid-line and at line start ---
pane >"$W/pane-initial.txt"
panee >"$W/pane-initial-ansi.txt"
pane | grep -qF 'a^A◌́x'
check "mid-line standalone mark paints a^A◌́x — ◌́ cell, x next" $?
pane | grep -qF '◌́lead'
check "line-start standalone mark paints ◌́lead" $?
pane | grep -qF 'café'
check "ordinary base+mark cluster still paints café" $?
# The current match (line 1's mark) is styled exactly one cell: the
# inverse+underline sequence opens immediately before the cell's
# bytes (E2 97 8C CC 81) and closes immediately after — nothing
# adjacent is styled.
LC_ALL=C grep -qP '\x1b\[4m\x1b\[30m\x1b\[47m\xe2\x97\x8c\xcc\x81\x1b\[0m' "$W/pane-initial-ansi.txt"
check "line-1 match highlight covers exactly the ◌́ fallback cell" $?

# --- Phase 2: n moves the same one-cell highlight to line 2's mark ---
tmux send-keys -t "$SES" n
settle
panee >"$W/pane-after-n-ansi.txt"
# Line 1's mark is now the non-current match — inverse only — and
# still exactly one cell.
LC_ALL=C grep -qP '\x1b\[30m\x1b\[47m\xe2\x97\x8c\xcc\x81\x1b\[0m' "$W/pane-after-n-ansi.txt"
check "line-1 mark keeps the plain-inverse one-cell highlight" $?
row2ansi=$(panee | grep -F 'lead')
echo "$row2ansi" | LC_ALL=C grep -qP '\x1b\[4m\x1b\[30m\x1b\[47m\xe2\x97\x8c\xcc\x81\x1b\[0m'
check "line-2 mark takes the current one-cell highlight" $?

# --- Phase 3: horizontal panning counts the fallback as one cell ---
# w leaves wrap mode; . pans right one column at a time. Cells on
# line 1 are a(0) ^(1) A(2) ◌́(3) x(4): at offset 3 the row's text is
# exactly ◌́x behind a hidden-left indicator; at offset 4 only x
# remains — the fallback spent exactly one cell.
tmux send-keys -t "$SES" w
settle
tmux send-keys -t "$SES" . . .
settle
pane >"$W/pane-offset3.txt"
pane | grep -E '^\s*1[ _*]' | grep -qF '◌́x'
check "offset 3: the fallback cell ◌́ paints first, x next" $?
tmux send-keys -t "$SES" .
settle
pane >"$W/pane-offset4.txt"
row1=$(pane | grep -E '^\s*1[ _*]')
case "$row1" in
*◌*) check "offset 4: only x remains — the fallback was one cell" 1 ;;
*x*)  check "offset 4: only x remains — the fallback was one cell" 0 ;;
*)    check "offset 4: only x remains — the fallback was one cell" 1 ;;
esac

tmux send-keys -t "$SES" q
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
tmux kill-session -t "$SES" 2>/dev/null
echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"

echo "--- pane at startup: fallback cells mid-line and at line start ---"
grep -v '^[[:space:]]*$' "$W/pane-initial.txt" | head -8
echo "--- pane at offset 3 (run-off-edge): ◌́ is a real cell ---"
grep -v '^[[:space:]]*$' "$W/pane-offset3.txt" | head -6
echo "--- pane at offset 4: one more column hides the whole fallback ---"
grep -v '^[[:space:]]*$' "$W/pane-offset4.txt" | head -6
exit $fail
