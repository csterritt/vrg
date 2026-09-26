#!/bin/sh
cd "/tmp/vrg32-fixture.cuWXJL"
unset RIPGREP_CONFIG_PATH
PATH="/tmp/vrg32-bin.nwPNEl/fatal:$PATH" "/home/chris/vrg/Notes/walkthroughs/032-04/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/032-04/code-walkthrough/run/stderr-fatal-q.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/032-04/code-walkthrough/run/exit-fatal-q.txt"
cat "/home/chris/vrg/Notes/walkthroughs/032-04/code-walkthrough/run/stderr-fatal-q.txt"
sleep 30
