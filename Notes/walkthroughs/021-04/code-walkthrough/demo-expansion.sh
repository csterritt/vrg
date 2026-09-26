#!/bin/bash
# Issue #21 manual check: grapheme-cluster highlight expansion on the
# real vrg binary in a real tmux PTY, with real rg doing the search.
#
#   Run A — pattern is the combining mark bytes \xcc\x81 themselves
#           (never precomposed é — ripgrep does not normalize, so é
#           would not match the decomposed bytes):
#     a.txt = "cafe\xcc\x81\n" — submatch {4,6} is the mark alone;
#             the whole é glyph (e + mark, one cell) must highlight.
#     c.txt = "\xcc\x81abc\n" — a standalone mark at line start gets a
#             provisional fallback cell; that cell must highlight.
#   Run B — pattern 世 over the same directory: only b.txt matches;
#           the two-cell CJK glyph highlights as one unit.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/expansion"
rm -rf "$W"
mkdir -p "$W/fixture"

printf 'cafe\xcc\x81\n' >"$W/fixture/a.txt"
printf 'ab\xe4\xb8\x96cd\n' >"$W/fixture/b.txt"
printf '\xcc\x81abc\n' >"$W/fixture/c.txt"

SES="vrg21-expansion-$$"
cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT

ESC=$(printf '\033')
MARK=$(printf '\xcc\x81')    # U+0301 combining acute, decomposed bytes
CJK=$(printf '\xe4\xb8\x96') # 世

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

# start <pattern> <name>: launch a vrg session over the fixture
start() {
	tmux new-session -d -s "$SES" -x 80 -y 24 \
		"cd '$W/fixture' && env -u RIPGREP_CONFIG_PATH '$D/vrg' '$1' .; echo \"\$?\" >'$W/$2.exit'; sleep 30"
	tmux set-option -t "$SES" window-size manual
}
stop() {
	key q
	for _ in $(seq 1 100); do [ -f "$W/$1.exit" ] && break; sleep 0.05; done
	check "$1 exit status" "$(cat "$W/$1.exit" 2>/dev/null)" "0"
	tmux kill-session -t "$SES" 2>/dev/null
	sleep 0.2
}

# --- Run A: the combining mark's own bytes -------------------------
start "$MARK" runA
wait_for "cafe" || exit 1
# a.txt is the current file: its match is the current match (inverse +
# underline — tmux re-emits it as separate [4m [30m [47m codes). The
# mark's submatch expands over the whole é cluster: the styled run is
# 'e' + the combining bytes, one painted cell.
if row_e 2 | grep -qF "${ESC}[47me${MARK}"; then
	echo "ok: 'café' — the whole é glyph is one styled cluster (match on mark bytes only)"
else
	echo "FAIL: é cluster not styled as one unit"; row_e 2 | cat -v; FAIL=1
fi
check "a.txt row shows café" "$(contains "cafe${MARK}")" "yes"
snap screen-01-a-decomposed.txt

# n to c.txt: the file crossing opens the Issue #15 pop-up for a
# second — let it expire. The standalone mark at line start takes a
# provisional cell on a dotted-circle base — the visible fallback —
# and it is the highlighted cell.
key n
wait_for "abc" || exit 1
sleep 1.2
DOT=$(printf '\xe2\x97\x8c') # ◌ dotted circle
if row_e 2 | grep -qF "${ESC}[47m${DOT}${MARK}"; then
	echo "ok: standalone mark at line start — the ◌́ fallback cell is highlighted"
else
	echo "FAIL: fallback cell not styled"; row_e 2 | cat -v; FAIL=1
fi
check "c.txt row shows the ◌́ fallback cell then abc" "$(contains "${DOT}${MARK}abc")" "yes"
snap screen-02-c-standalone.txt
stop runA

# --- Run B: the CJK glyph ------------------------------------------
start "$CJK" runB
wait_for "ab" || exit 1
# b.txt: '世' occupies cells 2-3; the match covers both — the glyph
# paints as one inverse unit.
if row_e 2 | grep -qF "${ESC}[47m${CJK}"; then
	echo "ok: 世 — both cells of the wide glyph highlighted together"
else
	echo "FAIL: 世 not styled"; row_e 2 | cat -v; FAIL=1
fi
check "b.txt row shows ab世cd" "$(contains "ab${CJK}cd")" "yes"
snap screen-03-b-cjk.txt
stop runB

[ "$FAIL" = 0 ] && echo "demo-expansion: all checks passed" || exit 1
