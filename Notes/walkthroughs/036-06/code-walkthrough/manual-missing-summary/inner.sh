#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-missing-summary/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-missing-summary/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/vrg" hit . 2>"/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-missing-summary/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-missing-summary/exit.txt"
sleep 60
