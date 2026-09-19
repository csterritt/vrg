#!/usr/bin/env python3
"""Issue #28 walkthrough: two-stage load-completion reveal on a real
pty.

Five sessions on real ptys against the disposable fixture tree
manual_route.sh prepared (argv[1]):

  <tmp>/c1/deep.txt    — 'needle deep 500' line 500, 'needle deep 560'
                         line 560 of 600
  <tmp>/c2/shallow.txt — 'needle shallow 03' line 3, 'needle shallow
                         30' line 30 of 40
  <tmp>/c3/two.txt     — 'needle two 10' and 'needle two 50' of 60
  <tmp>/c4/away.txt    — same two-stop shape
  <tmp>/c5/resize.txt  — same deep shape as c1
  <tmp>/gate.hold      — the VRG_TEST_LOAD_GATE file: while it exists
                         every file load is held
  <tmp>/vrg-stderr.log — each child's stderr

The issue's manual checks, one session each:

  c1  first file held by the gate: 'Loading…'; release → the line-500
      match lands a third down; n → the second stop
  c2  first match on line 3: the file opens from the top, no scroll
  c3  r with the gate held, then n: on completion the new stop is
      revealed — not the pre-reload position
  c4  match scrolled off-screen, r held, then n p away-and-back: on
      completion the match is revealed — navigation intent, not the
      equal cursor, decides
  c5  resize while the first file is held: the reveal commits
      against the new size
"""
import fcntl
import os
import pty
import re
import select
import struct
import sys
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
TMP = sys.argv[1]
GATE = os.path.join(TMP, "gate.hold")
ERRLOG = os.path.join(TMP, "vrg-stderr.log")


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
                grid[r][c:] = [" "] * cols
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
            n = arg(0, 1)
            del grid[r][c:c + n]
            grid[r].extend([" "] * n)
        elif final == "@":  # insert blank chars
            n = arg(0, 1)
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
            c, wrap, i = max(0, c - 1), False, i + 1
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

    The child runs with cwd and the fixture path as an absolute
    operand; stdout stays on the pty while stderr is appended to
    ERRLOG. VRG_TEST_LOAD_GATE=GATE is always in the environment —
    while GATE exists every file load holds.
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
        errfd = os.open(ERRLOG, os.O_CREAT | os.O_WRONLY | os.O_APPEND, 0o644)
        pid = os.fork()
        if pid == 0:
            os.setsid()
            fcntl.ioctl(slave, termios.TIOCSCTTY, 0)
            os.dup2(slave, 0)
            os.dup2(slave, 1)
            os.dup2(errfd, 2)
            os.close(master)
            if slave > 2:
                os.close(slave)
            if errfd > 2:
                os.close(errfd)
            os.chdir(workdir)
            os.execvpe(BIN, [BIN, pattern, operand], env)
            os._exit(127)
        os.close(slave)
        os.close(errfd)
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

    def send(self, data):
        os.write(self.master, data)

    def send_settle(self, data, quiet=0.4):
        """Send a key and settle into the frame it produced."""
        self.send(data)
        return self.settle(quiet)

    def resize(self, cols, rows):
        """Resize the pty — the child gets SIGWINCH and repaints."""
        self.cols, self.rows = cols, rows
        fcntl.ioctl(self.master, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))

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


def text(screen):
    return "\n".join(screen)


def disp(s):
    """Normalize the disposable directory so the transcript is stable —
    including the left-truncated path forms the file list shows, where
    the random mktemp suffix survives the truncation."""
    s = s.replace(TMP, "<tmp>")
    return re.sub(r"028-fixture\.[A-Za-z0-9]+", "028-fixture.XXXXXX", s)


def hold():
    """Create the gate file — every load holds until release()."""
    with open(GATE, "w"):
        pass


def release():
    os.remove(GATE)


def quit_ok(s, label):
    s.send(b"q")
    if s.wait_exit(label) != 0:
        fail("%s: exit=%d, want 0" % (label, s.code))


def case1():
    """The slow first file: held load, line-500 match lands a third
    down, then n to the second stop."""
    hold()
    s = Session(80, 24, os.path.join(TMP, "c1"),
                os.path.join(TMP, "c1", "deep.txt"), "needle")
    try:
        s.wait_for("Loading…".encode(), "c1 startup")
        screen = s.settle()
        if "deep.txt" not in screen[0] or "Loading…" not in text(screen):
            fail("c1 startup frame = %r, want the rule plus Loading…"
                 % screen)
        print("%-12s: 'Loading…' while the gate holds the load — %r"
              % ("c1 held", disp(screen[0].strip()[:56])))

        release()
        screen = s.settle(quiet=1.0)
        # top = row 499 − floor(23/3) = 492 → line 493 leads; the
        # match lands on content row 7.
        if "pad deep 493" not in screen[1]:
            fail("c1 top = %r, want 'pad deep 493' — one-third "
                 "placement" % screen[1])
        if "needle deep 500" not in screen[8]:
            fail("c1 match row = %r, want 'needle deep 500' a third "
                 "down" % screen[8])
        print("%-12s: reveal committed — top %r, match a third down"
              % ("c1 released", disp(screen[1].strip()[:56])))

        screen = s.send_settle(b"n", quiet=0.8)
        if "needle deep 560" not in screen[8]:
            fail("c1 second stop = %r, want 'needle deep 560' a third "
                 "down" % screen[8])
        print("%-12s: first n — second stop 'needle deep 560' revealed"
              % "c1 n")
        quit_ok(s, "c1 quit")
        print("%-12s: exit 0" % "c1 q")
    finally:
        s.kill()


def case2():
    """The visible startup target: line-3 match keeps the file at the
    top — no scroll."""
    hold()
    s = Session(80, 24, os.path.join(TMP, "c2"),
                os.path.join(TMP, "c2", "shallow.txt"), "needle")
    try:
        s.wait_for("Loading…".encode(), "c2 startup")
        s.settle()
        release()
        screen = s.settle(quiet=1.0)
        if "pad shallow 01" not in screen[1]:
            fail("c2 top = %r, want 'pad shallow 01' — visible target "
                 "kept top 0" % screen[1])
        if "needle shallow 03" not in screen[3]:
            fail("c2 match row = %r, want 'needle shallow 03' in place"
                 % screen[3])
        print("%-12s: first match on line 3 — file opens from the top, "
              "no scroll" % "c2 released")
        quit_ok(s, "c2 quit")
        print("%-12s: exit 0" % "c2 q")
    finally:
        s.kill()


def case3():
    """Reload then immediate n: the completion reveals the new stop,
    not the pre-reload position."""
    s = Session(80, 24, os.path.join(TMP, "c3"),
                os.path.join(TMP, "c3", "two.txt"), "needle")
    try:
        s.wait_for(b"two.txt", "c3 startup")
        screen = s.settle()
        if "pad two 01" not in screen[1]:
            fail("c3 startup top = %r, want 'pad two 01' — the line-10 "
                 "match was visible" % screen[1])

        hold()
        screen = s.send_settle(b"r", quiet=1.0)
        if "Loading…" not in text(screen):
            fail("c3 r frame = %r, want Loading… — the reload is held"
                 % screen)
        screen = s.send_settle(b"n", quiet=0.6)
        if "Loading…" not in text(screen):
            fail("c3 n frame = %r, want Loading… — the intent pends"
                 % screen)
        print("%-12s: r held, n during the load — placeholder stays"
              % "c3 r n")

        release()
        screen = s.settle(quiet=1.0)
        # The newest selection (line 50, row 49) reveals at
        # 49 − 7 = 42, clamped to MaxTop 37 — EOF content beats the
        # third — and not the pre-reload top 0.
        if "pad two 38" not in screen[1]:
            fail("c3 top = %r, want 'pad two 38' — the new stop's "
                 "EOF-clamped reveal" % screen[1])
        if "needle two 50" not in screen[13]:
            fail("c3 match row = %r, want 'needle two 50' revealed"
                 % screen[13])
        print("%-12s: completion — the new stop 'needle two 50' "
              "revealed, not the old position" % "c3 released")
        quit_ok(s, "c3 quit")
        print("%-12s: exit 0" % "c3 q")
    finally:
        s.kill()


def case4():
    """Scrolled-off match + r + n p away-and-back: the entry reveal
    commits — navigation intent, never cursor equality."""
    s = Session(80, 24, os.path.join(TMP, "c4"),
                os.path.join(TMP, "c4", "away.txt"), "needle")
    try:
        s.wait_for(b"away.txt", "c4 startup")
        screen = s.settle()
        # Scroll the line-10 match off-screen: three half pages.
        for _ in range(3):
            screen = s.send_settle(b"d", quiet=0.6)
        if "pad two 34" not in screen[1]:
            fail("c4 scrolled top = %r, want 'pad two 34' — the match "
                 "is off-screen" % screen[1])
        if "needle two 10" in text(screen):
            fail("c4 current match still visible after scrolling: %r"
                 % screen)
        print("%-12s: match scrolled off — top %r"
              % ("c4 d d d", disp(screen[1].strip()[:56])))

        hold()
        screen = s.send_settle(b"r", quiet=1.0)
        if "Loading…" not in text(screen):
            fail("c4 r frame = %r, want Loading… — the reload is held"
                 % screen)
        s.send(b"n")
        screen = s.settle(quiet=0.6)
        s.send(b"p")
        screen = s.settle(quiet=0.6)
        if "Loading…" not in text(screen):
            fail("c4 n p frame = %r, want Loading… — the intent pends"
                 % screen)
        print("%-12s: r held, n p away-and-back — placeholder stays"
              % "c4 r n p")

        release()
        screen = s.settle(quiet=1.0)
        # The cursor is back on the line-10 stop — row 9 — but the
        # entry reveal lands it at 9 − 7 = 2 rather than keeping the
        # scrolled top 33: navigation intent, not cursor equality.
        if "pad two 03" not in screen[1]:
            fail("c4 top = %r, want 'pad two 03' — the entry reveal "
                 "at one third" % screen[1])
        if "needle two 10" not in screen[8]:
            fail("c4 match row = %r, want 'needle two 10' a third down"
                 % screen[8])
        print("%-12s: completion — match revealed at the third, the "
              "scrolled position not preserved" % "c4 released")
        quit_ok(s, "c4 quit")
        print("%-12s: exit 0" % "c4 q")
    finally:
        s.kill()


def case5():
    """Resize while the first file is held: the reveal commits
    against the new size."""
    hold()
    s = Session(80, 24, os.path.join(TMP, "c5"),
                os.path.join(TMP, "c5", "resize.txt"), "needle")
    try:
        s.wait_for("Loading…".encode(), "c5 startup")
        s.settle()
        # 80x24 → 100x30 while the load is still held.
        s.resize(100, 30)
        screen = s.settle(quiet=0.8)
        if "Loading…" not in text(screen):
            fail("c5 resized frame = %r, want Loading… — the load is "
                 "still held" % screen)
        print("%-12s: resized 80x24 → 100x30 mid-load — placeholder "
              "stays" % "c5 resize")

        release()
        screen = s.settle(quiet=1.0)
        # New content height 29 → floor(29/3) = 9 → top = 499 − 9 =
        # 490: line 491 leads, the match lands on content row 9.
        if "pad deep 491" not in screen[1]:
            fail("c5 top = %r, want 'pad deep 491' — one-third "
                 "placement at the new size" % screen[1])
        if "needle deep 500" not in screen[10]:
            fail("c5 match row = %r, want 'needle deep 500' a third "
                 "down at the new size" % screen[10])
        if len(screen) != 30 or len(screen[0]) != 100:
            fail("c5 frame geometry = %dx%d, want 100x30"
                 % (len(screen[0]), len(screen)))
        print("%-12s: reveal committed at 100x30 — top %r"
              % ("c5 released", disp(screen[1].strip()[:56])))
        quit_ok(s, "c5 quit")
        print("%-12s: exit 0" % "c5 q")
    finally:
        s.kill()


def main():
    case1()
    case2()
    case3()
    case4()
    case5()
    print("OK")


if __name__ == "__main__":
    main()
