#!/usr/bin/env python3
"""Issue #48 PTY probe: run the vrg_testhooks binary on a real 120x30
PTY against real ripgrep, drive the deterministic-handshake contract
directly — baseline the per-event acknowledgement count before acting,
wait for it to grow past the baseline — and print the event log the
VRG_TEST_EVENT_ACK seam wrote.

Usage: ack_probe.py <binary> <workdir>

  <binary>   a vrg binary built with -tags vrg_testhooks
  <workdir>  directory to search; the probe creates fixture files there

Every wait is a bounded poll on the acknowledgement log's per-event
count — exactly what awaitAck does in cmd/vrg/handshake_test.go — so
the probe proves the seam works outside the test harness too. No fixed
settling delays anywhere.
"""

import fcntl
import os
import pty
import select
import struct
import subprocess
import sys
import termios
import time

BOUND = 20.0  # seconds; matches waitAck's standard bound


def count_event(path, event):
    """Number of '<seq> <event>' records of event in the log so far."""
    try:
        with open(path, "r", encoding="utf-8", errors="replace") as f:
            data = f.read()
    except OSError:
        return 0
    return sum(1 for ln in data.splitlines()
               if ln.split(" ", 1)[-1] == event)


def wait_ack(path, event, have, bound=BOUND):
    """Poll until the count of event exceeds the have baseline."""
    deadline = time.monotonic() + bound
    while time.monotonic() < deadline:
        if count_event(path, event) > have:
            return
        time.sleep(0.01)
    raise SystemExit(f"ack {event!r} occurrence {have + 1} never arrived "
                     f"within {bound}s")


def drain(fd):
    out = b""
    while True:
        r, _, _ = select.select([fd], [], [], 0.05)
        if not r:
            return out
        try:
            chunk = os.read(fd, 65536)
        except OSError:
            return out
        if not chunk:
            return out
        out += chunk


def main():
    binary = os.path.abspath(sys.argv[1])
    workdir = os.path.abspath(sys.argv[2])
    os.makedirs(workdir, exist_ok=True)
    for name in ("a.txt", "b.txt"):
        with open(os.path.join(workdir, name), "w") as f:
            f.write("needle one\nneedle two\n")

    eventlog = os.path.join(workdir, "events.log")
    env = dict(os.environ, VRG_TEST_EVENT_ACK=eventlog,
               TERM="xterm-256color")

    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ,
                struct.pack("HHHH", 30, 120, 0, 0))
    proc = subprocess.Popen([binary, "needle"], cwd=workdir, env=env,
                            stdin=slave, stdout=slave, stderr=slave,
                            close_fds=True)
    os.close(slave)

    # state:searching is emitted by Init; baseline 0 proves the seam is
    # live before we trust it.
    wait_ack(eventlog, "state:searching", 0)
    print("ack: state:searching")

    # The search→browse transition is the handshake for "rg's stream
    # was collected and the outcome decided" — wait for it, then for
    # the current file's load settling, before touching the pty.
    wait_ack(eventlog, "state:browse", 0)
    print("ack: state:browse")
    wait_ack(eventlog, "load:ok", 0)
    print("ack: load:ok")

    # Baseline the key record, send q, wait for the occurrence our send
    # caused — never a fixed inter-key delay.
    have = count_event(eventlog, "key:q")
    os.write(master, b"q")
    wait_ack(eventlog, "key:q", have)
    print(f"ack: key:q occurrence {have + 1}")

    code = proc.wait(timeout=BOUND)
    drain(master)
    os.close(master)
    print(f"exit={code}")

    with open(eventlog, "r", encoding="utf-8", errors="replace") as f:
        print("--- VRG_TEST_EVENT_ACK log ---")
        print(f.read().rstrip())


if __name__ == "__main__":
    main()
