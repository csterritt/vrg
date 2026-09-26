#!/bin/bash
# No-results demo on a real PTY: script(1) allocates the
# pseudo-terminal; the inner session sizes it 80x24 and runs
# ./vrg zzzznotfound . in a fixture dir whose one text file cannot
# match. rg exits 1 with a summary-only stream, so the app presents the
# centred no-results screen. The harness waits for the frame, sends q,
# and checks the exit status and the typescript for the message.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-empty"
rm -rf "$W"
mkdir -p "$W/fixture"
printf 'hello\n' >"$W/fixture/plain.txt"
mkfifo "$W/in"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fixture"
stty rows 24 cols 80
"$D/vrg" zzzznotfound .
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

wait_for "No results found"
sleep 0.3
printf 'q' >&9
wait "$spid" 2>/dev/null
exec 9>&-

check() { # check <label> <present|absent> <literal> [file]
	local label=$1 want=$2 lit=$3 f=${4:-$T} got
	if grep -aqF "$lit" "$f"; then got=present; else got=absent; fi
	if [ "$got" = "$want" ]; then echo "$label=$want"; else echo "$label=VIOLATION($got)"; fi
}

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
check screen-message present "No results found"
check no-suffix      absent  "binary files skipped"

echo "--- final screen ---"
# The settled frame's message line, styles stripped — centred by its
# leading padding.
sed $'s/\x1b\\[[0-9;]*m//g' "$T" | sed $'s/\x1b\\[[0-9;?><]*[a-zA-Z]/\\n/g' |
	tr -d '\r' | grep -a "No results" | tail -1
