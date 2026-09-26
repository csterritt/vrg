#!/bin/sh
cd "/tmp/vrg33-fixture.yFt23c"
unset RIPGREP_CONFIG_PATH
PATH="/tmp/vrg33-bin.1AKnq5:$PATH" "/home/chris/vrg/Notes/walkthroughs/033-04/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/033-04/code-walkthrough/run/stderr-ctrlc.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/033-04/code-walkthrough/run/exit-ctrlc.txt"
cat "/home/chris/vrg/Notes/walkthroughs/033-04/code-walkthrough/run/stderr-ctrlc.txt"
sleep 30
