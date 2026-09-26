#!/bin/bash
# Issue #37 scenario 1 — one oversized match record whose path field was
# parsed before the 64 MiB limit, followed by valid records and a
# summary. Usable results exist, so the warning overlay opens over
# browse carrying BOTH the per-record aggregate and the recovered
# per-path detail; dismissal reveals browse, the exit status is 0, and
# both lines reach the stderr replay. Driven on a real tmux PTY.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-named"
SES="vrg37-named-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}'
# Oversized match record for big.txt — path member precedes the giant
# lines value, so the 64 MiB cut leaves a recoverable path.
printf '%s' '{"type":"match","data":{"path":{"text":"big.txt"},"lines":{"text":"'
head -c 67108865 /dev/zero | tr '\0' 'x'
printf '%s\n' '"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}'
printf '%s\n' '{"type":"summary","data":{}}'
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

wait_for "1 oversized record skipped"            # the aggregate
wait_for "oversized record skipped for big.txt"  # the recovered-path detail
tmux capture-pane -p -t "$SES" >"$W/screen-overlay.txt"
tmux send-keys -t "$SES" q          # dismiss the overlay to browse
wait_gone "oversized record skipped"
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
