#!/usr/bin/env python3
"""Run vrg with a PTY, send keys, and print the final rendered frame
as readable text with SGR styling shown as \x1b[...m sequences.

Usage: python3 showframe.py <keys>
  keys: comma-separated keys to send (e.g. "n", "n,n", "p")
The script always appends 'q' to quit.
"""
import os, sys, subprocess, re

keys = sys.argv[1] if len(sys.argv) > 1 else ""
if keys:
    keys = keys + ",q"
else:
    keys = "q"

env = os.environ.copy()
env["VRG_KEYS"] = keys
env["VRG_DELAY"] = "0.3"
env["VRG_WIDTH"] = "80"
env["VRG_HEIGHT"] = "24"
env["VRG_RAW"] = "1"
env["VRG_STDERR"] = "stderr.txt"
env["PATH"] = os.path.dirname(os.path.abspath(__file__)) + "/fakebin:" + env.get("PATH", "")

result = subprocess.run(
    ["python3", "runpty.py", "./vrg", "match", "."],
    capture_output=True, text=True, env=env, cwd=os.path.dirname(os.path.abspath(__file__))
)
text = result.stdout
# Strip OSC, charset, keypad sequences but keep CSI (SGR) for styling.
text = re.sub(r'\x1b\][^\x07\x1b]*(?:\x07|\x1b\\)', '', text)
text = re.sub(r'\x1b[()][0B]', '', text)
text = re.sub(r'\x1b[=>]', '', text)
# Strip kitty keyboard protocol flags (>4m, <1u, etc.)
text = re.sub(r'\x1b\[>[\d;]*m', '', text)
text = re.sub(r'\x1b\[[\d;]*u', '', text)
text = re.sub(r'\x1b\[<[\d;]*u', '', text)
text = text.replace('\r', '')

# Cut off at the alt-screen exit sequence (quit). We only want the
# last frame before the quit.
quit_marker = '\x1b[?1049l'
if quit_marker in text:
    text = text[:text.index(quit_marker)]

# The output contains cursor positioning sequences interspersed with
# content. We need to reconstruct the visible screen. The terminal is
# 80x24, but SGR escape codes take up space in the raw output, so use
# a wider buffer to avoid truncation. Build a 2D buffer and apply the
# sequences.
buf_width = 256
rows = [[' '] * buf_width for _ in range(24)]
row, col = 0, 0
i = 0
while i < len(text):
    ch = text[i]
    if ch == '\x1b':
        # ESC sequence
        i += 1
        if i < len(text) and text[i] == '[':
            # CSI sequence
            i += 1
            params = ''
            while i < len(text) and text[i] in '0123456789;':
                params += text[i]
                i += 1
            if i < len(text):
                cmd = text[i]
                i += 1
                if cmd == 'H':
                    # Cursor position
                    parts = params.split(';')
                    r = int(parts[0]) - 1 if parts[0] else 0
                    c = int(parts[1]) - 1 if len(parts) > 1 and parts[1] else 0
                    row = max(0, min(23, r))
                    col = max(0, min(buf_width - 1, c))
                elif cmd == 'J':
                    # Erase display
                    n = int(params) if params else 0
                    if n == 2:
                        rows = [[' '] * buf_width for _ in range(24)]
                    elif n == 1:
                        for r in range(row + 1):
                            for c in range(buf_width):
                                if r < row or (r == row and c < col):
                                    rows[r][c] = ' '
                    elif n == 0:
                        for c in range(col, buf_width):
                            rows[row][c] = ' '
                        for r in range(row + 1, 24):
                            for c in range(buf_width):
                                rows[r][c] = ' '
                elif cmd == 'K':
                    # Erase line
                    n = int(params) if params else 0
                    if n == 1:
                        for c in range(col):
                            rows[row][c] = ' '
                    elif n == 2:
                        for c in range(buf_width):
                            rows[row][c] = ' '
                    else:
                        for c in range(col, buf_width):
                            rows[row][c] = ' '
                elif cmd == 'm':
                    # SGR - render as readable escape
                    sgr = '\\x1b[' + params + 'm'
                    for c in sgr:
                        if col < buf_width:
                            rows[row][col] = c
                            col += 1
                # Other commands (A, B, C, D, X, etc.) - handle simply
                elif cmd == 'A':
                    row = max(0, row - (int(params) if params else 1))
                elif cmd == 'B':
                    row = min(23, row + (int(params) if params else 1))
                elif cmd == 'C':
                    col = min(buf_width - 1, col + (int(params) if params else 1))
                elif cmd == 'D':
                    col = max(0, col - (int(params) if params else 1))
                elif cmd == 'X':
                    # Erase n characters
                    n = int(params) if params else 1
                    for c in range(col, min(buf_width, col + n)):
                        rows[row][c] = ' '
                # Ignore other CSI commands
        elif i < len(text) and text[i] == '?':
            # Private mode - skip to end
            i += 1
            while i < len(text) and text[i] not in 'hlp':
                i += 1
            if i < len(text):
                i += 1
        else:
            i += 1
    elif ch == '\n':
        row = min(23, row + 1)
        col = 0
        i += 1
    else:
        if row < 24 and col < buf_width:
            rows[row][col] = ch
            col += 1
        i += 1

# Print the screen, trimming trailing spaces and empty lines.
for r in range(24):
    line = ''.join(rows[r]).rstrip()
    if line:
        print(line)
