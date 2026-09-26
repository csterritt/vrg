#!/bin/bash
# Issue #37 scenario 2 — one oversized match record whose 64 MiB cut
# falls inside the giant lines value BEFORE the path member is ever
# parsed: the record is anonymous, so the only possible line is the
# aggregate. With zero usable results the fatal overlay carries exactly
# `1 oversized record skipped` — never empty, never absent — `q` exits
# with the fixed status 2, and the aggregate reaches the stderr replay.
# Driven on a real tmux PTY.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-anonymous"
SES="vrg37-anon-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
# Oversized match record with path AFTER the giant lines value: the
# 64 MiB limit cuts the lines text, so `data.path` is never parsed.
printf '%s' '{"type":"match","data":{"lines":{"text":"'
head -c 67108865 /dev/zero | tr '\0' 'x'
printf '%s\n' '"},"path":{"text":"late.txt"},"line_number":1,"submatches":[{"match":{"text":"x"},"start":0,"end":1}]}}'
printf '%s\n' '{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$W/fakebin/rg"

printf 'unrelated\n' >"$W/fixture/f.txt"

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

wait_for "1 oversized record skipped"            # the fatal overlay is never empty
if tmux capture-pane -p -t "$SES" | grep -qF "skipped for"; then
	echo "FAIL: anonymous oversized record produced a path detail"
	exit 1
fi
tmux capture-pane -p -t "$SES" >"$W/screen-overlay.txt"
tmux send-keys -t "$SES" q          # fatal overlay: q exits with status 2
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- screen with overlay ---"
grep -v '^[[:space:]]*$' "$W/screen-overlay.txt"
echo "--- stderr replay ---"
cat "$W/replay.txt"
