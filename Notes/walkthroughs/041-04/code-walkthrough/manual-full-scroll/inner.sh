#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/041-04/code-walkthrough/manual-full-scroll/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/041-04/code-walkthrough/manual-full-scroll/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/041-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/041-04/code-walkthrough/manual-full-scroll/exit.txt"
sleep 60
