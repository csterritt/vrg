#!/bin/bash
# Issue #29 manual check: stale-match validation on the real vrg binary
# in real tmux PTYs — a same-length edit of a matched word drops its
# highlight and pins the "file changed since search" note, a deleted
# trailing match lands navigation on the last source line, and reverting
# the bytes followed by r clears the note.
#
# The route never touches repository or user files: fixtures are
# generated into disposable mktemp directories and an EXIT trap removes
# them on completion or interruption.
#
# Route — session A (a.txt, stop a:1; b.txt, stop b:5):
#   startup -> a.txt loaded; b.txt edited on disk (MARK -> XARK,
#              same length) BEFORE its first load
#   n       -> b.txt's first load validates stale: no highlight, the
#              filename row carries "file changed since search"
#   sed back + r -> the reread validates clean: note gone, MARK
#              highlighted again
#   q       -> exit 0
# Route — session B (a.txt, stop a:1; tail.txt, stops t:1 < t:200):
#   tail.txt truncated to 150 lines before its first load
#   n       -> tail.txt loads stale (t:200's line is gone): the note
#              shows while t:1's own match still highlights
#   n       -> the vanished stop lands at the last source line's start,
#              clamped to the frame's bottom, no highlight painted
#   q       -> exit 0
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/stale"
rm -rf "$W"
mkdir -p "$W"

TR="$(mktemp -d /tmp/vrg29.XXXXXX)"
TA="$TR/a"; TB="$TR/b"
mkdir -p "$TA" "$TB"

# Session A fixtures: a.txt is the startup file; b.txt holds the
# same-length-edited match at line 5.
printf 'MARK aaa\n' >"$TA/a.txt"
for i in $(seq 2 10); do printf 'line-%06d\n' "$i" >>"$TA/a.txt"; done
for i in $(seq 1 40); do
	if [ "$i" = 5 ]; then printf 'MARK bee\n'; else printf 'line-%06d\n' "$i"; fi
done >"$TA/b.txt"

# Session B fixtures: a.txt again; tail.txt's stops sit at lines 1 and
# 200 of a 200-line file the harness will truncate to 150.
printf 'MARK aaa\n' >"$TB/a.txt"
for i in $(seq 1 200); do
	case "$i" in
	1) printf 'MARK top\n' ;;
	200) printf 'MARK tail\n' ;;
	*) printf 'line-%06d\n' "$i" ;;
	esac
done >"$TB/tail.txt"

SESA="vrg29-a-$$"
SESB="vrg29-b-$$"
cleanup() {
	jobs -p | xargs -r kill 2>/dev/null
	tmux kill-session -t "$SESA" "$SESB" 2>/dev/null
	rm -rf "$TR"
}
trap cleanup EXIT
trap 'exit 130' INT TERM

inner() { # inner <name> <dir>
	cat >"$W/inner-$1.sh" <<EOF
#!/bin/sh
cd "$2"
unset RIPGREP_CONFIG_PATH
"$D/vrg" MARK . 2>"$W/stderr-$1.txt"
echo "\$?" >"$W/exit-$1.txt"
cat "$W/stderr-$1.txt"
sleep 30
EOF
	chmod +x "$W/inner-$1.sh"
}
inner A "$TA"
inner B "$TB"

ESC=$(printf '\033')
export LC_ALL=C.UTF-8
screen() { tmux capture-pane -p -t "$SES" 2>/dev/null; }
screen_e() { tmux capture-pane -p -e -t "$SES" 2>/dev/null; }
plain() { sed "s/${ESC}\[[0-9;]*m//g"; }
snap() { screen | plain >"$W/$1"; }
snap_e() { screen_e >"$W/$1"; }

wait_for() { # wait_for <literal> anywhere on the pane
	for _ in $(seq 1 1200); do
		screen | grep -qF "$1" && return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for: $1"; return 1
}
wait_absent() { # wait_absent <literal> — wait until it leaves the pane
	for _ in $(seq 1 1200); do
		screen | grep -qF "$1" || return 0
		sleep 0.05
	done
	echo "TIMEOUT waiting for absence of: $1"; return 1
}

FAIL=0
check() {
	if [ "$2" = "$3" ]; then echo "ok: $1 -> $2"; else echo "FAIL: $1 -> got [$2] want [$3]"; FAIL=1; fi
}
present() { # present <desc> <literal> — plain capture
	if screen | grep -qF "$2"; then echo "ok: $1"; else echo "FAIL: $1"; FAIL=1; fi
}
absent() { # absent <desc> <literal> — plain capture
	if screen | grep -qF "$2"; then echo "FAIL: $1"; FAIL=1; else echo "ok: $1"; fi
}
# tmux's -e capture normalizes SGR sequences into separate parameters
# (^[[4m^[[30m^[[47mMARK), so the inverse match style is detected by the
# 47 (light background) parameter on the text's own row.
inv() { # inv <desc> <literal> — present inside the inverse match style
	if screen_e | grep -qF "$2" && screen_e | grep -F "$2" | grep -qE '(\[|;)47(;|m)'; then
		echo "ok: $1"
	else
		echo "FAIL: $1"; FAIL=1
	fi
}
noinv() { # noinv <desc> <literal> — present but never inverse-styled
	if screen_e | grep -F "$2" | grep -qE '(\[|;)47(;|m)'; then
		echo "FAIL: $1"; FAIL=1
	elif screen_e | grep -qF "$2"; then
		echo "ok: $1"
	else
		echo "FAIL: $1 (text missing)"; FAIL=1
	fi
}
key() { tmux send-keys -t "$SES" "$@"; }
topline() { # first content row's source-line marker
	screen | plain | sed -n '2p' | grep -oE 'line-[0-9]+' | head -1
}
lastline() { # last content row's source-line marker
	screen | plain | grep -oE 'line-[0-9]+|MARK [a-z]+' | tail -1
}

# ============ session A: same-length edit -> stale note, no highlight;
# revert + r -> clean again
SES="$SESA"
tmux new-session -d -s "$SESA" -x 80 -y 24 "$W/inner-A.sh"
tmux set-option -t "$SESA" window-size manual
wait_for "MARK aaa" || exit 1
present "startup: a.txt loaded and matched" "MARK aaa"
snap screen-00-startup.txt

# The edit lands before b.txt's first load: a same-length replacement
# of the matched word is exactly the staleness the byte check catches.
sed -i 's/MARK bee/XARK bee/' "$TA/b.txt"
key n
wait_for "file changed" || exit 1
present "b.txt: filename row carries the stale note" "b.txt file changed since search"
present "b.txt: the edited line is on screen" "XARK bee"
noinv "b.txt: the dropped submatch paints no highlight" "XARK bee"
snap screen-01-stale-note.txt
snap_e screen-02-stale-note-raw.txt

# Revert the bytes, then r: the reread's own verdict clears the note
# and the highlight returns.
sed -i 's/XARK bee/MARK bee/' "$TA/b.txt"
key r
wait_absent "file changed" || exit 1
absent "after r: the note cleared on a clean revalidation" "file changed since search"
# The -e capture splits styled text with SGR sequences (MARK^[[0m… bee),
# so inverse checks grep the match token itself.
inv "after r: the match highlights again" "MARK"
snap screen-03-reloaded-clean.txt
snap_e screen-04-reloaded-clean-raw.txt
key q
for _ in $(seq 1 100); do [ -f "$W/exit-A.txt" ] && break; sleep 0.05; done
check "session A: vrg exit status" "$(cat "$W/exit-A.txt" 2>/dev/null)" "0"

# ============ session B: deleted trailing match -> land on the last
# source line
SES="$SESB"
tmux new-session -d -s "$SESB" -x 80 -y 24 "$W/inner-B.sh"
tmux set-option -t "$SESB" window-size manual
wait_for "MARK aaa" || exit 1
# Truncate before tail.txt's first load: the recorded t:200 stop's line
# no longer exists.
head -150 "$TB/tail.txt" >"$TB/tail.tmp" && mv "$TB/tail.tmp" "$TB/tail.txt"
key n
wait_for "file changed" || exit 1
present "tail.txt: one vanished stop marks the whole buffer stale" "tail.txt file changed since search"
inv "tail.txt: the still-valid t:1 match keeps its highlight" "MARK"
snap screen-10-tail-stale.txt
key n
wait_for "line-000150" || exit 1
# 150 lines over a 23-row content area: the last line's start reveals
# clamped to the bottom — top row 128, line 150 last.
check "vanished t:200 lands at the last source line (top row)" "$(topline)" "line-000128"
check "last source line paints at the frame's bottom" "$(lastline)" "line-000150"
noinv "the landed line invents no highlight" "line-000150"
snap screen-11-last-line.txt
snap_e screen-12-last-line-raw.txt
key q
for _ in $(seq 1 100); do [ -f "$W/exit-B.txt" ] && break; sleep 0.05; done
check "session B: vrg exit status" "$(cat "$W/exit-B.txt" 2>/dev/null)" "0"

[ "$FAIL" = 0 ] && echo "demo-stale-validation: all checks passed" || exit 1
