#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/010-06/code-walkthrough/manual-damaged-stream/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/010-06/code-walkthrough/manual-damaged-stream/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/010-06/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/010-06/code-walkthrough/manual-damaged-stream/exit.txt"
sleep 60
