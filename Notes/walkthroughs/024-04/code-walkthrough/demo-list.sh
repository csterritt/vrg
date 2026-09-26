#!/bin/bash
# Issue #24 manual check: file-list width, truncation, the visibility
# toggle, and anchor-through-relayout on the real vrg binary in a real
# tmux PTY.
#
# Fixture at 80x24 (wrap mode): four files with paths longer than 30
# cells each. The list caps at floor(0.40*80) = 32 columns — entries
# truncate to "...basename" — the 5-digit-gutter file "d-..." carries
# 12001 lines.
#
# Manual cases (issue #24):
#   80 cols: list at most 32 columns, "..."-prefixed basenames
#   tab hides -> content widens, same text at top; shift+tab restores
#   a file with five-digit line numbers narrows the list where needed
#   shrink to 30 -> constrained but present; enlarge -> restored
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/list"
rm -rf "$W"
mkdir -p "$W/fixture"

F1="a-long-file-name-for-the-list-width-demo.txt"
F2="b-second-long-name-for-the-width-demo.txt"
F3="c-third-file-name-also-quite-long.txt"
F4="d-wide-gutter-five-digit-line-numbers.txt"

# a: line 1 "MARK one" (the first stop), line 2 a 250-cell line with
# NEEDLE at cells 44-49, then tail lines — enough rows to scroll the
# 24-row pane; the two-digit gutter makes the text area 44 cells, so
# the second scroll step lands the NEEDLE row at the top.
{
	printf 'MARK one\n'
	printf 'x%.0s' $(seq 44); printf 'NEEDLE'
	printf 'x%.0s' $(seq 200); printf '\n'
	awk 'BEGIN{for(i=3;i<=40;i++) printf "tail%02d\n", i}'
} >"$W/fixture/$F1"
printf 'MARK two\nx\nx\n' >"$W/fixture/$F2"
printf 'MARK three\nx\nx\n' >"$W/fixture/$F3"
{
	printf 'MARK four\n'
	awk 'BEGIN{for(i=2;i<=12001;i++) printf "x%05d\n", i}'
} >"$W/fixture/$F4"

SES="vrg24-list-$$"
cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
unset RIPGREP_CONFIG_PATH
"$D/vrg" MARK .
echo "\$?" >"$W/exit.txt"
sleep 30
EOF
chmod +x "$W/inner.sh"
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"
tmux set-option -t "$SES" window-size manual

ESC=$(printf '\033')
export LC_ALL=C.UTF-8
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
# capture-pane trims trailing blanks, so each row is padded back to
# the 80-cell frame before column probes. Column extraction goes
# through awk substr — character-aware under the UTF-8 locale (GNU
# cut -c counts bytes and would split the three-byte ─).
row() { screen | sed -n "${1}p" | plain | awk '{printf "%-80s", $0}'; }
col() { row "$1" | awk -v s="$2" -v n="${3:-1}" '{print substr($0,s,n)}'; }

# The filename rule's first dash sits at column listW+1: dashcol is
# the list width plus one — 33 for a 32-cell list, 1 when hidden.
dashcol() {
	local i c
	for i in $(seq 1 80); do
		c=$(col 1 "$i")
		[ "$c" = "─" ] && { echo "$i"; return; }
	done
	echo 0
}
wait_dash() { # wait_dash <expected column>
	for _ in $(seq 1 600); do
		[ "$(dashcol)" = "$1" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for the rule at column $1 (got $(dashcol))"
	return 1
}
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
probe() { # probe <desc> <pane row> <startcol> <len> <literal>
	if col "$2" "$3" "$4" | grep -qF "$5"; then echo "ok: $1"; else echo "FAIL: $1"; FAIL=1; fi
}
snap() { screen | plain >"$W/$1"; }
key() { tmux send-keys -t "$SES" "$@"; }
resize() { tmux resize-window -t "$SES" -x "$1" -y 24; }

# ============ 80 columns, wrap mode: the three-term width formula
wait_for "MARK one" || exit 1
wait_dash 33 || exit 1
check "80 cols: the 40%% cap binds — the list is exactly 32 cells" "$(($(dashcol) - 1))" "32"
probe "80 cols: a truncated entry leads with the … marker" 1 1 32 "…"
probe "80 cols: every listed path is left-truncated to its basename tail" 3 1 32 "…"
snap screen-01-80-wrap.txt

# ============ tab hides: same text at the top through the round trip
# Two downs scroll the top to cells 44-87 of file a's wrapped line 2 —
# mid-line, leading with NEEDLE.
key Down; key Down
for _ in $(seq 1 600); do
	T=$(col 2 37 44 | sed 's/ *$//')
	case "$T" in NEEDLE*) break ;; esac
	sleep 0.05
done
check "two downs: the top row leads with NEEDLE — a mid-line cell 44" "${T:0:6}" "NEEDLE"
BEFORE=$(col 2 37 44 | sed 's/ *$//')

key Tab
wait_dash 1 || exit 1
check "tab: the list is hidden — the rule runs from column 1" "$(($(dashcol) - 1))" "0"
HIDDEN=$(col 2 5 76 | sed 's/ *$//')
case "$HIDDEN" in
	*NEEDLE*) echo "ok: tab: the widened top row still shows NEEDLE" ;;
	*) echo "FAIL: tab: top row lost NEEDLE -> $HIDDEN"; FAIL=1 ;;
esac
snap screen-02-hidden.txt

key BTab
wait_dash 33 || exit 1
check "shift+tab: the list is restored at 32 cells" "$(($(dashcol) - 1))" "32"
RESTORED=$(col 2 37 44 | sed 's/ *$//')
check "shift+tab: identical text at the top — the anchor held" "$RESTORED" "$BEFORE"
snap screen-03-restored.txt

# ============ the five-digit gutter narrows the list where needed
# (key sends are spaced: an Escape immediately followed by another
# key would coalesce into an alt- sequence in the input parser)
key n; sleep 0.3; key n; sleep 0.3; key n
wait_for "d-wide" || exit 1
key Escape; sleep 0.3
wait_for "x00002" || exit 1 # the wide buffer's content has landed
# On file d (12001 lines): the gutter field is five digits wide — the
# list still caps at 32 here because term 2 binds; narrowing happens
# only where the panel minimum needs it.
check "80 cols on the 5-digit file: the cap still binds at 32" "$(($(dashcol) - 1))" "32"
check "80 cols: the gutter field widened to five digits" "$(col 3 33 7)" "    2  "
snap screen-04-wide-gutter-80.txt

# At 25 columns the panel minimum binds: the 7-cell gutter leaves
# term 3 = 25-(7+10) = 8, under the 40% cap of 10.
resize 25
wait_dash 9 || exit 1
check "25 cols on the 5-digit file: the list narrows to 8" "$(($(dashcol) - 1))" "8"
snap screen-05-narrow-25.txt
# The widths follow the current file's gutter synchronously — no
# text-based wait is needed (the rule truncates the path at 25 cols).
key p
key Escape; sleep 0.3
wait_dash 11 || exit 1
check "25 cols on a 1-digit file: the list widens back to 10" "$(($(dashcol) - 1))" "10"
key n
key Escape; sleep 0.3
wait_dash 9 || exit 1
check "25 cols back on the 5-digit file: narrowed to 8 again" "$(($(dashcol) - 1))" "8"
snap screen-06-narrowed-again.txt

# ============ constrained but present at 30; restored at 80
key w
resize 30
wait_dash 13 || exit 1
check "30 cols run-off-edge: min(longest+2, 12, 12) — a constrained but present list" "$(($(dashcol) - 1))" "12"
probe "30 cols: entries still show …-prefixed basenames" 2 1 12 "…"
snap screen-07-constrained-30.txt
resize 80
wait_dash 33 || exit 1
check "enlarged to 80: the list widens back to 32" "$(($(dashcol) - 1))" "32"
snap screen-08-restored-80.txt

key q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "vrg exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "0"

[ "$FAIL" = 0 ] && echo "demo-list: all checks passed" || exit 1
