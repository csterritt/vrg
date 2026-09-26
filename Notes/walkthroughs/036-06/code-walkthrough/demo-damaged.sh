#!/bin/bash
# Issue #36 scenario 3 — a damaged stream plus real child stderr: an
# orphaned end for g.txt, then three post-summary records (a match, a
# malformed line, an unknown type) while rg writes real stderr and
# exits 3. The universal composition shows every component together in
# order — the stderr process component (suppressing the generated
# line), each integrity cause one-per-record, the malformed aggregate,
# and the unknown-type warning — dismissal reveals browse, the fixed
# status is 2, and the same text reaches the stderr replay. Driven on
# a real tmux PTY.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-damaged"
SES="vrg36-damaged-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
echo "rg: something failed" >&2
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"end","data":{"path":{"text":"g.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}' \
'{"type":"match","data":{"path":{"text":"q.txt"},"lines":{"text":"late\n"},"line_number":2,"submatches":[{"match":{"text":"late"},"start":0,"end":4}]}}' \
'this is not json' \
'{"type":"weird","data":{"x":1}}'
exit 3
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

wait_for() {
	for _ in $(seq 1 600); do
		tmux capture-pane -p -t "$SES" 2>/dev/null | grep -qF "$1" && return 0
		sleep 0.05
	done
	return 1
}
wait_gone() {
	for _ in $(seq 1 600); do
		tmux capture-pane -p -t "$SES" 2>/dev/null | grep -qF "$1" || return 0
		sleep 0.05
	done
	return 1
}

wait_for "rg: something failed"     # real stderr — the process component
wait_for "record after summary"     # the post-summary causes
if tmux capture-pane -p -t "$SES" | grep -qF "rg failed"; then
	echo "FAIL: generated process line appeared alongside stderr"
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
