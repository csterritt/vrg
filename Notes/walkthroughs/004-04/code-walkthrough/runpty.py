#!/usr/bin/env python3
"""Run vrg with a PTY, send a key after a handshake, strip ANSI, print output.

Environment variables:
  VRG_HANDSHAKE  - file path to wait for before sending the key
  VRG_KEY        - the key to send (default: q)
  VRG_STTY_BEFORE - file path to write pre-run stty -a output
  VRG_STTY_AFTER  - file path to write post-run stty -a output
"""
import os, pty, sys, time, select, struct, fcntl, termios, subprocess

handshake = os.environ.get("VRG_HANDSHAKE", "")
key = os.environ.get("VRG_KEY", "q").encode()
stty_before = os.environ.get("VRG_STTY_BEFORE", "")
stty_after = os.environ.get("VRG_STTY_AFTER", "")
rows, cols = 24, 80

# Capture pre-run stty state of the controlling terminal.
if stty_before:
    try:
        r = subprocess.run(["stty", "-a"], capture_output=True, text=True)
        with open(stty_before, "w") as f:
            f.write(r.stdout)
    except Exception:
        pass

pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])
else:
    winsize = struct.pack("HHHH", rows, cols, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)

    if handshake:
        for _ in range(2000):
            if os.path.exists(handshake):
                break
            time.sleep(0.01)

    os.write(fd, key)

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

    # Capture post-run stty state.
    if stty_after:
        try:
            r = subprocess.run(["stty", "-a"], capture_output=True, text=True)
            with open(stty_after, "w") as f:
                f.write(r.stdout)
        except Exception:
            pass

    text = output.decode("utf-8", errors="replace")
    # Strip all escape sequences.
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
