#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-warning/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-warning/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-warning/exit.txt"
sleep 60
