package shellstrict

import (
	"bufio"
	"bytes"
	"regexp"
	"strings"
)

// directiveRE matches a "shellstrict disable=<rules>" comment anywhere a
// shell comment may appear. <rules> is a comma-separated list of rule names
// or "all".
var directiveRE = regexp.MustCompile(`#\s*shellstrict\s+disable=([\w,-]+)`)

// disableDirectives is what parseDirectives found in a script.
type disableDirectives struct {
	// file disables a rule (or "all") for every finding in the file.
	file map[string]bool
	// line disables a rule (or "all") only for findings on that exact
	// source line.
	line map[int]map[string]bool
}

func (d disableDirectives) suppresses(rule string, line int) bool {
	if d.file["all"] || d.file[rule] {
		return true
	}
	if perLine, ok := d.line[line]; ok {
		if perLine["all"] || perLine[rule] {
			return true
		}
	}
	return false
}

// parseDirectives scans src line by line for "# shellstrict disable=..."
// comments. A line whose only content (besides the directive comment) is
// whitespace disables the named rule(s) for the whole file; a directive
// trailing real code disables them only for findings reported on that line.
//
// This is a plain text scan rather than an AST walk: it is easier to reason
// about ("the comment on line N controls line N"), and it works uniformly
// for both rules without needing to reconcile where mvdan.cc/sh attaches a
// given comment. The known gap is a '#' inside a quoted string being
// mistaken for a comment marker; shellstrict accepts that trade-off (see
// README Limitations).
func parseDirectives(src []byte) disableDirectives {
	d := disableDirectives{file: map[string]bool{}, line: map[int]map[string]bool{}}

	scanner := bufio.NewScanner(bytes.NewReader(src))
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for scanner.Scan() {
		lineNo++
		line := scanner.Text()
		m := directiveRE.FindStringSubmatchIndex(line)
		if m == nil {
			continue
		}
		rules := strings.Split(line[m[2]:m[3]], ",")
		before := strings.TrimSpace(line[:m[0]])

		target := d.file
		if before != "" {
			if d.line[lineNo] == nil {
				d.line[lineNo] = map[string]bool{}
			}
			target = d.line[lineNo]
		}
		for _, r := range rules {
			target[strings.TrimSpace(r)] = true
		}
	}
	return d
}
