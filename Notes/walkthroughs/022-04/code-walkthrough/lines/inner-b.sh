#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough/lines/empty"
PATH="/home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough/lines/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough/vrg" foo .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/022-04/code-walkthrough/lines/runB.exit"
sleep 30
