#!/bin/bash
# Issue #12 manual check: the fake rg reports one match in a 200-line
# file; the real vrg binary runs on a real tmux PTY at 80x24, so the
# content height is 23 rows (24 minus the filename row). The script
# drives the scroll keys and asserts the top/bottom content rows:
# down/up move one rendered row, d/u move floor(23/2)=11, NPage/PPage
# move 23, EOF clamps with the last row at the bottom, and up at BOF
# leaves the frame byte-identical.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-scroll"
SES="vrg12-scroll-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

seq -f 'line-%03g' 1 200 >"$W/fixture/long.txt"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"long.txt"}}}' \
'{"type":"match","data":{"path":{"text":"long.txt"},"lines":{"text":"line-001\n"},"line_number":1,"submatches":[{"match":{"text":"line-001"},"start":0,"end":8}]}}' \
'{"type":"end","data":{"path":{"text":"long.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
PATH="$W/fakebin:\$PATH" "$D/vrg" line .
echo "\$?" >"$W/exit.txt"
sleep 60
EOF
chmod +x "$W/inner.sh"

cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"

screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
# Pane row 1 is the filename rule; content rows are pane lines 2..24.
first_content() { screen | sed -n '2p' | grep -o 'line-[0-9]*' | head -1; }
last_content() { screen | sed -n '24p' | grep -o 'line-[0-9]*' | tail -1; }

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 600); do
		screen | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1"; return 1
}
wait_first() { # wait_first <line-NNN> on the first content row
	for _ in $(seq 1 600); do
		[ "$(first_content)" = "$1" ] && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for first content row $1 (got $(first_content))"; return 1
}

FAIL=0
check() { # check <desc> <got> <want>
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got $2 want $3"; FAIL=1; fi
}

wait_for "line-001" || exit 1
screen >"$W/screen-top.txt"
check "initial top row" "$(first_content)" "line-001"

tmux send-keys -t "$SES" Down
wait_first "line-002" || exit 1
check "Down moves one rendered row" "$(first_content)" "line-002"
screen >"$W/screen-down.txt"

tmux send-keys -t "$SES" Up
wait_first "line-001" || exit 1
check "Up moves one rendered row back" "$(first_content)" "line-001"

tmux send-keys -t "$SES" d
wait_first "line-012" || exit 1
check "d moves half a page (floor(23/2)=11)" "$(first_content)" "line-012"
screen >"$W/screen-half.txt"

tmux send-keys -t "$SES" u
wait_first "line-001" || exit 1
check "u moves half a page back" "$(first_content)" "line-001"

tmux send-keys -t "$SES" NPage
wait_first "line-024" || exit 1
check "pgdn moves a full page (23 rows)" "$(first_content)" "line-024"
screen >"$W/screen-page.txt"

# Seven more pages overshoot EOF: top clamps to 200-23=177, so the
# first content row is line-178 and the last row sits at the bottom.
for _ in 1 2 3 4 5 6 7; do tmux send-keys -t "$SES" NPage; done
wait_first "line-178" || exit 1
tmux send-keys -t "$SES" Down   # one more past the clamp: no-op
sleep 0.2
check "EOF clamp top" "$(first_content)" "line-178"
check "last file row at the bottom" "$(last_content)" "line-200"
screen >"$W/screen-eof.txt"
tmux send-keys -t "$SES" NPage  # already clamped: identical frame
sleep 0.2
screen >"$W/screen-eof2.txt"
if cmp -s "$W/screen-eof.txt" "$W/screen-eof2.txt"; then
	echo "ok: further pgdn at EOF changes nothing"
else
	echo "FAIL: pgdn at EOF moved the view"; FAIL=1
fi

for _ in 1 2 3 4 5 6 7 8; do tmux send-keys -t "$SES" PPage; done
wait_first "line-001" || exit 1
check "pgup returns to the top" "$(first_content)" "line-001"
screen >"$W/screen-bof.txt"
tmux send-keys -t "$SES" Up     # at BOF: no-op
sleep 0.2
screen >"$W/screen-bof2.txt"
if cmp -s "$W/screen-bof.txt" "$W/screen-bof2.txt"; then
	echo "ok: up at the top does nothing"
else
	echo "FAIL: up at BOF moved the view"; FAIL=1
fi

tmux send-keys -t "$SES" q
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- screen at EOF clamp ---"
grep -v '^[[:space:]]*$' "$W/screen-eof.txt"
exit "$FAIL"
