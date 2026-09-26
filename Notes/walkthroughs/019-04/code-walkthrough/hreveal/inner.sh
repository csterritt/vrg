#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/019-04/code-walkthrough/hreveal/fixture"
PATH="/home/chris/vrg/Notes/walkthroughs/019-04/code-walkthrough/hreveal/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/019-04/code-walkthrough/vrg" hit .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/019-04/code-walkthrough/hreveal/exit.txt"
sleep 30
