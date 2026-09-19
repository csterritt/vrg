#!/usr/bin/env python3
"""Issue #40 smoke harness: bounded per-keystroke render cost at scale.

Drives the built vrg binary on a real PTY through the issue's manual
scenario: a large result set - 2000 files x 50 matched lines = 100,000
matched lines - browsed by holding `n` (a burst written at once, faster
than any autorepeat), holding `down`, and resizing repeatedly. Each
burst's wall-clock settle time is measured against the rendered frame
that proves the last keystroke landed, giving an honest per-keystroke
cost. The same burst is then replayed against a small index (8 files,
40 matched lines): if per-frame work scaled with index size - the old
whole-index scan this issue removes - the large-index per-key cost
would dwarf the small-index one. The guards assert both an absolute
per-key bound and a large/small ratio bound.

Layout at 80x24: the displayed paths are absolute (~88 cells), so the
file list sits at the floor(0.40 x 80) = 32 cap:

    [list 32][separator 1][gutter 3][text 43][reserved 1] = 80

Every key is sent only after the preceding expected frame is observed
in the emulated screen; each wait is a bounded poll on an explicit
rendered condition - never a fixed settling delay. assert_no_fixed_
delays() proves the first half mechanically by parsing this file's
AST; the bounded-poll structure makes the second half reviewable.

The harness also plays terminal honestly: it clears the SSH/session
environment variables that suppress bubbletea's DECRQM probes and
answers the mode-2027 (Unicode core) query with DECRPM 'set'.
(Emulator and PTY plumbing adapted from the Issue #39 harness.)
"""

import ast
import fcntl
import os
import pty
import select
import shutil
import signal
import struct
import subprocess
import sys
import termios
import time
import unicodedata

HERE = os.path.dirname(os.path.abspath(__file__))
VRG = os.path.join(HERE, "vrg")
FIXTURE = "fixture"
FIXTURE_ABS = os.path.join(HERE, FIXTURE)

# Large index: 2000 files x 50 matched lines = 100,000 matched lines.
BIG_SUB = "many"
BIG_FILES, BIG_LINES = 2000, 50
# Small index: 8 files x 5 matched lines = 40 matched lines.
SMALL_SUB = "few"
SMALL_FILES, SMALL_LINES = 8, 5
PATTERN = "hit"

COLS, ROWS = 80, 24
# Absolute paths (~88 cells) push the list onto the floor(0.40 x 80)
# cap: list [0,32), separator 32, gutter [33,36), text [36,80).
LISTW, SEP, GUT0, TXT0 = 32, 32, 33, 36

# POLL_IDLE paces one iteration of a bounded condition poll. In the
# waits it is a select idle on the real PTY descriptor (genuine I/O
# waiting); it never stands in as an assumed settling interval.
POLL_IDLE = 0.05

# Responsiveness guards. Per-key cost that scanned the whole index per
# frame would exceed these by orders of magnitude at 100k stops; the
# ratio bound catches subtler index-size growth that a generous
# absolute bound alone could miss.
MS_PER_KEY_LIMIT = 250.0
BIG_SMALL_RATIO_LIMIT = 10.0

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
            del grid[bot + 1:]
            del inv[bot + 1:]
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
        elif final == "L":  # insert blank lines
            for _ in range(arg(0, 1)):
                grid.insert(r, [" "] * cols)
                inv.insert(r, [False] * cols)
                del grid[rows:]
                del inv[rows:]
        elif final == "M":  # delete lines
            for _ in range(arg(0, 1)):
                del grid[r]
                del inv[r]
                grid.append([" "] * cols)
                inv.append([False] * cols)
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


class PtyRun:
    """One vrg run under a PTY.

    wait_screen polls the emulated grid for an explicit rendered
    predicate, wait_exit polls process completion, and drain reads the
    PTY until EOF - every synchronization point is a bounded poll on
    an externally observable condition.
    """

    def __init__(self, cmd, cwd=None, winsize=(ROWS, COLS), timeout=90.0):
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

    def timed_burst(self, desc, key_bytes, count, pred):
        """Write count copies of key_bytes at once - faster than any
        held-key autorepeat - then wait for the rendered condition
        that proves the last keystroke landed. Returns elapsed ms."""
        t0 = time.monotonic()
        self.send(key_bytes * count)
        self.wait_screen(desc, pred)
        return (time.monotonic() - t0) * 1000.0

    def resize(self, cols, rows):
        """Resize the PTY - the kernel delivers SIGWINCH to the child,
        the path a real terminal resize takes. The replay buffer is
        reset so the emulator reconstructs the post-resize repaint at
        the new geometry."""
        fcntl.ioctl(self.fd, termios.TIOCSWINSZ,
                    struct.pack("HHHH", rows, cols, 0, 0))
        self.cols, self.rows = cols, rows
        self.raw = bytearray()

    def timed_resize(self, desc, cols, rows, pred):
        """Resize, then wait for the rendered condition at the new
        geometry. Returns elapsed ms."""
        t0 = time.monotonic()
        self.resize(cols, rows)
        self.wait_screen(desc, pred)
        return (time.monotonic() - t0) * 1000.0

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


def write_fixtures():
    """many/: BIG_FILES files of BIG_LINES lines, every line matching
    the pattern - 100,000 matched lines across 2000 files. few/: a
    small 8-file/40-match index for the per-key cost comparison."""
    for sub, count, lines in ((BIG_SUB, BIG_FILES, BIG_LINES),
                              (SMALL_SUB, SMALL_FILES, SMALL_LINES)):
        d = os.path.join(FIXTURE_ABS, sub)
        if os.path.exists(d):
            shutil.rmtree(d, ignore_errors=True)
        os.makedirs(d, exist_ok=True)
        for i in range(count):
            name = "f%05d.txt" % i if sub == BIG_SUB else "s%04d.txt" % i
            body = "".join("%s line %04d hit\n" % (name, k)
                           for k in range(1, lines + 1))
            with open(os.path.join(d, name), "w") as f:
                f.write(body)


def path_in_rule(name):
    """The filename rule (row 0) shows the current file's escaped
    path left-truncated around the rule - the basename is the tail."""
    return lambda s: name in row_text(s[0])


def scenario_large_index():
    """The issue's manual scenario on the 100,000-stop index."""
    run = PtyRun([VRG, PATTERN, os.path.join(FIXTURE, BIG_SUB)],
                 cwd=HERE)
    try:
        scr, _ = run.wait_screen(
            "initial browse frame",
            lambda s: "f00000.txt" in row_text(s[0])
            and row_text(s[1][GUT0:TXT0]).strip() == "1")
        check("initial frame: file 0 current, list capped at 32",
              scr[1][SEP] == " "
              and row_text(scr[1][GUT0:TXT0]).strip() == "1",
              "sep=%r gutter=%r"
              % (scr[1][SEP], row_text(scr[1][GUT0:TXT0])))
        check("file list shows only the visible window's entries",
              "f00000.txt" in row_text(scr[1][:LISTW])
              and "f00022.txt" in row_text(scr[ROWS - 1][:LISTW])
              and not any("f00023.txt" in row_text(row)
                          for row in scr),
              "last=%r" % row_text(scr[ROWS - 1][:LISTW]))

        # Held-n burst 1: 200 stops land on f00004 (50 stops/file).
        ms = run.timed_burst("n burst to f00004", b"n", 200,
                             path_in_rule("f00004.txt"))
        per_key = ms / 200
        log("  n-burst 1: 200 keys over a 100,000-stop index settled "
            "in %.0fms (%.1fms/key)" % (ms, per_key))
        check("held-n burst per-key cost bounded at 100k stops",
              per_key < MS_PER_KEY_LIMIT,
              "%.1fms/key" % per_key)
        n_key_ms = per_key

        # Held-n burst 2: another 200 stops land on f00008.
        ms = run.timed_burst("n burst to f00008", b"n", 200,
                             path_in_rule("f00008.txt"))
        per_key = ms / 200
        log("  n-burst 2: 200 more keys settled in %.0fms "
            "(%.1fms/key)" % (ms, per_key))
        check("second held-n burst equally bounded",
              per_key < MS_PER_KEY_LIMIT,
              "%.1fms/key" % per_key)
        n_key_ms = (n_key_ms + per_key) / 2

        # Held-down burst: 40 downs clamp the 50-line file's viewport
        # at the bottom - the last content row shows line 50.
        ms = run.timed_burst(
            "down burst to EOF", b"\x1b[B", 40,
            lambda s: "50" in row_text(s[ROWS - 1][GUT0:TXT0]))
        per_key = ms / 40
        log("  down-burst: 40 keys settled in %.0fms (%.1fms/key)"
            % (ms, per_key))
        check("held-down burst per-key cost bounded",
              per_key < MS_PER_KEY_LIMIT,
              "%.1fms/key" % per_key)

        # Repeated resizes: each mints a fresh keyed layout and
        # re-truncates the visible list entries against the new width.
        # The filename rule repaints at every geometry.
        sizes = [(100, 30), (60, 20), (80, 24)]
        total_ms = 0.0
        for cols, rows in sizes:
            ms = run.timed_resize("resize to %dx%d" % (cols, rows),
                                  cols, rows,
                                  path_in_rule("f00008.txt"))
            total_ms += ms
            log("  resize to %dx%d settled in %.0fms" % (cols, rows, ms))
        check("repeated resizes settle bounded",
              total_ms / len(sizes) < 2000.0,
              "%.0fms/resize" % (total_ms / len(sizes)))

        # Back at 80x24 the list is again capped at 32 and the
        # current file still current.
        scr, _ = run.wait_screen("post-resize frame",
                                 path_in_rule("f00008.txt"))
        check("post-resize frame intact at 80x24",
              scr[1][SEP] == " " and "f00008.txt" in row_text(scr[0]))

        run.send(b"q")
        run.wait_exit()
        run.drain()
        run.finish()
        check("exit 0", run.exit_code == 0,
              "exit=%r" % run.exit_code)
        check_terminal_restored("browse", run)
        return n_key_ms
    finally:
        run.finish()
        if not run.exited:
            run.kill()


def scenario_small_index():
    """The same n-burst on a 40-stop index - the per-key comparison."""
    run = PtyRun([VRG, PATTERN, os.path.join(FIXTURE, SMALL_SUB)],
                 cwd=HERE)
    try:
        run.wait_screen("small initial frame", path_in_rule("s0000.txt"))
        # 15 stops land on s0003 (5 stops/file).
        ms = run.timed_burst("n burst to s0003", b"n", 15,
                             path_in_rule("s0003.txt"))
        per_key = ms / 15
        log("  small-index n-burst: 15 keys settled in %.0fms "
            "(%.1fms/key)" % (ms, per_key))
        run.send(b"q")
        run.wait_exit()
        run.drain()
        run.finish()
        check("small run exit 0", run.exit_code == 0,
              "exit=%r" % run.exit_code)
        return per_key
    finally:
        run.finish()
        if not run.exited:
            run.kill()


def main():
    assert_no_fixed_delays()
    log("scenario: bounded per-keystroke render cost at ~100,000 "
        "matched lines")
    write_fixtures()
    big_ms = small_ms = None
    try:
        big_ms = scenario_large_index()
        small_ms = scenario_small_index()
    except SmokeError as e:
        check("bounded-render scenario", False, str(e))
    except Exception as e:
        check("bounded-render scenario", False,
              "unexpected error: %r" % e)
    if big_ms is not None and small_ms is not None:
        # The scale-independence claim: per-key cost over 100,000
        # stops stays within a small factor of the same burst over 40
        # stops. An index-proportional per-frame cost would put the
        # ratio in the hundreds, not single digits. A floor on the
        # denominator keeps timer noise from dominating the ratio.
        ratio = big_ms / max(small_ms, 5.0)
        log("  per-key cost: %.1fms (100k stops) vs %.1fms (40 stops)"
            " - ratio %.1fx" % (big_ms, small_ms, ratio))
        check("per-key cost does not grow with index size",
              ratio < BIG_SMALL_RATIO_LIMIT,
              "ratio=%.1f" % ratio)
    if failures:
        log("FAILED: %d check(s)" % len(failures))
        sys.exit(1)
    log("all checks passed")


if __name__ == "__main__":
    main()
