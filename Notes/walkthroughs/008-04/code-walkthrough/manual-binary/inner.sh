#!/bin/sh
cd "/home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough/manual-binary/fixture"
stty rows 24 cols 80
"/home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough/vrg" foo .
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/008-04/code-walkthrough/manual-binary/exit.txt"
