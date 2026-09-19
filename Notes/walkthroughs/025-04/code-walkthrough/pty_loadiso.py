#!/usr/bin/env python3
"""Issue #25 walkthrough: asynchronous load isolation on a real pty.

One session on a real pty (80x24) over a fixture directory under this
walkthrough:

  fixture/aaa-huge.txt
      line 1 'needle a one' — the match — then millions of pad lines:
      a very large file whose read plus decode/map takes seconds
  fixture/bbb-small.txt
      line 1 'needle b one' — the match — then a handful of pad lines:
      a small file that loads at once
  gate — the VRG_TEST_LOAD_GATE file; while it exists every file-load
      worker holds before the read

The session drives the issue's manual check:

  startup   : gate held — A (aaa-huge) sits on 'Loading…'
  n         : pressed while A's load is held — the panel switches to
              B's placeholder immediately; navigation never waits
  gate gone : B (tiny) decodes first and shows content while A's huge
              load is still in flight
  p         : back to A — the panel shows 'Loading…', never content
              before the load completes
  (A lands) : A's completion arrives while current — content appears
  q         : exit 0
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
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
FIXTURE = os.path.join(HERE, "fixture")
GATE = os.path.join(HERE, "gate")

COLS, ROWS = 80, 24
PAD_LINES = 2_000_000  # ~90 MB of 'pad NNNNNNNN …' lines for aaa-huge


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def write_fixtures():
    os.makedirs(FIXTURE, exist_ok=True)
    with open(os.path.join(FIXTURE, "aaa-huge.txt"), "w") as f:
        f.write("needle a one\n")
        chunk = "".join("pad %08d padding padding padding padding\n" % i
                        for i in range(10000))
        for _ in range(PAD_LINES // 10000):
            f.write(chunk)
    with open(os.path.join(FIXTURE, "bbb-small.txt"), "w") as f:
        f.write("needle b one\n" + "".join("pad %d\n" % i for i in range(3, 12)))


def pump(fd, out, t=0.1):
    r, _, _ = select.select([fd], [], [], t)
    if r:
        try:
            out.extend(os.read(fd, 65536))
        except OSError:
            pass


def cell_width(ch):
    if unicodedata.combining(ch):
        return 0
    return 2 if unicodedata.east_asian_width(ch) in ("W", "F") else 1


def emulate(data, rows, cols):
    """Replay the byte stream into a ROWSxCOLS text grid.

    Handles CUP/CUx cursor motion, EL/ED erases, IL/DL, and deferred
    autowrap — enough for vrg's frames.
    """
    grid = [[" "] * cols for _ in range(rows)]
    r = c = 0
    wrap = False  # deferred wrap: cursor sits past the last column
    top, bot = 0, rows - 1  # DECSTBM scroll region

    def ri():
        """ESC M — reverse index: scroll the region down at its top."""
        nonlocal r
        if r == top:
            grid.insert(top, [" "] * cols)
            del grid[bot + 1:]
        else:
            r = max(0, r - 1)

    def csi(r, c, params, final):
        priv = params.lstrip("?><=!")
        nums = [int(p) for p in priv.split(";") if p.isdigit()]

        def arg(k, dflt):
            v = nums[k] if k < len(nums) else 0
            return v if v else dflt

        if final in "Hf":
            r, c = arg(0, 1) - 1, arg(1, 1) - 1
        elif final == "A":
            r -= arg(0, 1)
        elif final == "B":
            r += arg(0, 1)
        elif final == "C":
            c += arg(0, 1)
        elif final == "D":
            c -= arg(0, 1)
        elif final == "E":
            r, c = r + arg(0, 1), 0
        elif final == "F":
            r, c = r - arg(0, 1), 0
        elif final == "G":
            c = arg(0, 1) - 1
        elif final == "d":
            r = arg(0, 1) - 1
        elif final == "e":
            r += arg(0, 1)
        elif final == "J":
            mode = arg(0, 0)
            if mode in (2, 3):
                for i in range(rows):
                    grid[i][:] = [" "] * cols
            elif mode == 0:
                grid[r][c:] = [" "] * (cols - c)
                for i in range(r + 1, rows):
                    grid[i][:] = [" "] * cols
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                for i in range(r):
                    grid[i][:] = [" "] * cols
        elif final == "K":
            mode = arg(0, 0)
            if mode == 0:
                grid[r][c:] = [" "] * (cols - c)
            elif mode == 1:
                grid[r][:c] = [" "] * (c + 1)
            elif mode == 2:
                grid[r][:] = [" "] * cols
        elif final == "L":  # insert blank lines
            for _ in range(arg(0, 1)):
                grid.insert(r, [" "] * cols)
                del grid[rows:]
        elif final == "M":  # delete lines
            for _ in range(arg(0, 1)):
                del grid[r]
                grid.append([" "] * cols)
        elif final == "P":  # delete chars
            n = arg(0, 0)
            del grid[r][c:c + n]
            grid[r].extend([" "] * n)
        elif final == "@":  # insert blank chars
            n = arg(0, 0)
            grid[r][c:c] = [" "] * n
            del grid[r][cols:]
        elif final == "r":  # DECSTBM: set the scroll region
            nonlocal top, bot
            top, bot = arg(0, 1) - 1, arg(1, rows) - 1
        return max(0, min(r, rows - 1)), max(0, min(c, cols - 1))

    i, n = 0, len(data)
    while i < n:
        b = data[i]
        if b == 0x1b:
            if i + 1 < n and data[i + 1] == ord("["):
                j = i + 2
                while j < n and not (0x40 <= data[j] <= 0x7e):
                    j += 1
                if j >= n:
                    break
                params = data[i + 2:j].decode("ascii", "replace")
                r, c = csi(r, c, params, chr(data[j]))
                wrap = False
                i = j + 1
            elif i + 1 < n and data[i + 1] == ord("]"):
                j = i + 2
                while j < n:
                    if data[j] == 7:
                        break
                    if data[j] == 0x1b and j + 1 < n and data[j + 1] == ord("\\"):
                        break
                    j += 1
                i = j + 1
                if i < n and data[i - 1] == 0x1b:
                    i += 1
            elif i + 1 < n and data[i + 1] in b"()#":
                i += 3  # charset/alignment sequences: ESC X Y
            elif i + 1 < n and data[i + 1] == ord("M"):
                ri()
                i += 2
            else:
                i += 2  # ESC + single byte (modes, DECSC, …)
            continue
        if b == 0x0d:
            c, wrap, i = 0, False, i + 1
            continue
        if b == 0x0a:
            r, wrap, i = min(r + 1, rows - 1), False, i + 1
            continue
        if b == 0x08:
            c, wrap, i = max(c - 1, 0), False, i + 1
            continue
        if b == 0x09:
            c, wrap, i = min((c // 8 + 1) * 8, cols - 1), False, i + 1
            continue
        if b < 0x20 or b == 0x7f:
            i += 1
            continue
        # Decode one UTF-8 rune.
        if b < 0x80:
            ch, size = chr(b), 1
        else:
            size = 2 if b < 0xe0 else 3 if b < 0xf0 else 4
            ch = data[i:i + size].decode("utf-8", "replace")
        w = cell_width(ch)
        if wrap:
            r, c, wrap = min(r + 1, rows - 1), 0, False
        if w == 0:
            # A zero-width combining mark stacks on the previous
            # cell — the terminal's cell accounting, so differential
            # repaints keep landing on the right columns.
            if 0 <= r < rows and 0 < c <= cols:
                grid[r][c - 1] += ch
        else:
            if 0 <= r < rows and 0 <= c < cols:
                grid[r][c] = ch
                if w == 2 and c + 1 < cols:
                    grid[r][c + 1] = ""
            c += w
            if c >= cols:
                c, wrap = cols - 1, True
        i += size
    return ["".join(row) for row in grid]


class Session:
    """vrg running on its own pty; out accumulates the raw byte stream.

    The child runs with cwd=workdir and the fixture path as an
    absolute operand; VRG_TEST_LOAD_GATE points at the gate file, so
    every file-load worker holds while it exists.
    """

    def __init__(self, cols, rows, workdir, operand, pattern):
        self.cols, self.rows = cols, rows
        master, slave = pty.openpty()
        self.master = master
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        env = dict(os.environ)
        env["TERM"] = "xterm"
        env["VRG_TEST_LOAD_GATE"] = GATE
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
            os.chdir(workdir)
            os.execvpe(BIN, [BIN, pattern, operand], env)
            os._exit(127)
        os.close(slave)
        self.pid = pid
        self.out = bytearray()
        self.code = None

    def poll(self):
        if self.code is None:
            done, st = os.waitpid(self.pid, os.WNOHANG)
            if done:
                self.code = os.waitstatus_to_exitcode(st)
        return self.code

    def wait_for(self, needle, label, t=30):
        deadline = time.time() + t
        while time.time() < deadline:
            pump(self.master, self.out)
            if needle in self.out:
                return
            if self.poll() is not None:
                break
        self.kill()
        fail("%s: %r never appeared; output %r" % (label, needle, bytes(self.out)))

    def screen(self):
        return emulate(self.out, self.rows, self.cols)

    def settle(self, quiet=0.4, t=15):
        """Pump until the output is quiet, then return the screen."""
        deadline = time.time() + t
        last = -1
        while time.time() < deadline:
            pump(self.master, self.out, quiet)
            if len(self.out) == last:
                return self.screen()
            last = len(self.out)
        self.kill()
        fail("output never settled")

    def wait_panel(self, label, t=60):
        """Settle until the panel shows content, not 'Loading…'."""
        deadline = time.time() + t
        while time.time() < deadline:
            screen = self.settle(quiet=0.4)
            if "Loading…" not in "".join(screen):
                return screen
        self.kill()
        fail("%s: panel still Loading… after %ds" % (label, t))

    def send(self, data):
        os.write(self.master, data)

    def send_settle(self, data, quiet=0.4):
        """Send a key and settle into the frame it produced."""
        self.send(data)
        return self.settle(quiet)

    def wait_exit(self, label, t=15):
        deadline = time.time() + t
        while time.time() < deadline:
            pump(self.master, self.out)
            if self.poll() is not None:
                while True:
                    r, _, _ = select.select([self.master], [], [], 0.3)
                    if not r:
                        break
                    try:
                        self.out.extend(os.read(self.master, 65536))
                    except OSError:
                        break
                os.close(self.master)
                return self.code
        self.kill()
        fail("%s: vrg did not exit" % label)

    def kill(self):
        try:
            os.kill(self.pid, 9)
            os.waitpid(self.pid, 0)
        except OSError:
            pass


def main():
    write_fixtures()
    open(GATE, "w").close()

    s = Session(COLS, ROWS, HERE, FIXTURE, "needle")
    s.wait_for(b"aaa-huge.txt", "initial frame")
    screen = s.settle()
    if "aaa-huge.txt" not in screen[0]:
        fail("filename rule = %r, want aaa-huge.txt" % screen[0])
    if "Loading…" not in "".join(screen):
        fail("initial frame has no Loading… placeholder: %r" % screen)
    print("%-10s: gate held — A on 'Loading…' — %r"
          % ("startup", screen[1].strip()[:40]))

    # n while A's load is held: the crossing lands at once — the rule
    # names B and its own placeholder shows; navigation never waited
    # on A's worker.
    screen = s.send_settle(b"n", quiet=1.3)
    if "bbb-small.txt" not in screen[0]:
        fail("filename rule after n = %r, want bbb-small.txt" % screen[0])
    if "Loading…" not in "".join(screen):
        fail("B's placeholder missing after n: %r" % screen)
    print("%-10s: navigation live under the held load — now on "
          "bbb-small.txt 'Loading…'" % "n")

    # Release the gate: both workers proceed, but B is tiny — its
    # completion lands and its content shows while A's ~90 MB read
    # plus decode/map is still in flight.
    os.remove(GATE)
    screen = s.wait_panel("B load")
    if "bbb-small.txt" not in screen[0]:
        fail("filename rule after gate release = %r, want bbb-small.txt"
             % screen[0])
    if "needle b one" not in "".join(screen):
        fail("B's content missing: %r" % screen)
    print("%-10s: gate released — B shows content while A still loads — %r"
          % ("release", screen[1].strip()[:40]))

    # p back to A: still in flight, so the panel shows the
    # placeholder — A never shows content before its load completes.
    screen = s.send_settle(b"p", quiet=1.3)
    if "aaa-huge.txt" not in screen[0]:
        fail("filename rule after p = %r, want aaa-huge.txt" % screen[0])
    if "Loading…" not in "".join(screen):
        fail("A showed content before its load completed: %r" % screen)
    print("%-10s: back on A — still 'Loading…', never early content — %r"
          % ("p", screen[1].strip()[:40]))

    # A's completion lands while it is current: the content appears.
    screen = s.wait_panel("A load", t=120)
    if "aaa-huge.txt" not in screen[0]:
        fail("filename rule after A lands = %r, want aaa-huge.txt" % screen[0])
    if "needle a one" not in "".join(screen):
        fail("A's content missing after its load: %r" % screen)
    print("%-10s: A's completion arrived while current — content — %r"
          % ("A lands", screen[1].strip()[:40]))

    s.send(b"q")
    if s.wait_exit("quit") != 0:
        fail("quit: exit=%d, want 0" % s.code)
    print("%-10s: exit 0" % "q")
    print("OK")


if __name__ == "__main__":
    main()
