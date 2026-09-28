// Package shellstrict analyses bash and POSIX sh scripts for two patterns
// ShellCheck does not check: missing strict-mode flags (errexit, nounset,
// pipefail) and mktemp results with no trap-based cleanup.
//
// It parses scripts with mvdan.cc/sh/v3/syntax instead of matching text with
// regular expressions, so it understands quoting, command substitution and
// function bodies well enough to avoid the false positives a regex-based
// checker would produce (a "set -e" inside a comment or a string literal,
// for example).
package shellstrict

import "fmt"

// Rule names used in Finding.Rule, the --require flag, and
// "# shellstrict disable=" directives.
const (
	RuleStrictMode      = "strict-mode"
	RuleTempfileCleanup = "tempfile-cleanup"
)

// Finding is a single problem reported against one line of one script.
type Finding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

// String formats a Finding as "file:line:col: rule: message".
func (f Finding) String() string {
	return fmt.Sprintf("%s:%d:%d: %s: %s", f.File, f.Line, f.Col, f.Rule, f.Message)
}

// Result is the outcome of linting one script.
type Result struct {
	// File is the path (or "-" for stdin) the result is for.
	File string
	// Findings are the problems reported, in source order.
	Findings []Finding
	// Skipped is true when the file was recognised as out of scope (not a
	// bash/sh script, or a sourced library for strict-mode) rather than
	// analysed and found clean.
	Skipped bool
	// SkipReason explains why, when Skipped is true.
	SkipReason string
	// Err is set when the file could not be parsed as shell at all. The
	// caller should treat this as "could not analyse" (exit code 2).
	Err error
}
