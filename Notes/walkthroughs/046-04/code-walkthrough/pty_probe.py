#!/usr/bin/env python3
"""Issue #46 PTY probe: run a vrg binary on a real 80x24 PTY, wait until
the diagnostic-acknowledgement file proves two session diagnostics were
collected (the child's stderr processed into the model, not merely
written), send q, then report the exit status, the display-restoration
sequence, the termios restoration, the child's fate, the reap evidence,
and the post-restoration output.

Usage: pty_probe.py <binary> <workdir> <fakebin> <ackfile> <pidfile> <reapfile> [KEY=VAL ...]

  <fakebin>  directory prepended to PATH so the fixture rg answers the
             search
  KEY=VAL    extra environment entries — the probe sets the tagged
             runner controls (VRG_TEST_RUN_FINAL_MODEL/VRG_TEST_RUN_ERROR)
             and VRG_TEST_DIAGNOSTIC_TRIGGER/VRG_TEST_REAP this way

Every wait is a bounded poll on an explicit condition (the ack file's
line count, PTY EOF) — no fixed settling delays.
"""

import fcntl
import os
import pty
import select
import signal
import struct
import subprocess
import sys
import termios
import time

RESTORE = "\x1b[?1049l"  # leave-alternate-screen: display restored


def main():
    binary = os.path.abspath(sys.argv[1])
    workdir = os.path.abspath(sys.argv[2])
    fakebin = os.path.abspath(sys.argv[3])
    ackfile, pidfile, reapfile = (os.path.abspath(a) for a in sys.argv[4:7])
    extra = dict(kv.split("=", 1) for kv in sys.argv[7:])
    env = dict(os.environ)
    env["PATH"] = fakebin + os.pathsep + env["PATH"]
    env["TERM"] = "xterm"
    env.update(extra)

    master, slave = pty.openpty()
    fcntl.ioctl(master, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
    before = termios.tcgetattr(slave)
    p = subprocess.Popen([binary, "foo"], cwd=workdir, env=env,
                         stdin=slave, stdout=slave, stderr=slave,
                         start_new_session=True)

    # Wait until the ack file holds two lines: both diagnostics are
    # processed into the session collection before the exit key.
    deadline = time.monotonic() + 20
    while time.monotonic() < deadline:
        try:
            with open(ackfile) as f:
                if sum(1 for _ in f) >= 2:
                    break
        except OSError:
            pass
        time.sleep(0.01)
    else:
        p.kill()
        p.wait()
        print("FAIL: diagnostics never acknowledged as collected")
        sys.exit(1)

    os.write(master, b"q")
    raw = bytearray()
    while time.monotonic() < deadline:
        r, _, _ = select.select([master], [], [], 0.05)
        if not r:
            continue
        try:
            chunk = os.read(master, 65536)
        except OSError:
            break  # EIO: slave closed, process gone
        if not chunk:
            break
        raw += chunk
    code = p.wait(timeout=5)

    out = raw.decode(errors="replace")
    idx = out.find(RESTORE)
    after = termios.tcgetattr(slave)

    pid = int(open(pidfile).read().strip())
    try:
        os.kill(pid, 0)
        child = "still alive"
    except OSError:
        child = "gone"
    try:
        reap = open(reapfile).read().strip()
    except OSError:
        reap = "<missing>"

    print("exit=%d" % code)
    print("restoration-sequence seen: %s" % (idx >= 0))
    print("termios restored: %s" % (before == after))
    print("child pid %d: %s; reap evidence: %s" % (pid, child, reap))
    print("post-restoration output: %r" % (out[idx:] if idx >= 0 else out))


if __name__ == "__main__":
    main()
