#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough/manual-empty/fixture"
stty rows 24 cols 80
"/home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough/vrg" zzzznotfound .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough/manual-empty/exit.txt"
