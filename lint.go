package shellstrict

import (
	"bytes"
	"fmt"
	"sort"

	"mvdan.cc/sh/v3/syntax"
)

// Options configures Lint.
type Options struct {
	// Require lists which strict-mode flags to check for: any subset of
	// "errexit", "nounset", "pipefail". Nil means "use the default for the
	// detected shell" (see effectiveRequire); an empty non-nil slice means
	// "check nothing" (disables strict-mode entirely).
	Require []string
	// Shell forces shell detection to "bash" or "sh" instead of reading the
	// script's shebang. Mainly for stdin and extensionless files.
	Shell string
}

// Lint analyses one script's source and returns its findings. name is used
// for shell-family detection when there is no shebang (its extension) and
// is not otherwise required to be a real path.
func Lint(name string, src []byte, opts Options) Result {
	res := Result{File: name}

	sb := detectShebang(src, opts.Shell)
	shell := sb.shell
	if shell == "" && !sb.present {
		shell = shellForExtension(name)
	}
	if shell == "" {
		res.Skipped = true
		res.SkipReason = "not a recognised bash or sh script (no matching shebang, and --shell not given)"
		return res
	}

	parser := syntax.NewParser(syntax.KeepComments(true))
	f, err := parser.Parse(bytes.NewReader(src), name)
	if err != nil {
		res.Err = fmt.Errorf("parse: %w", err)
		return res
	}

	directives := parseDirectives(src)

	var findings []Finding
	if isSourcedLibrary(f, sb.present) {
		// strict-mode is skipped by design for a no-shebang, functions-only
		// file (see isSourcedLibrary); tempfile-cleanup still applies.
	} else if !directives.file["all"] && !directives.file[RuleStrictMode] {
		required := effectiveRequire(opts.Require, shell)
		findings = append(findings, checkStrictMode(f, sb.flags, required)...)
	}

	if !directives.file["all"] && !directives.file[RuleTempfileCleanup] {
		findings = append(findings, checkTempfileCleanup(f)...)
	}

	kept := findings[:0]
	for _, fd := range findings {
		if directives.suppresses(fd.Rule, fd.Line) {
			continue
		}
		fd.File = name
		kept = append(kept, fd)
	}
	sort.SliceStable(kept, func(i, j int) bool {
		if kept[i].Line != kept[j].Line {
			return kept[i].Line < kept[j].Line
		}
		return kept[i].Col < kept[j].Col
	})
	res.Findings = kept
	return res
}
