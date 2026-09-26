#!/bin/bash
# Issue #42 manual check: a dropped r while a load is in flight changes
# nothing — no reload mark, no intent switch, no presentation change —
# and the in-flight load then completes under its own classification:
# the destination reveal, never the reload's anchor preservation.
# Driven on a real tmux PTY at 80x24; capture-pane gives the composed
# screen.
#
# The slow file is a FIFO: the load worker's os.ReadFile blocks until a
# writer closes, so the load is genuinely in flight for as long as the
# harness wants — a deterministic slow-read seam instead of gigabytes
# of fixture data.
#
# Run A (startup load): slow.txt alone — the startup load parks on the
#   fifo, r is dropped with the pane byte-identical, and releasing the
#   fifo reveals the line-200 match at one-third placement.
# Run B (navigation load): fast.txt settles, n crosses to slow.txt —
#   the navigation load parks, an Up clears the file-change pop-up —
#   and r is dropped with the pane byte-identical; the completion again
#   reveals the line-200 match rather than keeping top-of-file.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-dropped-r"
rm -rf "$W"
mkdir -p "$W/fakebin"

cat >"$W/fakebin/rg" <<'RG'
#!/bin/sh
# Run B's stream: fast.txt's line-1 match then slow.txt's line-200
# match. The same script serves run A through $VRG_FIXTURE_FILTER.
if [ "${VRG_FIXTURE_FILTER:-b}" = "a" ]; then
	printf '%s\n' \
	'{"type":"begin","data":{"path":{"text":"slow.txt"}}}' \
	'{"type":"match","data":{"path":{"text":"slow.txt"},"lines":{"text":"hit00200\n"},"line_number":200,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
	'{"type":"end","data":{"path":{"text":"slow.txt"},"binary_offset":null}}' \
	'{"type":"summary","data":{}}'
else
	printf '%s\n' \
	'{"type":"begin","data":{"path":{"text":"fast.txt"}}}' \
	'{"type":"match","data":{"path":{"text":"fast.txt"},"lines":{"text":"hit one\n"},"line_number":1,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
	'{"type":"end","data":{"path":{"text":"fast.txt"},"binary_offset":null}}' \
	'{"type":"begin","data":{"path":{"text":"slow.txt"}}}' \
	'{"type":"match","data":{"path":{"text":"slow.txt"},"lines":{"text":"hit00200\n"},"line_number":200,"submatches":[{"match":{"text":"hit"},"start":0,"end":3}]}}' \
	'{"type":"end","data":{"path":{"text":"slow.txt"},"binary_offset":null}}' \
	'{"type":"summary","data":{}}'
fi
exit 0
RG
chmod +x "$W/fakebin/rg"

mkfixture() { # mkfixture <dir> <a|b>
	mkdir -p "$1"
	mkfifo "$1/slow.txt"
	if [ "$2" = "b" ]; then
		printf 'hit one\n' >"$1/fast.txt"
		for i in $(seq 2 20); do printf 'x%06d\n' "$i"; done >>"$1/fast.txt"
	fi
}

write_slow() { # release the fifo with 300 lines, line 200 the match
	{
		for i in $(seq 1 300); do
			if [ "$i" -eq 200 ]; then printf 'hit%05d\n' "$i";
			else printf 'x%06d\n' "$i"; fi
		done
	} >"$1/slow.txt"
}

fail=0
cleanup() {
	for n in startup navigation; do
		tmux kill-session -t "vrg42-$n-$$" 2>/dev/null
	done
}
trap cleanup EXIT
check() { # check <label> <0|1>
	if [ "$2" -eq 0 ]; then echo "ok: $1"; else echo "FAIL: $1"; fail=1; fi
}

run_case() { # run_case <name> <a|b>
	local name="$1" which="$2"
	local F="$W/fixture-$which" SES="vrg42-$name-$$"
	mkfixture "$F" "$which"

	cat >"$W/inner-$name.sh" <<EOF
#!/bin/sh
cd "$F"
VRG_FIXTURE_FILTER=$which PATH="$W/fakebin:\$PATH" "$D/vrg" hit .
echo "\$?" >"$W/exit-$name.txt"
sleep 60
EOF
	chmod +x "$W/inner-$name.sh"

	tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner-$name.sh"
	pane() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
	wait_for() {
		for _ in $(seq 1 600); do
			pane | grep -qF "$1" && return 0
			sleep 0.05
		done
		echo "timed out waiting for: $1" >&2
		return 1
	}
	settle() { sleep 0.3; }
	same() { cmp -s "$1" "$2"; }

	if [ "$which" = "b" ]; then
		# fast.txt loads first; n crosses to slow.txt, whose load
		# parks on the fifo behind the file-change pop-up.
		wait_for "hit one" || { tmux kill-session -t "$SES"; return 1; }
		tmux send-keys -t "$SES" n
		wait_for "Loading" || { tmux kill-session -t "$SES"; return 1; }
		# Any key dismisses the pop-up — a placeholder no-op scroll
		# clears it so the dropped-r comparison measures the panel.
		tmux send-keys -t "$SES" Up
		settle
	else
		wait_for "Loading" || { tmux kill-session -t "$SES"; return 1; }
		settle
	fi

	pane >"$W/$name-before-r.txt"
	pane | grep -qF "slow.txt" || { echo "FAIL: $name panel is not slow.txt's"; fail=1; }
	pane | grep -qF "hit00200" && { echo "FAIL: $name match visible before the load settled"; fail=1; }

	# The dropped r: pane byte-identical — no Loading… toggle, no
	# reload mark, no intent change visible.
	tmux send-keys -t "$SES" r
	settle
	pane >"$W/$name-after-r.txt"
	same "$W/$name-before-r.txt" "$W/$name-after-r.txt"
	check "$name: dropped r left the pane unchanged" $?
	pane | grep -qF "Loading" && check "$name: still Loading… after the dropped r" 0 \
		|| check "$name: still Loading… after the dropped r" 1

	# Release the fifo: the in-flight load completes under its own
	# classification — the destination reveal lands the line-200
	# match at one-third placement, not anchor-preserved top-of-file.
	write_slow "$F"
	wait_for "hit00200" || { tmux kill-session -t "$SES"; return 1; }
	settle
	pane >"$W/$name-revealed.txt"
	check "$name: load completed to the revealed match" 0
	pane | grep -qF "Loading" && check "$name: placeholder replaced by content" 1 \
		|| check "$name: placeholder replaced by content" 0
	pane | grep -qF "x000001" && check "$name: top-of-file still shown (anchor kept)" 1 \
		|| check "$name: viewport moved to the match (reveal ran)" 0

	tmux send-keys -t "$SES" q
	for _ in $(seq 1 600); do [ -f "$W/exit-$name.txt" ] && break; sleep 0.05; done
	tmux kill-session -t "$SES" 2>/dev/null
}

run_case startup a
run_case navigation b

for n in startup navigation; do
	echo "exit-$n=$(cat "$W/exit-$n.txt" 2>/dev/null || echo missing)"
done
echo "--- startup: pane while the load was in flight (before and after r — identical) ---"
grep -v '^[[:space:]]*$' "$W/startup-before-r.txt"
echo "--- startup: pane after the load completed (destination reveal) ---"
grep -v '^[[:space:]]*$' "$W/startup-revealed.txt" | head -12
echo "--- navigation: pane while the load was in flight (before and after r — identical) ---"
grep -v '^[[:space:]]*$' "$W/navigation-before-r.txt"
echo "--- navigation: pane after the load completed (destination reveal) ---"
grep -v '^[[:space:]]*$' "$W/navigation-revealed.txt" | head -12
exit $fail
