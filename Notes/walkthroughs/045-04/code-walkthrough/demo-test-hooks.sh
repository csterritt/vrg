#!/bin/bash
# Issue #45 manual scenario — prove the production binary carries none
# of the VRG_TEST_* hook seams and that the vrg_testhooks build answers
# the program-runner controls at the real program.Run() boundary.
#
# Phase 1 (production): build the untagged binary, arm every manifest
# name — reap/ack side files, held gate fifo, fired trigger fifos,
# runner overrides — and run a real search on a tmux PTY. None alter
# behaviour: browse appears despite the held gate, q quits at 0,
# stderr stays empty, and no side-channel files appear; `strings` finds
# none of the names in the artifact.
#
# Phase 2 (tagged): build with -tags vrg_testhooks, show the manifest
# names present, then drive the runner seam through a PTY lifecycle in
# which a diagnostic was collected before the injected return —
# VRG_TEST_COLLECT_ACK is the handshake — covering the baseline tuple
# and the nil-model and error overrides reaching the actual post-Run()
# branches.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-test-hooks"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin" "$W/fakebin-block"
printf 'hit\n' >"$W/fixture/f.txt"

# Streaming rg for the production run: one match, clean exit.
cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
sleep 0.2
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hit\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
RG

# Warn-then-block rg for the runner demos: "warn one" lands in the
# session collection (acknowledged through VRG_TEST_COLLECT_ACK) and
# the child stays alive so q is a cancellation.
cat >"$W/fakebin-block/rg" <<'RG'
#!/bin/sh
echo "warn one" >&2
exec sleep 3600
RG
chmod +x "$W/fakebin/rg" "$W/fakebin-block/rg"

cd /home/chris/vrg
go build -o "$D/vrg" ./cmd/vrg || exit 1
go build -tags vrg_testhooks -o "$D/vrg-hooks" ./cmd/vrg || exit 1

waitfile() { # path, label — poll until the file has content
	local deadline=$((SECONDS + 30))
	while [ ! -s "$1" ]; do
		if [ $SECONDS -gt "$deadline" ]; then
			echo "TIMEOUT waiting for $1"
			exit 1
		fi
		sleep 0.1
	done
}

SES="vrg45-$$"

echo "== production artifact: strings probe =="
for name in VRG_TEST_REAP VRG_TEST_GATE VRG_TEST_COLLECT_ACK \
	VRG_TEST_FAIL_TRIGGER VRG_TEST_FAIL_DIAGNOSTIC \
	VRG_TEST_DIAGNOSTIC_TRIGGER VRG_TEST_DIAGNOSTIC_TEXT \
	VRG_TEST_RUN_FINAL_MODEL VRG_TEST_RUN_ERROR; do
	if strings "$D/vrg" | grep -q "$name"; then
		echo "PRESENT: $name"
	else
		echo "absent: $name"
	fi
done
found="$(strings "$D/vrg" | grep -c 'VRG_TEST_')"
echo "total VRG_TEST_* strings in the production binary: $found"

echo
echo "== production run: every manifest name armed =="
mkfifo "$W/gate.fifo" "$W/fail.fifo" "$W/diag.fifo"
# Fire the trigger fifos — the opens only ever pair if the binary
# actually reads them.
(timeout 5 sh -c ": > \"$W/fail.fifo\"") &
(timeout 5 sh -c ": > \"$W/diag.fifo\"") &

cat >"$W/inner-prod.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
env \
  VRG_TEST_REAP="$W/reap" \
  VRG_TEST_COLLECT_ACK="$W/ack" \
  VRG_TEST_GATE="$W/gate.fifo" \
  VRG_TEST_FAIL_TRIGGER="$W/fail.fifo" \
  VRG_TEST_FAIL_DIAGNOSTIC="injected failure diagnostic" \
  VRG_TEST_DIAGNOSTIC_TRIGGER="$W/diag.fifo" \
  VRG_TEST_DIAGNOSTIC_TEXT="injected session diagnostic" \
  VRG_TEST_RUN_FINAL_MODEL=nil \
  VRG_TEST_RUN_ERROR="injected runtime error" \
  PATH="$W/fakebin:\$PATH" TERM=xterm-256color \
  "$D/vrg" hit . 2>"$W/err-prod.txt"
echo "\$?" >"$W/exit-prod.txt"
sleep 3
EOF
tmux new-session -d -s "$SES" -x 100 -y 30 "sh $W/inner-prod.sh"
# The gate fifo is never written: a hooked binary would hold the
# searching screen forever; the browse marker appearing proves it is
# ignored.
deadline=$((SECONDS + 30))
until tmux capture-pane -t "$SES" -p 2>/dev/null | grep -q 'f.txt'; do
	if [ $SECONDS -gt "$deadline" ]; then
		echo "TIMEOUT: browse view never appeared (gate held?)"
		tmux capture-pane -t "$SES" -p 2>/dev/null || true
		exit 1
	fi
	sleep 0.2
done
tmux capture-pane -t "$SES" -p >"$W/screen-browse.txt"
grep -m1 'f.txt' "$W/screen-browse.txt" | sed 's/^/browse row: /'
tmux send-keys -t "$SES" q
waitfile "$W/exit-prod.txt" exit
echo "exit status: $(cat "$W/exit-prod.txt")"
echo "stderr bytes: $(wc -c <"$W/err-prod.txt")"
for f in reap ack; do
	if [ -e "$W/$f" ]; then
		echo "SIDE CHANNEL WRITTEN: $f"
	else
		echo "no $f file — hook absent"
	fi
done
tmux kill-session -t "$SES" 2>/dev/null

echo
echo "== tagged artifact: manifest present =="
for name in VRG_TEST_REAP VRG_TEST_GATE VRG_TEST_COLLECT_ACK \
	VRG_TEST_FAIL_TRIGGER VRG_TEST_FAIL_DIAGNOSTIC \
	VRG_TEST_DIAGNOSTIC_TRIGGER VRG_TEST_DIAGNOSTIC_TEXT \
	VRG_TEST_RUN_FINAL_MODEL VRG_TEST_RUN_ERROR; do
	if strings "$D/vrg-hooks" | grep -q "$name"; then
		echo "present: $name"
	else
		echo "MISSING: $name"
	fi
done

run_tagged() { # name then VAR=value pairs for the runner seam
	local name="$1"; shift
	cat >"$W/inner-$name.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
env PATH="$W/fakebin-block:\$PATH" TERM=xterm-256color \
  VRG_TEST_COLLECT_ACK="$W/ack-$name" $* \
  "$D/vrg-hooks" hit . 2>"$W/err-$name.txt"
echo "\$?" >"$W/exit-$name.txt"
sleep 3
EOF
	tmux new-session -d -s "$SES-$name" -x 100 -y 30 "sh $W/inner-$name.sh"
	waitfile "$W/ack-$name" ack # the diagnostic is collected first
	tmux send-keys -t "$SES-$name" q
	waitfile "$W/exit-$name.txt" exit
	printf -- '-- %s: exit %s, stderr: ' "$name" "$(cat "$W/exit-$name.txt")"
	tr '\n' '|' <"$W/err-$name.txt"
	echo
	tmux kill-session -t "$SES-$name" 2>/dev/null
}

echo
echo "== tagged run: VRG_TEST_RUN_* reach the real Run() result branch =="
run_tagged baseline
run_tagged run-error VRG_TEST_RUN_ERROR=boom
run_tagged nil-model-error VRG_TEST_RUN_FINAL_MODEL=nil VRG_TEST_RUN_ERROR=boom
run_tagged nil-model VRG_TEST_RUN_FINAL_MODEL=nil
