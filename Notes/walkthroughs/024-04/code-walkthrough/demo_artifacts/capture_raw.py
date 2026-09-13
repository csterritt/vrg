#!/usr/bin/env python3
"""Run vrg with a PTY and capture raw output (with ANSI) for inspection."""
import os, pty, sys, time, select, struct, fcntl, termios

keys = os.environ.get("VRG_KEYS", "q").split(",")
delay = float(os.environ.get("VRG_DELAY", "1.0"))
width = int(os.environ.get("VRG_WIDTH", "60"))
height = int(os.environ.get("VRG_HEIGHT", "8"))

pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])
else:
    winsize = struct.pack("HHHH", height, width, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)
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
    for key in keys:
        time.sleep(delay)
        os.write(fd, key.encode())
    time.sleep(delay)
    try:
        while True:
            r, _, _ = select.select([fd], [], [], 1.0)
            if r:
                data = os.read(fd, 4096)
                if not data:
                    break
                output += data
            else:
                break
    except OSError:
        pass
    sys.stdout.buffer.write(output)
    sys.stdout.flush()
