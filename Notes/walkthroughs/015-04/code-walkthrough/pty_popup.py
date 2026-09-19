#!/usr/bin/env python3
"""Issue #15 walkthrough: the file-change pop-up on the real binary.

Builds a fixture inside this walkthrough directory — a.txt and b.txt
with one match each, plus a file whose name carries hostile bytes
(embedded newline, ESC, invalid UTF-8) — then drives the built binary
on a 100x24 pty. The renderer diffs at cell level, so the raw byte
stream is replayed through a small screen emulator; pop-up presence,
position, and content are read off the emulated grid:

  n->b    — n across the boundary shows a centred bordered box carrying
            b.txt's escaped path, while the filename rule already names
            b.txt: the pop-up starts at selection.
  n->evil — a quick second n dismisses the b.txt pop-up AND navigates:
            the fresh instance shows the new file's path — control
            bytes escaped (^[, \n, \xff) on a single box row.
  expire  — about one second later the pop-up repaints away.
  n->a    — wrapping back to a.txt opens another fresh instance.
  resize  — shrinking the pty to 60x15 recentres and left-truncates the
            same live pop-up; no key press was involved.
  q       — exits 0.
"""
import fcntl
import os
import pty
import select
import shutil
import struct
import sys
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
BIN = os.path.join(HERE, "vrg")
FIXTURE = os.path.join(HERE, "fixture")
EVIL_BYTES = b"zz-evil\nname\x1b[2J\xff.txt"
# What EscapePath produces for the fixture-resolved hostile path.
EVIL_ESCAPED = "./zz-evil\\nname^[[2J\\xff.txt"
COLS, ROWS = 100, 24


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def make_fixture():
    if os.path.isdir(FIXTURE):
        shutil.rmtree(FIXTURE)
    os.makedirs(FIXTURE)
    for name, content in (
        ("a.txt", "a row\nhit a\n"),
        ("b.txt", "b row\nhit b\n"),
    ):
        with open(os.path.join(FIXTURE, name), "w") as f:
            f.write(content)
    # A bytes path keeps the raw invalid-UTF-8 byte in the name — a
    # str path would UTF-8-encode \xff into a valid (printable) rune.
    fd = os.open(os.fsencode(FIXTURE) + b"/" + EVIL_BYTES,
                 os.O_CREAT | os.O_WRONLY | os.O_TRUNC)
    os.write(fd, b"z row\nhit z\n")
    os.close(fd)


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

    Handles CUP/CUx cursor motion, EL/ED erases, and deferred autowrap —
    enough for vrg's full-frame repaints.
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
        elif final == "L":
            for _ in range(arg(0, 1)):
                grid.insert(r, [" "] * cols)
                del grid[rows:]
        elif final == "M":
            for _ in range(arg(0, 1)):
                del grid[r]
                grid.append([" "] * cols)
        elif final == "P":
            n = arg(0, 1)
            del grid[r][c:c + n]
            grid[r].extend([" "] * n)
            del grid[r][cols:]
        elif final == "@":
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
    """vrg running on its own pty; out accumulates the raw byte stream."""

    def __init__(self, cols, rows):
        self.cols, self.rows = cols, rows
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
            os.chdir(FIXTURE)
            os.execvpe(BIN, [BIN, "hit", "."], env)
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

    def send(self, data):
        os.write(self.master, data)

    def send_settle(self, data, quiet=0.4):
        self.send(data)
        return self.settle(quiet)

    def resize(self, cols, rows):
        """Resize the pty — the SIGWINCH-driven path a real terminal
        resize takes — then emulate only the post-resize repaint."""
        fcntl.ioctl(self.master, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        self.cols, self.rows = cols, rows
        self.out = bytearray()
        return self.settle()

    def wait_exit(self, label, t=10):
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


def find_box(screen):
    """Locate the pop-up's top-left corner; None when no box is up."""
    for r, row in enumerate(screen):
        i = row.find("┌")
        if i >= 0:
            return r, i
    return None


def want_box(s, screen, needle, label):
    """Assert a centred box is up, carrying needle in its interior row,
    and return (top, left)."""
    box = find_box(screen)
    if box is None:
        s.kill()
        fail("%s: no pop-up box in %r" % (label, screen))
    top, left = box
    interior = screen[top + 1]
    if needle not in interior:
        s.kill()
        fail("%s: pop-up row %r lacks %r" % (label, interior, needle))
    if not screen[top + 2].lstrip().startswith("└"):
        s.kill()
        fail("%s: no bottom border under the pop-up: %r" % (label, screen[top + 2]))
    want_top = (s.rows - 3) // 2
    box_w = len(interior.strip())
    want_left = max(0, (s.cols - box_w) // 2)
    if top != want_top or left != want_left:
        s.kill()
        fail("%s: box at (%d,%d), want centred (%d,%d) on %dx%d"
             % (label, top, left, want_top, want_left, s.cols, s.rows))
    print("%-10s: box top=%d left=%d row=%r"
          % (label, top, left, interior.strip()))
    return box


def want_no_box(s, screen, label):
    if find_box(screen) is not None:
        s.kill()
        fail("%s: pop-up still up in %r" % (label, screen))
    print("%-10s: no pop-up box" % label)


def main():
    if not os.path.exists(BIN):
        fail("build the binary first: go build -o %s ./cmd/vrg" % BIN)
    make_fixture()
    s = Session(COLS, ROWS)

    # Startup selects a.txt — the first selection is not a file change,
    # so no pop-up is up.
    s.wait_for(b"./b.txt", "initial frame")
    screen = s.settle()
    want_no_box(s, screen, "init")

    # n crosses into b.txt: the centred pop-up carries the escaped path
    # while the filename rule already names b.txt — selection-time start.
    screen = s.send_settle(b"n", quiet=0.2)
    if "/./b.txt" not in screen[0]:
        s.kill()
        fail("n->b: filename rule %r does not name b.txt" % screen[0])
    want_box(s, screen, "/./b.txt", "n->b")

    # A quick second n — while the b.txt pop-up is still up — dismisses
    # it and navigates in the same press: the fresh instance shows the
    # new file's hostile path, escaped onto a single row.
    screen = s.send_settle(b"n", quiet=0.2)
    if EVIL_ESCAPED not in screen[0]:
        s.kill()
        fail("n->evil: filename rule %r lacks the escaped name" % screen[0])
    want_box(s, screen, EVIL_ESCAPED, "n->evil")
    # A raw ESC + [2J in the name would have cleared the emulated
    # screen; the intact file list proves only escaped bytes arrived.
    if "a.txt" not in "".join(screen[1:]):
        s.kill()
        fail("n->evil: the file list vanished — a raw control byte escaped")

    # The one-second expiry repaints the box away on its own. Gone well
    # past the 0.4s settle quiet — so the dismissal is the timer's, not
    # a missed keypress — and inside the two-second upper bound.
    t0 = time.time()
    while time.time() - t0 < 2:
        pump(s.master, s.out, 0.1)
        if find_box(s.screen()) is None:
            break
    gone = time.time() - t0
    if find_box(s.screen()) is not None:
        s.kill()
        fail("expire: pop-up still up after %.1fs" % gone)
    if gone < 0.3:
        s.kill()
        fail("expire: pop-up vanished in %.1fs — too fast for the 1s timer" % gone)
    print("%-10s: pop-up expired on its own — the one-second timer"
          % "expire")

    # n wraps back to a.txt — a fresh pop-up instance — then the pty
    # shrinks to 60x15: the same live pop-up recentres and the now
    # over-wide path left-truncates with a leading …, no key needed.
    screen = s.send_settle(b"n", quiet=0.2)
    want_box(s, screen, "a.txt", "n->a")
    screen = s.resize(60, 15)
    top, left = want_box(s, screen, "a.txt", "resize")
    if "…" not in screen[top + 1]:
        s.kill()
        fail("resize: pop-up row %r lacks the … truncation" % screen[top + 1])
    print("%-10s: recentred and truncated on 60x15" % "resize")

    s.send(b"q")
    if s.wait_exit("quit") != 0:
        fail("quit: exit=%d, want 0" % s.code)
    print("q         : exit 0")
    print("OK")


if __name__ == "__main__":
    main()
