#!/usr/bin/env python3
"""Generate the demo repo and fake rg for the Issue #43 walkthrough.

Creates:
  demo/combining.txt - a regular file whose line 1 begins with a
                       standalone combining acute (CC 81) followed by
                       'x'; line 2 carries a mid-line standalone mark.
                       Under Issue #43 the filebuffer prepends U+25CC
                       (E2 97 8C) to each standalone zero-width cluster
                       so it occupies one real display cell.
  fakerg/rg          - fake ripgrep: emits valid stdout JSON records
                       for combining.txt (one match on line 1 whose
                       submatch covers exactly the standalone mark,
                       start=0 end=2), touches the handshake, exits 0
"""
import os, stat

here = os.path.dirname(os.path.abspath(__file__))
demo = os.path.join(here, "demo")
fake = os.path.join(here, "fakerg")
os.makedirs(demo, exist_ok=True)
os.makedirs(fake, exist_ok=True)

content = ("́x standalone mark leads this line and continues past the panel width\n"
           "mid ́ mark inside a line\n"
           "plain tail line\n")
path = os.path.join(demo, "combining.txt")
with open(path, "w", encoding="utf-8") as f:
    f.write(content)

rg = r"""#!/bin/sh
echo '{"type":"begin","data":{"path":{"text":"combining.txt"}}}'
printf '%s\n' '{"type":"match","data":{"path":{"text":"combining.txt"},"lines":{"text":"́x standalone mark leads this line\n"},"line_number":1,"submatches":[{"match":{"text":"́"},"start":0,"end":2}],"absolute_offset":0}}'
echo '{"type":"end","data":{"path":{"text":"combining.txt"},"binary_offset":null}}'
echo '{"type":"summary","data":{}}'
touch "$VRG_TEST_HANDSHAKE"
exit 0
"""
rgpath = os.path.join(fake, "rg")
with open(rgpath, "w") as f:
    f.write(rg)
os.chmod(rgpath, os.stat(rgpath).st_mode | stat.S_IXUSR | stat.S_IXGRP | stat.S_IXOTH)

with open(path, "rb") as f:
    head = f.read(8)
print("created demo/combining.txt (line 1 = U+0301 + 'x...', "
      "line 2 has a mid-line U+0301) and fakerg/rg "
      "(match covers the mark bytes [0,2), exit 0)")
print("first 8 file bytes:", head.hex(" "))
