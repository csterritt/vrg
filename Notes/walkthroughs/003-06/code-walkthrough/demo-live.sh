#!/bin/bash
# Drive the built vrg binary through a piped stdin/stdout session from
# the repository root: run real rg over a large tree so collection
# outlives the first render frame, extract the rendered frames from the
# raw stream, send q once the interim summary appears, and report the
# process exit status.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
rm -f "$D/tty-in" "$D/tty-out" "$D/tty-err" "$D/tty-exit"
mkfifo "$D/tty-in"
(
	cd /home/chris/vrg || exit 1
	timeout 30 "$D/vrg" "charm.land" /home/chris/go/pkg/mod \
		<"$D/tty-in" >"$D/tty-out" 2>"$D/tty-err"
	echo "$?" >"$D/tty-exit"
) &
exec 9>"$D/tty-in"
for _ in $(seq 1 400); do
	grep -q 'matched line' "$D/tty-out" 2>/dev/null && break
	sleep 0.05
done
printf 'q' >&9
exec 9>&-
for _ in $(seq 1 40); do
	[ -f "$D/tty-exit" ] && break
	sleep 0.1
done
tr '\033' '\n' <"$D/tty-out" | grep -aoE 'Searching…|[0-9]+ files?, [0-9]+ matched lines?' | uniq
echo "process-exit=$(cat "$D/tty-exit" 2>/dev/null)"
echo "stderr-bytes=$(wc -c <"$D/tty-err")"
