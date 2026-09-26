#!/bin/bash
# Issue #11 manual check: the fake rg writes "warn one" to stderr,
# emits a complete one-match stream, and exits 0. The collected stderr
# diagnostic opens the warning overlay over browse; the first q
# dismisses it, the second quits — and after the TUI closes the shell
# shows "warn one" exactly once: the post-restoration stderr replay.
# Driven on a real tmux PTY.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-warn-replay"
SES="vrg11-replay-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
echo "warn one" >&2
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

printf 'hello\n' >"$W/fixture/f.txt"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
printf 'shell: about to run vrg\n'
PATH="$W/fakebin:\$PATH" "$D/vrg" hello .
echo "\$?" >"$W/exit.txt"
printf 'shell: vrg exited\n'
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

wait_for "shell: about to run vrg"  # the pane is live
wait_for "warn one"                 # the warning overlay over browse
tmux capture-pane -p -t "$SES" >"$W/screen-overlay.txt"
tmux send-keys -t "$SES" q          # dismiss the overlay → browse
wait_gone "warn one"
wait_for "hello"
tmux capture-pane -p -t "$SES" >"$W/screen-browse.txt"
tmux send-keys -t "$SES" q          # quit → restore → replay
wait_for "shell: vrg exited"
sleep 0.3                          # let the pane settle post-exit
tmux capture-pane -p -t "$SES" >"$W/screen-after.txt"

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- screen: warning overlay over browse ---"
grep -v '^[[:space:]]*$' "$W/screen-overlay.txt"
echo "--- screen: the shell after vrg exited ---"
grep -v '^[[:space:]]*$' "$W/screen-after.txt"
echo "post-exit 'warn one' lines: $(grep -c 'warn one' "$W/screen-after.txt")"
