#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-named/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-named/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/vrg" hit . 2>"/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-named/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-named/exit.txt"
sleep 60
