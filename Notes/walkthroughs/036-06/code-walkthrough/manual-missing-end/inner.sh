#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-missing-end/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-missing-end/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/vrg" hit . 2>"/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-missing-end/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/036-06/code-walkthrough/manual-missing-end/exit.txt"
sleep 60
