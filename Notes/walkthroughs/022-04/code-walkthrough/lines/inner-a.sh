#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough/lines/fixture"
env -u RIPGREP_CONFIG_PATH "/home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough/lines/runA.exit"
sleep 30
