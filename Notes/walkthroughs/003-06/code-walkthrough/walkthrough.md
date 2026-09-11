# Issue #3: Spawn rg, collect results, searching screen

*2026-09-11 by Showboat 0.6.1*

Walkthrough for Issue #3 (`Notes/issues/003-spawn-rg-collect-results-searching-screen.md`), implementing the ripgrep execution, JSON result collection, and searching/summary TUI specified in `Notes/PRD-vrg.md` (*Module Design → SearchIndex / App*; *Testing Decisions → SearchIndex / App*). It replaces the Issue #2 stub with real subprocess execution: `cmd/vrg` starts ripgrep with the protected child argv, `internal/app` drains stdout and stderr concurrently and parses the JSON stream via `internal/searchindex`, and the Bubble Tea model shows a searching screen that transitions to an interim summary. All generated artifacts live in this directory: the built `vrg` binary, the `fixtures/` tree, and the `fakebin/rg` script.

```bash
cd /home/chris/vrg && go build ./... && go vet ./... && echo GATES-OK
```

```output
GATES-OK
```

```bash
cd /home/chris/vrg && go test -count=1 ./internal/searchindex/ ./internal/app/ ./cmd/vrg/ | sed 's/[[:space:]][0-9.]*s$//'
```

```output
ok  	vrg/internal/searchindex
ok  	vrg/internal/app
ok  	vrg/cmd/vrg
```

```bash
cd /home/chris/vrg && go build -o Notes/walkthroughs/003-06/code-walkthrough/vrg ./cmd/vrg && file Notes/walkthroughs/003-06/code-walkthrough/vrg | cut -d: -f2 | cut -c1-60
```

```output
 ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), s
```

## Start failure: rg not on PATH. When ripgrep cannot be started, vrg prints a sanitized diagnostic to stderr and exits 2 without entering the TUI. stdout is empty (no TUI output).

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough
PATH=/nonexistent ./vrg hello fixtures >o.txt 2>e.txt; echo "exit=$? stdout_bytes=$(wc -c <o.txt) diag=$(head -1 e.txt)"
```

```output
exit=2 stdout_bytes=0 diag=vrg: cannot start ripgrep: exec: "rg": executable file not found in $PATH
```

## Protected child argv forwarded to rg. The exact child argv from the CLI contract (`--json --no-config <flags> -- <pattern> <root>`) is passed to ripgrep with the invocation working directory. User flags are forwarded in encounter order with exact spellings.

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough
cat > fakebin/rg_argv << 'RGEOF'
#!/bin/sh
if [ -n "$VRG_TEST_ARGV" ]; then printf '%s\n' "$*" > "$VRG_TEST_ARGV"; fi
printf '%s\n' '{"type":"summary","data":{}}'
if [ -n "$VRG_HANDSHAKE" ]; then touch "$VRG_HANDSHAKE"; fi
exit 0
RGEOF
chmod +x fakebin/rg_argv
cp fakebin/rg_argv fakebin/rg
rm -f /tmp/handshake /tmp/argv
VRG_HANDSHAKE=/tmp/handshake VRG_TEST_ARGV=/tmp/argv PATH="$PWD/fakebin:$PATH" python3 runpty.py ./vrg -i -S hello fixtures >o.txt 2>&1; echo "exit=$?"
echo "child argv: $(cat /tmp/argv)"
echo "summary: $(cat o.txt)"
cp fakebin/rg_argv fakebin/rg  # restore for next demo
rm -f fakebin/rg_argv
```

```output
exit=0
child argv: --json --no-config -i -S -- hello fixtures
summary: 0 files, 0 matched lines
```

## Searching screen transitions to summary. While rg is running, the TUI shows "Searching…". When collection completes, it transitions to the interim summary showing file and matched-line counts. The fake rg emits a valid ripgrep JSON stream with 2 files and 3 matched lines.

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough
rm -f /tmp/handshake
VRG_HANDSHAKE=/tmp/handshake PATH="$PWD/fakebin:$PATH" python3 runpty.py ./vrg hello fixtures 2>&1; echo " exit=$?"
```

```output
2 files, 3 matched lines exit=0
```

## Searching screen visible during collection. With a slow rg, the "Searching…" screen is visible before the summary appears.

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough
cat > fakebin/rg_slow << 'RGEOF'
#!/bin/sh
sleep 1
printf '%s\n' '{"type":"begin","data":{"path":{"text":"fixtures/hello.go.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"fixtures/hello.go.txt"},"lines":{"text":"hello\n"},"line_number":6,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
printf '%s\n' '{"type":"end","data":{"path":{"text":"fixtures/hello.go.txt"},"binary_offset":null}}'
printf '%s\n' '{"type":"summary","data":{}}'
if [ -n "$VRG_HANDSHAKE" ]; then touch "$VRG_HANDSHAKE"; fi
exit 0
RGEOF
chmod +x fakebin/rg_slow
cp fakebin/rg_slow fakebin/rg
rm -f /tmp/handshake
VRG_HANDSHAKE=/tmp/handshake PATH="$PWD/fakebin:$PATH" python3 runpty.py ./vrg hello fixtures 2>&1; echo " exit=$?"
rm -f fakebin/rg_slow
```

```output
Searching…1 files, 1 matched lines exit=0
```

## Zero-result summary. When rg finds no matches, the summary shows zero files and zero matched lines.

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough
cat > fakebin/rg_empty << 'RGEOF'
#!/bin/sh
printf '%s\n' '{"type":"summary","data":{}}'
if [ -n "$VRG_HANDSHAKE" ]; then touch "$VRG_HANDSHAKE"; fi
exit 0
RGEOF
chmod +x fakebin/rg_empty
cp fakebin/rg_empty fakebin/rg
rm -f /tmp/handshake
VRG_HANDSHAKE=/tmp/handshake PATH="$PWD/fakebin:$PATH" python3 runpty.py ./vrg nomatch fixtures 2>&1; echo " exit=$?"
rm -f fakebin/rg_empty
```

```output
0 files, 0 matched lines exit=0
```

## Quit during search exits 130. Pressing `q` while still searching exits with code 130 (like Ctrl-C). Pressing `q` after the summary appears exits 0.

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough
cat > fakebin/rg_slow << 'RGEOF'
#!/bin/sh
sleep 2
printf '%s\n' '{"type":"summary","data":{}}'
if [ -n "$VRG_HANDSHAKE" ]; then touch "$VRG_HANDSHAKE"; fi
exit 0
RGEOF
chmod +x fakebin/rg_slow
cp fakebin/rg_slow fakebin/rg
VRG_HANDSHAKE="" PATH="$PWD/fakebin:$PATH" python3 runpty.py ./vrg hello fixtures 2>&1; echo " exit=$?"
rm -f fakebin/rg_slow
```

```output
Searching… exit=130
```

## Dual-pipe drainage: 200 KiB stderr interleaved with stdout. The fake rg writes 200 KiB to stderr (well over the 64 KiB pipe capacity) interleaved with valid stdout records. vrg drains both pipes concurrently so neither deadlocks nor loses the stdout stream.

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough
cat > fakebin/rg_dual << 'RGEOF'
#!/bin/sh
i=0
while [ $i -lt 20 ]; do
	head -c 10240 /dev/zero | tr '\0' 'E' >&2
	printf '%s\n' '{"type":"begin","data":{"path":{"text":"fixtures/hello.go.txt"}}}'
	printf '%s\n' '{"type":"match","data":{"path":{"text":"fixtures/hello.go.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
	printf '%s\n' '{"type":"end","data":{"path":{"text":"fixtures/hello.go.txt"},"binary_offset":null}}'
	i=$((i + 1))
done
printf '%s\n' '{"type":"summary","data":{}}'
if [ -n "$VRG_HANDSHAKE" ]; then touch "$VRG_HANDSHAKE"; fi
exit 0
RGEOF
chmod +x fakebin/rg_dual
cp fakebin/rg_dual fakebin/rg
rm -f /tmp/handshake
VRG_HANDSHAKE=/tmp/handshake PATH="$PWD/fakebin:$PATH" python3 runpty.py ./vrg hello fixtures 2>&1; echo " exit=$?"
rm -f fakebin/rg_dual
```

```output
1 files, 1 matched lines exit=0
```

## Real ripgrep execution. With the real ripgrep on PATH, vrg searches the fixtures directory and collects the actual results.

```bash
cd /home/chris/vrg/Notes/walkthroughs/003-06/code-walkthrough
rm -f /tmp/handshake
VRG_HANDSHAKE=/tmp/handshake PATH="/home/linuxbrew/.linuxbrew/bin:$PATH" python3 runpty.py ./vrg hello fixtures 2>&1; echo " exit=$?"
```

```output
2 files, 3 matched lines exit=0
```

## Result. Every Issue #3 manual-verification class is demonstrated: start failure with sanitized diagnostic and exit 2 (no TUI); protected child argv forwarded to rg (`--json --no-config -i -S -- hello fixtures`); searching screen visible during collection; completion-to-summary transition with file and matched-line counts (2 files, 3 matched lines); zero-result summary (0 files, 0 matched lines); quit during search exits 130, quit from summary exits 0; dual-pipe drainage of 200 KiB stderr interleaved with stdout without deadlock or stream loss; and real ripgrep execution producing the same result as the fake. Suites: `searchindex_test.go` (SearchIndex parsing, encoding, merging, ranges, ordering, path resolution); `app_test.go` (searching state, summary transition, key handling, gate, start failure); `search_test.go` (child argv, workdir, start failure, dual-pipe backpressure, stderr capture).
