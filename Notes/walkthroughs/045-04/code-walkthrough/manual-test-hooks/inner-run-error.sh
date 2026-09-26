#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/fixture"
env PATH="/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/fakebin-block:$PATH" TERM=xterm-256color   VRG_TEST_COLLECT_ACK="/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/ack-run-error" VRG_TEST_RUN_ERROR=boom   "/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/vrg-hooks" hit . 2>"/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/err-run-error.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/exit-run-error.txt"
sleep 3
