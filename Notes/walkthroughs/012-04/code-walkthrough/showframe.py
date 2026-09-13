#!/usr/bin/env python3
"""Run vrg with a PTY, send keys, and print only the last rendered frame
with line numbers extracted from the content panel."""
import os, pty, sys, time, select, struct, fcntl, termios, re

keys = os.environ.get("VRG_KEYS", "q").split(",")
delay = float(os.environ.get("VRG_DELAY", "0.3"))
width = int(os.environ.get("VRG_WIDTH", "80"))
height = int(os.environ.get("VRG_HEIGHT", "24"))
stderr_file = os.environ.get("VRG_STDERR", "stderr.txt")

stderr_fd = os.open(stderr_file, os.O_WRONLY | os.O_CREAT | os.O_TRUNC, 0o644)
pid, fd = pty.fork()
if pid == 0:
    os.dup2(stderr_fd, 2)
    os.close(stderr_fd)
    os.execvp(sys.argv[1], sys.argv[1:])
else:
    os.close(stderr_fd)
    winsize = struct.pack("HHHH", height, width, 0, 0)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)
    output = []
    def drain(timeout):
        while True:
            try:
                r, _, _ = select.select([fd], [], [], timeout)
                if r:
                    data = os.read(fd, 4096)
                    if not data:
                        break
                    output.append(data)
                else:
                    break
            except OSError:
                break
    drain(1.0)
    for key in keys:
        time.sleep(delay)
        try:
            os.write(fd, key.encode())
        except OSError:
            break
        drain(0.3)
    drain(1.0)
    _, status = os.waitpid(pid, 0)
    exitcode = os.waitstatus_to_exitcode(status)
    text = b"".join(output).decode("utf-8", errors="replace")
    # Strip ANSI sequences.
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
    clean = ''.join(result)
    # The last frame is the last `height` lines of the output.
    lines = clean.split('\n')
    nonempty = [l for l in lines if l.strip()]
    frame = nonempty[-height:] if len(nonempty) > height else nonempty
    for line in frame:
        m = re.match(r'^.*?(\d+)\s+line (\d+):', line)
        if m:
            print(f"row {m.group(1)}: line {m.group(2)}")
        elif 'Loading' in line:
            print(line.strip())
        elif '\xe2\x80\x80' in line or '\u2500' in line or '──' in line:
            print(line.strip())
    sys.exit(exitcode)
