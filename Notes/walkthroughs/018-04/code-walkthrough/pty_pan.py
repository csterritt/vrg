#!/usr/bin/env python3
"""Issue #18 walkthrough: horizontal panning in run-off-edge mode.

One session on a real pty (100x24) over four fixture files, each a
single-stop match for "needle":

  a.txt — "needle" + 294 x's, then 40 "pad" lines (41 lines, gutter 4)
  b.txt — "needle second" + y's (file-change reset)
  c.txt — "needle ab文" + 200 c's (文 is one two-cell cluster)
  d.txt — "needle" + 293 x's + 文 (a trailing two-cell cluster)

The script drives:

  w       — wrap → run-off-edge (the pan keys' only live mode)
  ,       — at offset 0: a strict no-op (the frame is identical)
  .       — offset 1: the text shifts left by one cell
  >       — offset 11: by ten cells
  ]       — offset 11 + floor(textW / 2): the half-width unit
  w, w    — the offset is retained through the wrap toggle
  n       — file change: the destination starts at offset 0
  > on c  — offset 10 lands inside 文's cluster: its clipped cell
            renders blank, never half a glyph
  >… on d — the offset clamps at the paintable boundary (299): the
            trailing 文 paints both cells; further ., >, ] do nothing
  p, p, p — back to a.txt (offset reset on every crossing)
  ], ↓    — scrolling into the short "pad" lines re-clamps the offset
            leftward to their maximum (2)
  ↑       — the long line returns but the stored offset does not: the
            clamped 2 is what survives
  q       — exit 0
"""
import fcntl
import os
import pty
import select
import struct
import sys
import tempfile
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
FIXTURE = os.path.join(HERE, "fixture")

COLS, ROWS = 100, 24
GW_A = 4  # a.txt has 41 lines: two digit columns + two spaces
GW_1 = 3  # the single-line files: one digit column + two spaces


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def write_fixture():
    os.makedirs(FIXTURE, exist_ok=True)
    a = "needle" + "x" * 294
    b = "needle second " + "y" * 100
    c = "needle ab文" + "c" * 200
    d = "needle" + "x" * 293 + "文"
    with open(os.path.join(FIXTURE, "a.txt"), "w") as f:
        f.write(a + "\n" + "pad\n" * 40)
    for name, line in (("b.txt", b), ("c.txt", c), ("d.txt", d)):
        with open(os.path.join(FIXTURE, name), "w") as f:
            f.write(line + "\n")
    return a, b, c, d


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
                grid[r][:c + 1] = [" "] * (c + 1)
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
        if 0 <= r < rows and 0 <= c < cols:
            grid[r][c] = ch
            if w == 2 and c + 1 < cols:
                grid[r][c + 1] = ""
        c += max(w, 1)
        if c >= cols:
            c, wrap = cols - 1, True
        i += size
    return ["".join(row) for row in grid]


class Session:
    """vrg running on its own pty; out accumulates the raw byte stream.

    The child runs with cwd=workdir and searches operand — the file
    list then displays the resolved path workdir/operand/name, so a
    short tmp cwd keeps the file panel usable at narrow widths.
    """

    def __init__(self, cols, rows, workdir, operand, name):
        self.cols, self.rows = cols, rows
        self.panel = len(workdir + "/" + operand + "/" + name) + 1
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        env = dict(os.environ)
        env["TERM"] = "xterm"
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
            os.execvpe(BIN, [BIN, "needle", operand], env)
            os._exit(127)
        os.close(slave)
        self.pid = pid
        self.master = master
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

    def wait_loaded(self, label, t=30):
        """Settle until the panel shows installed rows, not 'Loading…'.

        A w toggle or file change repaints the placeholder first and
        the real rows once the off-update-path layout command
        completes; loop until the installed repaint lands.
        """
        deadline = time.time() + t
        while time.time() < deadline:
            screen = self.settle(quiet=0.4)
            if "Loading…" not in screen[1][self.panel:]:
                return screen
        self.kill()
        fail("%s: panel still Loading… after %ds" % (label, t))

    def send(self, data):
        os.write(self.master, data)

    def send_settle(self, data, quiet=0.4):
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


def panel(s, screen, r):
    return screen[r][s.panel:]


def main():
    a1, b1, c1, d1 = write_fixture()

    # A short /tmp cwd with a symlink f → fixture keeps the file list
    # narrow: displayed paths are workdir/f/<name>.
    workdir = tempfile.mkdtemp(prefix="vrgpan")
    os.symlink(FIXTURE, os.path.join(workdir, "f"))

    s = Session(COLS, ROWS, workdir, "f", "a.txt")
    # All four names share a length, so one panel width serves all.
    twa = COLS - s.panel - GW_A - 1  # a.txt flat text width (reserved col)
    tw1 = COLS - s.panel - GW_1 - 1  # the one-line files' flat text width
    if twa < 30 or tw1 < 30:
        fail("file list too wide for the demo: panel=%d" % s.panel)
    half = twa // 2

    def want_a(off, gw=GW_A):
        return "%*d  " % (gw - 2, 1) + a1[off:off + twa] + " "

    s.wait_for(b"a.txt", "initial frame")
    screen = s.wait_loaded("initial load")

    # w: run-off-edge. Line 1 is one row clipped at the text width.
    s.send(b"w")
    screen = s.wait_loaded("w off")
    if panel(s, screen, 1) != want_a(0):
        fail("w: row 1 = %r, want %r" % (panel(s, screen, 1), want_a(0)))
    print("%-11s: run-off-edge — %r…" % ("w", panel(s, screen, 1)[:GW_A + 14]))

    # , at offset 0 is a strict no-op — the frame does not change.
    before = screen
    screen = s.send_settle(b",")
    if screen != before:
        fail(",: the frame changed at offset 0")
    print("%-11s: offset 0 — frame unchanged" % ",")

    # . pans one cell: the text window starts at cell 1.
    screen = s.send_settle(b".")
    if panel(s, screen, 1) != want_a(1):
        fail(".: row 1 = %r, want %r" % (panel(s, screen, 1), want_a(1)))
    print("%-11s: shifted one cell — %r…" % (".", panel(s, screen, 1)[:GW_A + 14]))

    # > pans ten cells: offset 11.
    screen = s.send_settle(b">")
    if panel(s, screen, 1) != want_a(11):
        fail(">: row 1 = %r, want %r" % (panel(s, screen, 1), want_a(11)))
    print("%-11s: shifted to offset 11 — %r…" % (">", panel(s, screen, 1)[:GW_A + 14]))

    # ] pans half the text width: offset 11 + floor(twa / 2).
    off = 11 + half
    screen = s.send_settle(b"]")
    if panel(s, screen, 1) != want_a(off):
        fail("]: row 1 = %r, want %r" % (panel(s, screen, 1), want_a(off)))
    print("%-11s: half-width pan to offset %d — %r…"
          % ("]", off, panel(s, screen, 1)[:GW_A + 14]))

    # w, w: wrap on then off — the offset is retained; the same
    # visible set re-admits it on re-entry.
    s.send(b"w")
    s.wait_loaded("w on")
    s.send(b"w")
    screen = s.wait_loaded("w off again")
    if panel(s, screen, 1) != want_a(off):
        fail("w w: row 1 = %r, want the retained offset %d: %r"
             % (panel(s, screen, 1), off, want_a(off)))
    print("%-11s: offset %d retained through the toggle — %r…"
          % ("w, w", off, panel(s, screen, 1)[:GW_A + 14]))

    # n crosses to b.txt: the file change resets the offset — the new
    # panel starts at its left edge.
    s.send(b"n")
    screen = s.wait_loaded("n to b.txt")
    want = "1  " + b1[:tw1] + " "
    if panel(s, screen, 1) != want:
        fail("n: b.txt row 1 = %r, want the left edge %r"
             % (panel(s, screen, 1), want))
    print("%-11s: b.txt starts at offset 0 — %r…"
          % ("n", panel(s, screen, 1)[:GW_1 + 14]))

    # n to c.txt, then > once: offset 10 lands inside 文's cluster
    # (cells 9–10); the clipped cell renders a blank, then the c's —
    # never half the glyph.
    s.send(b"n")
    s.wait_loaded("n to c.txt")
    screen = s.send_settle(b">")
    want = "1  " + " " + "c" * (tw1 - 1) + " "
    if panel(s, screen, 1) != want:
        fail("c.txt @10: row 1 = %r, want a blank then c's: %r"
             % (panel(s, screen, 1), want))
    if "文" in panel(s, screen, 1):
        fail("c.txt @10: row shows a clipped 文 — want a blank cell")
    print("%-11s: offset 10 inside 文 — clipped cell is blank — %r…"
          % ("n, >", panel(s, screen, 1)[:GW_1 + 14]))

    # n to d.txt and pan until the clamp: the trailing 文 sits at
    # cells 299–300, so the maximum offset is 299 — its start — and the
    # cluster paints whole. Further pans move nothing.
    s.send(b"n")
    s.wait_loaded("n to d.txt")
    screen = s.send_settle(b">" * 40)
    want = "1  " + "文" + " " * (tw1 - 1)
    if panel(s, screen, 1) != want:
        fail("d.txt @max: row 1 = %r, want the fully painted 文 then "
             "blanks: %r" % (panel(s, screen, 1), want))
    print("%-11s: offset clamped at 299 — 文 painted whole — %r…"
          % ("n, >…", panel(s, screen, 1)[:GW_1 + 14]))
    before = screen
    for k in (b".", b">", b"]"):
        screen = s.send_settle(k)
        if screen != before:
            fail("pan past the maximum moved the frame")
    print("%-11s: ., >, ] at the maximum — frame unchanged" % ".,>,]")

    # p, p, p back to a.txt — each crossing resets the offset. Then ]
    # pans right by half the text width.
    s.send(b"p")
    s.wait_loaded("p to c.txt")
    s.send(b"p")
    s.wait_loaded("p to b.txt")
    s.send(b"p")
    screen = s.wait_loaded("p to a.txt")
    if panel(s, screen, 1) != want_a(0):
        fail("p p p: a.txt row 1 = %r, want the reset left edge %r"
             % (panel(s, screen, 1), want_a(0)))
    screen = s.send_settle(b"]")
    if panel(s, screen, 1) != want_a(half):
        fail("]: row 1 = %r, want %r" % (panel(s, screen, 1), want_a(half)))
    print("%-11s: back on a.txt — panned to offset %d — %r…"
          % ("ppp, ]", half, panel(s, screen, 1)[:GW_A + 14]))

    # ↓ scrolls the 300-cell line out of the window: the visible
    # set is only "pad" lines, so the stored offset re-clamps to their
    # maximum — cell 2, the 'd' of "pad".
    screen = s.send_settle(b"\x1b[B")
    want = " 2  " + "d" + " " * (twa - 1) + " "
    if panel(s, screen, 1) != want:
        fail("down: row 1 = %r, want the re-clamped pad line %r"
             % (panel(s, screen, 1), want))
    print("%-11s: only pads visible — offset re-clamped to 2 — %r"
          % ("down", panel(s, screen, 1).rstrip()))

    # ↑ brings the long line back, but the clamped offset is not
    # restored: the window starts at cell 2, not the earlier half
    # screen.
    screen = s.send_settle(b"\x1b[A")
    if panel(s, screen, 1) != want_a(2):
        fail("up: row 1 = %r, want the clamped offset 2: %r"
             % (panel(s, screen, 1), want_a(2)))
    print("%-11s: long line back — offset stays 2 — %r…"
          % ("up", panel(s, screen, 1)[:GW_A + 14]))

    s.send(b"q")
    if s.wait_exit("quit") != 0:
        fail("quit: exit=%d, want 0" % s.code)
    print("%-11s: exit 0" % "q")
    print("OK")


if __name__ == "__main__":
    main()
