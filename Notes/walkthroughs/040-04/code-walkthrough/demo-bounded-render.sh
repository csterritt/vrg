#!/bin/bash
# Issue #40 manual scenario — browse stays responsive on a very large
# result set, driven on a real tmux PTY at 80x24 against a fixture whose
# fakebin/rg emits FILES x 10 matched lines (default 10,000 files =
# 100,000 stops):
#
#   Stage 1: the search completes and the first file paints; the whole
#            stop set is materialized exactly once, here, off the
#            keystroke path.
#   Stage 2: a 500-key held-"n" burst (50 file crossings). Under the
#            pre-#40 code every crossing re-scanned the 10k-entry file
#            list for the layout-key width and re-materialized all
#            100k stops inside loadCmd; now each step is bounded. The
#            script times the burst and waits for the landing file's
#            filename rule to paint.
#   Stage 3: a 12x resize burst (SIGWINCH alternates 60x18 / 100x30)
#            followed by a 10-key probe across one more file boundary —
#            resize re-truncation and the next keystroke stay bounded.
#
# Assertions: the rule for the expected landing file appears within a
# generous bound after each burst, and the recorded wall times are
# printed for the document. q exits 0.
#
# Usage: demo-bounded-render.sh [binary] [label] — defaults to the
# freshly built ./vrg; passing ./vrg-before (a HEAD build, pre-#40)
# produces the contrast run. FILES scales the fixture (default 10000);
# STOPS stays 10 so the 500-key burst still lands on f-00050.txt.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
BIN="${1:-$D/vrg}"
LABEL="${2:-current}"
FILES="${FILES:-10000}"
W="$D/manual-bounded-$LABEL"
SES="vrg40-bounded-$LABEL-$$"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

# Fixture + records — 10,000 files of ten "hit" lines each, one begin/
# match*10/end block per file. Records are pre-generated so fakebin/rg
# is a plain cat; file contents match the emitted lines text so stale
# validation passes for every stop.
python3 - "$W" "$FILES" <<'PY'
import json, os, sys
W = sys.argv[1]
files, stops = int(sys.argv[2]), 10
with open(os.path.join(W, "records.jsonl"), "w") as out:
    for i in range(files):
        name = f"f-{i:05d}.txt"
        with open(os.path.join(W, "fixture", name), "w") as f:
            f.write("hit\n" * stops)
        out.write(json.dumps({"type": "begin", "data": {"path": {"text": name}}}) + "\n")
        for j in range(1, stops + 1):
            out.write(json.dumps({"type": "match", "data": {
                "path": {"text": name},
                "lines": {"text": "hit\n"},
                "line_number": j,
                "submatches": [{"match": {"text": "hit"}, "start": 0, "end": 3}],
            }}) + "\n")
        out.write(json.dumps({"type": "end", "data": {"path": {"text": name}, "binary_offset": None}}) + "\n")
    out.write('{"type":"summary","data":{}}\n')
print(f"fixture: {files} files x {stops} stops = {files * stops} matched lines")
PY

cat >"$W/fakebin/rg" <<RG
#!/bin/sh
cat "$W/records.jsonl"
exit 0
RG
chmod +x "$W/fakebin/rg"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
PATH="$W/fakebin:\$PATH" "$BIN" hit . 2>"$W/replay.txt"
echo "\$?" >"$W/exit.txt"
sleep 60
EOF
chmod +x "$W/inner.sh"

cleanup() { tmux kill-session -t "$SES" 2>/dev/null; }
trap cleanup EXIT
tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"

pane() { tmux capture-pane -p -t "$SES" 2>/dev/null; }

wait_for() {
	for _ in $(seq 1 600); do
		pane | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "timeout waiting for: $1" >&2
	pane >&2
	exit 1
}

elapsed() { awk "BEGIN{printf \"%.2f\", ($1 - $2) / 1e9}"; }

# --- Stage 1: search completes, first file paints --------------------
wait_for "── f-00000.txt"
sleep 0.3
pane >"$W/stage1.txt"
echo "stage 1 [$LABEL]: whole index materialized once at search"
echo "         completion; first file painted"

# --- Stage 2: 500-key held-n burst = 50 file crossings ----------------
# 500 n's land on f-00050.txt stop 1 (10 stops per file). Under the old
# shape each of the 50 crossings re-scanned 10k list entries and copied
# all 100k stops; the whole-stop materialization also hid inside the
# loadCmd closure Update() issues.
T0=$(date +%s%N)
tmux send-keys -t "$SES" "$(printf 'n%.0s' $(seq 500))"
wait_for "── f-00050.txt"
T1=$(date +%s%N)
pane >"$W/stage2.txt"
B=$(elapsed "$T1" "$T0")
echo "stage 2 [$LABEL]: 500 held-n keys (50 crossings) landed on f-00050.txt"
echo "         burst wall time: ${B}s ($(awk "BEGIN{printf \"%.2f\", $B * 1000 / 500}")ms per key)"

# --- Stage 3: resize burst, then a 10-key probe across one boundary --
T0=$(date +%s%N)
for _ in 1 2 3 4 5 6; do
	tmux resize-window -t "$SES" -x 60 -y 18
	tmux resize-window -t "$SES" -x 100 -y 30
done
tmux send-keys -t "$SES" "nnnnnnnnnn"
wait_for "── f-00051.txt"
T1=$(date +%s%N)
pane >"$W/stage3.txt"
echo "stage 3 [$LABEL]: 12 resizes + 10-key crossing probe landed on f-00051.txt"
echo "         probe wall time: $(elapsed "$T1" "$T0")s"

# --- Exit ------------------------------------------------------------
tmux send-keys -t "$SES" "q"
for _ in $(seq 1 200); do
	[ -f "$W/exit.txt" ] && break
	sleep 0.05
done
echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
[ -s "$W/replay.txt" ] && { echo "stderr replay:"; cat "$W/replay.txt"; } || echo "stderr replay: empty"

# The multi-GB-scale fixture (10k-30k files + records.jsonl) is fully
# regenerable — keep the small evidence (stage captures, exit status,
# fakebin, inner.sh) and drop the bulk.
rm -rf "$W/fixture" "$W/records.jsonl"
