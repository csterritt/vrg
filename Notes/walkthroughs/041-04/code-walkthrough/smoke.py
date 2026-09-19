#!/usr/bin/env python3
"""Issue #41 smoke harness: every wrapped overlay row is scrollable.

Drives the built vrg binary on a real 80x24 PTY through the issue's
manual scenario: a fake rg emits a valid one-match stream, writes 40
numbered stderr lines - longer than the overlay's 22-row visible
window - and exits 3, so the fatal outcome opens the error overlay
over the browse view. The harness then:

- confirms u, d, page up, and page down are ignored while the modal
  overlay is open (the overlay's window never moves, and the file
  viewport beneath provably stays at line 1 after dismissal);
- scrolls down one row at a time from the first row to the last,
  waiting each step for the scrolled-off row to leave the frame and
  the new top row to enter it - so every middle row is witnessed as a
  window top, with no ellipsis substituting for content at any step;
- scrolls back up to the first row the same way, and confirms the
  scroll clamps at both ends;
- dismisses to the browse frame and quits with the fatal status 2.

Every key is sent only after an explicit rendered condition is
observed in the emulated screen; each wait is a bounded poll - never
a fixed settling delay. assert_no_fixed_delays() proves the first
half mechanically by parsing this file's AST; the bounded-poll
structure makes the second half reviewable.

The harness also plays terminal honestly: it clears the SSH/session
environment variables that suppress bubbletea's DECRQM probes and
answers the mode-2027 (Unicode core) query with DECRPM 'set'.
(Emulator and PTY plumbing adapted from the Issue #39/#40 harnesses.)
"""

import ast
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
FIXTURE = "fixture"
FIXTURE_ABS = os.path.join(HERE, FIXTURE)
FAKEBIN = os.path.join(FIXTURE_ABS, "fakebin")
WORKDIR = os.path.join(FIXTURE_ABS, "work")

COLS, ROWS = 80, 24
# 40 numbered stderr lines: rows 0..39 of the overlay's complete
# wrapped set. The visible window is ROWS - 2 = 22 interior rows, so
# the scroll range is 18 - a diagnostic only modestly taller than the
# box, where a row-by-row traversal is bounded.
DIAGS = 40
VIS = ROWS - 2
MAXSCROLL = DIAGS - VIS
ELLIPSIS = "…"

# The file-panel geometry at 80x24: absolute fixture paths (~90 cells)
# push the list onto the floor(0.40 x 80) cap - list [0,32), separator
# 32, gutter [33,36), text [36,80). The overlay's box is the full
# frame: border row 0, interior rows 1..22, border row 23, interior
# text at columns 2..77.
LISTW, SEP, GUT0, TXT0 = 32, 32, 33, 36
TOP_ROW, BOT_ROW, INTERIOR = 1, ROWS - 2, slice(2, COLS - 2)

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
    """Replay the byte stream into a ROWSxCOLS text grid plus a
    parallel inverse-style grid.

    Handles CUP/CUx cursor motion, EL/ED erases, IL/DL, deferred
    autowrap, and SGR attributes: cells painted while the dark
    scheme's inverse pair '30;47' is active mark True on the second
    grid. Zero-width codepoints - combining marks, ZWJ, variation
    selectors, and any codepoint a pending ZWJ joins - stack onto the
    last painted cell. (Carried over from the Issue #39 harness.)
    """
    grid = [[" "] * cols for _ in range(rows)]
    inv = [[False] * cols for _ in range(rows)]
    r = c = 0
    lastc = -1
    zwj = False
    wrap = False
    cur_inv = False
    top, bot = 0, rows - 1

    def ri():
        """ESC M - reverse index: scroll the region down at its top."""
        nonlocal r
        if r == top:
            grid.insert(top, [" "] * cols)
            inv.insert(top, [False] * cols)
            del grid[bot + 1]
            del inv[bot + 1]
        else:
            r = max(0, r - 1)

    def csi(r, c, params, final):
        nonlocal cur_inv, top, bot
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
            elif mode == 0:
                grid[r][c:] = [" "] * (cols - c)
                inv[r][c:] = [False] * (cols - c)
                for i in range(r + 1, rows):
                    grid[i][:] = [" "] * cols
                    inv[i][:] = [False] * cols
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                inv[r][:c + 1] = [False] * (c + 1)
                for i in range(r):
                    grid[i][:] = [" "] * cols
                    inv[i][:] = [False] * cols
        elif final == "K":
            mode = arg(0, 0)
            if mode == 0:
                grid[r][c:] = [" "] * (cols - c)
                inv[r][c:] = [False] * (cols - c)
            elif mode == 1:
                grid[r][:c + 1] = [" "] * (c + 1)
                inv[r][:c + 1] = [False] * (c + 1)
            elif mode == 2:
                grid[r][:] = [" "] * cols
                inv[r][:] = [False] * cols
        elif final == "L":  # insert blank lines inside the region
            for _ in range(arg(0, 1)):
                del grid[bot]
                del inv[bot]
                grid.insert(r, [" "] * cols)
                inv.insert(r, [False] * cols)
        elif final == "M":  # delete lines inside the region
            for _ in range(arg(0, 1)):
                del grid[r]
                del inv[r]
                grid.insert(bot, [" "] * cols)
                inv.insert(bot, [False] * cols)
        elif final == "S":  # SU: scroll the region up n lines
            for _ in range(arg(0, 1)):
                del grid[top]
                del inv[top]
                grid.insert(bot, [" "] * cols)
                inv.insert(bot, [False] * cols)
        elif final == "T":  # SD: scroll the region down n lines
            for _ in range(arg(0, 1)):
                del grid[bot]
                del inv[bot]
                grid.insert(top, [" "] * cols)
                inv.insert(top, [False] * cols)
        elif final == "X":  # erase chars from the cursor rightward
            n = arg(0, 1)
            grid[r][c:c + n] = [" "] * n
            inv[r][c:c + n] = [False] * n
        elif final == "P":  # delete chars
            n = arg(0, 1)
            del grid[r][c:c + n]
            del inv[r][c:c + n]
            grid[r].extend([" "] * n)
            inv[r].extend([False] * n)
        elif final == "@":  # insert blank chars
            n = arg(0, 1)
            grid[r][c:c] = [" "] * n
            inv[r][c:c] = [False] * n
            del grid[r][cols:]
            del inv[r][cols:]
        elif final == "r":  # DECSTBM: set the scroll region
            top, bot = arg(0, 1) - 1, arg(1, rows) - 1
        elif final == "m":  # SGR: the dark scheme's inverse pair
            cur_inv = "30;47" in params
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
                grid.insert(bot, [" "] * cols)
                inv.insert(bot, [False] * cols)
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
                if w == 2 and c + 1 < cols:
                    grid[r][c + 1] = ""
                    inv[r][c + 1] = cur_inv
            lastc = c
            c += w
            if c >= cols:
                c, wrap = cols - 1, True
        i += size
    return grid, inv


def row_text(row):
    """Join a cell row for substring searches."""
    return "".join(row)


def screen_text(scr):
    """The whole emulated frame as one string."""
    return "\n".join(row_text(row) for row in scr)


def interior_text(scr):
    """Only the overlay's interior columns across the frame - where
    wrapped diagnostic rows live, never the border."""
    return "\n".join(row_text(row)[INTERIOR] for row in scr)


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
        externally observable post-state a following key relies on."""
        result = []

        def probe():
            scr, inv = self.screen()
            if pred(scr):
                result.append((scr, inv))
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


def marker(i):
    return "diag-%02d" % i


def top_is(i):
    """The overlay window's first interior row shows row i's marker."""
    return lambda s: marker(i) in row_text(s[TOP_ROW])


def window_is(i):
    """The overlay window shows the complete scroll-i row set: marker
    i tops the window and marker i+VIS-1 bottoms it - a settled frame,
    not a mid-repaint transient."""
    return lambda s: (marker(i) in row_text(s[TOP_ROW])
                      and marker(i + VIS - 1) in row_text(s[BOT_ROW]))


def frame_is(expected):
    """The whole emulated frame equals an earlier complete capture -
    the settled repaint, never a transient."""
    return lambda s: screen_text(s) == expected


def write_fixtures():
    """fixture/work/f: a real 20-line file the browse panel repaints
    after dismissal. fixture/fakebin/rg: the fake child - one valid
    match stream on stdout, DIAGS numbered lines on stderr, exit 3."""
    if os.path.exists(FIXTURE_ABS):
        shutil.rmtree(FIXTURE_ABS, ignore_errors=True)
    os.makedirs(FAKEBIN, exist_ok=True)
    os.makedirs(WORKDIR, exist_ok=True)
    with open(os.path.join(WORKDIR, "f"), "w") as f:
        for i in range(1, 21):
            f.write("alpha line %d\n" % i)
    script = """#!/bin/sh
cat <<'EOF'
{"type":"begin","data":{"path":{"text":"./f"}}}
{"type":"match","data":{"path":{"text":"./f"},"lines":{"text":"alpha line 2\\n"},"line_number":2,"absolute_offset":0,"submatches":[{"match":{"text":"alpha"},"start":0,"end":5}]}}
{"type":"end","data":{"path":{"text":"./f"},"binary_offset":null,"stats":{}}}
{"type":"summary","data":{"stats":{}}}
EOF
i=0
while [ $i -lt %d ]; do
    printf 'diag-%%02d\\n' "$i" >&2
    i=$((i + 1))
done
exit 3
""" % DIAGS
    path = os.path.join(FAKEBIN, "rg")
    with open(path, "w") as f:
        f.write(script)
    os.chmod(path, 0o755)


def scenario_full_scroll():
    """The issue's manual scenario: scroll the complete diagnostic."""
    env = {"PATH": FAKEBIN + os.pathsep + os.environ["PATH"]}
    run = PtyRun([VRG, "alpha"], cwd=WORKDIR, env_extra=env)
    try:
        # The fatal outcome opens the error overlay over browse, head
        # of the diagnostic at the window's top row, tail off-screen.
        scr, _ = run.wait_screen("error overlay over browse",
                                 top_is(0))
        check("overlay opens at the first row; tail off-screen",
              marker(0) in row_text(scr[TOP_ROW])
              and marker(DIAGS - 1) not in screen_text(scr),
              "top=%r" % row_text(scr[TOP_ROW]))
        check("no ellipsis substitutes for content at scroll 0",
              ELLIPSIS not in interior_text(scr))

        # The modal contract: u, d, page up, page down are base
        # file-content bindings - ignored while the overlay is open.
        # A down/up round-trip afterwards flushes the input queue and
        # lands back on the identical frame, proving none of them
        # scrolled the window or the file beneath.
        before = screen_text(scr)
        for key in (b"u", b"d", b"\x1b[5~", b"\x1b[6~"):
            run.send(key)
        run.send(b"\x1b[B")
        run.wait_screen("post-ignored down", top_is(1))
        run.send(b"\x1b[A")
        scr, _ = run.wait_screen("overlay unmoved after ignored keys",
                                 frame_is(before))
        check("u/d/pgup/pgdn ignored: overlay window unmoved",
              screen_text(scr) == before,
              "top=%r" % row_text(scr[TOP_ROW]))

        # Bounded row-by-row down traversal: each step moves the
        # window exactly one row - the scrolled-off row leaves the
        # frame, the new top row enters it - so every middle row is
        # witnessed as a window top, no ellipsis ever standing in.
        elided = []
        for s in range(1, MAXSCROLL + 1):
            run.send(b"\x1b[B")
            scr, _ = run.wait_screen("scrolled to row %d" % s,
                                     window_is(s))
            if marker(s - 1) in screen_text(scr):
                raise SmokeError(
                    "row %d failed to scroll off at scroll %d"
                    % (s - 1, s))
            if ELLIPSIS in interior_text(scr):
                elided.append(s)
        check("down traversal reached the final row one step at a time",
              marker(DIAGS - 1) in row_text(scr[BOT_ROW]),
              "bottom row=%r" % row_text(scr[BOT_ROW]))
        check("every middle row witnessed; head scrolled off",
              marker(0) not in screen_text(scr))
        check("no ellipsis row at any traversal step", not elided,
              "steps %r" % elided)

        # Clamp at the bottom: an extra down is a no-op - the
        # following up lands exactly one row higher (had the extra
        # down moved the window, the up would land back on the same
        # top row and this wait would time out).
        before = screen_text(scr)
        run.send(b"\x1b[B")
        run.send(b"\x1b[A")
        run.wait_screen("extra down was a clamp",
                        window_is(MAXSCROLL - 1))
        run.send(b"\x1b[B")
        scr, _ = run.wait_screen("back at the bottom",
                                 frame_is(before))
        check("down clamps at the bottom of the complete set",
              screen_text(scr) == before)

        # Bounded row-by-row up traversal back to the first row:
        # each step's bottom row scrolled off the window.
        for s in range(MAXSCROLL - 1, -1, -1):
            run.send(b"\x1b[A")
            scr, _ = run.wait_screen("scrolled back to row %d" % s,
                                     window_is(s))
            if marker(s + VIS) in screen_text(scr):
                raise SmokeError(
                    "row %d still rendered below the window at "
                    "scroll %d" % (s + VIS, s))
        check("up traversal returned to the first row",
              marker(0) in row_text(scr[TOP_ROW]))

        # Clamp at the top: an extra up is a no-op - the following
        # down lands exactly one row lower by the same argument.
        before = screen_text(scr)
        run.send(b"\x1b[A")
        run.send(b"\x1b[B")
        run.wait_screen("extra up was a clamp", window_is(1))
        run.send(b"\x1b[A")
        scr, _ = run.wait_screen("back at the top", frame_is(before))
        check("up clamps at the top of the complete set",
              screen_text(scr) == before)

        # Esc dismisses to the browse frame; the file viewport proves
        # the swallowed u/d/page keys never scrolled beneath - line 1
        # still tops the panel.
        run.send(b"\x1b")
        scr, _ = run.wait_screen(
            "browse frame after dismissal",
            lambda s: "alpha line" in screen_text(s)
            and marker(0) not in screen_text(s))
        check("dismissal reveals the browse frame",
              "alpha line" in screen_text(scr))
        check("ignored keys never scrolled the file viewport beneath",
              row_text(scr[TOP_ROW][GUT0:TXT0]).strip() == "1",
              "gutter=%r" % row_text(scr[TOP_ROW][GUT0:TXT0]))

        # q exits with the fatal outcome's fixed status 2.
        run.send(b"q")
        run.wait_exit()
        run.drain()
        run.finish()
        check("exit 2 (fatal outcome)", run.exit_code == 2,
              "exit=%r" % run.exit_code)
        check_terminal_restored("browse", run)
    finally:
        run.finish()
        if not run.exited:
            run.kill()


def main():
    assert_no_fixed_delays()
    log("scenario: fatal overlay scrolls the complete wrapped "
        "diagnostic (40 rows, 22-row window)")
    write_fixtures()
    try:
        scenario_full_scroll()
    except SmokeError as e:
        check("full-scroll scenario", False, str(e))
    except Exception as e:
        check("full-scroll scenario", False,
              "unexpected error: %r" % e)
    if failures:
        log("FAILED: %d check(s)" % len(failures))
        sys.exit(1)
    log("all checks passed")


if __name__ == "__main__":
    main()
