#!/usr/bin/env python3
"""Issue #9 walkthrough: fatal/warning outcomes and the modal error
overlay on a real pty, driven against scripted fake rg binaries.

Runs the real vrg binary on a pty in four sessions and asserts on the
RAW byte stream a terminal would execute:

  fatal-results — fake rg emits two valid matches, writes "boom" to
                  stderr, and exits 3: the error overlay opens over the
                  browse frame with "boom" inside; Esc dismisses to the
                  repainted browse content; q exits 2.
  fatal-no-out  — a bare "exit 2" with no output: the generated
                  diagnostic names the code; q exits 2 — and, in a
                  second session, so does Esc (the only state where Esc
                  terminates: no underlying screen exists).
  signal        — the fake rg emits an unclosed stream, records its pid,
                  and blocks; the driver SIGKILLs it mid-stream: the
                  overlay names the signal; Esc reveals the retained
                  match's browse frame; q exits 2.
  warn-summary  — "warn" on stderr beside a complete summary-only rg-1
                  stream: the warning overlay opens; Esc reveals
                  "No results found"; q exits 1.
"""
import fcntl
import os
import pty
import select
import signal
import struct
import sys
import tempfile
import termios
import time

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")

FATAL_RESULTS_RG = """\
cat <<'EOF'
{"type":"begin","data":{"path":{"text":"./f"}}}
{"type":"match","data":{"path":{"text":"./f"},"lines":{"text":"alpha line 2\\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"match","data":{"path":{"text":"./f"},"lines":{"text":"alpha line 5\\n"},"line_number":5,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./f"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"stats":{}}}
EOF
printf 'boom\\n' >&2
exit 3
"""

KILLED_RG = """\
printf '%s\\n' '{"type":"begin","data":{"path":{"text":"./f"}}}'
printf '%s\\n' '{"type":"match","data":{"path":{"text":"./f"},"lines":{"text":"alpha line 2\\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}'
echo $$ > "$RG_PID_FILE"
: > "$RG_READY"
exec sleep 600
"""

WARN_SUMMARY_RG = """\
printf 'warn\\n' >&2
printf '%s\\n' '{"type":"summary","data":{"stats":{}}}'
exit 1
"""


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def pump(fd, out, t=0.1):
    r, _, _ = select.select([fd], [], [], t)
    if r:
        try:
            out.extend(os.read(fd, 65536))
        except OSError:
            pass


class Session:
    """vrg running on its own pty; out accumulates the raw byte stream."""

    def __init__(self, cwd, args, rg_script=None, env_extra=None):
        if rg_script is not None:
            fakebin = tempfile.mkdtemp(prefix="fakebin-")
            with open(os.path.join(fakebin, "rg"), "w") as fh:
                fh.write("#!/bin/sh\n" + rg_script)
            os.chmod(os.path.join(fakebin, "rg"), 0o755)
        else:
            fakebin = None
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", 24, 80, 0, 0))
        env = dict(os.environ)
        env["TERM"] = "xterm"
        if fakebin:
            env["PATH"] = fakebin + ":" + env["PATH"]
        if env_extra:
            env.update(env_extra)
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
        self.pid = pid
        self.master = master
        self.out = bytearray()

    def poll(self):
        done, st = os.waitpid(self.pid, os.WNOHANG)
        return os.waitstatus_to_exitcode(st) if done else None

    def wait_for(self, needle, label, off=0, t=30):
        deadline = time.time() + t
        while time.time() < deadline:
            pump(self.master, self.out)
            if needle in bytes(self.out[off:]):
                return
            if self.poll() is not None:
                break
        self.kill()
        fail("%s: %r never appeared; output %r" % (label, needle, bytes(self.out)))

    def send(self, data):
        os.write(self.master, data)

    def wait_exit(self, label, t=30):
        deadline = time.time() + t
        while time.time() < deadline:
            pump(self.master, self.out)
            code = self.poll()
            if code is not None:
                while True:
                    r, _, _ = select.select([self.master], [], [], 0.3)
                    if not r:
                        break
                    try:
                        self.out.extend(os.read(self.master, 65536))
                    except OSError:
                        break
                os.close(self.master)
                return code
        self.kill()
        fail("%s: vrg did not exit" % label)

    def kill(self):
        try:
            os.kill(self.pid, 9)
            os.waitpid(self.pid, 0)
        except OSError:
            pass


def wait_file(path, label, t=30):
    deadline = time.time() + t
    while time.time() < deadline:
        if os.path.exists(path):
            return
        time.sleep(0.05)
    fail("%s: %s never appeared" % (label, path))


def write_fixture(dirpath):
    os.makedirs(dirpath, exist_ok=True)
    with open(os.path.join(dirpath, "f"), "w") as fh:
        for i in range(1, 21):
            fh.write("alpha line %d\n" % i)


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)

    # 1. fatal exit code with usable results: browse under the error
    #    overlay; Esc dismisses; q exits 2.
    dir1 = tempfile.mkdtemp(prefix="vrg-fatal-results-")
    write_fixture(dir1)
    s = Session(dir1, ["foo"], rg_script=FATAL_RESULTS_RG)
    s.wait_for(b"boom", "fatal-results overlay")
    off = len(s.out)
    s.send(b"\x1b")
    # The covered content row repaints only after the overlay closes.
    s.wait_for(b"alpha line 11", "fatal-results dismissal", off=off)
    s.send(b"q")
    code = s.wait_exit("fatal-results quit")
    if code != 2:
        fail("fatal-results: exit=%d, want 2" % code)
    if b"\x1b[?1049l" not in s.out:
        fail("fatal-results: alt-screen restoration missing")
    print("fatal-results: exit 3 + stderr 'boom' + 2 matches -> overlay "
          "over browse; Esc reveals the content; q exits 2")

    # 2a. exit 2 with no output: generated diagnostic names the code;
    #     q on the overlay-only presentation exits 2.
    dir2 = tempfile.mkdtemp(prefix="vrg-exit2-")
    s = Session(dir2, ["foo"], rg_script="exit 2\n")
    s.wait_for(b"code 2", "exit2 generated diagnostic")
    s.send(b"q")
    code = s.wait_exit("exit2 q")
    if code != 2:
        fail("exit2 q: exit=%d, want 2" % code)
    print("exit2        : bare 'exit 2' -> generated 'code 2' diagnostic; "
          "q exits 2")

    # 2b. the same overlay-only outcome exits on Esc too.
    dir2b = tempfile.mkdtemp(prefix="vrg-exit2esc-")
    s = Session(dir2b, ["foo"], rg_script="exit 2\n")
    s.wait_for(b"code 2", "exit2esc generated diagnostic")
    s.send(b"\x1b")
    code = s.wait_exit("exit2 Esc")
    if code != 2:
        fail("exit2 Esc: exit=%d, want 2" % code)
    print("exit2-esc    : the overlay-only fatal outcome exits 2 on Esc too")

    # 3. SIGKILL mid-stream: the overlay names the signal; Esc reveals
    #    the retained match's browse frame; q exits 2.
    dir3 = tempfile.mkdtemp(prefix="vrg-killed-")
    write_fixture(dir3)
    pidfile = os.path.join(dir3, "rg.pid")
    ready = os.path.join(dir3, "rg.ready")
    s = Session(dir3, ["foo"], rg_script=KILLED_RG,
                env_extra={"RG_PID_FILE": pidfile, "RG_READY": ready})
    wait_file(ready, "killed ready file")
    with open(pidfile) as fh:
        rg_pid = int(fh.read().strip())
    os.kill(rg_pid, signal.SIGKILL)
    s.wait_for(b"killed", "signal diagnostic")
    off = len(s.out)
    s.send(b"\x1b")
    s.wait_for(b"alpha line 11", "signal dismissal", off=off)
    s.send(b"q")
    code = s.wait_exit("signal quit")
    if code != 2:
        fail("signal: exit=%d, want 2" % code)
    print("signal       : SIGKILL mid-stream -> overlay names the signal; "
          "Esc reveals browse; q exits 2")

    # 4. "warn" stderr with a complete summary-only stream: the warning
    #    overlay dismisses to no-results; q exits 1.
    dir4 = tempfile.mkdtemp(prefix="vrg-warn-")
    s = Session(dir4, ["foo"], rg_script=WARN_SUMMARY_RG)
    s.wait_for(b"warn", "warning overlay")
    off = len(s.out)
    s.send(b"\x1b")
    s.wait_for(b"No results found", "warning dismissal", off=off)
    s.send(b"q")
    code = s.wait_exit("warning quit")
    if code != 1:
        fail("warning: exit=%d, want 1" % code)
    print("warn-summary : 'warn' + summary-only rg-1 -> warning overlay; "
          "Esc shows 'No results found'; q exits 1")
    print("OK")


if __name__ == "__main__":
    main()
