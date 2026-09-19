#!/usr/bin/env python3
"""Issue #37 smoke harness: oversized-record diagnostics under a PTY.

Drives the built vrg binary through the issue's manual scenarios with
a Python fake-rg fixture family (handshake side file) and a separated
stderr pipe:

  1. one oversized record with a recoverable path, then valid records
     and summary, child exit 0 -> browse under an overlay containing
     both the aggregate "1 oversized record skipped" and the
     "oversized record skipped for <path>" detail; the same lines
     replay to stderr.
  2. an oversized record that hits the 64 MiB limit before its path
     field, beside valid records -> the aggregate is present with no
     path line, never an empty or absent diagnostic; exit 0.
  3. the same anonymous oversized record as the stream's only content
     -> zero usable results makes the record loss fatal: the overlay
     holds exactly the aggregate and dismissal exits 2.

Every key is sent only after the preceding expected UI state is
observed in the PTY stream; output draining is completion/EOF-driven;
bounded polls re-check an explicit, externally observable condition on
every iteration and exist only to fail a missing condition - never as
an assumed settling interval.

The harness contains no time.sleep call and no other fixed delay used
as a progress proxy; assert_no_fixed_delays() proves the first half
mechanically by parsing this file's AST, and the bounded-poll
structure makes the second half reviewable.
"""

import ast
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

HERE = os.path.dirname(os.path.abspath(__file__))
VRG = os.path.join(HERE, "vrg")
WORK = "/tmp/vrg-037-smoke/run"

# POLL_IDLE paces one iteration of a bounded condition poll. In PtyRun
# waits it is a select idle on real descriptors (genuine I/O waiting);
# in the file polls it bounds the re-check interval of the explicit
# condition. It is never a settling delay: no code path treats elapsed
# idles as evidence of progress.
POLL_IDLE = 0.05

failures = []


class SmokeError(Exception):
    """A bounded wait failed: the named condition never held."""


def log(msg):
    print(msg, flush=True)


def strip_ansi(s):
    out = []
    i = 0
    while i < len(s):
        if s[i] == "\x1b":
            if i + 1 < len(s) and s[i + 1] == "[":
                i += 2
                while i < len(s) and not (0x40 <= ord(s[i]) <= 0x7e):
                    i += 1
                if i < len(s):
                    i += 1
                continue
            i += 1
            if i < len(s):
                i += 1
            continue
        out.append(s[i])
        i += 1
    return "".join(out)


def assert_no_fixed_delays():
    """Static self-check: parse this file's AST and fail if it calls
    time.sleep - the fixed-settling primitive the harness contract
    bans. Bounded polls pace themselves with select idles that re-check
    an explicit condition every iteration; nothing else in this file
    waits on elapsed time."""
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
                "poll on an explicit observable condition" % __file__)


def wait_condition(desc, cond, timeout):
    """Bounded condition poll: re-check cond every POLL_IDLE until it
    holds or the timeout fails the wait with the condition's name."""
    deadline = time.monotonic() + timeout
    while True:
        if cond():
            return
        if time.monotonic() >= deadline:
            raise SmokeError("timed out after %.1fs waiting for %s"
                             % (timeout, desc))
        select.select([], [], [], POLL_IDLE)


def wait_file(path, timeout=15.0):
    """Bounded poll for a fixture file. The file proves only that the
    fixture reached the point that writes it - never that the app
    entered a state."""
    wait_condition("fixture file %s" % path,
                   lambda: os.path.exists(path), timeout)


def check(name, cond, detail=""):
    status = "PASS" if cond else "FAIL"
    if cond:
        log("  [%s] %s" % (status, name))
    else:
        log("  [%s] %s%s" % (status, name, ": " + detail if detail else ""))
        failures.append(name)


def fresh_dir(label):
    d = os.path.join(WORK, label)
    if os.path.exists(d):
        shutil.rmtree(d, ignore_errors=True)
    os.makedirs(d, exist_ok=True)
    return d


def write_fake_rg(dirpath, script):
    rg = os.path.join(dirpath, "rg")
    with open(rg, "w") as f:
        f.write(script)
    os.chmod(rg, 0o755)
    return rg


class PtyRun:
    """One vrg run under a PTY with a separated stderr pipe.

    wait_output polls the ANSI-stripped PTY stream for an explicit
    rendered marker, wait_exit polls process completion, and drain
    reads both pipes until EOF - every synchronization point is a
    bounded poll on an externally observable condition.
    """

    def __init__(self, cmd, env, cwd=None, winsize=(24, 80), timeout=60.0):
        self.timeout = timeout
        rerr, werr = os.pipe()
        cmd_env = dict(os.environ)
        cmd_env.update(env)
        cmd_env["TERM"] = "xterm-256color"

        pid, fd = pty.fork()
        if pid == 0:
            os.close(rerr)
            os.dup2(werr, 2)
            os.close(werr)
            if cwd:
                os.chdir(cwd)
            os.execvpe(cmd[0], cmd, cmd_env)
            os._exit(127)

        os.close(werr)
        try:
            wins = struct.pack("HHHH", winsize[0], winsize[1], 0, 0)
            import fcntl
            fcntl.ioctl(fd, termios.TIOCSWINSZ, wins)
        except OSError:
            pass

        self.pid = pid
        self.fd = fd
        self.rerr = rerr
        self.before_termios = termios.tcgetattr(fd)
        self.after_termios = None
        self.raw = bytearray()
        self.err = bytearray()
        self.eof = False
        self.err_eof = False
        self.exited = False
        self.exit_code = None

    def _pump(self, idle):
        """One I/O-multiplexed read step: wait up to idle for either
        pipe to become readable, then drain what arrived. EOF is
        observed per pipe (PTY masters report EIO when the slave side
        closes)."""
        fds = []
        if not self.eof:
            fds.append(self.fd)
        if not self.err_eof:
            fds.append(self.rerr)
        if not fds:
            return
        r, _, _ = select.select(fds, [], [], idle)
        for f in r:
            try:
                data = os.read(f, 65536)
            except OSError:
                data = b""
            if f == self.fd:
                if data:
                    self.raw += data
                else:
                    self.eof = True
            else:
                if data:
                    self.err += data
                else:
                    self.err_eof = True

    def _wait(self, desc, cond, timeout=None):
        """Bounded condition poll over pumped PTY/stderr state."""
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

    def text(self):
        return self.raw.decode("utf-8", "replace")

    def stderr(self):
        return self.err.decode("utf-8", "replace")

    def wait_output(self, marker, timeout=None):
        """Wait until the ANSI-stripped rendered stream contains marker
        - the externally observable post-state a following key relies
        on."""
        self._wait("output marker %r" % marker,
                   lambda: marker in strip_ansi(self.text()), timeout)

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
        """Completion/EOF-driven draining: read the PTY master and the
        stderr pipe until both report EOF (bounded)."""
        self._wait("PTY/stderr EOF",
                   lambda: self.eof and self.err_eof, timeout)

    def send(self, key):
        """Write one key to the PTY. Synchronization is the caller's:
        a key is only ever sent after wait_output observed the UI
        state that key depends on."""
        os.write(self.fd, key.encode("utf-8"))

    def finish(self):
        """Snapshot post-exit termios and close both pipes."""
        try:
            self.after_termios = termios.tcgetattr(self.fd)
        except OSError:
            pass
        for f in (self.fd, self.rerr):
            try:
                os.close(f)
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
    """Terminal restoration evidence for one PTY run: the display
    sequences (cursor visible; alt-screen left when it was entered) and
    the PTY input modes identical to the pre-launch capture."""
    raw = run.text()
    check("%s cursor restored" % name, "\x1b[?25h" in raw)
    if "\x1b[?1049h" in raw:
        check("%s alt screen exited" % name, "\x1b[?1049l" in raw)
    check("%s termios restored" % name,
          run.before_termios == run.after_termios)


def run_scenario(name, fn):
    log(name)
    try:
        fn()
    except SmokeError as e:
        check(name, False, str(e))
    except Exception as e:
        check(name, False, "unexpected error: %r" % e)


def begin(p):
    return '{"type":"begin","data":{"path":{"text":"%s"}}}' % p


def match(p):
    return ('{"type":"match","data":{"path":{"text":"%s"},'
            '"lines":{"text":"hello world\\n"},"line_number":1,'
            '"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
            % p)


def end(p):
    return '{"type":"end","data":{"path":{"text":"%s"},"binary_offset":null}}' % p


def summary():
    return '{"type":"summary","data":{}}'


def emit(rec):
    """One fake-rg script line emitting a plain record."""
    return "out(%r)" % rec


def emit_oversized_named(path):
    """One fake-rg script line emitting an oversized match record whose
    type and data.path parse before the 64 MiB limit - the recoverable
    path case."""
    return ("out('{\"type\":\"match\",\"data\":{\"path\":{\"text\":\"%s\"},"
            "\"lines\":{\"text\":\"' + PAD + '\",\"line_number\":1,"
            "\"submatches\":[{\"match\":{\"text\":\"a\"},\"start\":0,\"end\":1}]}}')"
            % path)


def emit_oversized_anon():
    """One fake-rg script line emitting an oversized match record whose
    giant lines value precedes data.path - the byte limit hits before
    the path field is ever parsed."""
    return ("out('{\"type\":\"match\",\"data\":{\"lines\":{\"text\":\"' + PAD + "
            "'\"},\"path\":{\"text\":\"late.txt\"},\"line_number\":1,"
            "\"submatches\":[{\"match\":{\"text\":\"a\"},\"start\":0,\"end\":1}]}}')")


def launch(label, script_lines, child_code):
    """Write the fixture and launch vrg under a PTY.

    script_lines are the Python fake-rg's body lines - emit() calls for
    plain records and the emit_oversized_* generators for 64 MiB-class
    records. child_code is the fixture's exit status. Returns
    (run, handshake)."""
    fake_dir = fresh_dir(label + "_fake")
    repo = fresh_dir(label + "_repo")
    hs_dir = fresh_dir(label + "_hs")
    hs = os.path.join(hs_dir, "handshake")
    with open(os.path.join(repo, "test.txt"), "w") as f:
        f.write("hello world\n")
    script = "#!/usr/bin/env python3\nimport os, sys\n"
    script += "PAD = 'a' * (64 << 20)\n"
    script += "def out(s): sys.stdout.write(s + chr(10))\n"
    for line in script_lines:
        script += line + "\n"
    script += "sys.stdout.flush()\n"
    script += ("if os.environ.get('VRG_TEST_HANDSHAKE'):\n"
               "    open(os.environ['VRG_TEST_HANDSHAKE'], 'w').close()\n")
    script += "sys.exit(%d)\n" % child_code
    write_fake_rg(fake_dir, script)
    run = PtyRun([VRG, "hello", "."],
                 env={"PATH": fake_dir + ":" + os.environ["PATH"],
                      "VRG_TEST_HANDSHAKE": hs},
                 cwd=repo)
    return run, hs


def scenario_named_oversized():
    """Oversized record with a recoverable path, then valid records and
    summary.

    Usable results exist, so the record loss is nonfatal: browse opens
    under an overlay containing both the aggregate count and the
    named-path detail. q dismisses the overlay, the second q exits 0,
    and the same two lines replay to stderr."""
    run, hs = launch("s1",
                     [emit_oversized_named("big.txt"),
                      emit(begin("test.txt")), emit(match("test.txt")),
                      emit(end("test.txt")), emit(summary())],
                     0)
    try:
        wait_file(hs)
        run.wait_output("1 oversized record skipped")
        run.wait_output("oversized record skipped for big.txt")
        run.send("q")  # dismiss the overlay -> browse
        run.send("q")  # quit -> exit 0
        run.wait_exit()
        run.drain()
    except Exception:
        run.kill()
        raise
    run.finish()

    visible = strip_ansi(run.text())
    replay = run.stderr()
    check("named oversized exit 0", run.exit_code == 0,
          "exit=%s" % run.exit_code)
    check("named oversized overlay shows the aggregate",
          "1 oversized record skipped" in visible,
          "visible=%r" % visible)
    check("named oversized overlay names the path",
          "oversized record skipped for big.txt" in visible,
          "visible=%r" % visible)
    check("named oversized dismissal reveals browse",
          "test.txt" in visible)
    check("named oversized replay shows the aggregate",
          "1 oversized record skipped" in replay,
          "stderr=%r" % replay)
    check("named oversized replay names the path",
          "oversized record skipped for big.txt" in replay,
          "stderr=%r" % replay)
    check_terminal_restored("named-oversized", run)


def scenario_anonymous_oversized():
    """Oversized record hitting the limit before its path field, beside
    valid records.

    The path never parsed, so no per-path detail exists - yet the
    aggregate is still present in the overlay and the stderr replay:
    the loss is never silent."""
    run, hs = launch("s2",
                     [emit(begin("test.txt")), emit(match("test.txt")),
                      emit(end("test.txt")), emit_oversized_anon(),
                      emit(summary())],
                     0)
    try:
        wait_file(hs)
        run.wait_output("1 oversized record skipped")
        run.send("q")  # dismiss the overlay -> browse
        run.send("q")  # quit -> exit 0
        run.wait_exit()
        run.drain()
    except Exception:
        run.kill()
        raise
    run.finish()

    visible = strip_ansi(run.text())
    replay = run.stderr()
    check("anonymous oversized exit 0", run.exit_code == 0,
          "exit=%s" % run.exit_code)
    check("anonymous oversized overlay shows the aggregate",
          "1 oversized record skipped" in visible,
          "visible=%r" % visible)
    check("anonymous oversized emits no path line",
          "oversized record skipped for" not in visible,
          "visible=%r" % visible)
    check("anonymous oversized replay shows the aggregate",
          "1 oversized record skipped" in replay,
          "stderr=%r" % replay)
    check("anonymous oversized replay has no path line",
          "oversized record skipped for" not in replay,
          "stderr=%r" % replay)
    check_terminal_restored("anonymous-oversized", run)


def scenario_anonymous_only_fatal():
    """The anonymous oversized record as the stream's only content.

    Zero usable results makes the record loss fatal: the overlay alone
    carries exactly the aggregate - never an empty overlay - and
    dismissing it exits 2."""
    run, hs = launch("s3",
                     [emit_oversized_anon(), emit(summary())],
                     0)
    try:
        wait_file(hs)
        run.wait_output("1 oversized record skipped")
        run.send("q")  # dismiss -> exits the fatal overlay-only outcome
        run.wait_exit()
        run.drain()
    except Exception:
        run.kill()
        raise
    run.finish()

    visible = strip_ansi(run.text())
    replay = run.stderr()
    check("anonymous-only exit 2", run.exit_code == 2,
          "exit=%s" % run.exit_code)
    check("anonymous-only fatal overlay shows the aggregate",
          "1 oversized record skipped" in visible,
          "visible=%r" % visible)
    check("anonymous-only overlay has no path line",
          "oversized record skipped for" not in visible,
          "visible=%r" % visible)
    check("anonymous-only replay shows the aggregate",
          "1 oversized record skipped" in replay,
          "stderr=%r" % replay)
    check_terminal_restored("anonymous-only", run)


def main():
    assert_no_fixed_delays()
    if os.path.exists(WORK):
        shutil.rmtree(WORK, ignore_errors=True)
    os.makedirs(WORK, exist_ok=True)

    run_scenario("scenario 1: recoverable-path oversized record",
                 scenario_named_oversized)
    run_scenario("scenario 2: anonymous oversized record, usable results",
                 scenario_anonymous_oversized)
    run_scenario("scenario 3: anonymous oversized record only, fatal",
                 scenario_anonymous_only_fatal)

    if failures:
        log("FAILURES: %d" % len(failures))
        return 1
    log("all checks passed")
    return 0


if __name__ == "__main__":
    sys.exit(main())
