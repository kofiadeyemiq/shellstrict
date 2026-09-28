package shellstrict

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

// DefaultRequire is the default set of strict-mode flags shellstrict checks
// for on a bash script. See checkStrictMode for why a sh (POSIX) script
// defaults to a smaller set.
var DefaultRequire = []string{"errexit", "nounset", "pipefail"}

// checkStrictMode reports at most one finding: the point in the script,
// scanning top-level statements in order, where a command that could fail
// runs before all of required is enabled. Function bodies are not walked
// (defining a function does not run anything), and once a required flag is
// enabled it is assumed to stay enabled — shellstrict does not track a later
// "set +e" turning it back off.
//
// required should already reflect the caller's --require flag and the
// shell-specific default (see effectiveRequire).
func checkStrictMode(f *syntax.File, initial flagState, required []string) []Finding {
	if len(required) == 0 {
		return nil
	}

	state := initial
	for _, stmt := range f.Stmts {
		ce, isCall := stmt.Cmd.(*syntax.CallExpr)

		switch {
		case isCall && len(ce.Args) > 0 && ce.Args[0].Lit() == "set":
			words := make([]string, 0, len(ce.Args)-1)
			for _, a := range ce.Args[1:] {
				words = append(words, a.Lit())
			}
			applySetWords(words, &state)
			continue

		case isCall && len(ce.Args) == 0:
			// A bare assignment such as `FOO=bar`. Not a command that can
			// fail in the sense strict mode guards against.
			continue

		case stmt.Cmd == nil:
			continue

		default:
			if _, isFunc := stmt.Cmd.(*syntax.FuncDecl); isFunc {
				continue
			}
			if _, isDecl := stmt.Cmd.(*syntax.DeclClause); isDecl {
				// local/export/readonly/declare with assignments only;
				// treated the same as a bare assignment.
				continue
			}
		}

		// This is the first command that could fail.
		missing := state.missing(required)
		if len(missing) == 0 {
			return nil
		}
		pos := stmt.Pos()
		return []Finding{{
			Line:    int(pos.Line()),
			Col:     int(pos.Col()),
			Rule:    RuleStrictMode,
			Message: fmt.Sprintf("missing %s before this command (required: %s)", strings.Join(missing, ", "), strings.Join(required, ", ")),
		}}
	}
	return nil
}

// isSourcedLibrary reports whether f looks like a file meant to be sourced
// rather than executed: no shebang, and every top-level statement is a
// function definition. strict-mode is skipped by default for such files,
// since `set -e`/`set -u` inside a sourced file changes the *calling*
// shell's options too, which a library should not do unilaterally.
func isSourcedLibrary(f *syntax.File, hasShebang bool) bool {
	if hasShebang || len(f.Stmts) == 0 {
		return false
	}
	for _, stmt := range f.Stmts {
		if _, ok := stmt.Cmd.(*syntax.FuncDecl); !ok {
			return false
		}
	}
	return true
}

// effectiveRequire resolves the flags to check for, given the user's
// --require choice (userRequire, nil if not set) and the detected shell.
//
// set -o pipefail is a bash/ksh/zsh extension; POSIX sh (as implemented by
// dash, the shell behind /bin/sh on Debian and Ubuntu) does not support it
// at all, so checking for it by default on every #!/bin/sh script would be a
// permanent false positive. When the user has not passed --require
// explicitly, sh scripts are checked for errexit and nounset only.
func effectiveRequire(userRequire []string, shell string) []string {
	if userRequire != nil {
		return userRequire
	}
	if shell == "sh" {
		return []string{"errexit", "nounset"}
	}
	return DefaultRequire
}
