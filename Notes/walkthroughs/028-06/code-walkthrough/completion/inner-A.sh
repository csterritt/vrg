#!/bin/sh
cd "/tmp/vrg28.fBTBrk/a"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/stderr-A.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/exit-A.txt"
cat "/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/stderr-A.txt"
sleep 30
