#!/bin/bash
# Damaged stream with usable results: the fake rg emits a valid begin,
# a valid match, a garbage line (malformed), an unknown-type record, a
# valid end, and a summary, then exits 0. vrg skips the malformed
# record, tallies the unknown type, keeps the match, and opens the
# warning overlay over the browse view listing both skip counts; Esc
# dismisses to browse and q quits with status 0 — record
# loss with usable results is nonfatal. Driven on a real tmux PTY;
# capture-pane gives the composed screen at each step.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-damaged-stream"
SES="vrg10-damaged-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f1.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f1.txt"},"lines":{"text":"hit one\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'this is not json at all' \
'{"type":"weird","data":{"x":1}}' \
'{"type":"end","data":{"path":{"text":"f1.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
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

wait_for "1 malformed record skipped"           # the skip tally is in the overlay
wait_for "1 unrecognised record types skipped"  # the unknown tally too
tmux capture-pane -p -t "$SES" >"$W/screen-overlay.txt"
tmux send-keys -t "$SES" Escape                 # dismiss the overlay to browse
wait_gone "malformed record skipped"
wait_for "hit one"
tmux capture-pane -p -t "$SES" >"$W/screen-browse.txt"
tmux send-keys -t "$SES" q                      # quit browse with status 0
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- screen with overlay ---"
grep -v '^[[:space:]]*$' "$W/screen-overlay.txt"
echo "--- screen after dismissal ---"
grep -v '^[[:space:]]*$' "$W/screen-browse.txt"
