#!/bin/sh
cd "/tmp/vrg34-fixture.8WzxRN"
unset RIPGREP_CONFIG_PATH
PATH="/tmp/vrg34-bin.wObYz9:$PATH" "/home/chris/vrg/Notes/walkthroughs/034-04/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/034-04/code-walkthrough/run/stderr-browse.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/034-04/code-walkthrough/run/exit-browse.txt"
cat "/home/chris/vrg/Notes/walkthroughs/034-04/code-walkthrough/run/stderr-browse.txt"
sleep 30
