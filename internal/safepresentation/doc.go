// Package safepresentation owns the core escaping rules and byte→cell
// mappings that turn raw external path, content, and diagnostic bytes
// into safe, display-ready text for every output sink. Issue #5 landed
// the path and content rules for the browse sinks; Issue #6 generalized
// this core into the shared all-sink utility, added the diagnostic
// escaper, and unified it with the Issue #1 cli.Escape escaper.
package safepresentation
