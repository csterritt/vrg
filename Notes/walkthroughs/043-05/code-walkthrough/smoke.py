#!/usr/bin/env python3
"""Issue #43 smoke harness: the standalone-combining-cluster fallback cell.

Drives the built vrg binary on a real 80x24 PTY through the issue's
manual scenario. A fake rg emits two matches in fixture/work/a.txt:
the combining mark U+0301 at byte range [0,2) of line 1 - a cluster
with no base, opening the line - and the same mark at [3,5) of line 2,
standalone mid-line after a control byte (displayed 'ab^A...'). Line 1
carries enough trailing x's that wrap mode shows it on two rows while
run-off-edge clips it to one, making the 'w' mode flip observable.

The checks prove the recorded Candidate-A fallback - '◌' (U+25CC) plus
the cluster's original mark bytes - behaves as a real one-cell unit:

- the fallback cell paints '◌́' as ONE grid cell holding both
  codepoints (the zero-width mark stacks onto the dotted circle), the
  'x' of the following cluster sits in the very next cell - no overlap,
  no shared cell - and the match covering the mark styles exactly that
  one cell, nothing adjacent;
- after 'n' the same holds mid-line: the '◌́' between the '^A' escape
  cells and 'c' is the only styled cell;
- after 'w' + '.', one pan step hides the fallback's single cell whole
  (no '◌' remains on the row), the x run shifts exactly one cell left,
  and the entirely hidden match upgrades the gutter mark to '*' - the
  fallback counted like any other cell by clipping and panning.

Every key is sent only after an explicit rendered condition is
observed in the emulated screen; each wait is a bounded poll - never
a fixed settling delay. assert_no_fixed_delays() proves that
mechanically by parsing this file's AST.

The harness also plays terminal honestly: it clears the SSH/session
environment variables that suppress bubbletea's DECRQM probes and
answers the mode-2027 (Unicode core) query with DECRPM 'set'.
(Emulator and PTY plumbing carried over from the Issue #41/#42
harnesses.)
"""

import ast
import base64
import fcntl
import os
import pty
import select
import shutil
import signal
import struct
import sys
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
VRG = os.path.join(HERE, "vrg")
FIXTURE_ABS = os.path.join(HERE, "fixture")
FAKEBIN = os.path.join(FIXTURE_ABS, "fakebin")
WORKDIR = os.path.join(FIXTURE_ABS, "work")

COLS, ROWS = 80, 24

# POLL_IDLE paces one iteration of a bounded condition poll. In the
# waits it is a select idle on the real PTY descriptor (genuine I/O
# waiting); it never stands in as an assumed settling interval.
POLL_IDLE = 0.05

failures = []


class SmokeError(Exception):
    """A bounded wait failed: the named condition never held."""


def log(msg):
    print(msg, flush=True)


def assert_no_fixed_delays():
    """Static self-check: parse this file's AST and fail if it calls
    time.sleep - the fixed-settling primitive the harness contract
    bans. Bounded polls pace themselves with select idles that
    re-check an explicit condition every iteration; nothing else in
    this file waits on elapsed time."""
    with open(__file__) as f:
        tree = ast.parse(f.read())
    for node in ast.walk(tree):
        if (isinstance(node, ast.Call)
                and isinstance(node.func, ast.Attribute)
                and node.func.attr == "sleep"
                and isinstance(node.func.value, ast.Name)
                and node.func.value.id == "time"):
            raise SmokeError(
                "time.sleep call found in %s - use a bounded "
                "poll on an explicit condition" % __file__)


def check(name, cond, detail=""):
    status = "PASS" if cond else "FAIL"
    if cond:
        log("  [%s] %s" % (status, name))
    else:
        log("  [%s] %s%s" % (status, name, ": " + detail if detail else ""))
        failures.append(name)


def cell_width(ch):
    if unicodedata.combining(ch):
        return 0
    if ch in ("‍", "︎", "️"):
        return 0  # ZWJ and variation selectors join the cluster
    return 2 if unicodedata.east_asian_width(ch) in ("W", "F") else 1


def emulate(data, rows, cols):
    """Replay the byte stream into a ROWSxCOLS text grid plus parallel
    inverse-style and underline grids.

    Handles CUP/CUx cursor motion, EL/ED erases, IL/DL, deferred
    autowrap, and SGR attributes: cells painted while the dark
    scheme's inverse pair '30;47' is active mark True on the second
    grid, and cells painted while SGR 4 (underline - the current
    matched line's style) is active mark True on the third.
    Zero-width codepoints - combining marks, ZWJ, variation
    selectors, and any codepoint a pending ZWJ joins - stack onto the
    last painted cell, so the ◌ fallback cell ends up holding '◌́' as
    one grid cell. (Carried over from the Issue #41/#42 harnesses,
    extended with the underline grid.)
    """
    grid = [[" "] * cols for _ in range(rows)]
    inv = [[False] * cols for _ in range(rows)]
    und = [[False] * cols for _ in range(rows)]
    r = c = 0
    lastc = -1
    zwj = False
    wrap = False
    cur_inv = False
    cur_und = False
    top, bot = 0, rows - 1

    def ri():
        """ESC M - reverse index: scroll the region down at its top."""
        nonlocal r
        if r == top:
            grid.insert(top, [" "] * cols)
            inv.insert(top, [False] * cols)
            und.insert(top, [False] * cols)
            del grid[bot + 1]
            del inv[bot + 1]
            del und[bot + 1]
        else:
            r = max(0, r - 1)

    def csi(r, c, params, final):
        nonlocal cur_inv, cur_und, top, bot
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
                    inv[i][:] = [False] * cols
                    und[i][:] = [False] * cols
            elif mode == 0:
                grid[r][c:] = [" "] * (cols - c)
                inv[r][c:] = [False] * (cols - c)
                und[r][c:] = [False] * (cols - c)
                for i in range(r + 1, rows):
                    grid[i][:] = [" "] * cols
                    inv[i][:] = [False] * cols
                    und[i][:] = [False] * cols
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                inv[r][:c + 1] = [False] * (c + 1)
                und[r][:c + 1] = [False] * (c + 1)
                for i in range(r):
                    grid[i][:] = [" "] * cols
                    inv[i][:] = [False] * cols
                    und[i][:] = [False] * cols
        elif final == "K":
            mode = arg(0, 0)
            if mode == 0:
                grid[r][c:] = [" "] * (cols - c)
                inv[r][c:] = [False] * (cols - c)
                und[r][c:] = [False] * (cols - c)
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                inv[r][:c + 1] = [False] * (c + 1)
                und[r][:c + 1] = [False] * (c + 1)
            elif mode == 2:
                grid[r][:] = [" "] * cols
                inv[r][:] = [False] * cols
                und[r][:] = [False] * cols
        elif final == "L":  # insert blank lines inside the region
            for _ in range(arg(0, 1)):
                del grid[bot]
                del inv[bot]
                del und[bot]
                grid.insert(r, [" "] * cols)
                inv.insert(r, [False] * cols)
                und.insert(r, [False] * cols)
        elif final == "M":  # delete lines inside the region
            for _ in range(arg(0, 1)):
                del grid[r]
                del inv[r]
                del und[r]
                grid.insert(bot, [" "] * cols)
                inv.insert(bot, [False] * cols)
                und.insert(bot, [False] * cols)
        elif final == "S":  # SU: scroll the region up n lines
            for _ in range(arg(0, 1)):
                del grid[top]
                del inv[top]
                del und[top]
                grid.insert(bot, [" "] * cols)
                inv.insert(bot, [False] * cols)
                und.insert(bot, [False] * cols)
        elif final == "T":  # SD: scroll the region down n lines
            for _ in range(arg(0, 1)):
                del grid[bot]
                del inv[bot]
                del und[bot]
                grid.insert(top, [" "] * cols)
                inv.insert(top, [False] * cols)
                und.insert(top, [False] * cols)
        elif final == "X":  # erase chars from the cursor rightward
            n = arg(0, 1)
            grid[r][c:c + n] = [" "] * n
            inv[r][c:c + n] = [False] * n
            und[r][c:c + n] = [False] * n
        elif final == "P":  # delete chars
            n = arg(0, 1)
            del grid[r][c:c + n]
            grid[r].extend([" "] * n)
            del grid[r][cols:]
        elif final == "@":  # insert blank chars
            n = arg(0, 1)
            grid[r][c:c] = [" "] * n
            inv[r][c:c] = [False] * n
            und[r][c:c] = [False] * n
            del grid[r][cols:]
        elif final == "r":  # DECSTBM: set the scroll region
            top, bot = arg(0, 1) - 1, arg(1, rows) - 1
        elif final == "m":  # SGR: inverse pair and underline
            cur_inv = "30;47" in params
            cur_und = 4 in nums
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
                i += 2  # ESC + single byte (modes, DECSC, ...)
            continue
        if b == 0x0d:
            c, wrap, i = 0, False, i + 1
            continue
        if b == 0x0a:
            # LF at the bottom margin scrolls the region up one line;
            # otherwise it just moves the cursor down. bubbletea's
            # scroll repaint relies on this (DECSTBM + LF).
            if r == bot:
                del grid[top]
                del inv[top]
                del und[top]
                grid.insert(bot, [" "] * cols)
                inv.insert(bot, [False] * cols)
                und.insert(bot, [False] * cols)
            else:
                r = min(r + 1, rows - 1)
            wrap, i = False, i + 1
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
        if zwj:
            w, zwj = 0, False
        if ch == "‍":
            w, zwj = 0, True
        if wrap:
            r, c, wrap = min(r + 1, rows - 1), 0, False
        if w == 0:
            if 0 <= r < rows and lastc >= 0:
                grid[r][lastc] += ch
        else:
            if 0 <= r < rows and 0 <= c < cols:
                grid[r][c] = ch
                inv[r][c] = cur_inv
                und[r][c] = cur_und
                if w == 2 and c + 1 < cols:
                    grid[r][c + 1] = ""
                    inv[r][c + 1] = cur_inv
                    und[r][c + 1] = cur_und
            lastc = c
            c += w
            if c >= cols:
                c, wrap = cols - 1, True
        i += size
    return grid, inv, und


def row_text(row):
    """Join a cell row for substring searches."""
    return "".join(row)


def screen_text(scr):
    """The whole emulated frame as one string."""
    return "\n".join(row_text(row) for row in scr)


def find_fallback_cell(scr, r):
    """The column of the cell containing '◌' on row r, or -1."""
    for c, cell in enumerate(scr[r]):
        if "◌" in cell:
            return c
    return -1


class PtyRun:
    """One vrg run under a PTY.

    wait_screen polls the emulated grid for an explicit rendered
    predicate, wait_exit polls process completion, and drain reads the
    PTY until EOF - every synchronization point is a bounded poll on
    an externally observable condition.
    """

    def __init__(self, cmd, cwd=None, env_extra=None,
                 winsize=(ROWS, COLS), timeout=90.0):
        self.cols, self.rows = winsize[1], winsize[0]
        self.timeout = timeout
        env = dict(os.environ)
        env["TERM"] = "xterm"
        # Clear the SSH/session vars that keep bubbletea from asking
        # the terminal about Unicode core mode (DECRQM ?2027); the
        # harness answers the query like a terminal that supports it.
        for k in ("TERM_PROGRAM", "SSH_TTY", "SSH_CLIENT",
                  "SSH_CONNECTION", "WT_SESSION"):
            env.pop(k, None)
        if env_extra:
            env.update(env_extra)

        pid, fd = pty.fork()
        if pid == 0:
            if cwd:
                os.chdir(cwd)
            os.execvpe(cmd[0], cmd, env)
            os._exit(127)

        try:
            wins = struct.pack("HHHH", winsize[0], winsize[1], 0, 0)
            fcntl.ioctl(fd, termios.TIOCSWINSZ, wins)
        except OSError:
            pass

        self.pid = pid
        self.fd = fd
        # The baseline is a fresh PTY pair's slave settings: the child
        # may already have set raw mode by the time the parent reads
        # the shared line discipline, so the run's own fd cannot time
        # the "before" reliably. vrg restores those defaults on exit.
        m, s = pty.openpty()
        try:
            self.before_termios = termios.tcgetattr(s)
        finally:
            os.close(m)
            os.close(s)
        self.after_termios = None
        self.raw = bytearray()
        self.eof = False
        self.exited = False
        self.exit_code = None
        self._answered = set()

    def _pump(self, idle):
        """One I/O-multiplexed read step: wait up to idle for the PTY
        to become readable, then drain what arrived. DECRQM mode
        queries are answered like a supporting terminal (DECRPM
        'set'). EOF is observed when the slave side closes (PTY
        masters report EIO)."""
        if self.eof:
            return
        r, _, _ = select.select([self.fd], [], [], idle)
        for f in r:
            try:
                data = os.read(f, 65536)
            except OSError:
                data = b""
            if data:
                self.raw += data
            else:
                self.eof = True
        for mode in (b"2026", b"2027"):
            if (mode not in self._answered
                    and b"\x1b[?" + mode + b"$p" in self.raw):
                os.write(self.fd, b"\x1b[?" + mode + b";1$y")
                self._answered.add(mode)

    def _wait(self, desc, cond, timeout=None):
        """Bounded condition poll over pumped PTY state."""
        timeout = self.timeout if timeout is None else timeout
        deadline = time.monotonic() + timeout
        while True:
            if cond():
                return
            remaining = deadline - time.monotonic()
            if remaining <= 0:
                raise SmokeError("timed out after %.1fs waiting for %s"
                                 % (timeout, desc))
            self._pump(min(POLL_IDLE, remaining))

    def screen(self):
        return emulate(bytes(self.raw), self.rows, self.cols)

    def wait_screen(self, desc, pred, timeout=None):
        """Wait until the emulated screen satisfies pred - the
        externally observable post-state a following key relies on.
        pred receives the (grid, inverse, underline) emulation."""
        result = []

        def probe():
            scr, inv, und = self.screen()
            if pred(scr, inv, und):
                result.append((scr, inv, und))
                return True
            return False

        self._wait(desc, probe, timeout)
        return result[0]

    def send(self, key):
        """Write bytes to the PTY. Synchronization is the caller's:
        a key is only ever sent after wait_screen observed the UI
        state that key depends on."""
        os.write(self.fd, key)

    def wait_exit(self, timeout=None):
        """Wait for process completion (bounded)."""
        self._wait("process exit", self._reap, timeout)

    def _reap(self):
        if self.exited:
            return True
        try:
            wpid, status = os.waitpid(self.pid, os.WNOHANG)
        except ChildProcessError:
            self.exited = True
            return True
        if wpid != self.pid:
            return False
        self.exited = True
        if os.WIFEXITED(status):
            self.exit_code = os.WEXITSTATUS(status)
        elif os.WIFSIGNALED(status):
            self.exit_code = 128 + os.WTERMSIG(status)
        return True

    def drain(self, timeout=None):
        """Completion/EOF-driven draining: read the PTY master until
        EOF (bounded)."""
        self._wait("PTY EOF", lambda: self.eof, timeout)

    def finish(self):
        """Snapshot post-exit termios and close the PTY (idempotent)."""
        if self.fd is None:
            return
        fd, self.fd = self.fd, None
        try:
            self.after_termios = termios.tcgetattr(fd)
        except (OSError, termios.error):
            pass
        try:
            os.close(fd)
        except OSError:
            pass

    def kill(self):
        """Failure path for a run that never completes."""
        try:
            os.kill(self.pid, signal.SIGKILL)
        except OSError:
            pass
        try:
            os.waitpid(self.pid, 0)
        except ChildProcessError:
            pass
        self.finish()


def check_terminal_restored(name, run):
    """Terminal restoration evidence: the display sequences (cursor
    visible; alt-screen left when it was entered) and the PTY input
    modes identical to the pre-launch capture."""
    raw = run.raw.decode("utf-8", "replace")
    check("%s cursor restored" % name, "\x1b[?25h" in raw)
    if "\x1b[?1049h" in raw:
        check("%s alt screen exited" % name, "\x1b[?1049l" in raw)
    check("%s termios restored" % name,
          run.before_termios == run.after_termios)


# The fixture line bytes. Line 1 opens with the standalone combining
# acute U+0301 (no base) followed by a long x run - long enough that
# wrap mode shows it on two rows while run-off-edge clips it to one,
# making the 'w' flip observable as line 2 moving up a row. Line 2
# holds the same mark standalone mid-line, after a control byte that
# displays as the ^A escape.
LINE1 = "́".encode() + b"x" * 98 + b"\n"
LINE2 = b"ab\x01" + "́".encode() + b"cd the mark sits mid-line\n"
MARK = "́".encode()


def write_fixtures():
    """fixture/work/a.txt plus fixture/fakebin/rg: the fake child emits
    one match record per mark - base64 'bytes' payloads so the raw
    combining-mark and control bytes survive - and exits 0."""
    if os.path.exists(FIXTURE_ABS):
        shutil.rmtree(FIXTURE_ABS, ignore_errors=True)
    os.makedirs(FAKEBIN, exist_ok=True)
    os.makedirs(WORKDIR, exist_ok=True)
    with open(os.path.join(WORKDIR, "a.txt"), "wb") as f:
        f.write(LINE1)
        f.write(LINE2)
        for i in range(3, 31):
            f.write(("pad line %d\n" % i).encode())
    b64 = lambda b: base64.b64encode(b).decode()
    script = """#!/bin/sh
cat <<'EOF'
{"type":"begin","data":{"path":{"text":"./a.txt"}}}
{"type":"match","data":{"path":{"text":"./a.txt"},"lines":{"bytes":"%s"},"line_number":1,"absolute_offset":0,"submatches":[{"match":{"bytes":"%s"},"start":0,"end":2}]}}
{"type":"match","data":{"path":{"text":"./a.txt"},"lines":{"bytes":"%s"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"bytes":"%s"},"start":3,"end":5}]}}
{"type":"end","data":{"path":{"text":"./a.txt"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"stats":{}}}
EOF
exit 0
""" % (b64(LINE1), b64(MARK), b64(LINE2), b64(MARK))
    path = os.path.join(FAKEBIN, "rg")
    with open(path, "w") as f:
        f.write(script)
    os.chmod(path, 0o755)


def row_with(scr, needle):
    """The index of the first row whose text holds needle, or -1."""
    for i, row in enumerate(scr):
        if needle in row_text(row):
            return i
    return -1


def midline_mark_current(scr, inv, und):
    """Line 2's mid-line fallback cell carries the current-match
    underline - the 'n' selection's rendered post-state."""
    r = row_with(scr, "mid-line")
    if r < 0:
        return False
    c = find_fallback_cell(scr, r)
    return c >= 0 and und[r][c]


def scenario_fallback():
    """The issue's manual scenario: view a file whose lines begin with
    and contain standalone combining marks, then n to the mid-line
    mark, then w + . to pan the fallback cell off whole."""
    env = {"PATH": FAKEBIN + os.pathsep + os.environ["PATH"]}
    run = PtyRun([VRG, "mark"], cwd=WORKDIR, env_extra=env)
    try:
        # --- Leg 1: the line-opening mark in wrap mode ---
        scr, inv, und = run.wait_screen(
            "browse with the ◌ fallback cell painted",
            lambda s, i, u: "◌" in screen_text(s)
            and "mid-line" in screen_text(s))
        r1 = row_with(scr, "◌")
        c1 = find_fallback_cell(scr, r1)
        check("line 1 opens with the ◌ fallback cell", c1 >= 0)
        if c1 >= 0:
            check("the fallback is one cell holding ◌+marks",
                  scr[r1][c1] == "◌́",
                  "cell=%r" % scr[r1][c1])
            check("the next cluster paints the next cell - no overlap",
                  scr[r1][c1 + 1] == "x",
                  "next cell=%r" % scr[r1][c1 + 1])
            check("the match highlights exactly the fallback cell",
                  inv[r1][c1] and und[r1][c1],
                  "inv=%r und=%r" % (inv[r1][c1], und[r1][c1]))
            check("nothing adjacent is highlighted",
                  not inv[r1][c1 + 1] and not inv[r1][c1 - 1],
                  "inv left=%r right=%r" % (inv[r1][c1 - 1],
                                            inv[r1][c1 + 1]))

        # --- Leg 2: n to the mid-line mark ---
        run.send(b"n")
        scr, inv, und = run.wait_screen(
            "n selects the mid-line mark's stop",
            midline_mark_current)
        r2 = row_with(scr, "mid-line")
        c2 = find_fallback_cell(scr, r2)
        check("line 2 shows the ◌ fallback mid-line", c2 >= 0)
        if c2 >= 0:
            check("mid-line fallback is one cell holding ◌+marks",
                  scr[r2][c2] == "◌́",
                  "cell=%r" % scr[r2][c2])
            check("the following 'c' paints the next cell",
                  scr[r2][c2 + 1] == "c",
                  "next cell=%r" % scr[r2][c2 + 1])
            check("the mid-line match highlights the fallback cell",
                  inv[r2][c2] and und[r2][c2])
            check("adjacent cells never highlighted",
                  not inv[r2][c2 + 1] and scr[r2][c2 - 1] == "A"
                  and not inv[r2][c2 - 1],
                  "prev=%r inv=%r" % (scr[r2][c2 - 1],
                                      inv[r2][c2 - 1]))
        check("line 1's mark keeps its plain-match style after n",
              r1 >= 0 and c1 >= 0 and inv[r1][c1] and not und[r1][c1])

        # --- Leg 3: w + . - the fallback pans off as one cell ---
        # In wrap mode line 1's 99 cells occupy two rows, so line 2
        # sits on a later row; run-off-edge gives each line one row -
        # line 2's '^A' reaching screen row 2 marks the flat model's
        # install.
        run.send(b"w")
        scr, _, _ = run.wait_screen(
            "run-off-edge layout installed (line 2 reaches row 2)",
            lambda s, i, u: "^A" in row_text(s[2]))
        run.send(b".")
        scr, inv, und = run.wait_screen(
            "one pan step hides the fallback cell whole",
            lambda s, i, u: "◌" not in row_text(s[1])
            and "1* " in row_text(s[1]))
        check("one pan step hides the fallback's single cell",
              "◌" not in row_text(scr[1]),
              "row=%r" % row_text(scr[1]))
        t = row_text(scr[1])
        check("the x run shifted exactly one cell left",
              t[t.index("1* ") + 3] == "x",
              "row=%r" % t[:30])
        check("the hidden-left match stars the gutter",
              "1* " in t)
        # Line 2's mark at cells [4,5) is still visible at off 1:
        # text is hidden left but no match is - '_' not '*'.
        check("line 2 keeps the text-hidden '_' mark (match visible)",
              "2_ " in row_text(scr[2]),
              "row=%r" % row_text(scr[2])[:30])

        # --- Exit ---
        run.send(b"q")
        run.wait_exit()
        run.drain()
        run.finish()
        check("exit 0 (clean results, q)", run.exit_code == 0,
              "exit=%r" % run.exit_code)
        check_terminal_restored("browse", run)
    finally:
        run.finish()
        if not run.exited:
            run.kill()


def main():
    assert_no_fixed_delays()
    log("scenario: standalone combining marks paint the one-cell ◌ "
        "fallback; the match styles exactly that cell; one pan step "
        "hides it whole")
    write_fixtures()
    try:
        scenario_fallback()
    except SmokeError as e:
        check("fallback-cell scenario", False, str(e))
    except Exception as e:
        check("fallback-cell scenario", False,
              "unexpected error: %r" % e)
    if failures:
        log("FAILED: %d check(s)" % len(failures))
        sys.exit(1)
    log("all checks passed")


if __name__ == "__main__":
    main()
