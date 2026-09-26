#!/bin/bash
# Issue #44 manual scenario — a fake rg emits a valid begin/match/end
# stream, the summary, then a context record, and exits 0. Under the
# summary-is-final contract (Issue #36 removed the context exemption;
# Issue #44 pinned the regression coverage) the post-summary context is
# a stream-integrity failure: the fatal overlay names the sole
# "record after summary" cause — never silently accepted — the first q
# dismisses to browse, the second quits with the fixed status 2, and
# the same diagnostic lands in the stderr replay (the overlay and the
# replay are one composition). Driven on a real tmux PTY; capture-pane
# gives the composed screen at each step.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-context-after-summary"
SES="vrg44-context-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}' \
'{"type":"context","data":{"path":{"text":"f.txt"},"lines":{"text":"ctx\n"},"line_number":2,"submatches":[]}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

printf 'hit\n' >"$W/fixture/f.txt"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
PATH="$W/fakebin:\$PATH" "$D/vrg" hit . 2>"$W/replay.txt"
echo "\$?" >"$W/exit.txt"
sleep 60
EOF
chmod +x "$W/inner.sh"

cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"

wait_for() { # wait_for <literal> — the composed pane screen
	for _ in $(seq 1 600); do
		tmux capture-pane -p -t "$SES" 2>/dev/null | grep -qF "$1" && return 0
		sleep 0.05
	done
	return 1
}
wait_gone() { # wait_gone <literal> — marker leaves the screen
	for _ in $(seq 1 600); do
		tmux capture-pane -p -t "$SES" 2>/dev/null | grep -qF "$1" || return 0
		sleep 0.05
	done
	return 1
}

wait_for "record after summary"     # the fatal overlay names the cause
if tmux capture-pane -p -t "$SES" | grep -qF "exited with code 0"; then
	echo "FAIL: a process-status line appeared for exit 0"
	exit 1
fi
tmux capture-pane -p -t "$SES" >"$W/screen-overlay.txt"
tmux send-keys -t "$SES" q          # dismiss the overlay to browse
wait_gone "record after summary"
wait_for "f.txt"
tmux capture-pane -p -t "$SES" >"$W/screen-browse.txt"
tmux send-keys -t "$SES" q          # quit browse with the fixed status
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- screen with overlay ---"
grep -v '^[[:space:]]*$' "$W/screen-overlay.txt"
echo "--- screen after dismissal ---"
grep -v '^[[:space:]]*$' "$W/screen-browse.txt"
echo "--- stderr replay ---"
cat "$W/replay.txt"
