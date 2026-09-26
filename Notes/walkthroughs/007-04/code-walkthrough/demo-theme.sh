#!/bin/bash
# Theme-toggle demo on a real PTY: script(1) allocates a
# pseudo-terminal; the inner session sizes it 80x24 and runs
# ./vrg func . in a one-file Go fixture whose only matched line is the
# current stop. The harness sends c, c, q and checks each phase's bytes
# for the scheme discriminator: the dark scheme paints base 37;40 and
# the current-line match 30;47;4 (a bare 30;47m never appears), the
# light scheme paints base 30;47 and the current-line match 37;40;4 (a
# bare 37;40m never appears). Byte offsets into the typescript keep the
# phases apart.
set -u
D="$(CDPATH= cd -- "$(dirname -- "$0")" && pwd)"
W="$D/manual-theme"
rm -rf "$W"
# The fixture is a Go source file inside the module tree, so it lives
# under testdata/ — the go tool ignores that directory name for ./...
# package matching while rg searches it normally.
mkdir -p "$W/testdata"

cat >"$W/testdata/main.go" <<'EOF'
package main

func main() {
	println("hi")
}
EOF

mkfifo "$W/in"

cat >"$W/inner.sh" <<EOF
#!/bin/sh
cd "$W/testdata"
stty rows 24 cols 80
"$D/vrg" func .
echo "\$?" >"$W/exit.txt"
EOF
chmod +x "$W/inner.sh"

timeout 30 script -qfec "$W/inner.sh" "$W/typescript" <"$W/in" >/dev/null 2>&1 &
spid=$!
exec 9>"$W/in"

T="$W/typescript"
sz() { stat -c%s "$T" 2>/dev/null || echo 1; }

# wait_for <offset> <literal> — block until the literal appears in the
# typescript at or after the byte offset.
wait_for() {
	for _ in $(seq 1 600); do
		tail -c +"$1" "$T" 2>/dev/null | grep -aqF "$2" && return 0
		sleep 0.05
	done
	return 1
}

wait_for 1 $'\x1b[30;47;4m'       # dark browse frame: underlined inverse match
sleep 0.3                       # let the frame finish flushing
off1=$(( $(sz) + 1 ))
printf 'c' >&9                  # c → light
wait_for "$off1" $'\x1b[30;47m' # light base pair
sleep 0.3
off2=$(( $(sz) + 1 ))
printf 'c' >&9                  # c → dark again
wait_for "$off2" $'\x1b[37;40m' # dark base pair
sleep 0.3
printf 'q' >&9
wait "$spid" 2>/dev/null
exec 9>&-

seg1() { head -c "$((off1 - 1))" "$T"; }
seg2() { tail -c +"$off1" "$T" | head -c "$((off2 - off1))"; }
seg3() { tail -c +"$off2" "$T"; }

check() { # check <seg> <label> <present|absent> <literal>
	local seg=$1 label=$2 want=$3 lit=$4 got
	if $seg | grep -aqF "$lit"; then got=present; else got=absent; fi
	if [ "$got" = "$want" ]; then echo "$label=$want"; else echo "$label=VIOLATION($got)"; fi
}

echo "exit=$(cat "$W/exit.txt" 2>/dev/null || echo missing)"
# The terminal renderer canonicalizes embedded SGR per cell: underline
# merges with the ambient pair, so the current file entry arrives as
# 37;40;4 (dark) or 30;47;4 (light) rather than a bare 4.
check seg1 dark-base               present $'\x1b[37;40m'
check seg1 dark-match-inv-uline    present $'\x1b[30;47;4m'
check seg1 dark-current-file-uline present $'\x1b[37;40;4m./main.go'
check seg1 dark-no-light-base      absent  $'\x1b[30;47m'
check seg2 light-base              present $'\x1b[30;47m'
check seg2 light-match-inv-uline   present $'\x1b[37;40;4m'
check seg2 light-current-file      present $'\x1b[30;47;4m./main.go'
check seg2 light-no-dark-base      absent  $'\x1b[37;40m'
check seg3 dark-base-again         present $'\x1b[37;40m'
check seg3 dark-match-again        present $'\x1b[30;47;4m'
check seg3 dark-no-light-base      absent  $'\x1b[30;47m'

# Rendered rows, styles stripped: the list, filename rule, gutter, and
# the matched line.
echo "--- rendered rows ---"
sed $'s/\x1b\\[[0-9;]*m//g' "$T" | sed $'s/\x1b\\[[0-9;?><]*[a-zA-Z]/\\n/g' |
	tr -d '\r' | grep -avE '^\s*$|^Script ' | head -8
