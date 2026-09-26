#!/bin/bash
# Issue #47 manual scenario — hostile filenames driven through real
# os.ReadFile failures inside a disposable directory.
#
# Four fixture files carry embedded-byte names — LF, TAB, an invalid
# UTF-8 byte, and ESC — and a fake rg indexes them through the base64
# "bytes" path form. The tagged binary runs on a tmux PTY with
# VRG_TEST_GATE holding index preparation: every fixture is removed
# while the gate holds, so the released loads fail for real. Each
# failure surfaces as exactly one diagnostic line — the escaped path
# plus the bare errno reason — in the overlay (captured per file) and
# in the exit stderr replay (acknowledged line-by-line through
# VRG_TEST_COLLECT_ACK).
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-single-line"
rm -rf "$W"
mkdir -p "$W/fixture" "$W/fakebin"

# The hostile filename set, created as real files in the disposable
# fixture dir.
N1=$'bad\nname.txt'   # embedded LF — a diagnostic boundary if unescaped
N2=$'bad\tname.txt'   # embedded TAB
N3=$'bad\xffname.txt' # invalid UTF-8 byte
N4=$'bad\x1bname.txt' # ESC introducer
NAMES=("$N1" "$N2" "$N3" "$N4")
for n in "${NAMES[@]}"; do printf 'hit\n' >"$W/fixture/$n"; done
echo "== fixtures indexed (names shown escaped) =="
for n in "${NAMES[@]}"; do printf '   %q\n' "$n"; done

# Fake rg: emit each file's begin/match/end records — the hostile path
# bytes travel as base64 "bytes" values — then exit cleanly.
{
	for n in "${NAMES[@]}"; do
		p=$(printf '%s' "$n" | base64)
		printf '%s\n' "{\"type\":\"begin\",\"data\":{\"path\":{\"bytes\":\"$p\"}}}"
		printf '%s\n' "{\"type\":\"match\",\"data\":{\"path\":{\"bytes\":\"$p\"},\"lines\":{\"text\":\"hit\\n\"},\"line_number\":1,\"submatches\":[{\"match\":{\"text\":\"hit\"},\"start\":0,\"end\":3}]}}"
		printf '%s\n' "{\"type\":\"end\",\"data\":{\"path\":{\"bytes\":\"$p\"},\"binary_offset\":null}}"
	done
	printf '%s\n' '{"type":"summary","data":{}}'
} >"$W/stream.json"
printf '#!/bin/sh\ncat "%s"\n' "$W/stream.json" >"$W/fakebin/rg"
chmod +x "$W/fakebin/rg"

cd /home/chris/vrg || exit 1
go build -tags vrg_testhooks -o "$D/vrg-hooks" ./cmd/vrg || exit 1

mkfifo "$W/gate"
SES="vrg47-$$"
trap 'tmux kill-session -t "$SES" 2>/dev/null' EXIT

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
env PATH="$W/fakebin:\$PATH" TERM=xterm-256color \
  VRG_TEST_GATE="$W/gate" VRG_TEST_COLLECT_ACK="$W/ack" \
  "$D/vrg-hooks" hit . 2>"$W/err.txt"
echo "\$?" >"$W/exit.txt"
sleep 5
EOF

waitfile() { # path — poll until non-empty
	local deadline=$((SECONDS + 30))
	while [ ! -s "$1" ]; do
		if [ $SECONDS -gt "$deadline" ]; then
			echo "TIMEOUT waiting for $1"
			exit 1
		fi
		sleep 0.1
	done
}
pane() { tmux capture-pane -p -t "$SES" | sed 's/[[:space:]]*$//'; }
waitpane() { # text — poll until the pane shows it
	local deadline=$((SECONDS + 30))
	until pane | grep -qF "$1"; do
		if [ $SECONDS -gt "$deadline" ]; then
			echo "TIMEOUT waiting for pane text: $1"
			exit 1
		fi
		sleep 0.1
	done
}
waitlines() { # path count — poll until the file has count lines
	local deadline=$((SECONDS + 30))
	while :; do
		local n
		n=$(wc -l <"$1" 2>/dev/null) || n=0
		[ "${n:-0}" -ge "$2" ] && return
		if [ $SECONDS -gt "$deadline" ]; then
			echo "TIMEOUT waiting for $1 to reach $2 lines"
			exit 1
		fi
		sleep 0.1
	done
}

# Start the TUI gate-held: rg runs, the records stream in, but index
# preparation — and therefore any file load — waits on the fifo.
tmux new-session -d -s "$SES" -x 100 -y 30 "sh $W/inner.sh"
waitpane "Searching"

# Remove every fixture while the gate holds: the loads released below
# meet genuine ENOENT, not a permission denial that elevated
# privileges could sail through.
for n in "${NAMES[@]}"; do rm -f "$W/fixture/$n"; done
echo "== fixtures removed while preparation gate held =="
ls -A "$W/fixture" | sed 's/^/   /'
echo "   (empty)"

echo x >"$W/gate" # release preparation → browse → the real read attempts

# Each file's failure: one collected diagnostic (the ack) and one
# overlay line on screen. The sorted stop order is byte-wise: TAB,
# LF, ESC, then the invalid byte.
for i in 1 2 3 4; do
	waitlines "$W/ack" "$i"
	waitpane "cannot read"
	pane >"$W/screen-0$i.txt"
	echo "-- failure $i, overlay row:"
	grep -o 'cannot read[^│]*' "$W/screen-0$i.txt" | head -1 | sed 's/^/   /'
	[ "$i" -lt 4 ] && {
		# Space the key sends: an ESC immediately followed by n on
		# the PTY coalesces into Alt+n instead of dismiss-then-next.
		tmux send-keys -t "$SES" Escape
		sleep 0.3
		tmux send-keys -t "$SES" n
	}
done
tmux send-keys -t "$SES" Escape # dismiss the last overlay
sleep 0.3
tmux send-keys -t "$SES" q      # quit browse
waitfile "$W/exit.txt"

echo "== stderr replay: $(wc -l <"$W/err.txt") lines, exit $(cat "$W/exit.txt") =="
sed 's/^/   /' "$W/err.txt"
if LC_ALL=C grep -qP '[\x00-\x09\x0b-\x1f\x7f\x80-\xff]' "$W/err.txt"; then
	echo "FAIL: a raw control or high byte survived in the replay"
	exit 1
fi
echo "   no raw control or high bytes in the replay"
