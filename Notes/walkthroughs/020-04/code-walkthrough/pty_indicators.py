#!/usr/bin/env python3
"""Issue #20 walkthrough: hidden-content indicators in run-off-edge mode.

One session on a real pty (100x24) over a single fixture file whose
lines all exceed the flat text width tw (computed from the file-list
width at runtime). Match positions derive from tw so the window edges
land where the scenario needs:

  a.txt — line 1: "needle" + x*(tw+30) + "needle"
                  matches [0,6) and [tw+36,tw+42) — a near visible match
                  and one far enough right to star at offsets 0 and 10
          line 2: x*10 + "needle" + x*(tw-11) + "needle" + x*60
                  matches [10,16) and [tw+5,tw+11) — the far match
                  straddles the right edge at offset 10 (partially
                  visible) and is entirely hidden right at offset 5
          line 3: x*4 + "needle" + x*(tw+50)
                  match [4,10) — entirely hidden left at offset 10
          line 4: x*20 + "needle" + x*(tw+50)
                  match [20,26) — painted at offset 10
          line 5: x*(tw+60) — no match at all

The pattern is the fixed string "needle".

The script drives:

  w       — wrap → run-off-edge: the current line's far match is
            entirely hidden right, so its row ends with the
            reserved-column '*'
  >       — pan to offset 10: every line has text hidden left so
            every gutter shows '_', upgraded to '*' where a match is
            entirely hidden left (lines 1 and 3); the current line
            keeps its right '*' — both stars together — while line
            2's half-visible far match earns none
  n       — the cursor moves to line 2: its far match straddles the
            right edge, so the half-visible match shows NO star; the
            departed line 1's right column goes blank (the star
            belongs to the current matched line alone)
  ,,,,,   — pan left to offset 5: line 2's far match is now entirely
            hidden right and the current line's row shows '*'
  w       — back to wrap mode: no gutter marks, no reserved column —
            the text area claims the last cell
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
GW = 3  # five lines: one digit column + two spaces


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def write_fixture(tw):
    os.makedirs(FIXTURE, exist_ok=True)
    lines = [
        "needle" + "x" * (tw + 30) + "needle",
        "x" * 10 + "needle" + "x" * (tw - 11) + "needle" + "x" * 60,
        "x" * 4 + "needle" + "x" * (tw + 50),
        "x" * 20 + "needle" + "x" * (tw + 50),
        "x" * (tw + 60),
    ]
    with open(os.path.join(FIXTURE, "a.txt"), "w") as f:
        f.write("\n".join(lines) + "\n")
    return lines


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


def cellslice(line, off, tw):
    """The line's display cells [off, off + tw), blank-padded.

    Cells come from display columns, not string indices — a two-cell
    文 consumes two slots of the window.
    """
    cells = []
    for ch in line:
        cells.append(ch)
        cells.extend([""] * (cell_width(ch) - 1))
    win = cells[off:off + tw]
    pad = tw - sum(1 if c == "" else max(cell_width(c), 1) for c in win)
    return "".join(win) + " " * pad


def main():
    # A short /tmp cwd with a symlink f → fixture keeps the file list
    # narrow: displayed paths are workdir/f/<name>.
    workdir = tempfile.mkdtemp(prefix="vrghind")
    os.symlink(FIXTURE, os.path.join(workdir, "f"))

    listw = len(workdir + "/f/a.txt") + 1
    tw = COLS - listw - GW - 1  # flat text width (reserved column out)
    if tw < 30:
        fail("file list too wide for the demo: listw=%d" % listw)
    lines = write_fixture(tw)
    a1, a2, a3, a4, a5 = lines

    s = Session(COLS, ROWS, workdir, "f", "a.txt")

    def want(lineno, line, off, left=" ", right=" "):
        return "%d%s " % (lineno, left) + cellslice(line, off, tw) + right

    def expect(label, lineno, line, off, left, right, row=0):
        got = panel(s, s.screen(), row or lineno)
        w = want(lineno, line, off, left, right)
        if got != w:
            fail("%s: row %d = %r, want %r" % (label, row or lineno, got, w))
        return got

    s.wait_for(b"a.txt", "initial frame")
    s.wait_loaded("initial load")
    print("%-9s: %d-col file list, %d-cell flat text width"
          % ("setup", listw, tw))

    # w: run-off-edge. At offset 0 nothing hides left, but the current
    # line's second "needle" is entirely hidden right — the reserved
    # rightmost column shows '*' on its row alone.
    s.send(b"w")
    s.wait_loaded("w off")
    got = expect("w", 1, a1, 0, " ", "*")
    print("%-9s: run-off-edge — current line's far match hidden right — %r…%r"
          % ("w", got[:GW + 7], got[-3:]))
    for n, line in ((2, a2), (3, a3), (4, a4), (5, a5)):
        expect("w", n, line, 0, " ", " ")

    # > pans to offset 10. Every line now has text hidden left — the
    # gutters all mark: '*' where a match is entirely hidden left
    # (line 1's cell-0 needle and line 3's cells-4..9 needle), '_'
    # elsewhere. The current line's far needle still hides right —
    # both stars together — while line 2's far needle straddles the
    # right edge: partially visible, so no hidden-match indicator for
    # that side.
    screen = s.send_settle(b">")
    checks = [
        (1, a1, "*", "*"),
        (2, a2, "_", " "),
        (3, a3, "*", " "),
        (4, a4, "_", " "),
        (5, a5, "_", " "),
    ]
    for n, line, left, right in checks:
        expect(">", n, line, 10, left, right)
        print("%-9s: row %d gutter %r right %r"
              % (">", n, left, right))
    print("%-9s: both stars on the current line — %r…%r"
          % (">", panel(s, screen, 1)[:GW + 4], panel(s, screen, 1)[-3:]))

    # n to line 2: its first submatch at cell 10 is already painted,
    # so the offset stays 10. Line 2's far match only straddles the
    # right edge — a half-visible match earns NO star — and the
    # departed line 1's right column blanks: the star belongs to the
    # current matched line alone.
    screen = s.send_settle(b"n")
    expect("n", 2, a2, 10, "_", " ")
    expect("n", 1, a1, 10, "*", " ")
    print("%-9s: current line 2 — far match half-visible, no star — %r…%r"
          % ("n", panel(s, screen, 2)[:GW + 4], panel(s, screen, 2)[-3:]))

    # , five times pans left to offset 5: line 2's far match is now
    # entirely beyond the right edge — the star appears.
    screen = s.send_settle(b",,,,,")
    expect(",,,,,", 2, a2, 5, "_", "*")
    print("%-9s: far match now entirely hidden right — %r…%r"
          % (",,,,,", panel(s, screen, 2)[:GW + 4], panel(s, screen, 2)[-3:]))

    # w back to wrap mode: no indicators and no reserved column — the
    # text area claims the cell the flat layout reserves (tw + 1).
    s.send(b"w")
    screen = s.wait_loaded("w on")
    wraprow = panel(s, screen, 1)
    if wraprow != "1  " + cellslice(a1, 0, tw + 1):
        fail("w on: row 1 = %r, want text through the last column"
             % wraprow)
    for r in range(1, ROWS):
        p = panel(s, screen, r)
        if "*" in p or "_" in p:
            fail("w on: wrapped row %d = %r carries an indicator" % (r, p))
    print("%-9s: wrap mode — no marks, no reserved column — %r…%r"
          % ("w", wraprow[:GW + 4], wraprow[-3:]))

    s.send(b"q")
    if s.wait_exit("quit") != 0:
        fail("quit: exit=%d, want 0" % s.code)
    print("%-9s: exit 0" % "q")
    print("OK")


if __name__ == "__main__":
    main()
