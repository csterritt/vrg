#!/bin/sh
stty rows 24 cols 80
stty -a >"/home/chris/vrg/Notes/walkthroughs/004-04/code-walkthrough/manual-run/stty-before.txt"
"/home/chris/vrg/Notes/walkthroughs/004-04/code-walkthrough/vrg" foo /usr
echo "$?" >"/home/chris/vrg/Notes/walkthroughs/004-04/code-walkthrough/manual-run/exit.txt"
stty -a >"/home/chris/vrg/Notes/walkthroughs/004-04/code-walkthrough/manual-run/stty-after.txt"
