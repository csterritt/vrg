#!/usr/bin/env python3
"""Issue #3 walkthrough: drive the built vrg binary on a pseudo-terminal.

The VRG_TEST_GATE seam holds index preparation after ripgrep has exited
(VRG_TEST_COLLECT_ACK proves the stream was fully collected), so the
"Searching..." screen is observable mid-flight. Releasing the gate lets
the interim summary render; q then exits with status 0.
"""
import fcntl
import os
import pty
import select
import struct
import sys
import termios
import time

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def spawn_pty(args, env, cwd, rows=24, cols=80):
    """Fork vrg onto a pty whose window size is set before exec."""
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", rows, cols, 0, 0))
    pid = os.fork()
    if pid == 0:
        os.setsid()
        fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
        os.dup2(slave, 0)
        os.dup2(slave, 1)
        os.dup2(slave, 2)
        os.close(master)
        if slave > 2:
            os.close(slave)
        os.chdir(cwd)
        os.execvpe(BIN, [BIN] + args, env)
        os._exit(127)
    os.close(slave)
    return pid, master


def main():
    gate = os.path.join(HERE, "gate-held")
    ack = os.path.join(HERE, "collect-ack")
    for p in (gate, ack):
        try:
            os.remove(p)
        except FileNotFoundError:
            pass
    with open(gate, "w") as f:
        f.write("held")

    env = dict(os.environ)
    env["TERM"] = "xterm"
    env["VRG_TEST_GATE"] = gate
    env["VRG_TEST_COLLECT_ACK"] = ack

    pid, fd = spawn_pty(["needle", "fixtures"], env, HERE)
    out = bytearray()
    deadline = time.time() + 30

    def pump(t=0.1):
        r, _, _ = select.select([fd], [], [], t)
        if r:
            try:
                out.extend(os.read(fd, 65536))
            except OSError:
                pass

    # While the gate holds, rg has already exited (the collect-ack file
    # exists) yet the only possible frame is the searching screen.
    acked = False
    while time.time() < deadline:
        pump()
        acked = acked or os.path.exists(ack)
        if acked and b"Searching" in out:
            break
    if not acked:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
        fail("collect-ack never appeared; output: %r" % bytes(out))
    if b"Searching" not in out:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
        fail("searching screen never rendered; output: %r" % bytes(out))
    if b"matched lines" in out:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
        fail("summary rendered while the gate was held")
    print("gate-held : rg exited and stream collected; screen shows Searching, no summary")

    os.remove(gate)
    while time.time() < deadline and b"matched lines" not in out:
        pump()
    if b"2 files, 3 matched lines" not in out:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
        fail("interim summary missing; output: %r" % bytes(out))
    print("released  : interim summary '2 files, 3 matched lines'")

    os.write(fd, b"q")
    code = None
    while time.time() < deadline:
        pump()
        done, st = os.waitpid(pid, os.WNOHANG)
        if done:
            code = os.waitstatus_to_exitcode(st)
            break
    if code is None:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
        fail("q did not exit vrg")
    print("q         : exit=%d" % code)
    if code != 0:
        fail("exit code %d, want 0" % code)

    for p in (gate, ack):
        try:
            os.remove(p)
        except FileNotFoundError:
            pass
    print("OK")


if __name__ == "__main__":
    main()
