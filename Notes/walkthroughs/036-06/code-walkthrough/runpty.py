#!/usr/bin/env python3
"""Run vrg with a PTY, wait for output to settle, send keys, capture raw output.

Environment variables:
  VRG_KEYS     - comma-separated keys to send (default: q)
  VRG_DELAY    - seconds to wait before sending each key (default: 0.5)
  VRG_WIDTH    - terminal width (default: 100)
  VRG_HEIGHT   - terminal height (default: 30)
  VRG_RAW      - if set, do not strip ANSI sequences (default: strip)
"""
import os, pty, sys, time, select, struct, fcntl, termios

keys = os.environ.get("VRG_KEYS", "q").split(",")
delay = float(os.environ.get("VRG_DELAY", "0.5"))
width = int(os.environ.get("VRG_WIDTH", "100"))
height = int(os.environ.get("VRG_HEIGHT", "30"))
raw = bool(os.environ.get("VRG_RAW", ""))

pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])
else:
    winsize = struct.pack("HHHH", height, width, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)

    # Wait for output to settle (fake rg completes instantly).
    output = b""
    while True:
        try:
            r, _, _ = select.select([fd], [], [], 1.0)
            if r:
                data = os.read(fd, 4096)
                if not data:
                    break
                output += data
            else:
                break
        except OSError:
            break

    # Send each key after a delay, capturing output between keys.
    for key in keys:
        time.sleep(delay)
        try:
            os.write(fd, key.encode())
        except OSError:
            break
        # Read output triggered by the key.
        while True:
            try:
                r, _, _ = select.select([fd], [], [], 0.3)
                if r:
                    data = os.read(fd, 4096)
                    if not data:
                        break
                    output += data
                else:
                    break
            except OSError:
                break

    # Read remaining output.
    while True:
        try:
            r, _, _ = select.select([fd], [], [], 1.0)
            if r:
                data = os.read(fd, 4096)
                if not data:
                    break
                output += data
            else:
                break
        except OSError:
            break

    _, status = os.waitpid(pid, 0)
    exitcode = os.waitstatus_to_exitcode(status)

    text = output.decode("utf-8", errors="replace")
    if raw:
        sys.stdout.write(text)
        if text and not text.endswith("\n"):
            sys.stdout.write("\n")
    else:
        # Strip all escape sequences for readable output.
        result = []
        i = 0
        while i < len(text):
            if text[i] == '\x1b':
                i += 1
                if i < len(text) and text[i] == '[':
                    i += 1
                    while i < len(text) and not (0x40 <= ord(text[i]) <= 0x7e):
                        i += 1
                    if i < len(text):
                        i += 1
                elif i < len(text) and text[i] == ']':
                    i += 1
                    while i < len(text):
                        if text[i] == '\x07':
                            i += 1
                            break
                        if text[i] == '\x1b' and i+1 < len(text) and text[i+1] == '\\':
                            i += 2
                            break
                        i += 1
                else:
                    i += 1
            elif text[i] == '\r':
                i += 1
            else:
                result.append(text[i])
                i += 1
        sys.stdout.write(''.join(result))
        if result and not result[-1] == '\n':
            sys.stdout.write('\n')
    sys.exit(exitcode)
