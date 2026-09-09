## VRG project

- TUI, using bubbletea, bubbles, lipgloss
- invoked with a pattern and an optional directory ('.' default dir)
- use github.com/jawher/mow.cli for command-line parsing and built-in help; invoking vrg with no command-line arguments shows help by default without starting the search or TUI
- runs, as a subprocess, 'rg --json pattern dir'
- collects results, sorts them on path text
- show two panels:
	- one is list of files
	- other is file with matches highlighted
		- starts with file scrolled so that the first match is visible
	- list of files is 'above', and shown on left, just wide enough for filenames
- arrow keys and tab/shift-tab:
	- when list of files is showing, left/tab hides it
	- when list of files is hidden, right/shift-tab shows it
	- up/down scroll up/down one line
	- current file panel key bindings:
		- page up/page down, half page up, half page down
	- page-up/page-down keys if possible for page up/page down
	- up/down arrow keys scroll up/down one line
- next/previous match
	- 'n' moves to next match
	- 'p' moves to previous match
	- on file change, briefly show pop-up with new file name
- two 'color schemes'
	- white-on-black and black-on-white
	- key to switch
- file panel wraps text by default
	- key to switch to letting lines run off edge
	- in run-off-edge mode:
		- keys to scroll left/right by one, 10, or half-screen amounts
		- if a match is not visible, there's a '\*' shown in inverse colors on the right if the match is off to the right, on the left if it's off to the left
- file panel design:
	- top-most row has the current file name embedded in a display line
	- left-hand column with line number, right justified in a field with enough space for every line number
	- line-number column has a two spaces between the last digit. if the file panel is in run-off-edge mode, and there are characters not shown to the left since the user has scrolled to the right, then the first space is replaced by an inverse colors '\_' if there is no match hidden to the left, or an inverse colors '\*' if there is.
	- there are no 'display lines' on the right or left or bottom
- exit:
	- q/ctrl-c
	- immediate, no confirmation
	- help - '?' or 'h' gives dialog with key bindings
- example 'rg --json' output, from 'rg --json hitl Notes/issues/':

~~~ json
{"type":"begin","data":{"path":{"text":"Notes/issues/056-release-capability-suite-and-modernc-pin.md"}}}
{"type":"match","data":{"path":{"text":"Notes/issues/056-release-capability-suite-and-modernc-pin.md"},"lines":{"text":"**Type**: HITL\n"},"line_number":3,"absolute_offset":75,"submatches":[{"match":{"text":"HITL"},"start":10,"end":14}]}}
{"type":"end","data":{"path":{"text":"Notes/issues/056-release-capability-suite-and-modernc-pin.md"},"binary_offset":null,"stats":{"elapsed":{"secs":0,"nanos":65791,"human":"0.000066s"},"searches":1,"searches_with_match":1,"bytes_searched":4571,"bytes_printed":344,"matched_lines":1,"matches":1}}}
{"type":"begin","data":{"path":{"text":"Notes/issues/008-responsive-tui-shell-and-minimum-size-restoration.md"}}}
{"type":"match","data":{"path":{"text":"Notes/issues/008-responsive-tui-shell-and-minimum-size-restoration.md"},"lines":{"text":"**Type**: HITL\n"},"line_number":3,"absolute_offset":63,"submatches":[{"match":{"text":"HITL"},"start":10,"end":14}]}}
{"type":"end","data":{"path":{"text":"Notes/issues/008-responsive-tui-shell-and-minimum-size-restoration.md"},"binary_offset":null,"stats":{"elapsed":{"secs":0,"nanos":21417,"human":"0.000021s"},"searches":1,"searches_with_match":1,"bytes_searched":1852,"bytes_printed":362,"matched_lines":1,"matches":1}}}
{"type":"begin","data":{"path":{"text":"Notes/issues/028-scoped-ctrl-w-cancellation-and-bounded-settlement.md"}}}
{"type":"match","data":{"path":{"text":"Notes/issues/028-scoped-ctrl-w-cancellation-and-bounded-settlement.md"},"lines":{"text":"**Type**: HITL\n"},"line_number":3,"absolute_offset":64,"submatches":[{"match":{"text":"HITL"},"start":10,"end":14}]}}
{"type":"end","data":{"path":{"text":"Notes/issues/028-scoped-ctrl-w-cancellation-and-bounded-settlement.md"},"binary_offset":null,"stats":{"elapsed":{"secs":0,"nanos":19709,"human":"0.000020s"},"searches":1,"searches_with_match":1,"bytes_searched":1783,"bytes_printed":362,"matched_lines":1,"matches":1}}}
{"type":"begin","data":{"path":{"text":"Notes/issues/024-concurrent-first-page-and-independent-result-count.md"}}}
{"type":"match","data":{"path":{"text":"Notes/issues/024-concurrent-first-page-and-independent-result-count.md"},"lines":{"text":"**Type**: HITL\n"},"line_number":3,"absolute_offset":65,"submatches":[{"match":{"text":"HITL"},"start":10,"end":14}]}}
{"type":"end","data":{"path":{"text":"Notes/issues/024-concurrent-first-page-and-independent-result-count.md"},"binary_offset":null,"stats":{"elapsed":{"secs":0,"nanos":19334,"human":"0.000019s"},"searches":1,"searches_with_match":1,"bytes_searched":2164,"bytes_printed":364,"matched_lines":1,"matches":1}}}
{"data":{"elapsed_total":{"human":"0.005415s","nanos":5415397,"secs":0},"stats":{"bytes_printed":1432,"bytes_searched":169188,"elapsed":{"human":"0.001944s","nanos":1944004,"secs":0},"matched_lines":4,"matches":4,"searches":88,"searches_with_match":4}},"type":"summary"}
~~~