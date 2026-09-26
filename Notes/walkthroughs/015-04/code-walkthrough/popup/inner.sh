#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough/popup/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough/popup/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/015-04/code-walkthrough/popup/exit.txt"
sleep 60
