#!/usr/bin/env python3
"""Run vrg with a PTY, wait for output to settle, send a key, strip ANSI, print output.

Environment variables:
  VRG_KEY       - the key to send (default: q)
  VRG_DELAY     - seconds to wait before sending the key (default: 0.5)
  VRG_WIDTH     - terminal width (default: 100)
  VRG_HEIGHT    - terminal height (default: 30)
"""
import os, pty, sys, time, select, struct, fcntl, termios

key = os.environ.get("VRG_KEY", "q").encode()
delay = float(os.environ.get("VRG_DELAY", "0.5"))
width = int(os.environ.get("VRG_WIDTH", "100"))
height = int(os.environ.get("VRG_HEIGHT", "30"))

pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])
else:
    winsize = struct.pack("HHHH", height, width, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)

    # Wait for output to settle (fake rg completes instantly).
    output = b""
    while True:
        r, _, _ = select.select([fd], [], [], 1.0)
        if r:
            data = os.read(fd, 4096)
            if not data:
                break
            output += data
        else:
            break

    # Send the key after output has settled.
    time.sleep(delay)
    os.write(fd, key)

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
    sys.exit(exitcode)
