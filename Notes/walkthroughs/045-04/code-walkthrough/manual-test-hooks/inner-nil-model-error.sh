#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/fixture"
env PATH="/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/fakebin-block:$PATH" TERM=xterm-256color   VRG_TEST_COLLECT_ACK="/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/ack-nil-model-error" VRG_TEST_RUN_FINAL_MODEL=nil VRG_TEST_RUN_ERROR=boom   "/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/vrg-hooks" hit . 2>"/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/err-nil-model-error.txt"
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/045-04/code-walkthrough/manual-test-hooks/exit-nil-model-error.txt"
sleep 3
