#!/usr/bin/env python3
"""Run vrg under a PTY and measure per-keystroke repaint latency
(Issue #40 walkthrough).

Each key is sent only after the previous frame finished arriving
(serialized round-trips), so the measured latency is the full
Update()+View() turnaround for that key. A key spec of 'n', 'p',
'down', etc. sends a keystroke; 'resize:WxH' issues a TIOCSWINSZ
resize that produces a WindowSizeMsg (terminal resize transition).

Environment variables:
  VRG_KEYS     - comma-separated key/resize spec (default: n,n,n,q)
  VRG_WIDTH    - initial terminal width (default: 80)
  VRG_HEIGHT   - initial terminal height (default: 24)
  VRG_READY    - seconds to wait for the browse screen (default: 120)

Prints one line per key with its repaint latency and a summary with
min/median/max. The final line is the vrg exit status.
"""
import os, pty, sys, time, select, signal, struct, fcntl, termios
import pyte

width = int(os.environ.get("VRG_WIDTH", "80"))
height = int(os.environ.get("VRG_HEIGHT", "24"))
ready_timeout = float(os.environ.get("VRG_READY", "120"))
keys = os.environ.get("VRG_KEYS", "n,n,n,q").split(",")

screen = pyte.Screen(width, height)
stream = pyte.ByteStream(screen)

pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])

winsize = struct.pack("HHHH", height, width, 0, 0)
fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)


def drain(timeout):
    """Read until output is quiet for `timeout` seconds; returns bytes."""
    out = b""
    while True:
        try:
            r, _, _ = select.select([fd], [], [], timeout)
            if r:
                data = os.read(fd, 65536)
                if not data:
                    break
                out += data
            else:
                break
        except OSError:
            break
    return out


def wait_browse():
    """Wait until the browse view (filename rule) is on screen."""
    t0 = time.monotonic()
    while time.monotonic() - t0 < ready_timeout:
        stream.feed(drain(0.5))
        if any("\u2500\u2500" in row for row in screen.display):
            return time.monotonic() - t0
    return -1


def roundtrip(spec):
    """Send one key or resize; return repaint latency in ms."""
    t0 = time.monotonic()
    if spec.startswith("resize:"):
        w, h = spec[7:].split("x")
        fcntl.ioctl(fd, termios.TIOCSWINSZ,
                    struct.pack("HHHH", int(h), int(w), 0, 0))
    else:
        os.write(fd, spec.encode())
    # Wait for the first output byte = the frame started rendering.
    try:
        r, _, _ = select.select([fd], [], [], 5.0)
        if not r:
            return -1.0
    except OSError:
        return -1.0
    first = time.monotonic() - t0
    stream.feed(os.read(fd, 65536))
    stream.feed(drain(0.25))
    return first * 1000.0


ready = wait_browse()
print(f"browse ready after {ready:.1f}s")
if ready < 0:
    os.kill(pid, signal.SIGTERM)
    os.waitpid(pid, 0)
    print("exit=NOT-READY")
    sys.exit(1)

lat = []
for spec in keys:
    if spec == "q":
        os.write(fd, b"q")
        break
    ms = roundtrip(spec)
    lat.append(ms)
    print(f"key {spec:>10s} repaint {ms:6.1f}ms")

if lat:
    s = sorted(lat)
    med = s[len(s) // 2]
    print(f"summary: {len(lat)} transitions, "
          f"min {s[0]:.1f}ms, median {med:.1f}ms, max {s[-1]:.1f}ms")
    print(f"verdict: {'PASS' if s[-1] < 400 else 'FAIL'} "
          f"(every repaint under 400ms)")

drain(1.0)
finished, status = os.waitpid(pid, os.WNOHANG)
if finished == 0:
    os.kill(pid, signal.SIGTERM)
    _, status = os.waitpid(pid, 0)
print(f"exit={os.waitstatus_to_exitcode(status)}")
