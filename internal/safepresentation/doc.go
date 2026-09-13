// Package safepresentation owns the core escaping rules and byte→cell
// mappings that turn raw external path and content bytes into safe,
// display-ready text for every output sink. Issue #5 lands the path and
// content rules for the browse sinks; Issue #6 generalizes this core into
// the shared all-sink utility and unifies it with the Issue #1 cli.Escape
// escaper.
package safepresentation
