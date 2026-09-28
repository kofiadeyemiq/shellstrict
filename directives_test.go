package shellstrict

import "testing"

func TestDirectives_LineDisablesOnlyThatLine(t *testing.T) {
	script := "#!/bin/bash\n" +
		"tmp=$(mktemp) # shellstrict disable=tempfile-cleanup\n" +
		"other=$(mktemp)\n" +
		"echo \"$tmp $other\"\n"

	res := Lint("script.sh", []byte(script), Options{})
	if res.Err != nil {
		t.Fatalf("unexpected parse error: %v", res.Err)
	}
	var lines []int
	for _, f := range res.Findings {
		if f.Rule == RuleTempfileCleanup {
			lines = append(lines, f.Line)
		}
	}
	if len(lines) != 1 || lines[0] != 3 {
		t.Fatalf("expected exactly one tempfile-cleanup finding on line 3, got %v", lines)
	}
}

func TestDirectives_FileWideDisablesRule(t *testing.T) {
	script := "#!/bin/bash\n" +
		"# shellstrict disable=strict-mode\n" +
		"tmp=$(mktemp)\n" +
		"echo \"$tmp\"\n"

	res := Lint("script.sh", []byte(script), Options{})
	if res.Err != nil {
		t.Fatalf("unexpected parse error: %v", res.Err)
	}
	if hasRule(res.Findings, RuleStrictMode) {
		t.Fatalf("strict-mode should be disabled file-wide: %v", res.Findings)
	}
	if !hasRule(res.Findings, RuleTempfileCleanup) {
		t.Fatalf("tempfile-cleanup should still fire: %v", res.Findings)
	}
}

func TestDirectives_FileWideAllDisablesEverything(t *testing.T) {
	script := "#!/bin/bash\n" +
		"# shellstrict disable=all\n" +
		"tmp=$(mktemp)\n" +
		"echo \"$tmp\"\n"

	res := Lint("script.sh", []byte(script), Options{})
	if res.Err != nil {
		t.Fatalf("unexpected parse error: %v", res.Err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("expected no findings with disable=all, got %v", res.Findings)
	}
}

func TestDirectives_MultipleRulesOnOneLine(t *testing.T) {
	script := "#!/bin/bash\n" +
		"# shellstrict disable=strict-mode,tempfile-cleanup\n" +
		"tmp=$(mktemp)\n" +
		"echo \"$tmp\"\n"

	res := Lint("script.sh", []byte(script), Options{})
	if res.Err != nil {
		t.Fatalf("unexpected parse error: %v", res.Err)
	}
	if len(res.Findings) != 0 {
		t.Fatalf("expected no findings, got %v", res.Findings)
	}
}
