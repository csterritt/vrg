#!/bin/sh
cd "/tmp/vrg27-fixture-a.VmzT0n"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/027-04/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/027-04/code-walkthrough/reload/stderr-a.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/027-04/code-walkthrough/reload/exit-a.txt"
cat "/home/chris/vrg/Notes/walkthroughs/027-04/code-walkthrough/reload/stderr-a.txt"
sleep 30
