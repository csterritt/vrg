#!/bin/bash
# Hostile-fixture demo on a real PTY: a filename containing an embedded
# newline and an ESC byte, and a matched line carrying an OSC
# title-set sequence. Asserts the escaped forms appear in the captured
# pty stream and that no fixture control byte survives verbatim — with
# no OSC reaching the stream, the terminal title cannot be rewritten.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-hostile"
rm -rf "$W"
mkdir -p "$W/fix"

printf 'pre \x1b]0;pwned\x07 mid hit\nplain line\n' >"$W/fix/"$'evil\nesc\x1bname.txt'
printf 'hit here\n' >"$W/fix/plain.txt"

mkfifo "$W/in"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/fix"
stty rows 24 cols 80
"$D/vrg" hit .
echo "\$?" >"$W/exit.txt"
EOF
chmod +x "$W/inner.sh"

timeout 30 script -qfec "$W/inner.sh" "$W/typescript" <"$W/in" >/dev/null 2>&1 &
spid=$!
exec 9>"$W/in"

# Wait for the browse frame, then quit.
for _ in $(seq 1 600); do
	grep -aq 'pwned' "$W/typescript" 2>/dev/null && break
	sleep 0.05
done
printf 'q' >&9
wait "$spid" 2>/dev/null
exec 9>&-

T="$W/typescript"
strip() { sed $'s/\x1b\\[[0-9;?><]*[a-zA-Z]//g' | tr -d '\r'; }

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
check() { # check <label> <present|absent> <grep-args...>
	label=$1; want=$2; shift 2
	if grep -aq "$@" "$T"; then got=present; else got=absent; fi
	if [ "$got" = "$want" ]; then echo "$label=$got"; else echo "$label=VIOLATION($got)"; fi
}
check "escaped-filename"  present -F 'evil\nesc^[name.txt'
check "escaped-osc"       present -F '^[]0;pwned^G'
check "raw-osc-title"     absent  -F $'\x1b]0;pwned'
check "raw-osc-bel"       absent  -F $'pwned\x07'
check "raw-esc-filename"  absent  -F $'esc\x1bname'
echo "--- escaped sinks ---"
strip <"$T" | grep -av '^Script ' | grep -aF 'evil\nesc' | head -3
strip <"$T" | grep -av '^Script ' | grep -aF 'pwned' | head -3
