#!/bin/sh
cd "/tmp/vrg27-fixture-b.VvkXII"
unset RIPGREP_CONFIG_PATH
"/home/chris/vrg/Notes/walkthroughs/027-04/code-walkthrough/vrg" MARK . 2>"/home/chris/vrg/Notes/walkthroughs/027-04/code-walkthrough/reload/stderr-b.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/027-04/code-walkthrough/reload/exit-b.txt"
cat "/home/chris/vrg/Notes/walkthroughs/027-04/code-walkthrough/reload/stderr-b.txt"
sleep 30
