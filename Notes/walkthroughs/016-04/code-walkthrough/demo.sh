#!/usr/bin/env bash
# Demonstration script for Issue #16 wrap mode and grapheme policy.
# Runs the vrg binary against a file with a 500-character line plus a
# tab-indented line, and captures the screen at each step:
#   - Initial wrapped rows with blank continuation gutters
#   - `w` toggling to run-off-edge (one clipped row)
#   - Tabs aligned to eight-column stops
#   - A match near the end of the long line revealed on its own row after `n`
#
# This script is self-contained: it builds the binary and creates the
# test fixture file if they are missing.
set -euo pipefail
cd "$(dirname "$0")"
VRG_ROOT="$(cd ../../../.. && pwd)"

# Build the binary if missing.
if [ ! -x ./vrg-demo ]; then
  (cd "$VRG_ROOT" && go build -o Notes/walkthroughs/016-04/code-walkthrough/vrg-demo ./cmd/vrg)
fi

# Create the test fixture file if missing.
if [ ! -f ./wrapfile.txt ]; then
  python3 -c "
long_line = 'x' * 500 + ' TARGET'
tab_line = '\tcol1\tcol2\tcol3'
lines = [
    'short line one TARGET',
    long_line,
    tab_line,
    'short line two',
]
with open('wrapfile.txt', 'w') as f:
    f.write('\n'.join(lines) + '\n')
"
fi

python3 << 'PYEOF'
import os
import pty
import select
import time
import re
import struct
import fcntl
import termios

ROWS, COLS = 24, 60

def drain(fd, settle=0.4):
    time.sleep(settle)
    out = b""
    while True:
        r, _, _ = select.select([fd], [], [], 0.15)
        if not r:
            break
        try:
            chunk = os.read(fd, 65536)
        except OSError:
            break
        if not chunk:
            break
        out += chunk
    return out

def render_screen(data, rows=ROWS, cols=COLS):
    """Interpret ANSI cursor positioning to reconstruct the screen."""
    screen = [[' '] * cols for _ in range(rows)]
    r, c = 0, 0
    i = 0
    text = data.decode('utf-8', errors='replace')
    while i < len(text):
        ch = text[i]
        if ch == '\x1b':
            j = i + 1
            if j < len(text) and text[j] == '[':
                j += 1
                params = ''
                while j < len(text) and text[j] not in 'ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz':
                    params += text[j]
                    j += 1
                if j < len(text):
                    cmd = text[j]
                    j += 1
                    if cmd == 'H':
                        parts = params.split(';')
                        try:
                            nr = int(parts[0]) if parts[0] else 1
                            nc = int(parts[1]) if len(parts) > 1 and parts[1] else 1
                            r, c = nr - 1, nc - 1
                        except ValueError:
                            r, c = 0, 0
                    elif cmd == 'J':
                        if params == '2':
                            screen = [[' '] * cols for _ in range(rows)]
                            r, c = 0, 0
                    elif cmd == 'K':
                        if params == '1':
                            for k in range(c):
                                screen[r][k] = ' '
                        elif params == '2':
                            for k in range(cols):
                                screen[r][k] = ' '
                        else:
                            for k in range(c, cols):
                                screen[r][k] = ' '
                    elif cmd == 'S':
                        # Scroll up N lines (content moves up, bottom N
                        # lines become blank).
                        try:
                            n = int(params) if params else 1
                        except ValueError:
                            n = 1
                        for k in range(n):
                            screen.pop(0)
                            screen.append([' '] * cols)
                    elif cmd == 'T':
                        # Scroll down N lines (content moves down, top N
                        # lines become blank).
                        try:
                            n = int(params) if params else 1
                        except ValueError:
                            n = 1
                        for k in range(n):
                            screen.insert(0, [' '] * cols)
                            screen.pop()
                    elif cmd == 'd':
                        # Vertical position absolute (move to row N).
                        try:
                            r = int(params) - 1 if params else 0
                        except ValueError:
                            pass
                    i = j
                    continue
            elif j < len(text) and text[j] == ']':
                while j < len(text) and text[j] != '\x07':
                    j += 1
                if j < len(text):
                    j += 1
                i = j
                continue
            elif j < len(text) and text[j] in '=>':
                i = j + 1
                continue
            else:
                i = j
                continue
        elif ch == '\r':
            c = 0
            i += 1
        elif ch == '\n':
            r = min(r + 1, rows - 1)
            i += 1
        elif ch == '\b':
            c = max(c - 1, 0)
            i += 1
        elif ch >= ' ':
            if 0 <= r < rows and 0 <= c < cols:
                screen[r][c] = ch
            c = min(c + 1, cols - 1)
            i += 1
        else:
            i += 1
    return [''.join(row).rstrip() for row in screen]

def show_screen(data, label, rows=ROWS, cols=COLS, width=70):
    print(f"=== {label} ===")
    lines = render_screen(data, rows, cols)
    for line in lines:
        if line.strip():
            print(f"  {line[:width]}")
    print()

# Start the vrg binary in a pty. Search for TARGET so there are two
# stops: line 1 and the end of line 2 (the 500-char line).
pid, fd = pty.fork()
if pid == 0:
    os.execv("./vrg-demo", ["vrg-demo", "--", "TARGET", "wrapfile.txt"])
    os._exit(1)

# Set terminal size to 24 rows, 60 columns (narrow to force wrapping).
winsize = struct.pack("HHHH", ROWS, COLS, 0, 0)
fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)

# Accumulate all output so the screen interpreter always renders the
# full current screen, not just the incremental diff.
all_output = b""

# Wait for search + startup load + reveal of line 1's match.
all_output += drain(fd, 1.5)
show_screen(all_output, "Initial screen (wrap on, startup reveal of line 1 match)")

# Press w to toggle to run-off-edge mode (one clipped row per source line).
os.write(fd, b'w')
all_output += drain(fd, 0.6)
show_screen(all_output, "After w (run-off-edge: long line is one clipped row)")

# Press w to toggle back to wrap mode.
os.write(fd, b'w')
all_output += drain(fd, 0.6)
show_screen(all_output, "After w again (wrap back on)")

# Press n to navigate to the TARGET at the end of the 500-char line.
# The reveal finds the wrapped row containing the match and places it
# at one-third of the viewport height.
os.write(fd, b'n')
all_output += drain(fd, 0.8)
show_screen(all_output, "After n (match near end of long line revealed on its own row)")

# Quit
os.write(fd, b'q')
time.sleep(0.2)
try:
    os.close(fd)
except OSError:
    pass
try:
    os.waitpid(pid, 0)
except OSError:
    pass
print("=== Demo complete ===")
PYEOF
