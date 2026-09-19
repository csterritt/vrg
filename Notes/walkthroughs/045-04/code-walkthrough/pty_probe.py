#!/usr/bin/env python3
"""Issue #45 PTY probe: run one vrg binary on a real 80x24 PTY under a
caller-chosen environment, wait for the browse view, send q, and report
the exit status and the post-restoration output.

Usage: pty_probe.py <binary> <workdir> <fakebin> [KEY=VAL ...]

  <fakebin>  directory prepended to PATH so the fixture rg answers the
             search
  KEY=VAL    extra environment entries - the probe sets the explicit
             vrg-consumed hook manifest this way

Every wait is a bounded poll on an explicit condition (the rendered
browse marker, PTY EOF) — no fixed settling delays.
"""

import fcntl
import os
import pty
import select
import signal
import struct
import sys
import termios
import time

BROWSE = "─ ".encode()  # the filename rule marks the browse view
RESTORE = "\x1b[?1049l"  # leave-alternate-screen: terminal restored


def fail(msg):
    print("FAIL: %s" % msg)
    sys.exit(1)


def main():
    binary = os.path.abspath(sys.argv[1])
    workdir = os.path.abspath(sys.argv[2])
    fakebin = os.path.abspath(sys.argv[3])
    extra = dict(kv.split("=", 1) for kv in sys.argv[4:])
    env = dict(os.environ)
    env["PATH"] = fakebin + os.pathsep + env["PATH"]
    env["TERM"] = "xterm"
    env.update(extra)

    pid, fd = pty.fork()
    if pid == 0:
        os.chdir(workdir)
        os.execvpe(binary, [binary, "foo"], env)
        os._exit(127)
    fcntl.ioctl(fd, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))

    raw = bytearray()
    deadline = time.monotonic() + 20
    while BROWSE not in raw and time.monotonic() < deadline:
        r, _, _ = select.select([fd], [], [], 0.05)
        if r:
            try:
                raw += os.read(fd, 65536)
            except OSError:
                break
    if BROWSE not in raw:
        os.kill(pid, signal.SIGKILL)
        os.waitpid(pid, 0)
        fail("browse view never appeared; output tail: %r"
             % raw.decode(errors="replace")[-800:])

    os.write(fd, b"q")
    while time.monotonic() < deadline:
        r, _, _ = select.select([fd], [], [], 0.05)
        if not r:
            continue
        try:
            chunk = os.read(fd, 65536)
        except OSError:
            break  # EIO: slave closed, process gone
        if not chunk:
            break
        raw += chunk
    _, status = os.waitpid(pid, 0)
    code = os.waitstatus_to_exitcode(status)

    out = raw.decode(errors="replace")
    idx = out.find(RESTORE)
    print("exit=%d" % code)
    print("restoration-sequence seen: %s" % (idx >= 0))
    print("post-restoration output: %r" % (out[idx:] if idx >= 0 else out))


if __name__ == "__main__":
    main()
