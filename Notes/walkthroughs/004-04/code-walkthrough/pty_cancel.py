#!/usr/bin/env python3
"""Issue #4 walkthrough: manual cancellation on a real long search.

Spawns the built vrg on a pty running `vrg . /` — a search of the whole
filesystem that does not finish quickly — with the real rg from PATH.
Captures `stty -a` on the slave before launch, cancels mid-search with
q and again with ctrl+c, and verifies each run: exit 130, the display-
restoration sequences, no orphaned rg, `stty -a` identical to the
pre-run state, and a shell on the same pty answering at a usable prompt.
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

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def stty_a(slave):
    """`stty -a` reading the slave's termios — the manual check itself."""
    return subprocess.run(
        ["stty", "-a"], stdin=slave, capture_output=True, text=True, check=True
    ).stdout


def spawn_on_pty(args, slave, env):
    pid = os.fork()
    if pid == 0:
        os.setsid()
        fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
        os.dup2(slave, 0)
        os.dup2(slave, 1)
        os.dup2(slave, 2)
        if slave > 2:
            os.close(slave)
        os.execvpe(args[0], args, env)
        os._exit(127)
    return pid


def pump(fd, out, t=0.1):
    r, _, _ = select.select([fd], [], [], t)
    if r:
        try:
            out.extend(os.read(fd, 65536))
        except OSError:
            pass


def rg_children(pid):
    """Direct children of vrg: the running rg process, if any."""
    r = subprocess.run(["pgrep", "-P", str(pid)], capture_output=True, text=True)
    return [int(p) for p in r.stdout.split()]


def run_cancel(key, label):
    master, slave = pty.openpty()
    fcntl.ioctl(slave, termios.TIOCSWINSZ, struct.pack("HHHH", 24, 80, 0, 0))
    before = stty_a(slave)

    env = dict(os.environ)
    env["TERM"] = "xterm"
    pid = spawn_on_pty([BIN, ".", "/"], slave, env)
    out = bytearray()
    deadline = time.time() + 30

    children = []
    while time.time() < deadline:
        pump(master, out)
        children = children or rg_children(pid)
        if b"Searching" in out and children:
            break
    if b"Searching" not in out:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
        fail("%s: searching screen never appeared; output %r" % (label, bytes(out)))
    if not children:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
        fail("%s: no rg child observed while searching" % label)
    rg_pid = children[0]

    os.write(master, key)
    code = None
    while time.time() < deadline:
        pump(master, out)
        done, st = os.waitpid(pid, os.WNOHANG)
        if done:
            code = os.waitstatus_to_exitcode(st)
            break
    if code is None:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
        fail("%s: vrg did not exit after %r" % (label, key))
    while True:
        r, _, _ = select.select([master], [], [], 0.3)
        if not r:
            break
        try:
            out.extend(os.read(master, 65536))
        except OSError:
            break

    if code != 130:
        fail("%s: exit=%d, want 130" % (label, code))
    for seq in (b"\x1b[?1049l", b"\x1b[?25h"):
        if seq not in out:
            fail("%s: display restoration %r missing" % (label, seq))
    if b"matched lines" in out:
        fail("%s: a further screen rendered after cancellation" % label)

    try:
        os.kill(rg_pid, 0)
        fail("%s: rg pid %d still exists after vrg exited" % (label, rg_pid))
    except ProcessLookupError:
        pass
    except PermissionError:
        fail("%s: rg pid %d still exists (owned by another user)" % (label, rg_pid))
    orphans = [p for p in rg_children(os.getpid()) if p == rg_pid]
    if orphans:
        fail("%s: rg pid %d was reparented, not reaped" % (label, rg_pid))

    after = stty_a(slave)
    if after != before:
        fail("%s: stty -a differs after the run\nbefore: %s\nafter: %s" % (label, before, after))

    shell = spawn_on_pty(["/bin/sh"], slave, env)
    os.write(master, b"echo back-at-prompt\nexit\n")
    deadline = time.time() + 10
    while time.time() < deadline:
        pump(master, out)
        done, _ = os.waitpid(shell, os.WNOHANG)
        if done:
            break
    else:
        os.kill(shell, 9)
        os.waitpid(shell, 0)
        fail("%s: shell on the restored pty did not exit" % label)
    if b"back-at-prompt" not in out:
        fail("%s: shell on the restored pty did not answer: %r" % (label, bytes(out)))

    print("%s : exit=130, rg child gone, alt-screen/cursor restored, stty -a identical, prompt alive" % label)
    os.close(master)
    os.close(slave)


def main():
    run_cancel(b"q", "q     ")
    run_cancel(b"\x03", "ctrl+c")
    print("OK")


if __name__ == "__main__":
    main()
