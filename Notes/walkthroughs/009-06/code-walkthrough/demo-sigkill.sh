#!/bin/bash
# SIGKILL mid-stream: the fake rg emits a begin and a match, then kills
# itself with SIGKILL before any end or summary. The process death is
# fatal; the retained match stays usable (marked incomplete — its file
# never closed), so vrg shows the error overlay over browse naming the
# signal plus the missing-end and missing-summary integrity failures.
# Esc dismisses to browse; q quits with the fixed status 2. Driven on a
# real tmux PTY.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-sigkill"
SES="vrg9-sigkill-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f1.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f1.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}'
kill -9 $$
RG
chmod +x "$W/fakebin/rg"

printf 'hit\n' >"$W/fixture/f1.txt"

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

wait_for "rg failed: signal: killed" # the overlay names the signal
wait_for "missing end for f1.txt"    # integrity failures are listed too
wait_for "missing summary"
tmux capture-pane -p -t "$SES" >"$W/screen-overlay.txt"
tmux send-keys -t "$SES" Escape      # Esc dismisses the overlay
wait_gone "rg failed: signal: killed"
wait_for "f1.txt"
tmux capture-pane -p -t "$SES" >"$W/screen-browse.txt"
tmux send-keys -t "$SES" q           # quit browse with the fixed status
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- screen with overlay ---"
grep -v '^[[:space:]]*$' "$W/screen-overlay.txt"
echo "--- screen after dismissal ---"
grep -v '^[[:space:]]*$' "$W/screen-browse.txt"
