#!/usr/bin/env python3
"""Issue #14 walkthrough: vertical destination reveal.

Builds a fixture inside this walkthrough directory — a.txt, a long file
of 250 lines with matches on lines 5 and 200 only, and b.txt, a short
file with matches on lines 3 and 8 — then drives the built binary on a
100x24 pty (content height 23, so the one-third placement is content
row 7). The renderer diffs at cell level, so the raw byte stream is
replayed through a small screen emulator that also tracks the SGR
underline attribute: the current matched line's match is
inverse+underline and the current file-list entry is base+underline, so
screen state proves where the cursor sits and which row the panel
starts on:

  init    — startup reveal: a.txt line 5 is visible from the
            first-visit top, so the viewport does not scroll.
  n       — line 200 is hidden: the reveal puts its row at
            floor(23/3) = 7, i.e. top = 199 - 7 = 192 and the underline
            lands about a third down the panel.
  p       — back to line 5: row 4 - 7 < 0, so the BOF clamp wins over
            one-third placement and the match lands near the top.
  n->b    — first visit to b.txt starts at its top; the line-3 target
            is visible, so the reveal does not scroll.
  n       — moving between b.txt's two on-screen matches leaves the
            viewport exactly where it was.
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
# The file list holds the escaped raw paths — resolvePath joins vrg's
# cwd with rg's "./a.txt" — padded by one cell; the panel starts after it.
PANEL = len(FIXTURE + "/./a.txt") + 1


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def make_fixture():
    os.makedirs(FIXTURE, exist_ok=True)

    def numbered(name, n, stops):
        out = []
        for i in range(1, n + 1):
            out.append(stops.get(i, "%s row %02d" % (name, i)))
        return "\n".join(out) + "\n"

    with open(os.path.join(FIXTURE, "a.txt"), "w") as f:
        f.write(numbered("a", 250, {5: "match a-five", 200: "match a-twohundred"}))
    with open(os.path.join(FIXTURE, "b.txt"), "w") as f:
        f.write(numbered("b", 30, {3: "match b-three", 8: "match b-eight"}))


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


def describe(screen, ul):
    """Summarize the cursor's on-screen position: the filename rule's
    file, the underlined list entry, the underlined panel row's screen
    row and text, and the panel's first content row."""
    rule = screen[0]
    cur_file = ""
    for name in ("a.txt", "b.txt"):
        if "/./" + name in rule:
            cur_file = name
    list_ul = ""
    match_row = None
    for r in range(1, ROWS):
        if any(ul[r][:PANEL]):
            list_ul = screen[r][:PANEL].strip().rsplit("/", 1)[-1]
        if any(ul[r][PANEL:]):
            match_row = r
    match_desc = "off-screen"
    if match_row is not None:
        match_desc = "row %d: %r" % (match_row, screen[match_row][PANEL:].strip())
    return cur_file, list_ul, match_row, match_desc, screen[1][PANEL:].strip()


def expect(s, key, label, want_file, want_list, want_match,
           want_row=None, want_top=None):
    """Send key, settle, and assert the cursor's visible position."""
    screen, ul = s.send_settle(key)
    cur_file, list_ul, match_row, match_desc, top = describe(screen, ul)
    if cur_file != want_file:
        s.kill()
        fail("%s: filename rule names %r, want %r" % (label, cur_file, want_file))
    if list_ul != want_list:
        s.kill()
        fail("%s: underlined list entry %r, want %r" % (label, list_ul, want_list))
    if want_match not in match_desc:
        s.kill()
        fail("%s: current-line underline at %s, want it on %r"
             % (label, match_desc, want_match))
    if want_row is not None and match_row != want_row:
        s.kill()
        fail("%s: underlined match at screen row %r, want %r"
             % (label, match_row, want_row))
    if want_top is not None and want_top not in top:
        s.kill()
        fail("%s: first content row %r, want %r" % (label, top, want_top))
    print("%-10s: file=%s list=%s underline=%s top=%r"
          % (label, cur_file, list_ul, match_desc, top))
    return screen, ul


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)
    make_fixture()
    s = Session()

    # Startup: a.txt's line-5 stop is the cursor's first position. The
    # first-visit reveal starts at the top of the file, where row 4 is
    # already visible — a no-scroll reveal, so the top stays row 1.
    # (The match text is split by styling escapes, so the wait needle is
    # the last file-list entry instead.)
    s.wait_for(b"./b.txt", "initial frame")
    screen, ul = s.settle()
    cur_file, list_ul, match_row, match_desc, top = describe(screen, ul)
    if (cur_file != "a.txt" or list_ul != "a.txt"
            or "match a-five" not in match_desc or match_row != 5
            or "a row 01" not in top):
        s.kill()
        fail("init: %r" % ((cur_file, list_ul, match_row, match_desc, top),))
    print("%-10s: file=%s list=%s underline=%s top=%r"
          % ("init", cur_file, list_ul, match_desc, top))

    # n to the hidden line-200 stop: the reveal lands the target row at
    # floor(23/3) = content row 7 — screen row 8 — with top = 199-7 =
    # 192, i.e. line 193 first.
    expect(s, b"n", "n", "a.txt", "a.txt", "match a-twohundred",
           want_row=8, want_top="a row 193")

    # p back to line 5: row 4 - 7 clamps to top 0 — BOF content beats
    # one-third placement — and the match lands near the top.
    expect(s, b"p", "p", "a.txt", "a.txt", "match a-five",
           want_row=5, want_top="a row 01")

    # n again reveals line 200 a third down once more, then n crosses
    # into b.txt: the first visit starts at the file's top and its
    # line-3 target is already visible — a no-scroll reveal.
    expect(s, b"n", "n", "a.txt", "a.txt", "match a-twohundred",
           want_row=8, want_top="a row 193")
    expect(s, b"n", "n->b", "b.txt", "b.txt", "match b-three",
           want_row=3, want_top="b row 01")

    # n between b.txt's two on-screen matches moves only the underline;
    # the viewport does not scroll.
    expect(s, b"n", "n", "b.txt", "b.txt", "match b-eight",
           want_row=8, want_top="b row 01")

    s.send(b"q")
    if s.wait_exit("quit") != 0:
        fail("quit: exit=%d, want 0" % s.code)
    print("q         : exit 0")
    print("OK")


if __name__ == "__main__":
    main()
