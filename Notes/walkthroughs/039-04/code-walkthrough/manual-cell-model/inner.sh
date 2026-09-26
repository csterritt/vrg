#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/039-04/code-walkthrough/manual-cell-model/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/039-04/code-walkthrough/manual-cell-model/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/039-04/code-walkthrough/vrg" content . 2>"/home/chris/vrg/Notes/walkthroughs/039-04/code-walkthrough/manual-cell-model/replay.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/039-04/code-walkthrough/manual-cell-model/exit.txt"
sleep 60
