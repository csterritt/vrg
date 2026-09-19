#!/usr/bin/env python3
"""Issue #12 walkthrough: manual vertical scrolling on the real binary.

Builds a fixture directory inside this walkthrough directory containing a
60-line matched file, then drives the built binary on a 100x24 pty —
content height 23 rows below the filename rule — replaying the raw byte
stream through a small screen emulator (the renderer diffs at cell
level, so screen state, not substrings, proves what is displayed):

  scroll — down/up move one rendered row; d/u move
           max(1, floor(23/2)) = 11; pgdn/pgup move 23. Scrolling past
           EOF stops with the last line on the bottom row and emits no
           repaint; up at the top does nothing; q exits 0.
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
CONTENT = ROWS - 1  # filename rule occupies row 0
# The file list holds the escaped raw paths — resolvePath joins vrg's
# cwd with rg's "./a.txt" — padded by one cell; the panel starts after it.
PANEL = len(FIXTURE + "/./a.txt") + 1

# Keys as the terminal delivers them.
DOWN, UP = b"\x1b[B", b"\x1b[A"
PGDN, PGUP = b"\x1b[6~", b"\x1b[5~"


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def make_fixture():
    os.makedirs(FIXTURE, exist_ok=True)
    with open(os.path.join(FIXTURE, "a.txt"), "w") as f:
        f.write("match top line\n")
        for i in range(2, 61):
            f.write("row %02d\n" % i)
    with open(os.path.join(FIXTURE, "b.txt"), "w") as f:
        f.write("match second file\n")


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
    """Replay the byte stream into a ROWSxCOLS character grid.

    Handles CUP/CUx cursor motion, EL/ED erases, and deferred autowrap;
    SGR and mode sequences are ignored. Enough for vrg's frames.
    """
    grid = [[" "] * COLS for _ in range(ROWS)]
    r = c = 0
    wrap = False  # deferred wrap: cursor sits past the last column

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
                for row in grid:
                    row[:] = [" "] * COLS
            elif mode == 0:
                grid[r][c:] = [" "] * (COLS - c)
                for row in grid[r + 1:]:
                    row[:] = [" "] * COLS
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                for row in grid[:r]:
                    row[:] = [" "] * COLS
        elif final == "K":
            mode = arg(0, 0)
            if mode == 0:
                grid[r][c:] = [" "] * (COLS - c)
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
            elif mode == 2:
                grid[r][:] = [" "] * COLS
        elif final == "L":  # insert blank lines
            for _ in range(arg(0, 1)):
                grid.insert(r, [" "] * COLS)
                del grid[ROWS:]
        elif final == "M":  # delete lines
            for _ in range(arg(0, 1)):
                del grid[r]
                grid.append([" "] * COLS)
        elif final == "P":  # delete chars
            n = arg(0, 1)
            del grid[r][c:c + n]
            grid[r].extend([" "] * n)
        elif final == "@":  # insert blank chars
            n = arg(0, 1)
            grid[r][c:c] = [" "] * n
            del grid[r][COLS:]
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
            if w == 2 and c + 1 < COLS:
                grid[r][c + 1] = ""
        c += max(w, 1)
        if c >= COLS:
            c, wrap = COLS - 1, True
        i += size
    return ["".join(row) for row in grid]


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
            os.execvpe(BIN, [BIN, "match", "."], env)
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
        """Send a key and return (screen, bytes written in response)."""
        mark = len(self.out)
        self.send(key)
        screen = self.settle()
        return screen, bytes(self.out[mark:])

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


def panel(screen, row):
    """The file-panel text of a content row: the frame row's list cell
    is len(path)+1 cells wide, so the panel starts at column PANEL."""
    return screen[row][PANEL:].strip() if row < len(screen) else ""


def expect(s, key, label, top_frag, bot_frag):
    """Send key, settle, and assert the panel's first and last content
    rows show top_frag and bot_frag respectively."""
    screen, _ = s.send_settle(key)
    top, bot = panel(screen, 1), panel(screen, ROWS - 1)
    if top_frag not in top:
        s.kill()
        fail("%s: top content row %r does not contain %r" % (label, top, top_frag))
    if bot_frag not in bot:
        s.kill()
        fail("%s: bottom content row %r does not contain %r" % (label, bot, bot_frag))
    print("%-8s: top=%-22r bottom=%r" % (label, top, bot))


def expect_still(s, key, label, screen_before):
    """A clamped scroll emits no repaint and changes no cell."""
    mark = len(s.out)
    s.send(key)
    s.settle()
    if len(s.out) != mark:
        s.kill()
        fail("%s: %r produced a repaint; want a strict no-op" % (label, key))
    top, bot = panel(screen_before, 1), panel(screen_before, ROWS - 1)
    print("%-8s: no repaint; top still %-19r bottom still %r" % (label, top, bot))


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)
    make_fixture()
    s = Session()

    # The load completes and the first visit starts at top-of-file:
    # line 1 heads the panel and line 23 sits on the bottom row.
    s.wait_for(b"top line", "initial frame")
    screen = s.settle()
    if "match top line" not in panel(screen, 1):
        s.kill()
        fail("initial top row %r, want line 1" % panel(screen, 1))
    if "row 23" not in panel(screen, ROWS - 1):
        s.kill()
        fail("initial bottom row %r, want line 23" % panel(screen, ROWS - 1))
    print("%-8s: top=%-22r bottom=%r" % ("init", panel(screen, 1), panel(screen, ROWS - 1)))

    # One rendered row per keypress, there and back.
    expect(s, DOWN, "down", "row 02", "row 24")
    expect(s, UP, "up", "match top line", "row 23")
    expect_still(s, UP, "up@BOF", screen)

    # Half page: max(1, floor(23/2)) = 11 rows.
    expect(s, b"d", "d", "row 12", "row 34")
    expect(s, b"u", "u", "match top line", "row 23")

    # Full page: the content height, 23 rows.
    expect(s, PGDN, "pgdn", "row 24", "row 46")

    # Second page-down clamps at top = 60 - 23 = 37: line 60 on the
    # bottom row, no avoidable blank rows below EOF.
    expect(s, PGDN, "pgdn", "row 38", "row 60")
    eof = emulate(s.out)
    expect_still(s, PGDN, "pgdn@EOF", eof)
    expect_still(s, DOWN, "down@EOF", eof)

    # Page-up returns a full content height.
    expect(s, PGUP, "pgup", "row 15", "row 37")

    s.send(b"q")
    if s.wait_exit("quit") != 0:
        fail("quit: exit=%d, want 0" % s.code)
    print("q       : exit 0")
    print("OK")


if __name__ == "__main__":
    main()
