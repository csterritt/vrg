#!/bin/bash
# Fatal code with no results: the fake rg emits nothing at all and
# exits 2. With no stderr the app generates a diagnostic naming the
# exit code; the stream is also missing its summary, so the outcome is
# fatal with no usable results — an error overlay alone on a blank
# screen. Esc (or q) exits with the fixed status 2. Driven on a real
# tmux PTY.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-fatal-empty"
SES="vrg9-fatal-empty-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
exit 2
RG
chmod +x "$W/fakebin/rg"

printf 'hit\n' >"$W/fixture/f.txt"

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

wait_for "rg failed: exit status 2" # generated diagnostic names the code
wait_for "missing summary"          # the integrity failure is listed
tmux capture-pane -p -t "$SES" >"$W/screen-overlay.txt"
tmux send-keys -t "$SES" Escape     # Esc exits the fatal-only overlay
for _ in $(seq 1 600); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
echo "--- fatal-only overlay ---"
grep -v '^[[:space:]]*$' "$W/screen-overlay.txt"
