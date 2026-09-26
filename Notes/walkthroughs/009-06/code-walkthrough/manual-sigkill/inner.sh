#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-sigkill/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-sigkill/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/009-06/code-walkthrough/manual-sigkill/exit.txt"
sleep 60
