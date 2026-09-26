#!/bin/bash
# Warning with no results: the fake rg writes "warn" to stderr, emits
# a complete summary-only stream, and exits 0. The exit is benign, so
# stderr becomes a warning diagnostic — the warning overlay opens over
# the no-results screen. q dismisses to "No results found"; the second
# q exits with the ordinary no-results status 1. Driven on a real tmux
# PTY.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-warning"
SES="vrg9-warning-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
echo "warn" >&2
printf '%s\n' '{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

printf 'hello\n' >"$W/fixture/plain.txt"

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

wait_for "warn"                   # the warning overlay is open
tmux capture-pane -p -t "$SES" >"$W/screen-overlay.txt"
tmux send-keys -t "$SES" q        # dismiss the warning to no-results
wait_gone "│warn│"
wait_for "No results found"
tmux capture-pane -p -t "$SES" >"$W/screen-noresults.txt"
tmux send-keys -t "$SES" q        # quit with the no-results status
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- screen with warning overlay ---"
grep -v '^[[:space:]]*$' "$W/screen-overlay.txt"
echo "--- screen after dismissal ---"
grep -v '^[[:space:]]*$' "$W/screen-noresults.txt"
