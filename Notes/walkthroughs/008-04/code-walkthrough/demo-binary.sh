#!/bin/bash
# All-binary demo on a real PTY: the fixture dir holds only b.bin —
# thousands of "foo hit N" lines with a NUL past rg's initial binary
# detection window, so directory-walk rg emits thousands of match
# records and then an end record carrying a non-null binary_offset. The
# index drops every collected stop and counts the one excluded file, so
# the app shows "No results found (1 binary files skipped)". The
# harness waits for the frame, sends q, and checks exit status 1.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-binary"
rm -rf "$W"
mkdir -p "$W/fixture"
# The NUL must lie beyond rg's initial sniff window (~64 KiB observed
# with rg 15.2.0) or a walked file is suppressed without any events.
{ for i in $(seq 1 7000); do echo "foo hit $i"; done; printf 'x\0\n'; } >"$W/fixture/b.bin"
mkfifo "$W/in"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
stty rows 24 cols 80
"$D/vrg" foo .
echo "\$?" >"$W/exit.txt"
EOF
chmod +x "$W/inner.sh"

timeout 30 script -qfec "$W/inner.sh" "$W/typescript" <"$W/in" >/dev/null 2>&1 &
spid=$!
exec 9>"$W/in"

T="$W/typescript"
wait_for() {
	for _ in $(seq 1 600); do
		grep -aqF "$1" "$T" 2>/dev/null && return 0
		sleep 0.05
	done
	return 1
}

wait_for "binary files skipped"
sleep 0.3
printf 'q' >&9
wait "$spid" 2>/dev/null
exec 9>&-

check() { # check <label> <present|absent> <literal>
	local label=$1 want=$2 lit=$3 got
	if grep -aqF "$lit" "$T"; then got=present; else got=absent; fi
	if [ "$got" = "$want" ]; then echo "$label=$want"; else echo "$label=VIOLATION($got)"; fi
}

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
check screen-with-suffix present "No results found (1 binary files skipped)"
check excluded-absent    absent  "── b.bin"

echo "--- final screen ---"
# The settled frame's message line, styles stripped — centred by its
# leading padding.
sed $'s/\x1b\\[[0-9;]*m//g' "$T" | sed $'s/\x1b\\[[0-9;?><]*[a-zA-Z]/\\n/g' |
	tr -d '\r' | grep -a "No results" | tail -1
