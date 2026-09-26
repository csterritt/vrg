#!/usr/bin/env bash
# demo-handshake.sh — Issue #48: the event-acknowledgement seam drives a
# real vrg run deterministically. The vrg_testhooks binary runs on pipes
# inside a disposable directory; a fake rg emits one complete match
# stream and exits; every key is written only after the acknowledgement
# record for the transition it depends on lands in the event file —
# never after a settling delay. Artifacts (the event log and both output
# streams) are copied into manual-handshake/.
set -euo pipefail
cd "$(dirname "$0")"
root=$(git rev-parse --show-toplevel)

work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

# fake rg: one complete one-match stream, then exit 0.
mkdir "$work/fakebin"
cat > "$work/fakebin/rg" <<'RG'
#!/bin/sh
printf '%s\n' \
'{"type":"begin","data":{"path":{"text":"f.txt"}}}' \
'{"type":"match","data":{"path":{"text":"f.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}' \
'{"type":"end","data":{"path":{"text":"f.txt"},"binary_offset":null}}' \
'{"type":"summary","data":{}}'
exit 0
RG
chmod +x "$work/fakebin/rg"

# The load fixture and the hooked binary.
printf 'hello\n' > "$work/f.txt"
go build -tags vrg_testhooks -o "$work/vrg" "$root/cmd/vrg"

# fifo stdin: opening read-write pairs both ends, so keys can be sent on
# demand and vrg's reader never sees EOF until we close.
mkfifo "$work/stdin.fifo"
exec 3<>"$work/stdin.fifo"

EV="$work/events"
cd "$work"
PATH="$work/fakebin:$PATH" VRG_TEST_EVENT_ACK="$EV" TERM=xterm-256color \
    "$work/vrg" foo < "$work/stdin.fifo" > out.log 2> err.log &
pid=$!

# wait_ack <n> <kind> [detail] — bounded condition poll: re-read the
# event file until the n-th matching record has landed. Each iteration
# checks the explicit condition; a missing record fails, never hangs.
wait_ack() {
    local n=$1 k=$2 d=${3:-}
    local i
    for i in $(seq 1 2000); do
        if [ -f "$EV" ] && awk -v k="$k" -v d="$d" '
                $2 == k && (d == "" || index($0, " " k " " d) > 0) { c++ }
                END { exit !(c >= n) }' "$EV"; then
            echo "-- ack: $n x $k $d"
            return 0
        fi
        sleep 0.01
    done
    echo "timed out waiting for $k $d" >&2
    exit 1
}

wait_ack 1 phase searching    # the model entered searching
wait_ack 1 phase browse       # the completion resolved to browse
wait_ack 1 load ok            # the current file's load installed
printf 'h' >&3                # open help — gated on the browse phase
wait_ack 1 key h              # the press was consumed
wait_ack 1 help open          # help owns the keyboard
printf 'q' >&3                # close help — gated on help open
wait_ack 1 help closed        # help released the keyboard
printf 'q' >&3                # quit — gated on help closed
wait_ack 1 quitting 0         # the model committed to quit
wait "$pid"
echo "-- exit status: 0"

echo "== acknowledgement records (per-session monotonic sequence) =="
cat "$EV"
echo "== stderr (empty: no diagnostics collected) =="
cat err.log

mkdir -p "$root/Notes/walkthroughs/048-04/code-walkthrough/manual-handshake"
cp "$EV" out.log err.log "$root/Notes/walkthroughs/048-04/code-walkthrough/manual-handshake/"
