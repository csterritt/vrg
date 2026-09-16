#!/usr/bin/env python3
"""Generate the demo repo and fake rg for the Issue #41 walkthrough.

Creates:
  demo/test.txt   - the file the fake rg reports one match in
  fakerg/rg       - fake ripgrep: 60 numbered stderr rows between
                    FIRST-MARKER and LAST-MARKER (taller than the
                    20-row visible overlay at 80x24), valid stdout
                    JSON records, handshake touch, exit 3
"""
import os, stat

here = os.path.dirname(os.path.abspath(__file__))
demo = os.path.join(here, "demo")
fake = os.path.join(here, "fakerg")
os.makedirs(demo, exist_ok=True)
os.makedirs(fake, exist_ok=True)

with open(os.path.join(demo, "test.txt"), "w") as f:
    f.write("hello\n")

rg = r"""#!/bin/sh
printf 'FIRST-MARKER\n' >&2
i=1
while [ $i -le 58 ]; do
  printf 'diag-row-%03d\n' "$i" >&2
  i=$((i + 1))
done
printf 'LAST-MARKER\n' >&2
echo '{"type":"begin","data":{"path":{"text":"test.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"test.txt"},"lines":{"text":"hello\n"},"line_number":1,"submatches":[{"match":{"text":"hello"},"start":0,"end":5}]}}'
echo '{"type":"end","data":{"path":{"text":"test.txt"},"binary_offset":null}}'
echo '{"type":"summary","data":{}}'
touch "$VRG_TEST_HANDSHAKE"
exit 3
"""
path = os.path.join(fake, "rg")
with open(path, "w") as f:
    f.write(rg)
os.chmod(path, os.stat(path).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

print("created demo/test.txt and fakerg/rg (60 stderr rows, exit 3)")
