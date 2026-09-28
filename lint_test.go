package shellstrict

import "testing"

func TestLint_SkipsNonBashShScripts(t *testing.T) {
	tests := map[string]struct {
		name   string
		script string
	}{
		"python shebang":                    {"script.py", "#!/usr/bin/env python3\nprint('hi')\n"},
		"perl shebang":                      {"script.pl", "#!/usr/bin/perl\nprint \"hi\\n\";\n"},
		"zsh shebang":                       {"script.zsh", "#!/bin/zsh\necho hi\n"},
		"unknown, no shebang, no extension": {"script", "echo hi\n"},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			res := Lint(tt.name, []byte(tt.script), Options{})
			if !res.Skipped {
				t.Fatalf("expected file to be skipped, got findings=%v err=%v", res.Findings, res.Err)
			}
		})
	}
}

func TestLint_RecognisesBashAndShViaShebangOrEnv(t *testing.T) {
	tests := map[string]string{
		"bash direct":    "#!/bin/bash\necho hi\n",
		"sh direct":      "#!/bin/sh\necho hi\n",
		"dash":           "#!/bin/dash\necho hi\n",
		"env bash":       "#!/usr/bin/env bash\necho hi\n",
		"env -S bash -e": "#!/usr/bin/env -S bash -e\necho hi\n",
	}
	for name, script := range tests {
		t.Run(name, func(t *testing.T) {
			res := Lint("script", []byte(script), Options{})
			if res.Skipped {
				t.Fatalf("expected script to be recognised, got skipped: %s", res.SkipReason)
			}
		})
	}
}

func TestLint_ShellFlagOverridesDetection(t *testing.T) {
	res := Lint("script", []byte("echo hi\n"), Options{Shell: "bash"})
	if res.Skipped {
		t.Fatalf("--shell override should force detection, got skipped: %s", res.SkipReason)
	}
	if !hasRule(res.Findings, RuleStrictMode) {
		t.Fatalf("expected strict-mode finding once forced to bash: %v", res.Findings)
	}
}

func TestFinding_String(t *testing.T) {
	f := Finding{File: "a.sh", Line: 3, Col: 1, Rule: RuleStrictMode, Message: "missing errexit"}
	want := "a.sh:3:1: strict-mode: missing errexit"
	if got := f.String(); got != want {
		t.Fatalf("String() = %q, want %q", got, want)
	}
}

func TestLint_MultipleFindingsSortedBySourceOrder(t *testing.T) {
	script := "#!/bin/bash\n" +
		"tmp=$(mktemp)\n" +
		"echo \"$tmp\"\n"

	res := Lint("script.sh", []byte(script), Options{})
	if res.Err != nil {
		t.Fatalf("unexpected parse error: %v", res.Err)
	}
	if len(res.Findings) < 2 {
		t.Fatalf("expected at least two findings (strict-mode + tempfile-cleanup), got %v", res.Findings)
	}
	for i := 1; i < len(res.Findings); i++ {
		if res.Findings[i].Line < res.Findings[i-1].Line {
			t.Fatalf("findings not sorted by line: %v", res.Findings)
		}
	}
}

func TestLint_ParseErrorIsReported(t *testing.T) {
	res := Lint("script.sh", []byte("#!/bin/bash\nif [ 1 -eq 1\n"), Options{})
	if res.Err == nil {
		t.Fatalf("expected a parse error for unterminated if")
	}
}
