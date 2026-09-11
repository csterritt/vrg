#!/usr/bin/env python3
"""Run vrg with a PTY, send q after handshake, strip ANSI, print output."""
import os, pty, sys, time, select, re, struct, fcntl, termios

handshake = os.environ.get("VRG_HANDSHAKE", "")
rows, cols = 24, 80

pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])
else:
    winsize = struct.pack("HHHH", rows, cols, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)
    
    if handshake:
        for _ in range(1000):
            if os.path.exists(handshake):
                break
            time.sleep(0.01)
    
    os.write(fd, b"q")
    
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
    
    _, status = os.waitpid(pid, 0)
    exitcode = os.waitstatus_to_exitcode(status)
    
    text = output.decode("utf-8", errors="replace")
    # Strip all escape sequences: ESC followed by any bytes until a
    # terminator in the range 0x40-0x7e (for CSI) or a single byte.
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
                # OSC: until BEL or ST (ESC \)
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
