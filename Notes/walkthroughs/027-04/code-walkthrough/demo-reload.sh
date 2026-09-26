#!/bin/bash
# Issue #27 manual check: `r` rereads the current file from disk without
# rerunning rg — on the real vrg binary in a real tmux PTY.
#
# The route never touches repository or user files: the matched files
# are copied from fixture-src/ into disposable mktemp directories and
# an EXIT trap removes them on completion or interruption.
#
# Route — session A (stops a:2 < a:70 < b:3):
#   startup                -> a-long.txt renders, cursor on line 2
#   PageDown               -> scroll: the top row leaves line 1
#   append lines 81-90     -> external edit: the display is unchanged
#   r                      -> one reread; new content at the same top
#   PageDown x3            -> the appended lines are reachable at EOF
#   rm a-long.txt, r       -> "(unreadable)" + the failure overlay
#   Esc                    -> overlay dismissed, placeholder stays
#   restore the file, r    -> the prior-failure overlay reopens over
#                             the reloaded content; Esc reveals it
#   swap for a writerless FIFO, r
#                          -> the read blocks on open(): "Loading…" is
#                             painted and held; a duplicate r is
#                             dropped, not queued (a queued press would
#                             re-block on the FIFO after settlement)
#   writer pulse           -> the load settles; content replaces the
#                             placeholder and stays settled
#   q                      -> exit 0; stderr replays the one failure
# Route — session B (one stop, s:5):
#   n                      -> strict no-op on a one-stop index
#   append, r              -> the single-match file reloads via r
#   q                      -> exit 0
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/reload"
rm -rf "$W"
mkdir -p "$W"

# Disposable fixtures: copies only; the originals stay pristine.
TA="$(mktemp -d /tmp/vrg27-fixture-a.XXXXXX)"
TB="$(mktemp -d /tmp/vrg27-fixture-b.XXXXXX)"
cp "$D/fixture-src/a-long.txt" "$D/fixture-src/b-short.txt" "$TA/"
cp "$D/fixture-src/solo.txt" "$TB/"

SESA="vrg27-a-$$"
SESB="vrg27-b-$$"
cleanup() {
	tmux kill-session -t "$SESA" 2>/dev/null
	tmux kill-session -t "$SESB" 2>/dev/null
	rm -rf "$TA" "$TB"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

cat >"$W/inner-a.sh" <<EOF
#!/bin/sh
cd "$TA"
unset RIPGREP_CONFIG_PATH
"$D/vrg" MARK . 2>"$W/stderr-a.txt"
echo "\$?" >"$W/exit-a.txt"
cat "$W/stderr-a.txt"
sleep 30
EOF
cat >"$W/inner-b.sh" <<EOF
#!/bin/sh
cd "$TB"
unset RIPGREP_CONFIG_PATH
"$D/vrg" MARK . 2>"$W/stderr-b.txt"
echo "\$?" >"$W/exit-b.txt"
cat "$W/stderr-b.txt"
sleep 30
EOF
chmod +x "$W/inner-a.sh" "$W/inner-b.sh"

tmux new-session -d -s "$SESA" -x 80 -y 24 "$W/inner-a.sh"
tmux set-option -t "$SESA" window-size manual

SES="$SESA"
ESC=$(printf '\033')
export LC_ALL=C.UTF-8
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
snap() { screen | plain >"$W/$1"; }

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 600); do
		screen | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1"; return 1
}

FAIL=0
check() {
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}
present() { # present <desc> <literal>
	if screen | grep -qF "$2"; then echo "ok: $1"; else echo "FAIL: $1"; FAIL=1; fi
}
absent() { # absent <desc> <literal>
	if screen | grep -qF "$2"; then echo "FAIL: $1"; FAIL=1; else echo "ok: $1"; fi
}
key() { tmux send-keys -t "$SES" "$@"; }
topline() { # first content row's source-line marker
	screen | plain | sed -n '2p' | grep -oE 'line-[0-9]+' | head -1
}

# ============ session A startup: a-long.txt renders
wait_for "MARK alpha" || exit 1
present "startup: a-long.txt renders" "MARK alpha"
present "startup: filename rule names the path" "a-long.txt"
TOP0="$(topline)"
snap screen-00-startup.txt

# ============ scroll: the top row leaves line 1
key PageDown; sleep 0.4
TOP1="$(topline)"
check "pgdown scrolls" "$([ -n "$TOP1" ] && [ "$TOP1" != "$TOP0" ] && echo moved)" "moved"
snap screen-01-scrolled.txt

# ============ external append: the cached display does not change
screen | plain >"$W/pre-append.txt"
for i in $(seq -w 81 90); do echo "line-$i appended" >>"$TA/a-long.txt"; done
sleep 0.5
screen | plain >"$W/post-append.txt"
check "external append leaves display unchanged" "$(cmp -s "$W/pre-append.txt" "$W/post-append.txt" && echo identical || echo CHANGED)" "identical"
snap screen-02-append-unchanged.txt

# ============ r: one reread; new content lands at the same top
key r; sleep 1.0
check "r: same top position" "$(topline)" "$TOP1"
snap screen-03-after-r.txt
key PageDown; sleep 0.2
key PageDown; sleep 0.2
key PageDown; sleep 0.4
present "r: appended lines are loaded" "line-90 appended"
snap screen-04-eof-appended.txt

# ============ delete the file, r: "(unreadable)" + the failure overlay
rm "$TA/a-long.txt"
key r
wait_for "cannot read" || exit 1
present "r on deleted file: failure overlay opens" "cannot read"
present "r on deleted file: (unreadable) placeholder" "(unreadable)"
snap screen-05-deleted-overlay.txt
key Escape; sleep 0.3
absent "Esc: overlay dismissed" "cannot read"
present "Esc: (unreadable) stays" "(unreadable)"
snap screen-06-esc-unreadable.txt

# ============ restore the file, r: prior overlay reopens over content
cp "$D/fixture-src/a-long.txt" "$TA/a-long.txt"
key r
wait_for "cannot read" || exit 1
for _ in $(seq 1 100); do screen | grep -qF "line-" && break; sleep 0.05; done
present "restore + r: prior failure overlay reopens" "cannot read"
present "restore + r: content reloaded behind it" "line-"
snap screen-07-restore-overlay.txt
key Escape; sleep 0.3
absent "restore + r + Esc: overlay gone" "cannot read"
absent "restore + r + Esc: (unreadable) cleared" "(unreadable)"
present "restore + r + Esc: content readable" "line-"
snap screen-08-restored.txt

# ============ writerless FIFO: the read blocks, "Loading…" is held,
# and a duplicate r is dropped — not queued
rm "$TA/a-long.txt" && mkfifo "$TA/a-long.txt"
key r
wait_for "Loading…" || exit 1
present "r on fifo: Loading… while the read blocks" "Loading…"
present "r on fifo: filename row still names the path" "a-long.txt"
snap screen-09-loading.txt
key r; sleep 0.4
present "duplicate r mid-load: still just Loading…" "Loading…"
timeout 10 sh -c 'cat "$1" >"$2"' _ "$D/fixture-src/a-long.txt" "$TA/a-long.txt"
for _ in $(seq 1 200); do screen | grep -qF "line-80" && break; sleep 0.05; done
sleep 0.6
absent "fifo settled: no queued second reread blocks" "Loading…"
absent "fifo settled: no (unreadable)" "(unreadable)"
present "fifo settled: content painted" "line-80"
snap screen-10-fifo-settled.txt

# ============ q: exit 0; stderr replays the collected failure
key q
for _ in $(seq 1 100); do [ -f "$W/exit-a.txt" ] && break; sleep 0.05; done
check "session A: vrg exit status" "$(cat "$W/exit-a.txt" 2>/dev/null)" "0"
check "session A: stderr replays one failure" "$(grep -cF 'cannot read' "$W/stderr-a.txt" 2>/dev/null)" "1"
snap screen-11-exit-replay.txt

# ============ session B: a one-stop index reloads via r
SES="$SESB"
tmux new-session -d -s "$SESB" -x 80 -y 24 "$W/inner-b.sh"
tmux set-option -t "$SESB" window-size manual
wait_for "MARK solo" || exit 1
present "one-stop index: solo.txt renders" "MARK solo"
screen | plain >"$W/pre-n-b.txt"
key n; sleep 0.4
screen | plain >"$W/post-n-b.txt"
check "one-stop index: n is a strict no-op" "$(cmp -s "$W/pre-n-b.txt" "$W/post-n-b.txt" && echo identical || echo CHANGED)" "identical"
echo "s-appended after reload" >>"$TB/solo.txt"
sleep 0.4
absent "external append: still the cached display" "s-appended"
key r
wait_for "s-appended" || exit 1
present "one-stop index: r reloads" "s-appended"
snap screen-12-solo-reloaded.txt
key q
for _ in $(seq 1 100); do [ -f "$W/exit-b.txt" ] && break; sleep 0.05; done
check "session B: vrg exit status" "$(cat "$W/exit-b.txt" 2>/dev/null)" "0"

[ "$FAIL" = 0 ] && echo "demo-reload: all checks passed" || exit 1
