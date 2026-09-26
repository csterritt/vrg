#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/012-04/code-walkthrough/manual-scroll/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/012-04/code-walkthrough/manual-scroll/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/012-04/code-walkthrough/vrg" line .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/012-04/code-walkthrough/manual-scroll/exit.txt"
sleep 60
