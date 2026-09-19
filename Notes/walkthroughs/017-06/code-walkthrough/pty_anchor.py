#!/usr/bin/env python3
"""Issue #17 walkthrough: the logical anchor through rewrap and resize.

Session A — fixture/long.txt on a real pty (starts at 100x24):

  d       — one half-page scroll lands the top inside the wrapped
            2000-cell line: the anchor is {line 2, cell C}.
  60x24   — narrowing rewraps; the same text location stays at the
            top — the row containing cell C, not the former ordinal.
  140x24  — widening likewise.
  100x24  — the round trip restores the original top row exactly.
  w       — run-off-edge: the anchor's line shows as one numbered row
            and the logical column is retained.
  w       — wrapped again: the original top row returns.
  d...    — scroll to EOF at 60 columns: the tail line's end sits on
            the bottom row and the top is a mid-tail row (TAIL).
  140x24  — the EOF clamp pulls the top up to an earlier location
            (CLMP) and rewrites the anchor.
  60x24   — the clamped position stays (WIDE): the pre-clamp top is
            gone — the documented lossy case.
  q       — exit 0.

Session B — fixture-huge/huge.txt (~50 MB): a burst of resizes queues
layout preparations that run off the update path; ctrl+c mid-rewrap
exits promptly with status 130.
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
HUGE = os.path.join(HERE, "fixture-huge")
GW = 4  # gutter: two digit columns + two spaces (43 lines)

LONG_CELLS = 2000   # line 2: the wrapped line the anchor lands inside
PADS = 40           # lines 3..42: single-row padding
TAIL_CELLS = 3000   # line 43: a second long line straddling EOF

HUGE_LINES = 410_000  # ~128 bytes per line ~= 50 MB


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def make_dirs():
    os.makedirs(FIXTURE, exist_ok=True)
    os.makedirs(HUGE, exist_ok=True)


def write_huge():
    path = os.path.join(HUGE, "huge.txt")
    if os.path.exists(path) and os.path.getsize(path) > 50 << 20:
        return
    with open(path, "w") as f:
        for i in range(HUGE_LINES):
            pad = "p" * (111 if i % 1000 == 0 else 118)
            mark = " needle" if i % 1000 == 0 else ""
            f.write("row %07d%s %s\n" % (i, mark, pad))


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

        A resize or w repaints the placeholder first and the real rows
        once the off-update-path layout command completes; loop until
        the installed repaint lands.
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

    def resize_raw(self, cols, rows):
        """Resize the pty — the SIGWINCH path a real resize takes —
        without waiting for any repaint."""
        fcntl.ioctl(self.master, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        self.cols, self.rows = cols, rows

    def resize(self, cols, rows, label):
        """Resize, then emulate only the post-resize repaints until the
        fresh layout installs."""
        self.resize_raw(cols, rows)
        self.out = bytearray()
        return self.wait_loaded(label)

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


def splice(line, at, text):
    """Return line (a list of chars) with text written at cell at."""
    for i, ch in enumerate(text):
        line[at + i] = ch
    return line


def session_a(workdir):
    """The anchor round trip, the w round trip, and the lossy EOF
    clamp — all on the same file."""
    s = Session(100, 24, workdir, "f", "long.txt")
    tw = lambda cols: cols - s.panel - GW  # text width, wrap on
    tw0, tw1, tw2 = tw(100), tw(60), tw(140)
    if tw0 < 20 or tw1 < 20:
        fail("panel too wide for the demo widths: panel=%d" % s.panel)

    # Row geometry per width. line 1 is one row; line 2 wraps into
    # ceil(2000/tw) rows; the 40 pads are one row each; the tail wraps
    # into ceil(3000/tw) rows and straddles EOF at both widths.
    rows_before_tail = [1 + -(-LONG_CELLS // w) + PADS for w in (tw0, tw1, tw2)]
    tail_rows = [-(-TAIL_CELLS // w) for w in (tw0, tw1, tw2)]

    # d scrolls a half page (11 rows): the top lands on line 2's row 10.
    anchor_cell = 10 * tw0
    line2 = list("x" * LONG_CELLS)
    splice(line2, anchor_cell, "MARKER")
    line2 = "".join(line2)

    # EOF at 60 columns: MaxTop's row sits inside the tail; TAIL marks
    # its first cells. After the 140-column clamp the anchor moves to
    # the clamped row's first cell (CLMP); re-narrowing restores the
    # row containing it (WIDE) — not the pre-clamp row.
    tail0_60, tail0_140 = rows_before_tail[1], rows_before_tail[2]
    eof_top_60 = tail0_60 + tail_rows[1] - 23
    cell_eof = (eof_top_60 - tail0_60) * tw1
    eof_top_140 = tail0_140 + tail_rows[2] - 23
    cell_clamp = (eof_top_140 - tail0_140) * tw2
    if eof_top_60 < tail0_60 or eof_top_140 < tail0_140:
        fail("fixture too short: EOF top not inside the tail line")
    cell_back = (cell_clamp // tw1) * tw1
    tail = list("x" * TAIL_CELLS)
    splice(tail, cell_eof + 2, "TAIL")
    splice(tail, cell_clamp + 2, "CLMP")
    splice(tail, cell_back + 2, "WIDE")
    tail = "".join(tail)

    lines = ["needle first", line2]
    lines += ["pad %02d" % i for i in range(3, 3 + PADS)]
    lines.append(tail)
    with open(os.path.join(FIXTURE, "long.txt"), "w") as f:
        f.write("\n".join(lines) + "\n")

    s.wait_for(b"long.txt", "initial frame")
    screen = s.wait_loaded("initial load")
    if not panel(s, screen, 1).startswith(" 1  needle first"):
        fail("row 1 = %r, want the numbered first line" % panel(s, screen, 1))
    print("%-9s: loaded — %r" % ("init", panel(s, screen, 1).rstrip()))

    # d: the top lands inside the long line; the anchor becomes
    # {line 2, cell anchor_cell} — the top row starts with MARKER.
    screen = s.send_settle(b"d")
    top100 = " " * GW + line2[anchor_cell:anchor_cell + tw0]
    if panel(s, screen, 1) != top100:
        fail("d: top = %r, want %r" % (panel(s, screen, 1), top100))
    print("%-9s: anchor is {line 2, cell %d} — %r"
          % ("d@100", anchor_cell, panel(s, screen, 1).rstrip()))

    # Narrow to 60: the anchor is retained, so the top is the row
    # containing cell anchor_cell — a different ordinal, the same text.
    screen = s.resize(60, 24, "narrow")
    start = (anchor_cell // tw1) * tw1
    want = " " * GW + line2[start:start + tw1]
    if panel(s, screen, 1) != want:
        fail("60: top = %r, want the row holding cell %d: %r"
             % (panel(s, screen, 1), anchor_cell, want))
    print("%-9s: rewrapped — same text at top — %r"
          % ("60", panel(s, screen, 1).rstrip()))

    # Widen to 140: same anchor, still the containing row.
    screen = s.resize(140, 24, "widen")
    start = (anchor_cell // tw2) * tw2
    want = " " * GW + line2[start:start + tw2]
    if panel(s, screen, 1) != want:
        fail("140: top = %r, want %r" % (panel(s, screen, 1), want))
    print("%-9s: rewrapped — same text at top — %r…"
          % ("140", panel(s, screen, 1)[:GW + 18].rstrip()))

    # Back to 100: the round trip restores the original row exactly.
    screen = s.resize(100, 24, "restore")
    if panel(s, screen, 1) != top100:
        fail("100: top = %r, want the original %r"
             % (panel(s, screen, 1), top100))
    print("%-9s: original top row restored — %r"
          % ("100", panel(s, screen, 1).rstrip()))

    # w: run-off-edge. The anchor's line shows as its single numbered
    # row; the logical column is retained even though unrendered.
    s.send(b"w")
    screen = s.wait_loaded("w off")
    want = " 2  " + line2[:100 - s.panel - GW - 1] + " "
    if panel(s, screen, 1) != want:
        fail("w: top = %r, want %r (clipped + reserved blank)"
             % (panel(s, screen, 1), want))
    print("%-9s: line 2 as one clipped row — %r…"
          % ("w", panel(s, screen, 1)[:GW + 12].rstrip()))

    # w again: the retained column restores the original wrapped row.
    s.send(b"w")
    screen = s.wait_loaded("w on")
    if panel(s, screen, 1) != top100:
        fail("w: top = %r, want the original %r"
             % (panel(s, screen, 1), top100))
    print("%-9s: wrapped again — same top row — %r"
          % ("w", panel(s, screen, 1).rstrip()))

    # EOF at 60 columns: scroll until the tail's end sits on the bottom
    # row; the top is a mid-tail row showing the TAIL marker.
    s.resize(60, 24, "narrow for EOF")
    screen = s.send_settle(b"d" * 20)
    top_eof = panel(s, screen, 1)
    want = " " * GW + tail[cell_eof:cell_eof + tw1]
    if top_eof != want:
        fail("EOF@60: top = %r, want %r" % (top_eof, want))
    rem = TAIL_CELLS - (tail_rows[1] - 1) * tw1
    want_bottom = " " * GW + "x" * rem + " " * (tw1 - rem)
    if panel(s, screen, 23) != want_bottom:
        fail("EOF@60: bottom = %r, want the tail's last %d cells: %r"
             % (panel(s, screen, 23), rem, want_bottom))
    print("%-9s: EOF — top %r, bottom is the tail end %r"
          % ("d…@60", top_eof.rstrip(), panel(s, screen, 23).rstrip()))

    # Widen to 140: fewer rendered rows, so the EOF clamp pulls the top
    # up — the anchor is rewritten to the clamped row (CLMP marker).
    screen = s.resize(140, 24, "widen at EOF")
    want = " " * GW + tail[cell_clamp:cell_clamp + tw2]
    if panel(s, screen, 1) != want:
        fail("EOF@140: top = %r, want the clamped row %r"
             % (panel(s, screen, 1), want))
    print("%-9s: clamp moved the top to cell %d — %r…"
          % ("140", cell_clamp, panel(s, screen, 1)[:GW + 12].rstrip()))

    # Narrow back to 60: the anchor is the clamped location now, so the
    # pre-clamp top is gone — the top shows the WIDE-marker row instead
    # of returning to the TAIL-marker row. Lossy, on purpose.
    screen = s.resize(60, 24, "re-narrow")
    want = " " * GW + tail[cell_back:cell_back + tw1]
    if panel(s, screen, 1) != want:
        fail("EOF@60 again: top = %r, want the clamped location %r"
             % (panel(s, screen, 1), want))
    if panel(s, screen, 1) == top_eof:
        fail("EOF@60 again: pre-clamp top came back — anchor not lossy")
    print("%-9s: stays at the clamped location — %r"
          % ("60", panel(s, screen, 1).rstrip()))

    s.send(b"q")
    if s.wait_exit("quit") != 0:
        fail("quit: exit=%d, want 0" % s.code)
    print("%-9s: exit 0" % "q")


def session_b(workdir):
    """~50 MB file: a resize burst queues layout preparations off the
    update path; ctrl+c mid-rewrap exits promptly with 130."""
    s = Session(100, 24, workdir, "h", "huge.txt")
    s.wait_for(b"needle", "browse", t=120)
    s.settle(quiet=0.6, t=60)
    print("%-9s: ~50 MB loaded and laid out" % "huge")

    # Fire a burst of resizes without waiting: each queues a fresh
    # layout command whose ~50 MB preparation runs off the update
    # path; the pump keeps the pty drained while repaints land.
    for cols in (130, 75, 150, 65, 110, 90, 140, 70):
        s.resize_raw(cols, 24)
        pump(s.master, s.out, 0.05)
    t0 = time.time()
    s.send(b"\x03")
    code = s.wait_exit("ctrl+c", t=15)
    elapsed = time.time() - t0
    if code != 130:
        fail("ctrl+c mid-rewrap: exit=%d, want 130" % code)
    if elapsed >= 5:
        fail("ctrl+c mid-rewrap took %.1fs — the exit waited on the "
             "in-flight rewrap" % elapsed)
    print("%-9s: exit 130 with rewraps in flight — well under the 5s "
          "promptness bound" % "ctrl+c")


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)
    make_dirs()
    write_huge()
    workdir = tempfile.mkdtemp(prefix="vrg017")
    os.symlink(FIXTURE, os.path.join(workdir, "f"))
    os.symlink(HUGE, os.path.join(workdir, "h"))
    session_a(workdir)
    session_b(workdir)
    print("OK")


if __name__ == "__main__":
    main()
