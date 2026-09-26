#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/011-06/code-walkthrough/manual-warn-replay/fixture"
printf 'shell: about to run vrg\n'
PATH="/home/chris/vrg/Notes/walkthroughs/011-06/code-walkthrough/manual-warn-replay/fakebin:$PATH" "/home/chris/vrg/Notes/walkthroughs/011-06/code-walkthrough/vrg" hello .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/011-06/code-walkthrough/manual-warn-replay/exit.txt"
printf 'shell: vrg exited\n'
sleep 60
