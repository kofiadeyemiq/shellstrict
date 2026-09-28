package shellstrict

import (
	"fmt"
	"strings"

	"mvdan.cc/sh/v3/syntax"
)

type mktempAssign struct {
	name string
	line int
	col  int
}

// checkTempfileCleanup reports a finding for every `VAR=$(mktemp ...)` (or
// backtick, or `mktemp -d`/`-t`) assignment anywhere in the file — including
// inside function bodies — whose variable is never referenced by an EXIT
// trap, either directly or through one level of function-call indirection.
func checkTempfileCleanup(f *syntax.File) []Finding {
	assigns := collectMktempAssigns(f)
	if len(assigns) == 0 {
		return nil
	}
	cleaned := collectTrapCleanedVars(f)

	var findings []Finding
	for _, a := range assigns {
		if cleaned[a.name] {
			continue
		}
		findings = append(findings, Finding{
			Line:    a.line,
			Col:     a.col,
			Rule:    RuleTempfileCleanup,
			Message: fmt.Sprintf("mktemp result assigned to $%s has no EXIT trap that cleans it up", a.name),
		})
	}
	return findings
}

// collectMktempAssigns walks the whole file (function bodies included) for
// assignments whose value is a command substitution invoking mktemp.
func collectMktempAssigns(f *syntax.File) []mktempAssign {
	var out []mktempAssign
	syntax.Walk(f, func(n syntax.Node) bool {
		a, ok := n.(*syntax.Assign)
		if !ok || a.Value == nil {
			return true
		}
		if !wordCallsMktemp(a.Value) {
			return true
		}
		pos := a.Pos()
		out = append(out, mktempAssign{name: a.Name.Value, line: int(pos.Line()), col: int(pos.Col())})
		return true
	})
	return out
}

// wordCallsMktemp reports whether w contains a command substitution whose
// first word is a literal "mktemp".
func wordCallsMktemp(w *syntax.Word) bool {
	for _, p := range w.Parts {
		cs, ok := p.(*syntax.CmdSubst)
		if !ok || len(cs.Stmts) == 0 {
			continue
		}
		ce, ok := cs.Stmts[0].Cmd.(*syntax.CallExpr)
		if !ok || len(ce.Args) == 0 {
			continue
		}
		if ce.Args[0].Lit() == "mktemp" {
			return true
		}
	}
	return false
}

// collectTrapCleanedVars returns the set of variable names referenced by any
// EXIT trap in the file (`trap ACTION EXIT ...`), resolving one level of
// function-call indirection: if ACTION is a bare call to a function defined
// in the file, that function's body is inspected too.
func collectTrapCleanedVars(f *syntax.File) map[string]bool {
	funcs := collectFuncDecls(f)
	cleaned := map[string]bool{}

	syntax.Walk(f, func(n syntax.Node) bool {
		ce, ok := n.(*syntax.CallExpr)
		if !ok || len(ce.Args) < 3 || ce.Args[0].Lit() != "trap" {
			return true
		}
		if !trapSignalsIncludeExit(ce.Args[2:]) {
			return true
		}
		action := ce.Args[1]
		for name := range paramNamesIn(action) {
			cleaned[name] = true
		}
		if fn := bareFuncCall(action, funcs); fn != nil {
			syntax.Walk(fn, func(n syntax.Node) bool {
				if pe, ok := n.(*syntax.ParamExp); ok {
					cleaned[pe.Param.Value] = true
				}
				return true
			})
		}
		return true
	})
	return cleaned
}

func trapSignalsIncludeExit(sigWords []*syntax.Word) bool {
	for _, w := range sigWords {
		if w.Lit() == "EXIT" {
			return true
		}
	}
	return false
}

// collectFuncDecls maps every function name defined in the file to its body.
func collectFuncDecls(f *syntax.File) map[string]*syntax.Stmt {
	out := map[string]*syntax.Stmt{}
	syntax.Walk(f, func(n syntax.Node) bool {
		fd, ok := n.(*syntax.FuncDecl)
		if !ok {
			return true
		}
		out[fd.Name.Value] = fd.Body
		return true
	})
	return out
}

// bareFuncCall reports whether action, once unwrapped from its outer
// quoting, is nothing but a call to a known function with no arguments
// (e.g. the action of `trap cleanup EXIT`).
func bareFuncCall(action *syntax.Word, funcs map[string]*syntax.Stmt) *syntax.Stmt {
	text := strings.TrimSpace(unwrapOuterQuotes(rawWordText(action)))
	if text == "" {
		return nil
	}
	if strings.ContainsAny(text, " \t\n;|&$(){}") {
		return nil // more than a single bare word
	}
	return funcs[text]
}

// paramNamesIn re-parses the literal text behind a trap action word (which
// may be single-quoted, double-quoted, or bare — trap re-evaluates it as a
// shell command when the signal fires, regardless of how it was quoted when
// passed to trap) and returns every variable name referenced in it.
func paramNamesIn(action *syntax.Word) map[string]bool {
	text := unwrapOuterQuotes(rawWordText(action))
	names := map[string]bool{}
	if text == "" {
		return names
	}
	parser := syntax.NewParser()
	sub, err := parser.Parse(strings.NewReader(text), "")
	if err != nil {
		// Fall back to a plain substring scan for $name / ${name} — still
		// useful for text we could not cleanly re-parse (e.g. leftover
		// escaped quotes from an originally double-quoted action).
		return paramNamesByScan(text)
	}
	syntax.Walk(sub, func(n syntax.Node) bool {
		if pe, ok := n.(*syntax.ParamExp); ok {
			names[pe.Param.Value] = true
		}
		return true
	})
	return names
}

func paramNamesByScan(text string) map[string]bool {
	names := map[string]bool{}
	for i := 0; i < len(text); i++ {
		if text[i] != '$' {
			continue
		}
		i++
		braced := i < len(text) && text[i] == '{'
		if braced {
			i++
		}
		start := i
		for i < len(text) && isNameByte(text[i]) {
			i++
		}
		if i > start {
			names[text[start:i]] = true
		}
		i--
	}
	return names
}

func isNameByte(c byte) bool {
	return c == '_' || (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')
}

// rawWordText returns the source text spanned by w, using its position
// range against... shellstrict re-renders the word instead of slicing the
// original source, since the word's own literal/quote structure is enough
// to reconstruct exactly what a shell would see, without needing to thread
// the original []byte through every call site.
func rawWordText(w *syntax.Word) string {
	var sb strings.Builder
	syntax.NewPrinter().Print(&sb, w)
	return sb.String()
}

// unwrapOuterQuotes strips one matching pair of leading/trailing quote
// characters, if present.
func unwrapOuterQuotes(s string) string {
	if len(s) >= 2 {
		if (s[0] == '\'' && s[len(s)-1] == '\'') || (s[0] == '"' && s[len(s)-1] == '"') {
			return s[1 : len(s)-1]
		}
	}
	return s
}
