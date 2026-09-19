#!/usr/bin/env python3
"""Issue #16 walkthrough: wrap mode and the `w` toggle on a real pty.

Runs the real vrg binary on a pty against `fixture/long.txt`, which
holds a 500-cell line, a tabbed line, and padding lines, then replays
the raw byte stream through a small terminal emulator and asserts on
the screen grid:

  init    — wrap is on by default: line 1 is numbered, the tab line's
            first wrapped row lands "bb" on column 8 (the source-line
            stop, independent of the gutter), and the 500-cell line's
            rows carry blank continuation gutters aligned under the
            first row's text.
  n       — the match near the end of the long line is a hidden
            rendered row; the reveal lands it at floor(23/3) on a
            continuation row behind a blank gutter, underlined as the
            current match.
  w       — run-off-edge mode: every source line is exactly one row —
            sequential numbered gutters, the long line clipped to one
            row — and the rightmost panel cell stays blank (the
            reserved indicator column Issue #20 populates).
  w again — back to wrapped rows with blank continuation gutters.
  q       — exit 0 after leaving the alt screen.
"""
import fcntl
import os
import pty
import select
import struct
import sys
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
FIXTURE = os.path.join(HERE, "fixture")
COLS, ROWS = 100, 24
# The file list holds the escaped raw path — resolvePath joins vrg's
# cwd with rg's "./long.txt" — padded by one cell; the panel starts
# after it.
PANEL = len(FIXTURE + "/./long.txt") + 1
GW = 4          # gutter: two digit columns + two spaces (30 lines)
TEXTW = COLS - PANEL - GW          # wrapped text band
# The deep match's start cell — a multiple of TEXTW so the six-cell
# needle sits whole inside one wrapped row at the row's first column.
NEEDLE_CELL = (484 // TEXTW) * TEXTW


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def make_fixture():
    os.makedirs(FIXTURE, exist_ok=True)
    lines = ["needle first",
             "aa\tbb\tcc",
             "x" * NEEDLE_CELL + "needle" + "x" * (500 - NEEDLE_CELL - 6)]
    lines += ["pad %02d" % i for i in range(4, 31)]
    with open(os.path.join(FIXTURE, "long.txt"), "w") as f:
        f.write("\n".join(lines) + "\n")


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


def emulate(data):
    """Replay the byte stream into (text, underline) ROWSxCOLS grids.

    Handles CUP/CUx cursor motion, EL/ED erases, and deferred autowrap;
    SGR parameters 4/24/0 toggle the tracked underline state. Enough for
    vrg's frames.
    """
    grid = [[" "] * COLS for _ in range(ROWS)]
    ul = [[False] * COLS for _ in range(ROWS)]
    r = c = 0
    under = False
    wrap = False  # deferred wrap: cursor sits past the last column

    def csi(r, c, params, final):
        nonlocal under
        priv = params.lstrip("?><=!")
        nums = [int(p) for p in priv.split(";") if p.isdigit()]

        def arg(k, dflt):
            v = nums[k] if k < len(nums) else 0
            return v if v else dflt

        if final == "m":
            for p in nums or [0]:
                if p == 4:
                    under = True
                elif p in (0, 24):
                    under = False
        elif final in "Hf":
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
                for i in range(ROWS):
                    grid[i][:] = [" "] * COLS
                    ul[i][:] = [False] * COLS
            elif mode == 0:
                grid[r][c:] = [" "] * (COLS - c)
                ul[r][c:] = [False] * (COLS - c)
                for i in range(r + 1, ROWS):
                    grid[i][:] = [" "] * COLS
                    ul[i][:] = [False] * COLS
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                ul[r][:c + 1] = [False] * (c + 1)
                for i in range(r):
                    grid[i][:] = [" "] * COLS
                    ul[i][:] = [False] * COLS
        elif final == "K":
            mode = arg(0, 0)
            if mode == 0:
                grid[r][c:] = [" "] * (COLS - c)
                ul[r][c:] = [False] * (COLS - c)
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                ul[r][:c + 1] = [False] * (c + 1)
            elif mode == 2:
                grid[r][:] = [" "] * COLS
                ul[r][:] = [False] * COLS
        elif final == "L":  # insert blank lines
            for _ in range(arg(0, 1)):
                grid.insert(r, [" "] * COLS)
                ul.insert(r, [False] * COLS)
                del grid[ROWS:]
                del ul[ROWS:]
        elif final == "M":  # delete lines
            for _ in range(arg(0, 1)):
                del grid[r]
                del ul[r]
                grid.append([" "] * COLS)
                ul.append([False] * COLS)
        elif final == "P":  # delete chars
            n = arg(0, 1)
            del grid[r][c:c + n]
            del ul[r][c:c + n]
            grid[r].extend([" "] * n)
            ul[r].extend([False] * n)
        elif final == "@":  # insert blank chars
            n = arg(0, 1)
            grid[r][c:c] = [" "] * n
            ul[r][c:c] = [False] * n
            del grid[r][COLS:]
            del ul[r][COLS:]
        return max(0, min(r, ROWS - 1)), max(0, min(c, COLS - 1))

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
            else:
                i += 2  # ESC + single byte (modes, DECSC, …)
            continue
        if b == 0x0d:
            c, wrap, i = 0, False, i + 1
            continue
        if b == 0x0a:
            r, wrap, i = min(r + 1, ROWS - 1), False, i + 1
            continue
        if b == 0x08:
            c, wrap, i = max(c - 1, 0), False, i + 1
            continue
        if b == 0x09:
            c, wrap, i = min((c // 8 + 1) * 8, COLS - 1), False, i + 1
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
            r, c, wrap = min(r + 1, ROWS - 1), 0, False
        if 0 <= r < ROWS and 0 <= c < COLS:
            grid[r][c] = ch
            ul[r][c] = under
            if w == 2 and c + 1 < COLS:
                grid[r][c + 1] = ""
                ul[r][c + 1] = under
        c += max(w, 1)
        if c >= COLS:
            c, wrap = COLS - 1, True
        i += size
    return ["".join(row) for row in grid], ul


class Session:
    """vrg running on its own pty; out accumulates the raw byte stream."""

    def __init__(self):
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", ROWS, COLS, 0, 0))
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
            os.chdir(FIXTURE)
            os.execvpe(BIN, [BIN, "needle", "."], env)
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

    def settle(self, quiet=0.5, t=15):
        """Pump until the output is quiet, then return the screen."""
        deadline = time.time() + t
        last = -1
        while time.time() < deadline:
            pump(self.master, self.out, quiet)
            if len(self.out) == last:
                return emulate(self.out)
            last = len(self.out)
        self.kill()
        fail("output never settled")

    def send(self, data):
        os.write(self.master, data)

    def send_settle(self, key):
        self.send(key)
        return self.settle()

    def wait_exit(self, label, t=30):
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


def panel(screen, r):
    return screen[r][PANEL:]


def underline_row(ul, lo=1):
    """The content row carrying the underlined current match."""
    for r in range(lo, ROWS):
        if any(ul[r][PANEL:]):
            return r
    return None


def expect_blank_continuation(screen, rows):
    for r in rows:
        seg = panel(screen, r)[:GW]
        if seg != " " * GW:
            fail("row %d gutter %r, want %d blank cells — %r"
                 % (r, seg, GW, screen[r]))


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)
    make_fixture()
    s = Session()

    # Startup: wrap on by default. Line 1 holds the first stop — a
    # visible target, so no scroll. Line 2's tab expansion and line
    # 3's wrapped rows fill the screen.
    s.wait_for(b"./long.txt", "initial frame")
    screen, ul = s.settle()
    p1, p2, p3, p4 = (panel(screen, r) for r in (1, 2, 3, 4))
    if not p1.startswith(" 1  needle"):
        fail("row 1 = %r, want the numbered first line" % p1)
    if underline_row(ul) != 1:
        fail("current-match underline not on row 1")
    # "aa\tbb\tcc" is 18 cells — exactly the text width — and shows both
    # eight-column stops: bb at cell 8, cc at cell 16 — the line's own
    # cell positions, never shifted by the gutter.
    if not p2.startswith(" 2  aa") or p2[GW + 8:GW + 10] != "bb" \
            or p2[GW + 16:GW + 18] != "cc":
        fail("row 2 = %r, want 'aa', 'bb' on stop 8, 'cc' on stop 16" % p2)
    # Line 3's first row is numbered; its continuation rows carry a
    # blank gutter and a full text-width band of x's.
    if not p3.startswith(" 3  " + "x" * TEXTW):
        fail("row 3 = %r, want gutter then %d x's" % (p3, TEXTW))
    if not p4.startswith(" " * GW + "x" * TEXTW):
        fail("row 4 = %r, want a blank continuation gutter then x's" % p4)
    expect_blank_continuation(screen, (4, 5, 6))
    print("%-8s: wrap on — %r / %r / continuation %r" % ("init", p1.rstrip(), p2.rstrip(), p4[:GW + 6].rstrip()))

    # n: the match near the end of the 500-cell line sits on a wrapped
    # row past the screen's bottom — hidden — so the reveal lands it at
    # content row floor(23/3) = 7, screen row 8, behind a blank
    # continuation gutter.
    screen, ul = s.send_settle(b"n")
    match_row = underline_row(ul)
    if match_row != 8:
        fail("n: underlined match on screen row %r, want 8" % match_row)
    seg = panel(screen, match_row)
    if seg[:GW] != " " * GW or not seg[GW:].startswith("needle"):
        fail("n: match row = %r, want blank gutter then the needle" % seg)
    # The row after still continues the same source line.
    expect_blank_continuation(screen, (match_row - 1, match_row + 1))
    print("%-8s: match revealed on continuation row 8 — %r" % ("n", seg[:GW + 12].rstrip()))

    # w: run-off-edge mode — one row per source line. The saved top
    # re-clamps; scroll home to see the first rows.
    s.send(b"w")
    screen, ul = s.send_settle(b"pgup")
    rows = [panel(screen, r)[:GW].strip() for r in range(1, 7)]
    if rows != ["1", "2", "3", "4", "5", "6"]:
        fail("w: gutters %r, want one numbered row per source line" % rows)
    p3 = panel(screen, 3)
    textw = COLS - PANEL - GW - 1  # run-off-edge reserves one cell
    if p3 != " 3  " + "x" * textw + " ":
        fail("w: line 3 = %r, want one clipped row ending in the "
             "reserved blank column" % p3)
    # The tab line shows its expansion on one clipped row: bb on stop 8.
    p2 = panel(screen, 2)
    if p2[GW + 8:GW + 10] != "bb":
        fail("w: tab row = %r, want 'bb' still on stop 8" % p2)
    print("%-8s: one row per line — line 3 clipped %r…%r (last cell blank)" % ("w", p3[:8], p3[-4:]))

    # w again: back to wrapped rows with blank continuation gutters.
    screen, ul = s.send_settle(b"w")
    if not panel(screen, 4).startswith(" " * GW + "x" * TEXTW):
        fail("w: row 4 = %r, want wrapped continuation rows back" % panel(screen, 4))
    print("%-8s: wrapped again — %r" % ("w", panel(screen, 4)[:GW + 8].rstrip()))

    s.send(b"q")
    if s.wait_exit("quit") != 0:
        fail("quit: exit=%d, want 0" % s.code)
    print("q       : exit 0")
    print("OK")


if __name__ == "__main__":
    main()
