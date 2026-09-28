package shellstrict

import (
	"strings"
	"testing"
)

func findingRules(fs []Finding) []string {
	out := make([]string, len(fs))
	for i, f := range fs {
		out[i] = f.Rule
	}
	return out
}

func hasRule(fs []Finding, rule string) bool {
	for _, f := range fs {
		if f.Rule == rule {
			return true
		}
	}
	return false
}

func TestStrictMode_MissingFlagCombinations(t *testing.T) {
	tests := []struct {
		name    string
		script  string
		wantMsg string // substring expected in the strict-mode message, "" if no finding wanted
	}{
		{
			name: "no flags at all",
			script: "#!/bin/bash\n" +
				"echo hi\n",
			wantMsg: "missing errexit, nounset, pipefail",
		},
		{
			name: "only errexit",
			script: "#!/bin/bash\n" +
				"set -e\n" +
				"echo hi\n",
			wantMsg: "missing nounset, pipefail",
		},
		{
			name: "only nounset",
			script: "#!/bin/bash\n" +
				"set -u\n" +
				"echo hi\n",
			wantMsg: "missing errexit, pipefail",
		},
		{
			name: "only pipefail",
			script: "#!/bin/bash\n" +
				"set -o pipefail\n" +
				"echo hi\n",
			wantMsg: "missing errexit, nounset",
		},
		{
			name: "errexit and nounset, missing pipefail",
			script: "#!/bin/bash\n" +
				"set -eu\n" +
				"echo hi\n",
			wantMsg: "missing pipefail",
		},
		{
			name: "errexit and pipefail, missing nounset",
			script: "#!/bin/bash\n" +
				"set -e\n" +
				"set -o pipefail\n" +
				"echo hi\n",
			wantMsg: "missing nounset",
		},
		{
			name: "nounset and pipefail, missing errexit",
			script: "#!/bin/bash\n" +
				"set -u -o pipefail\n" +
				"echo hi\n",
			wantMsg: "missing errexit",
		},
		{
			name: "all three missing via unrelated set usage",
			script: "#!/bin/bash\n" +
				"set -x\n" +
				"echo hi\n",
			wantMsg: "missing errexit, nounset, pipefail",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			res := Lint("script.sh", []byte(tt.script), Options{})
			if res.Err != nil {
				t.Fatalf("unexpected parse error: %v", res.Err)
			}
			got := ""
			for _, f := range res.Findings {
				if f.Rule == RuleStrictMode {
					got = f.Message
				}
			}
			if tt.wantMsg == "" {
				if got != "" {
					t.Fatalf("expected no strict-mode finding, got %q", got)
				}
				return
			}
			if !strings.Contains(got, tt.wantMsg) {
				t.Fatalf("strict-mode message = %q, want substring %q", got, tt.wantMsg)
			}
		})
	}
}

func TestStrictMode_EquivalentSpellingsAreNotFlagged(t *testing.T) {
	tests := map[string]string{
		"set -euo pipefail on one line": "#!/bin/bash\n" +
			"set -euo pipefail\n" +
			"echo hi\n",
		"set -e -u -o pipefail separate flags": "#!/bin/bash\n" +
			"set -e -u -o pipefail\n" +
			"echo hi\n",
		"set -o errexit -o nounset -o pipefail": "#!/bin/bash\n" +
			"set -o errexit -o nounset -o pipefail\n" +
			"echo hi\n",
		"separate set lines": "#!/bin/bash\n" +
			"set -e\n" +
			"set -u\n" +
			"set -o pipefail\n" +
			"echo hi\n",
		"shebang flags -eu plus pipefail line": "#!/bin/bash -eu\n" +
			"set -o pipefail\n" +
			"echo hi\n",
		"set -Eeuo pipefail (errtrace ignored)": "#!/bin/bash\n" +
			"set -Eeuo pipefail\n" +
			"echo hi\n",
		"glued long option -oerrexit": "#!/bin/bash\n" +
			"set -uoerrexit -o pipefail\n" +
			"echo hi\n",
		"nothing risky happens": "#!/bin/bash\n" +
			"FOO=bar\n" +
			"myfunc() {\n" +
			"  echo inside\n" +
			"}\n",
	}

	for name, script := range tests {
		t.Run(name, func(t *testing.T) {
			res := Lint("script.sh", []byte(script), Options{})
			if res.Err != nil {
				t.Fatalf("unexpected parse error: %v", res.Err)
			}
			if hasRule(res.Findings, RuleStrictMode) {
				t.Fatalf("unexpected strict-mode finding(s): %v", res.Findings)
			}
		})
	}
}

func TestStrictMode_RequireIsConfigurable(t *testing.T) {
	script := "#!/bin/bash\n" +
		"set -e\n" +
		"echo hi\n"

	res := Lint("script.sh", []byte(script), Options{Require: []string{"errexit"}})
	if res.Err != nil {
		t.Fatalf("unexpected parse error: %v", res.Err)
	}
	if hasRule(res.Findings, RuleStrictMode) {
		t.Fatalf("expected no finding when only errexit is required and set: %v", res.Findings)
	}
}

func TestStrictMode_ShDefaultDropsPipefail(t *testing.T) {
	script := "#!/bin/sh\n" +
		"set -eu\n" +
		"echo hi\n"

	res := Lint("script.sh", []byte(script), Options{})
	if res.Err != nil {
		t.Fatalf("unexpected parse error: %v", res.Err)
	}
	if hasRule(res.Findings, RuleStrictMode) {
		t.Fatalf("sh script with errexit+nounset should not be flagged for pipefail by default: %v", res.Findings)
	}

	// Explicitly asking for pipefail on a sh script still works.
	res = Lint("script.sh", []byte(script), Options{Require: []string{"errexit", "nounset", "pipefail"}})
	if !hasRule(res.Findings, RuleStrictMode) {
		t.Fatalf("expected strict-mode finding when pipefail explicitly required on sh script")
	}
}

func TestStrictMode_SourcedLibrarySkipped(t *testing.T) {
	script := "" +
		"greet() {\n" +
		"  echo hello\n" +
		"}\n" +
		"farewell() {\n" +
		"  echo bye\n" +
		"}\n"

	res := Lint("lib.sh", []byte(script), Options{})
	if res.Err != nil {
		t.Fatalf("unexpected parse error: %v", res.Err)
	}
	if hasRule(res.Findings, RuleStrictMode) {
		t.Fatalf("sourced library (no shebang, only functions) should skip strict-mode: %v", res.Findings)
	}
	if !res.Skipped {
		// Not globally skipped -- tempfile-cleanup still ran -- but there
		// should be no strict-mode finding either way. This branch just
		// documents that Skipped is about shell detection, not this rule.
		t.Log("file was analysed, not globally skipped, as expected")
	}
}

func TestStrictMode_NoShebangWithTopLevelCodeIsChecked(t *testing.T) {
	// Not a "sourced library" (has more than just function defs), and has
	// a .sh extension, so it is still linted.
	script := "" +
		"echo hi\n"

	res := Lint("script.sh", []byte(script), Options{})
	if res.Err != nil {
		t.Fatalf("unexpected parse error: %v", res.Err)
	}
	if !hasRule(res.Findings, RuleStrictMode) {
		t.Fatalf("expected strict-mode finding for extensionless-shebang script with top-level code: %v", res.Findings)
	}
}
