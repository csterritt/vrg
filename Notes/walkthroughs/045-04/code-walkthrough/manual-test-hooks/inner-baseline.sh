#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/fixture"
env PATH="/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/fakebin-block:$PATH" TERM=xterm-256color   VRG_TEST_COLLECT_ACK="/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/ack-baseline"    "/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/vrg-hooks" hit . 2>"/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/err-baseline.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/exit-baseline.txt"
sleep 3
