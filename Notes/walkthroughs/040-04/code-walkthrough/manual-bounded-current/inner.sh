#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-current/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-current/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/vrg" hit . 2>"/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-current/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/040-04/code-walkthrough/manual-bounded-current/exit.txt"
sleep 60
