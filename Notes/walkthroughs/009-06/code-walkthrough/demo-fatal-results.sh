#!/bin/bash
# Fatal code with usable results: the fake rg emits two valid matches
# (a complete stream), writes "boom" to stderr, and exits 3. vrg shows
# the error overlay over the browse view naming the exit status and
# carrying the stderr line; the first q dismisses to browse, the second
# quits with the fixed fatal status 2. Driven on a real tmux PTY;
# capture-pane gives the composed screen at each step.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-fatal-results"
SES="vrg9-fatal-results-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f1.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f1.txt"},"lines":{"text":"hit one\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"f1.txt"},"binary_offset":null}}' \
'{"type":"begin","data":{"path":{"text":"f2.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f2.txt"},"lines":{"text":"hit two\n"},"line_number":2,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"f2.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
echo "boom" >&2
exit 3
RG
chmod +x "$W/fakebin/rg"

printf 'hit one\n' >"$W/fixture/f1.txt"
printf 'x\nhit two\n' >"$W/fixture/f2.txt"

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

wait_for "rg failed: exit status 3" # the error overlay is open
wait_for "boom"                     # stderr text is in the diagnostics
tmux capture-pane -p -t "$SES" >"$W/screen-overlay.txt"
tmux send-keys -t "$SES" q          # dismiss the overlay to browse
wait_gone "rg failed: exit status 3"
wait_for "f2.txt"
tmux capture-pane -p -t "$SES" >"$W/screen-browse.txt"
tmux send-keys -t "$SES" q          # quit browse with the fixed status
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- screen with overlay ---"
grep -v '^[[:space:]]*$' "$W/screen-overlay.txt"
echo "--- screen after dismissal ---"
grep -v '^[[:space:]]*$' "$W/screen-browse.txt"
