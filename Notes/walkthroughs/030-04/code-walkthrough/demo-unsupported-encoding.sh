#!/bin/bash
# Issue #30 manual check: a BOM-marked UTF-16 file is detected at load
# and presented as "(unsupported encoding)" — the explanatory overlay
# for the current file, diagnostic collection for exit-time replay,
# and the `r` reload route — on the real vrg binary in a real tmux PTY
# with real rg.
#
# The route never touches repository or user files: the fixture lives
# in a disposable mktemp directory an EXIT trap removes.
#
# Route — one session (stops a.txt:1 < u16.txt:1):
#   startup                -> a.txt renders, cursor on its match
#   n                      -> u16.txt: "(unsupported encoding)" in the
#                             panel and the filename-row note, plus the
#                             "cannot display ./u16.txt: unsupported
#                             encoding UTF-16 LE" overlay; no encoded
#                             bytes and no highlight paint
#   r (overlay open)       -> ignored: the overlay owns the keyboard
#   Esc                    -> overlay dismissed; placeholder stays
#   swap for a writerless FIFO, r
#                          -> the read blocks on open(): "Loading…" is
#                             painted and held; a duplicate r is
#                             dropped, not queued (a queued press would
#                             re-block on the FIFO after settlement)
#   writer pulse           -> the load settles; the placeholder returns
#                             with a fresh overlay
#   Esc, q                 -> exit 0; stderr replays both detections
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/encoding"
rm -rf "$W"
mkdir -p "$W"

# Disposable fixtures: copies only; the originals stay pristine.
TA="$(mktemp -d /tmp/vrg30-fixture.XXXXXX)"
printf 'hi alpha\n' >"$TA/a.txt"
printf '\xff\xfeh\0i\0\n\0' >"$TA/u16.txt"

SES="vrg30-$$"
cleanup() {
	tmux kill-session -t "$SES" 2>/dev/null
	rm -rf "$TA"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$TA"
unset RIPGREP_CONFIG_PATH
"$D/vrg" hi . 2>"$W/stderr.txt"
echo "\$?" >"$W/exit.txt"
cat "$W/stderr.txt"
sleep 30
EOF
chmod +x "$W/inner.sh"

tmux new-session -d -s "$SES" -x 80 -y 24 "$W/inner.sh"
tmux set-option -t "$SES" window-size manual

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

# ============ startup: a.txt renders on its match
wait_for "hi alpha" || exit 1
present "startup: a.txt renders" "hi alpha"
present "startup: filename rule names the path" "a.txt"
snap screen-00-startup.txt

# ============ n: u16.txt classifies — placeholder + explanatory overlay
key n
wait_for "unsupported encoding UTF-16 LE" || exit 1
present "entering u16.txt: explanatory overlay" "cannot display ./u16.txt: unsupported encoding UTF-16 LE"
present "entering u16.txt: panel placeholder" "(unsupported encoding)"
present "entering u16.txt: filename row keeps the path" "u16.txt"
absent "entering u16.txt: no encoded bytes leak" "^@"
snap screen-01-unsupported-overlay.txt

# ============ r under the open overlay is the overlay's, not a reload
key r; sleep 0.4
absent "r under the overlay: no Loading…" "Loading…"
present "r under the overlay: overlay stays" "unsupported encoding UTF-16 LE"
snap screen-02-r-blocked.txt

# ============ Esc dismisses; the placeholder stays
key Escape; sleep 0.3
absent "Esc: overlay dismissed" "cannot display"
present "Esc: placeholder stays" "(unsupported encoding)"
snap screen-03-dismissed.txt

# ============ writerless FIFO: the reread blocks, "Loading…" is held,
# and a duplicate r is dropped — not queued
rm "$TA/u16.txt" && mkfifo "$TA/u16.txt"
key r
wait_for "Loading…" || exit 1
present "r on fifo: Loading… while the read blocks" "Loading…"
present "r on fifo: filename row still names the path" "u16.txt"
snap screen-04-loading.txt
key r; sleep 0.4
present "duplicate r mid-load: still just Loading…" "Loading…"

# ============ writer pulse: the load settles — placeholder + new overlay
printf '\xff\xfeh\0i\0\n\0' >"$TA/u16.txt"
wait_for "unsupported encoding UTF-16 LE" || exit 1
absent "fifo settled: no queued second reread blocks" "Loading…"
present "fifo settled: placeholder restored" "(unsupported encoding)"
present "fifo settled: fresh overlay" "cannot display ./u16.txt: unsupported encoding UTF-16 LE"
snap screen-05-reloaded-overlay.txt

# ============ Esc, q: exit 0; stderr replays each detection
key Escape; sleep 0.3
key q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "vrg exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "0"
check "stderr replays both detections" "$(grep -cF 'unsupported encoding UTF-16 LE' "$W/stderr.txt" 2>/dev/null)" "2"
snap screen-06-exit-replay.txt

[ "$FAIL" = 0 ] && echo "demo-unsupported-encoding: all checks passed" || exit 1
