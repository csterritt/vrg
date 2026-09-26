#!/bin/sh
cd "/tmp/vrg31-fixture.VOEr5z"
unset RIPGREP_CONFIG_PATH
"$@" 2>"/home/chris/vrg/Notes/walkthroughs/031-04/code-walkthrough/help/stderr.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/031-04/code-walkthrough/help/exit.txt"
cat "/home/chris/vrg/Notes/walkthroughs/031-04/code-walkthrough/help/stderr.txt"
sleep 30
