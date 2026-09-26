#!/bin/sh
cd "/tmp/vrg28.fBTBrk/c"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/stderr-C.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/exit-C.txt"
cat "/home/chris/vrg/Notes/walkthroughs/028-06/code-walkthrough/completion/stderr-C.txt"
sleep 30
