#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/007-04/code-walkthrough/manual-theme/testdata"
stty rows 24 cols 80
"/home/chris/vrg/Notes/walkthroughs/007-04/code-walkthrough/vrg" func .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/007-04/code-walkthrough/manual-theme/exit.txt"
