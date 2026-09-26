#!/bin/bash
# Issue #46 manual scenario — drive the Issue #45 tagged program-runner
# control (VRG_TEST_RUN_FINAL_MODEL / VRG_TEST_RUN_ERROR) through a real
# PTY lifecycle in which a diagnostic was collected before the injected
# return, and show the unified shutdown contract:
#
#   - the process exits 2 on every failing return shape;
#   - the terminal is restored (the tmux pane returns to the shell);
#   - stderr carries the session's collected diagnostics in collection
#     order, then — for invalid final models — the invalid-model
#     diagnostic, then the application error exactly once.
#
# The warn-then-block fake rg puts "warn one" into the session
# collection (acknowledged through VRG_TEST_COLLECT_ACK) and stays
# alive so q is a searching-quit whose real tuple would exit 130 — the
# injected overrides are what decide the observed outcome.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-runtime-error"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"
printf 'hit\n' >"$W/fixture/f.txt"

# Warn-then-block rg: "warn one" lands in the session collection and the
# child stays alive, so q is a cancellation (real tuple: exit 130).
cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
echo "warn one" >&2
exec sleep 3600
RG
chmod +x "$W/fakebin/rg"

cd /home/chris/vrg
go build -tags vrg_testhooks -o "$D/vrg-hooks" ./cmd/vrg || exit 1

waitfile() { # path — poll until the file has content
	local deadline=$((SECONDS + 30))
	while [ ! -s "$1" ]; do
		if [ $SECONDS -gt "$deadline" ]; then
			echo "TIMEOUT waiting for $1"
			exit 1
		fi
		sleep 0.1
	done
}

SES="vrg46-$$"

run_shape() { # name, then VAR=value runner-seam pairs
	local name="$1"; shift
	cat >"$W/inner-$name.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
env PATH="$W/fakebin:\$PATH" TERM=xterm-256color \
  VRG_TEST_COLLECT_ACK="$W/ack-$name" VRG_TEST_REAP="$W/reap-$name" $* \
  "$D/vrg-hooks" hit . 2>"$W/err-$name.txt"
echo "\$?" >"$W/exit-$name.txt"
sleep 3
EOF
	tmux new-session -d -s "$SES-$name" -x 100 -y 30 "sh $W/inner-$name.sh"
	waitfile "$W/ack-$name"      # "warn one" was collected before the keypress
	tmux send-keys -t "$SES-$name" q
	waitfile "$W/exit-$name.txt"
	tmux kill-session -t "$SES-$name" 2>/dev/null
	printf -- '-- %s: exit %s, reap: %s\n' "$name" \
		"$(cat "$W/exit-$name.txt")" "$(cat "$W/reap-$name")"
	sed 's/^/   stderr: /' "$W/err-$name.txt"
}

echo "== tagged runner seam: VRG_TEST_RUN_* through a real PTY lifecycle =="
run_shape baseline
run_shape run-error VRG_TEST_RUN_ERROR=boom
run_shape nil-model-error VRG_TEST_RUN_FINAL_MODEL=nil VRG_TEST_RUN_ERROR=boom
run_shape invalid-model-error VRG_TEST_RUN_FINAL_MODEL=invalid VRG_TEST_RUN_ERROR=boom
run_shape nil-model VRG_TEST_RUN_FINAL_MODEL=nil
run_shape invalid-model VRG_TEST_RUN_FINAL_MODEL=invalid
