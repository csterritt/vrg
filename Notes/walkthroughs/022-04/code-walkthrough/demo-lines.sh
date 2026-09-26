#!/bin/bash
# Issue #22 manual check: structural line handling on the real vrg
# binary in a real tmux PTY at 80x24.
#
#   Run A — real rg, pattern "hit", over three files:
#     bom.txt  = BOM + "hit\nrest\n" — the BOM paints nothing; the
#                first-line match highlights cells 0-2 right after the
#                gutter.
#     cr.txt   = "a\rb hit\n"        — the standalone CR escapes to
#                ^M inside the line; hit still highlights.
#     crlf.txt = "hit\r\nbye\r\n"    — CRLF terminators never show as
#                ^M; hit highlights on line 1.
#   Run B — the fake-rg harness over a zero-byte a.txt: a valid
#           begin/match(line 1 "foo")/end/summary stream whose match
#           names content the file does not have — the panel is empty,
#           the gutter region is three blank cells, no source rows.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/lines"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/empty" "$W/fakebin"

printf '\xef\xbb\xbfhit\nrest\n' >"$W/fixture/bom.txt"
printf 'a\rb hit\n' >"$W/fixture/cr.txt"
printf 'hit\r\nbye\r\n' >"$W/fixture/crlf.txt"
: >"$W/empty/a.txt" # zero bytes on disk

# The fake rg emits one structurally valid stream claiming a.txt's
# line 1 is "foo\n" with a match at bytes 0-3 — a lie the empty file
# on disk drops, leaving an empty panel.
cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"./a.txt"}}}' \
'{"type":"match","data":{"path":{"text":"./a.txt"},"lines":{"text":"foo\n"},"line_number":1,"submatches":[{"match":{"text":"foo"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"./a.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

cat >"$W/inner-a.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
env -u RIPGREP_CONFIG_PATH "$D/vrg" hit .
echo "\$?" >"$W/runA.exit"
sleep 30
EOF
cat >"$W/inner-b.sh" <<EOF
#!/bin/sh
cd "$W/empty"
PATH="$W/fakebin:\$PATH" "$D/vrg" foo .
echo "\$?" >"$W/runB.exit"
sleep 30
EOF
chmod +x "$W"/inner-a.sh "$W"/inner-b.sh

SES="vrg22-lines-$$"
cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT

ESC=$(printf '\033')

screen()   { tmux capture-pane -p -t "$SES" 2>/dev/null; }
screen_e() { tmux capture-pane -p -e -t "$SES" 2>/dev/null; }
plain()    { sed "s/${ESC}\[[0-9;]*m//g"; }
# Pane line 1 is the filename rule; content rows are pane lines 2..24
# (source line N -> pane line N+1). capture-pane trims trailing
# blanks, so pad rows back to the 80-cell frame before probing.
row()   { screen   | sed -n "${1}p" | plain | awk '{printf "%-80s", $0}'; }
row_e() { screen_e | sed -n "${1}p"; }

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 600); do
		screen | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1"; return 1
}

FAIL=0
check() {
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}
contains() { screen | grep -qF "$1" && echo "yes" || echo "no"; }
snap() { screen | plain >"$W/$1"; }
key() { tmux send-keys -t "$SES" -l "$1"; }

# styled <pane-line> <word>: an SGR code opens directly before the word
# on the row — the word is the start of a styled (highlighted) run.
styled() { row_e "$1" | grep -qE "${ESC}\[[0-9;]*m$2"; }

start() {
	tmux new-session -d -s "$SES" -x 80 -y 24 "$W/$1"
	tmux set-option -t "$SES" window-size manual
}
stop() {
	key q
	for _ in $(seq 1 100); do [ -f "$W/$1.exit" ] && break; sleep 0.05; done
	check "$1 exit status" "$(cat "$W/$1.exit" 2>/dev/null)" "0"
	tmux kill-session -t "$SES" 2>/dev/null
	sleep 0.2
}

# --- Run A: real rg, pattern hit -----------------------------------
start inner-a.sh
# Startup lands on the first stop in raw-path order: ./bom.txt. The
# BOM's three bytes paint nothing — the row opens "1  hit" with hit
# styled — and no BOM bytes reach the pane.
wait_for "hit" || exit 1
check "bom.txt row 1 shows hit right after the gutter" \
	"$(row 2 | sed 's/ *$//' | grep -qF '1  hit' && echo yes || echo no)" "yes"
styled 2 "hit" && echo "ok: bom.txt hit is the styled match at cells 0-2" || { echo "FAIL: bom.txt hit not styled"; row_e 2 | cat -v; FAIL=1; }
check "no visible BOM or fallback cell on the row" \
	"$(row 2 | grep -qF '◌' && echo yes || echo no)" "no"
snap screen-01-bom.txt

# n to cr.txt: the standalone CR is content, escaped ^M; hit still
# lands on its own cells after the escape.
key n
wait_for "a^Mb hit" || exit 1
check "cr.txt row 1 shows the standalone CR as ^M" "$(contains 'a^Mb hit')" "yes"
styled 2 "hit" && echo "ok: cr.txt hit is styled after the ^M escape" || { echo "FAIL: cr.txt hit not styled"; row_e 2 | cat -v; FAIL=1; }
snap screen-02-cr.txt

# n to crlf.txt: CRLF terminators paint nothing — no ^M anywhere —
# and the line-1 match highlights hit.
key n
wait_for "bye" || exit 1
check "crlf.txt shows no ^M" "$(contains '^M')" "no"
check "crlf.txt row 1 is '1  hit'" \
	"$(row 2 | sed 's/ *$//' | grep -qF '1  hit' && echo yes || echo no)" "yes"
check "crlf.txt row 2 is '2  bye'" \
	"$(row 3 | sed 's/ *$//' | grep -qF '2  bye' && echo yes || echo no)" "yes"
styled 2 "hit" && echo "ok: crlf.txt hit is styled at cells 0-2" || { echo "FAIL: crlf.txt hit not styled"; row_e 2 | cat -v; FAIL=1; }
snap screen-03-crlf.txt
stop runA

# --- Run B: fake rg, zero-byte a.txt -------------------------------
start inner-b.sh
wait_for "a.txt" || exit 1
# Wait out the "Loading…" placeholder so the settled empty panel is
# what the probes see.
for _ in $(seq 1 200); do screen | grep -qF "Loading" || break; sleep 0.05; done
# The match record claims line 1 is "foo\n"; the file is zero bytes,
# so there are no source rows: the panel is empty and the gutter is
# three blank cells.
check "empty file: the claimed foo never displays" "$(contains 'foo')" "no"
check "empty file: pane row 2 is blank" "$(row 2 | sed 's/ *$//')" ""
check "empty file: pane row 3 is blank" "$(row 3 | sed 's/ *$//')" ""
check "empty file: pane row 4 is blank" "$(row 4 | sed 's/ *$//')" ""
snap screen-04-empty.txt
stop runB

[ "$FAIL" = 0 ] && echo "demo-lines: all checks passed" || exit 1
