#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-anonymous/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-anonymous/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/vrg" hit . 2>"/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-anonymous/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/037-04/code-walkthrough/manual-anonymous/exit.txt"
sleep 60
