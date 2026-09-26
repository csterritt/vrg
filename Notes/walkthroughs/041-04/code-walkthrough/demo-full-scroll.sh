#!/bin/bash
# Issue #41 manual check: a fatal outcome from a fake rg whose stderr
# is longer than the overlay's visible height. Every wrapped row is
# scrollable — the first row shows at open, a bounded down traversal
# reaches the last, an up traversal returns to the first, the browse
# scroll keys u/d/pgup/pgdown are ignored, no ellipsis row substitutes
# for content, and scrolling clamps at both ends. Driven on a real
# tmux PTY at 80x24; capture-pane gives the composed screen.
#
# Route:
#   startup           -> valid one-match stream + 40 stderr lines +
#                        exit 3: the error overlay opens over browse at
#                        the head (interior height 22, 40 rows,
#                        maxScroll 18)
#   u, d, PPage, NPage -> all ignored: the pane is unchanged
#   Down x9            -> mid-scroll: every middle row reachable
#   Down x9            -> the last row err-39 is visible
#   Down x5            -> clamped at the bottom: pane unchanged
#   Up x18             -> back at the first row
#   Up x3              -> clamped at the top: pane unchanged
#   q, q               -> dismiss to browse, then quit with the fixed
#                         fatal status 2
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-full-scroll"
SES="vrg41-full-scroll-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f1.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f1.txt"},"lines":{"text":"hit one\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"f1.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
i=0
while [ "$i" -lt 40 ]; do printf 'err-%02d\n' "$i" >&2; i=$((i+1)); done
exit 3
RG
chmod +x "$W/fakebin/rg"

printf 'hit one\n' >"$W/fixture/f1.txt"

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

pane() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
wait_for() { # wait_for <literal> — the composed pane screen
	for _ in $(seq 1 600); do
		pane | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "timed out waiting for: $1" >&2
	return 1
}
wait_gone() { # wait_gone <literal> — marker leaves the screen
	for _ in $(seq 1 600); do
		pane | grep -qF "$1" || return 0
		sleep 0.05
	done
	echo "timed out waiting for $1 to leave" >&2
	return 1
}
settle() { sleep 0.3; } # let a key's frame paint before capture
same() { cmp -s "$1" "$2"; } # two captures identical

fail=0
check() { # check <label> <0|1>
	if [ "$2" -eq 0 ]; then echo "ok: $1"; else echo "FAIL: $1"; fail=1; fi
}

wait_for "err-00" || exit 1   # the overlay is open at the head
pane >"$W/top.txt"
pane | grep -qF "err-39" && check "tail hidden before scrolling" 1 || check "tail hidden before scrolling" 0

# Browse scroll keys are ignored while the error overlay is open.
for k in u d PPage NPage; do
	tmux send-keys -t "$SES" "$k"
	settle
	pane >"$W/after-$k.txt"
	same "$W/top.txt" "$W/after-$k.txt"
	check "$k ignored (pane unchanged)" $?
done

# A bounded traversal reaches the middle and then the last row.
tmux send-keys -t "$SES" Down Down Down Down Down Down Down Down Down
wait_for "err-30" || exit 1 # a row that was off-screen at the top
settle
pane >"$W/mid.txt"
pane | grep -qF "err-15" && check "middle row err-15 reachable" 0 || check "middle row err-15 reachable" 1
pane | grep -qF "err-00" && check "head scrolled off at mid position" 1 || check "head scrolled off at mid position" 0
pane | grep -qF "…" && check "no ellipsis row in the scrollable set" 1 || check "no ellipsis row in the scrollable set" 0

tmux send-keys -t "$SES" Down Down Down Down Down Down Down Down Down
wait_for "err-39" || exit 1
settle
pane >"$W/bottom.txt"
pane | grep -qF "err-00" && check "head left the frame at the bottom" 1 || check "head left the frame at the bottom" 0
pane | grep -qF "…" && check "no ellipsis row at the bottom" 1 || check "no ellipsis row at the bottom" 0

# Extra downs clamp at the bottom: the pane does not move.
tmux send-keys -t "$SES" Down Down Down Down Down
settle
pane >"$W/bottom-clamp.txt"
same "$W/bottom.txt" "$W/bottom-clamp.txt"
check "scroll clamped at the bottom" $?

# The same traversal back up returns to the first row; extra ups clamp.
tmux send-keys -t "$SES" Up Up Up Up Up Up Up Up Up Up Up Up Up Up Up Up Up Up
wait_for "err-00" || exit 1
settle
pane >"$W/top-again.txt"
same "$W/top.txt" "$W/top-again.txt"
check "up traversal returned to the first row" $?
tmux send-keys -t "$SES" Up Up Up
settle
pane >"$W/top-clamp.txt"
same "$W/top.txt" "$W/top-clamp.txt"
check "scroll clamped at the top" $?

# Dismiss to the browse view beneath, then quit at the fixed status 2.
tmux send-keys -t "$SES" q
wait_gone "err-00" || exit 1
wait_for "hit one" || exit 1
pane >"$W/browse.txt"
tmux send-keys -t "$SES" q
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- overlay at open (top) ---"
grep -v '^[[:space:]]*$' "$W/top.txt"
echo "--- overlay mid-scroll ---"
grep -v '^[[:space:]]*$' "$W/mid.txt"
echo "--- overlay at the bottom ---"
grep -v '^[[:space:]]*$' "$W/bottom.txt"
echo "--- browse after dismissal ---"
grep -v '^[[:space:]]*$' "$W/browse.txt"
exit $fail
