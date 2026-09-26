#!/bin/bash
# Issue #26 manual check: a read-failed file notifies with the error
# overlay and the "(unreadable)" placeholder, same-file steps never
# retry, and a cross-file re-entry re-opens the prior failure behind a
# retrying "Loading…" — on the real vrg binary in a real tmux PTY.
#
# The route never touches repository or user files: the two matched
# files are copied from fixture-src/ into a disposable mktemp
# directory, the second file's original mode is recorded, and an EXIT
# trap restores it (then removes the fixture) on completion or
# interruption. The script refuses to run as root — chmod 000 does not
# deny root.
#
# Route (stops f1s1 < f2s1 < f2s2):
#   startup                -> file 1 renders; file 2 not yet loaded
#   chmod 000 file 2
#   n                      -> into file 2: overlay + "(unreadable)"
#   Esc                    -> overlay dismissed, placeholder stays
#   n                      -> same-file step: no new overlay
#   n                      -> wrap back to file 1 (session cache)
#   p                      -> re-entry: prior overlay reopens; the retry
#                             fails fast, appending a second occurrence
#   p                      -> swallowed by the open overlay
#   Esc, p, p              -> dismissed; back to file 1
#   swap file 2 for a writerless FIFO, n
#                          -> second re-entry: overlay + a retry whose
#                             open() blocks, so "Loading…" is painted
#                             and held — the transient state a fast
#                             EACCES never lets a PTY capture
#   Esc                    -> dismissed; the in-flight retry is
#                             undisturbed (Loading… stays)
#   writer pulse           -> the fifo read completes; content replaces
#                             the placeholder without any overlay
#   q                      -> exit 0; stderr replays both failures
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/fail"
rm -rf "$W"
mkdir -p "$W"

if [ "$(id -u)" = 0 ]; then
	echo "SKIP: must run unprivileged — chmod 000 does not deny root"
	exit 1
fi

# Disposable fixture: copies only; the originals stay pristine.
T="$(mktemp -d /tmp/vrg26-fixture.XXXXXX)"
FA="a-first.txt"
FB="b-second.txt"
cp "$D/fixture-src/$FA" "$D/fixture-src/$FB" "$T/"
MODE_B="$(stat -c %a "$T/$FB")"

SES="vrg26-fail-$$"
cleanup() {
	tmux kill-session -t "$SES" 2>/dev/null
	chmod -R u+rwX "$T" 2>/dev/null
	rm -rf "$T"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$T"
unset RIPGREP_CONFIG_PATH
"$D/vrg" MARK . 2>"$W/stderr.txt"
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

# ============ startup: file 1 renders; file 2 not yet loaded
wait_for "MARK alpha" || exit 1
present "startup: file 1's content renders" "MARK alpha"
present "startup: file 1's filename rule" "a-first.txt"
snap screen-00-startup-file1.txt

# ============ make file 2 unreadable (rg already read it)
chmod 000 "$T/$FB"
check "file 2 mode after chmod" "$(stat -c %a "$T/$FB")" "0"

# ============ n into file 2: overlay + (unreadable)
key n
wait_for "cannot read" || exit 1
present "n into file 2: error overlay opens" "cannot read"
present "n into file 2: overlay names the path" "b-second.txt"
present "n into file 2: the (unreadable) placeholder" "(unreadable)"
snap screen-01-n-into-f2-overlay.txt

# ============ Esc dismisses; the placeholder stays
key Escape; sleep 0.3
absent "Esc: overlay dismissed" "cannot read"
present "Esc: (unreadable) stays" "(unreadable)"
snap screen-02-esc-dismissed.txt

# ============ same-file n: no reload, no new overlay
key n; sleep 0.4
absent "same-file n: no new overlay" "cannot read"
present "same-file n: still (unreadable)" "(unreadable)"
snap screen-03-samefile-n.txt

# ============ n wraps to file 1 (cached); Esc clears its pop-up
key n
wait_for "MARK alpha" || exit 1
present "n wrap to file 1: cached content" "MARK alpha"
key Escape; sleep 0.3

# ============ p re-enters file 2: prior overlay reopens and exactly one
# retry runs — a chmod-000 read fails inside a render tick, so the
# screen already shows the settled second occurrence
key p
wait_for "cannot read" || exit 1
sleep 0.3
present "re-entry p: the prior failure overlay reopened" "cannot read"
present "re-entry p: retry settled to (unreadable)" "(unreadable)"
check "second failure appended one occurrence" "$(screen | plain | grep -cF 'cannot read')" "2"
snap screen-04-reentry-overlay-unreadable.txt

# ============ keys under the open overlay are swallowed (overlay owns
# the keyboard); dismiss, then two p steps return to file 1
key p; sleep 0.3
check "p under the open overlay: still two occurrences" "$(screen | plain | grep -cF 'cannot read')" "2"
key Escape; sleep 0.3
key p; sleep 0.2
absent "same-file p after dismissal: no new overlay" "cannot read"
key p
wait_for "MARK alpha" || exit 1
key Escape; sleep 0.3

# ============ swap file 2 for a writerless FIFO and re-enter: the
# retry's open() blocks, so the reopened overlay and the in-flight
# "Loading…" are both painted and held
rm -f "$T/$FB" && mkfifo "$T/$FB"
key n
wait_for "Loading…" || exit 1
present "fifo re-entry: the prior failure overlay reopened" "cannot read"
present "fifo re-entry: the in-flight retry paints Loading…" "Loading…"
snap screen-05-reentry-overlay-loading.txt

# ============ Esc dismisses the overlay without disturbing the load
key Escape; sleep 0.3
absent "Esc mid-retry: overlay dismissed" "cannot read"
present "Esc mid-retry: the retry is undisturbed" "Loading…"
snap screen-06-esc-retry-inflight.txt

# ============ a writer pulse lets the retry settle — content replaces
# the placeholder with no overlay involved
timeout 10 sh -c 'printf "MARK beta one\nx\nMARK beta two\nx\n" >"$1"' _ "$T/$FB"
wait_for "MARK beta" || exit 1
present "retry settled: content replaced Loading…" "MARK beta"
absent "retry settled: the placeholder is gone" "Loading…"
snap screen-07-retry-settled.txt

# ============ q quits 0; stderr replays the collected failures
key q
for _ in $(seq 1 100); do [ -f "$W/exit.txt" ] && break; sleep 0.05; done
check "vrg exit status" "$(cat "$W/exit.txt" 2>/dev/null)" "0"
wait_for "cannot read" || exit 1
check "stderr lists both failure occurrences" "$(grep -cF 'cannot read ./b-second.txt' "$W/stderr.txt")" "2"
present "post-exit pane shows the replayed diagnostics" "cannot read"
snap screen-08-exit-replay.txt

[ "$FAIL" = 0 ] && echo "demo-failures: all checks passed" || exit 1
