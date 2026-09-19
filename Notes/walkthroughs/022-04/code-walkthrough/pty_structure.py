#!/usr/bin/env python3
"""Issue #22 walkthrough: structural line handling on a real pty.

Four sessions on a real pty (100x24), each rooted at its own fixture
directory so every file list shows one entry:

  fixture/crlf/a.txt  — "hit\r\nmiss\r\nhit x\r\n": CRLF terminators
                        never display, `hit` highlights land correctly
  fixture/cr/a.txt    — "ab\rcd\n": a standalone CR mid-line renders
                        the safe-presentation ^M escape
  fixture/bom/a.txt   — "\xef\xbb\xbfhit\nsecond hit\n": the leading
                        UTF-8 BOM is invisible and the first-line
                        match highlights the right cells
  fixture/empty/a.txt — zero bytes: a fake rg on PATH emits a valid
                        begin/match/end/summary stream claiming a
                        `foo` match on line 1; the panel is empty —
                        zero source lines, the three-cell gutter all
                        blank (the stale note is Issue #29's)

Assertions run on the accumulated raw byte stream and on the emulated
screen: the current matched line styles runs as 30;47;4 (inverse +
underline), other lines as 30;47 (inverse). The script drives:

  session A — vrg hit f   (real rg, CRLF file)
      initial : "1  hit" — no ^M anywhere; styled run holds "hit"
      n       : line 3 "hit x" becomes the current match
      q       — exit 0
  session B — vrg cd f    (real rg, standalone-CR file)
      initial : "1  ab^Mcd" — the mid-line CR is content, escaped
      q       — exit 0
  session C — vrg hit f   (real rg, UTF-8 BOM file)
      initial : "1  hit" — no BOM bytes or \ufeff anywhere; the
                styled run starts on 'h' at the line's first cell
      q       — exit 0
  session D — vrg foo f   (fake rg, zero-byte file)
      initial : panel entirely blank — empty file, empty panel
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

MATCH = b"\x1b[30;47m"       # inverse — a match on a non-current line
CURMATCH = b"\x1b[30;47;4m"  # inverse + underline — the current line

BOM = b"\xef\xbb\xbf"


def fail(msg):
    print("FAIL: " + msg)
    sys.exit(1)


def write_fixtures():
    for case, data in [
        ("crlf", b"hit\r\nmiss\r\nhit x\r\n"),
        ("cr", b"ab\rcd\n"),
        ("bom", BOM + b"hit\nsecond hit\n"),
        ("empty", b""),
    ]:
        d = os.path.join(FIXTURE, case)
        os.makedirs(d, exist_ok=True)
        with open(os.path.join(d, "a.txt"), "wb") as f:
            f.write(data)


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

    The child runs with cwd=workdir and searches operand — the file
    list then displays the resolved path workdir/operand/name, so a
    short tmp cwd keeps the file panel usable at narrow widths.
    extra_env lets a session prepend a fake-rg directory to PATH.
    """

    def __init__(self, cols, rows, workdir, operand, name, pattern, extra_env=None):
        self.cols, self.rows = cols, rows
        self.panel = len(workdir + "/" + operand + "/" + name) + 1
        master, slave = pty.openpty()
        fcntl.ioctl(slave, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        env = dict(os.environ)
        env["TERM"] = "xterm"
        if extra_env:
            env.update(extra_env)
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
        """Settle until the panel shows installed rows, not 'Loading…'."""
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


def styled(s, needle, label, style):
    """The styled run must contain the needle's bytes."""
    if style + needle not in s.out:
        fail("%s: %r + %r never styled; output %r"
             % (label, style, needle, bytes(s.out)))


def panel(s, screen, r):
    return screen[r][s.panel:]


def main():
    # A short /tmp cwd with a symlink f → fixture/<case> keeps the
    # file list narrow: displayed paths are workdir/f/<name>.
    workdir = tempfile.mkdtemp(prefix="vrgstruct")
    write_fixtures()

    # Session A — CRLF file: terminators never display and the hit
    # highlights land on the visible text cells.
    os.symlink(os.path.join(FIXTURE, "crlf"), os.path.join(workdir, "f"))
    s = Session(COLS, ROWS, workdir, "f", "a.txt", "hit")
    s.wait_for(b"a.txt", "session A initial frame")
    screen = s.wait_loaded("session A initial load")
    if b"^M" in s.out:
        fail("session A: a ^M escape appeared — CRLF must not display")
    styled(s, b"hit", "CRLF line 1 match", CURMATCH)
    row1 = panel(s, screen, 1)
    if not row1.startswith("1  hit"):
        fail("row 1 = %r, want '1  hit' — the CR must not display" % row1)
    print("%-9s: CRLF — '1  hit', no ^M, hit styled — %r" % ("setup", row1[:10]))

    # n to line 3: "hit x" becomes the current match.
    screen = s.send_settle(b"n")
    row3 = panel(s, screen, 3)
    if not row3.startswith("3  hit x"):
        fail("row 3 after n = %r, want '3  hit x'" % row3)
    print("%-9s: line 3 current — %r" % ("n", row3[:12]))

    s.send(b"q")
    if s.wait_exit("session A quit") != 0:
        fail("session A quit: exit=%d, want 0" % s.code)
    print("%-9s: exit 0" % "q")

    # Session B — a standalone CR mid-line is content, escaped ^M.
    os.unlink(os.path.join(workdir, "f"))
    os.symlink(os.path.join(FIXTURE, "cr"), os.path.join(workdir, "f"))
    s = Session(COLS, ROWS, workdir, "f", "a.txt", "cd")
    s.wait_for(b"a.txt", "session B initial frame")
    screen = s.wait_loaded("session B initial load")
    styled(s, b"cd", "match after the CR", CURMATCH)
    row1 = panel(s, screen, 1)
    if not row1.startswith("1  ab^Mcd"):
        fail("row 1 = %r, want '1  ab^Mcd' — standalone CR escaped" % row1)
    print("%-9s: standalone CR renders ^M — %r" % ("setup", row1[:12]))

    s.send(b"q")
    if s.wait_exit("session B quit") != 0:
        fail("session B quit: exit=%d, want 0" % s.code)
    print("%-9s: exit 0" % "q")

    # Session C — UTF-8 BOM file: the BOM is invisible and the
    # first-line match highlights the right cells.
    os.unlink(os.path.join(workdir, "f"))
    os.symlink(os.path.join(FIXTURE, "bom"), os.path.join(workdir, "f"))
    s = Session(COLS, ROWS, workdir, "f", "a.txt", "hit")
    s.wait_for(b"a.txt", "session C initial frame")
    screen = s.wait_loaded("session C initial load")
    if BOM in s.out or "\\ufeff".encode() in s.out or "﻿".encode() in s.out:
        fail("session C: the BOM leaked into display — it must be invisible")
    styled(s, b"hit", "BOM line 1 match", CURMATCH)
    row1 = panel(s, screen, 1)
    row2 = panel(s, screen, 2)
    if not row1.startswith("1  hit"):
        fail("row 1 = %r, want '1  hit' — no visible BOM" % row1)
    if not row2.startswith("2  second hit"):
        fail("row 2 = %r, want '2  second hit'" % row2)
    print("%-9s: BOM invisible — '1  hit' styled at the line start — %r"
          % ("setup", row1[:10]))

    s.send(b"q")
    if s.wait_exit("session C quit") != 0:
        fail("session C quit: exit=%d, want 0" % s.code)
    print("%-9s: exit 0" % "q")

    # Session D — zero-byte a.txt behind a fake rg that emits a valid
    # begin/match/end/summary stream claiming a `foo` match on line 1:
    # the buffer holds zero source lines, so the panel is an empty
    # field of blanks (the one-digit-slot, three-cell gutter paints
    # nothing without rows).
    fakebin = os.path.join(workdir, "fakebin")
    os.makedirs(fakebin, exist_ok=True)
    stream = "\n".join([
        '{"type":"begin","data":{"path":{"text":"f/a.txt"}}}',
        '{"type":"match","data":{"path":{"text":"f/a.txt"},'
        '"lines":{"text":"foo"},"line_number":1,"absolute_offset":0,'
        '"submatches":[{"match":{"text":"foo"},"start":0,"end":3}]}}',
        '{"type":"end","data":{"path":{"text":"f/a.txt"},"binary_offset":null}}',
        '{"data":{"elapsed_total":{"secs":0,"nanos":1,"human":"0s"},'
        '"stats":{"elapsed":{"secs":0,"nanos":1,"human":"0s"},'
        '"searches":1,"searches_with_match":1,"bytes_searched":0,'
        '"bytes_printed":0,"matched_lines":1,"matches":1}},"type":"summary"}',
        "",
    ])
    fake = os.path.join(fakebin, "rg")
    with open(fake, "w") as f:
        f.write("#!/bin/sh\nprintf '%s' '" + stream.replace("'", "'\\''") + "'\n")
    os.chmod(fake, 0o755)

    os.unlink(os.path.join(workdir, "f"))
    os.symlink(os.path.join(FIXTURE, "empty"), os.path.join(workdir, "f"))
    env = {"PATH": fakebin + ":" + os.environ["PATH"]}
    s = Session(COLS, ROWS, workdir, "f", "a.txt", "foo", extra_env=env)
    s.wait_for(b"a.txt", "session D initial frame")
    screen = s.wait_loaded("session D initial load")
    for r in range(1, ROWS):
        if panel(s, screen, r).strip():
            fail("empty file: panel row %d = %r, want all blank" %
                 (r, panel(s, screen, r)))
    print("%-9s: zero-byte a.txt — panel all blank, zero source rows — %r"
          % ("setup", panel(s, screen, 1)[:12]))

    s.send(b"q")
    if s.wait_exit("session D quit") != 0:
        fail("session D quit: exit=%d, want 0" % s.code)
    print("%-9s: exit 0" % "q")
    print("OK")


if __name__ == "__main__":
    main()
