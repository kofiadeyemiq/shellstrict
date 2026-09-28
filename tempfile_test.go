package shellstrict

import "testing"

func TestTempfileCleanup_UncleanedPatterns(t *testing.T) {
	tests := map[string]string{
		"$(mktemp)": "#!/bin/bash\n" +
			"tmp=$(mktemp)\n" +
			"echo \"$tmp\"\n",
		"backticks": "#!/bin/bash\n" +
			"tmp=`mktemp`\n" +
			"echo \"$tmp\"\n",
		"mktemp -d": "#!/bin/bash\n" +
			"dir=$(mktemp -d)\n" +
			"echo \"$dir\"\n",
		"mktemp -t": "#!/bin/bash\n" +
			"tmp=$(mktemp -t foo.XXXXXX)\n" +
			"echo \"$tmp\"\n",
		"inside a function": "#!/bin/bash\n" +
			"work() {\n" +
			"  local tmp\n" +
			"  tmp=$(mktemp)\n" +
			"  echo \"$tmp\"\n" +
			"}\n" +
			"work\n",
		"trap exists but for a different variable": "#!/bin/bash\n" +
			"tmp=$(mktemp)\n" +
			"other=$(mktemp)\n" +
			"trap 'rm -f \"$other\"' EXIT\n",
		"trap without EXIT does not count": "#!/bin/bash\n" +
			"tmp=$(mktemp)\n" +
			"trap 'rm -f \"$tmp\"' INT\n",
	}

	for name, script := range tests {
		t.Run(name, func(t *testing.T) {
			res := Lint("script.sh", []byte(script), Options{})
			if res.Err != nil {
				t.Fatalf("unexpected parse error: %v", res.Err)
			}
			if !hasRule(res.Findings, RuleTempfileCleanup) {
				t.Fatalf("expected tempfile-cleanup finding, got: %v", res.Findings)
			}
		})
	}
}

func TestTempfileCleanup_CleanedUpIsQuiet(t *testing.T) {
	tests := map[string]string{
		"inline single-quoted trap": "#!/bin/bash\n" +
			"tmp=$(mktemp)\n" +
			"trap 'rm -f \"$tmp\"' EXIT\n" +
			"echo \"$tmp\"\n",
		"inline double-quoted trap": "#!/bin/bash\n" +
			"tmp=$(mktemp)\n" +
			"trap \"rm -f $tmp\" EXIT\n" +
			"echo \"$tmp\"\n",
		"trap EXIT INT TERM": "#!/bin/bash\n" +
			"tmp=$(mktemp)\n" +
			"trap 'rm -f \"$tmp\"' EXIT INT TERM\n" +
			"echo \"$tmp\"\n",
		"cleanup function indirection": "#!/bin/bash\n" +
			"tmp=$(mktemp)\n" +
			"cleanup() {\n" +
			"  rm -f \"$tmp\"\n" +
			"}\n" +
			"trap cleanup EXIT\n" +
			"echo \"$tmp\"\n",
		"cleanup function uses braces": "#!/bin/bash\n" +
			"dir=$(mktemp -d)\n" +
			"cleanup() {\n" +
			"  rm -rf \"${dir}\"\n" +
			"}\n" +
			"trap cleanup EXIT\n",
		"trap set inside the same function as mktemp": "#!/bin/bash\n" +
			"work() {\n" +
			"  local tmp\n" +
			"  tmp=$(mktemp)\n" +
			"  trap 'rm -f \"$tmp\"' EXIT\n" +
			"  echo \"$tmp\"\n" +
			"}\n" +
			"work\n",
		"no mktemp at all": "#!/bin/bash\n" +
			"echo hi\n",
	}

	for name, script := range tests {
		t.Run(name, func(t *testing.T) {
			res := Lint("script.sh", []byte(script), Options{})
			if res.Err != nil {
				t.Fatalf("unexpected parse error: %v", res.Err)
			}
			if hasRule(res.Findings, RuleTempfileCleanup) {
				t.Fatalf("unexpected tempfile-cleanup finding(s): %v", res.Findings)
			}
		})
	}
}

func TestTempfileCleanup_ReportsMktempLine(t *testing.T) {
	script := "#!/bin/bash\n" +
		"echo before\n" +
		"tmp=$(mktemp)\n" +
		"echo \"$tmp\"\n"

	res := Lint("script.sh", []byte(script), Options{})
	if res.Err != nil {
		t.Fatalf("unexpected parse error: %v", res.Err)
	}
	var found bool
	for _, f := range res.Findings {
		if f.Rule == RuleTempfileCleanup {
			found = true
			if f.Line != 3 {
				t.Fatalf("expected finding on line 3 (the mktemp assignment), got line %d", f.Line)
			}
		}
	}
	if !found {
		t.Fatalf("expected a tempfile-cleanup finding")
	}
}
