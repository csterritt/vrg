#!/bin/bash
# Issue #14 manual check: the vertical destination reveal. The fake rg
# reports three stops in one 300-line file — long.txt lines 5, 200, and
# 210 — so a single file exercises every placement rule: the startup
# file opens at the top with line 5 already on screen (no scroll), n
# to the hidden line-200 target lands it on zero-based content row
# floor(23/3) = 7 (top 192 at 80x24), n onward to the also-visible
# line-210 match does not scroll, p back to line 200 stays put, and p
# to line 5 is BOF-clamped to the top rather than placed a third down.
# The real vrg binary runs on a real tmux PTY; the script sends real
# keypresses and asserts the current match (inverse + underline), the
# first content row, and the match's row on the pane.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/reveal"
SES="vrg14-reveal-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

{ for i in $(seq 1 300); do
	case "$i" in
	5|200|210) printf 'hit %05d\n' "$i" ;;
	*) printf 'x%06d\n' "$i" ;;
	esac
done; } >"$W/fixture/long.txt"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"long.txt"}}}' \
'{"type":"match","data":{"path":{"text":"long.txt"},"lines":{"text":"hit 00005\n"},"line_number":5,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"match","data":{"path":{"text":"long.txt"},"lines":{"text":"hit 00200\n"},"line_number":200,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"match","data":{"path":{"text":"long.txt"},"lines":{"text":"hit 00210\n"},"line_number":210,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"long.txt"},"binary_offset":null}}' \
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
# Pane line 1 is the filename rule ("-- <path> ---"); content rows are
# pane lines 2..24, so content row r is pane line r+2. The current
# match is inverse + underline.
cur_file() { screen | sed -n '1p' | plain | sed -n 's/.*── \([^ ]*\) .*/\1/p'; }
cur_match() {
	screen_e | while IFS= read -r l; do
		case "$l" in *"${ESC}[4m"*) printf '%s\n' "$l" | plain ;; esac
	done | grep -o 'hit [0-9]*' | head -1
}
first_content() { screen | sed -n '2p' | plain | sed -n 's/.* [0-9]*  //p'; }
match_on_row() { # match_on_row <pane line> — the hit text shown there
	screen | sed -n "${1}p" | plain | grep -o 'hit [0-9]*'
}

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 600); do
		screen | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1"; return 1
}
wait_match() { # wait_match "hit NNNNN" underlined
	for _ in $(seq 1 600); do
		[ "$(cur_match)" = "$1" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for match $1 (got $(cur_match))"; return 1
}

FAIL=0
check() { # check <desc> <got> <want>
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got $2 want $3"; FAIL=1; fi
}
snap() { screen | plain >"$W/$1"; }

# Startup: first visit begins at the top; line 5 is on screen, so the
# reveal does not scroll.
wait_match "hit 00005" || exit 1
check "startup: file panel" "$(cur_file)" "long.txt"
check "startup: current match" "$(cur_match)" "hit 00005"
check "startup: top of file" "$(first_content)" "x000001"
check "startup: match row" "$(match_on_row 6)" "hit 00005"
snap screen-01-start.txt

# n to the hidden line-200 target: reveal lands it on content row 7
# (pane line 9), i.e. top 192 -> first content row is line 193.
tmux send-keys -t "$SES" n
wait_match "hit 00200" || exit 1
check "n hidden target: current match" "$(cur_match)" "hit 00200"
check "n hidden target: first content row" "$(first_content)" "x000193"
check "n hidden target: match one-third down" "$(match_on_row 9)" "hit 00200"
snap screen-02-n-hidden.txt

# n onward to line 210: row 209 is inside the visible window
# 192..214, so the reveal does not scroll — only the underline moves.
tmux send-keys -t "$SES" n
wait_match "hit 00210" || exit 1
check "n on-screen target: current match" "$(cur_match)" "hit 00210"
check "n on-screen target: viewport unmoved" "$(first_content)" "x000193"
snap screen-03-n-visible.txt

# p back to line 200: still visible, still no scroll.
tmux send-keys -t "$SES" p
wait_match "hit 00200" || exit 1
check "p on-screen target: current match" "$(cur_match)" "hit 00200"
check "p on-screen target: viewport unmoved" "$(first_content)" "x000193"
snap screen-04-p-visible.txt

# p to line 5: hidden above the window; the reveal wants top
# 4 - 7 < 0, so the BOF clamp puts the file's top back on screen with
# the match on content row 4 (pane line 6) — near the top, not a
# third down.
tmux send-keys -t "$SES" p
wait_match "hit 00005" || exit 1
check "p to BOF: current match" "$(cur_match)" "hit 00005"
check "p to BOF: first content row" "$(first_content)" "x000001"
check "p to BOF: match near top" "$(match_on_row 6)" "hit 00005"
snap screen-05-p-bof.txt

tmux send-keys -t "$SES" q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "exit status" "$(cat "$W/exit.txt")" "0"
[ "$FAIL" = 0 ] && echo "demo-reveal: all checks passed" || exit 1
