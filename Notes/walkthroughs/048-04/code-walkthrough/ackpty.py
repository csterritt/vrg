#!/usr/bin/env python3
"""Run vrg under a PTY synchronized on the Issue #48 VRG_TEST_UPDATE_ACK
acknowledgement log — the same contract the Go PTY helpers use.

Environment variables:
  VRG_ACK_FILE    - required: the update-acknowledgement log path
  VRG_KEYS        - comma-separated keys to send (default: q)
  VRG_WIDTH       - terminal width (default: 100)
  VRG_HEIGHT      - terminal height (default: 30)
  VRG_TIMEOUT     - per-wait timeout in seconds (default: 20)

For each key the script polls the acknowledgement log until a
`msg=key key=<label>` record with a sequence number greater than the
last consumed one appears — per-occurrence correlation, never elapsed
time. Before the first key it waits for `msg=search-complete`.
"""
import os, pty, re, sys, time, struct, fcntl, termios

ack_file = os.environ["VRG_ACK_FILE"]
keys = os.environ.get("VRG_KEYS", "q").split(",")
width = int(os.environ.get("VRG_WIDTH", "100"))
height = int(os.environ.get("VRG_HEIGHT", "30"))
timeout = float(os.environ.get("VRG_TIMEOUT", "20"))

ESC = "\x1b"
LABELS = {ESC: "esc", "\x03": "ctrl+c", ESC + "[A": "up",
          ESC + "[B": "down", ESC + "[C": "right", ESC + "[D": "left",
          "\t": "tab", "\r": "enter"}


def label(key):
    if key in LABELS:
        return LABELS[key]
    if len(key) == 1 and 0x20 <= ord(key) < 0x7F:
        return key
    raise SystemExit(f"ackpty: no ack label for key {key!r}")


def read_events():
    try:
        with open(ack_file) as f:
            text = f.read()
    except OSError:
        return []
    events = []
    for line in text.splitlines():
        m = re.match(r"(\d+) msg=(\S+) key=(\S*) state=(\S+) overlay=(\S+) dismissed=(\S+)", line)
        if m:
            events.append((int(m.group(1)), m.group(2), m.group(3)))
    return events


def wait_for(pred, desc):
    """Poll the ack log for an event after `wait_for.last` matching pred."""
    deadline = time.time() + timeout
    while True:
        for ev in read_events():
            if ev[0] > wait_for.last and pred(ev):
                wait_for.last = ev[0]
                return ev
        if time.time() > deadline:
            raise SystemExit(f"ackpty: timed out after {timeout}s waiting for {desc}")
        time.sleep(0.01)


wait_for.last = 0

pid, fd = pty.fork()
if pid == 0:
    os.execvp(sys.argv[1], sys.argv[1:])

winsize = struct.pack("HHHH", height, width, 0, 0)
fcntl.ioctl(fd, termios.TIOCSWINSZ, winsize)

# Drain PTY output in the background so the child never blocks on a
# full PTY buffer.
import threading


def drain():
    try:
        while os.read(fd, 4096):
            pass
    except OSError:
        pass


threading.Thread(target=drain, daemon=True).start()

# Matrix row: search-complete acknowledgement before the first key.
ev = wait_for(lambda e: e[1] == "search-complete", "msg=search-complete")
print(f"ack: seq={ev[0]} msg=search-complete -> first key unlocked",
      flush=True)

for key in keys:
    os.write(fd, key.encode())
    want = label(key)
    ev = wait_for(lambda e: e[1] == "key" and e[2] == want,
                  f"msg=key key={want}")
    print(f"ack: seq={ev[0]} msg=key key={want} -> key processed",
          flush=True)

# Bounded exit wait — same contract as the Go helpers' bounded waits.
deadline = time.time() + timeout
while True:
    wpid, status = os.waitpid(pid, os.WNOHANG)
    if wpid == pid:
        print(f"exit={os.waitstatus_to_exitcode(status)}", flush=True)
        break
    if time.time() > deadline:
        os.kill(pid, 9)
        os.waitpid(pid, 0)
        raise SystemExit(f"ackpty: vrg did not exit within {timeout}s")
    time.sleep(0.01)
