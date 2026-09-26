#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/fixture"
env PATH="/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/fakebin:$PATH" TERM=xterm-256color   VRG_TEST_COLLECT_ACK="/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/ack-baseline" VRG_TEST_REAP="/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/reap-baseline"    "/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/vrg-hooks" hit . 2>"/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/err-baseline.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/046-04/code-walkthrough/manual-runtime-error/exit-baseline.txt"
sleep 3
