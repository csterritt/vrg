#!/bin/bash
# Issue #37 scenario 3 — two oversized match records naming the same
# file, once through the `text` encoding and once through `bytes`
# (YmlnLnR4dA== is base64 for big.txt). The raw decoded paths agree, so
# the overlay shows the per-record aggregate `2 oversized records
# skipped` but only ONE `oversized record skipped for big.txt` detail
# line — details deduplicate by raw path, the count never does. Driven
# on a real tmux PTY.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-dedup"
SES="vrg37-dedup-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}'
# First oversized record for big.txt via the `text` path encoding.
printf '%s' '{"type":"match","data":{"path":{"text":"big.txt"},"lines":{"text":"'
head -c 67108865 /dev/zero | tr '\0' 'x'
printf '%s\n' '"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}'
# Second oversized record for the same raw path via `bytes` (base64 of
# big.txt): deduplication keys on the decoded raw path, so this record
# raises the count but adds no second detail line.
printf '%s' '{"type":"match","data":{"path":{"bytes":"YmlnLnR4dA=="},"lines":{"text":"'
head -c 67108865 /dev/zero | tr '\0' 'x'
printf '%s\n' '"},"line_number":2,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}'
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

wait_for "2 oversized records skipped"           # both records counted
wait_for "oversized record skipped for big.txt"  # the single detail line
tmux capture-pane -p -t "$SES" >"$W/screen-overlay.txt"
if [ "$(grep -c 'skipped for big.txt' "$W/screen-overlay.txt")" != 1 ]; then
	echo "FAIL: duplicate per-path detail lines"
	exit 1
fi
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
