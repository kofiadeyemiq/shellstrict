package main

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeScript(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestRun_ExitCodes(t *testing.T) {
	dir := t.TempDir()
	clean := writeScript(t, dir, "clean.sh", "#!/bin/bash\nset -euo pipefail\necho hi\n")
	dirty := writeScript(t, dir, "dirty.sh", "#!/bin/bash\necho hi\n")
	broken := writeScript(t, dir, "broken.sh", "#!/bin/bash\nif [ 1 -eq 1\n")

	tests := []struct {
		name     string
		args     []string
		wantCode int
	}{
		{"clean script", []string{clean}, 0},
		{"dirty script", []string{dirty}, 1},
		{"unparsable script", []string{broken}, 2},
		{"missing file", []string{filepath.Join(dir, "nope.sh")}, 2},
		{"no args", []string{}, 2},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			code := run(tt.args, &stdout, &stderr)
			if code != tt.wantCode {
				t.Fatalf("exit code = %d, want %d (stdout=%q stderr=%q)", code, tt.wantCode, stdout.String(), stderr.String())
			}
		})
	}
}

func TestRun_JSONFormat(t *testing.T) {
	dir := t.TempDir()
	dirty := writeScript(t, dir, "dirty.sh", "#!/bin/bash\ntmp=$(mktemp)\necho \"$tmp\"\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json", dirty}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stderr=%q", code, stderr.String())
	}
	var findings []jsonFinding
	if err := json.Unmarshal(stdout.Bytes(), &findings); err != nil {
		t.Fatalf("output is not valid JSON: %v\n%s", err, stdout.String())
	}
	if len(findings) == 0 {
		t.Fatalf("expected at least one finding in JSON output")
	}
	for _, f := range findings {
		if f.File != dirty {
			t.Fatalf("finding file = %q, want %q", f.File, dirty)
		}
	}
}

func TestRun_JSONFormatEmptyIsEmptyArray(t *testing.T) {
	dir := t.TempDir()
	clean := writeScript(t, dir, "clean.sh", "#!/bin/bash\nset -euo pipefail\necho hi\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--format", "json", clean}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0; stderr=%q", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "[]" {
		t.Fatalf("expected an empty JSON array, got %q", stdout.String())
	}
}

func TestRun_RequireFlagNarrowsChecks(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "script.sh", "#!/bin/bash\nset -e\necho hi\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{"--require", "errexit", script}, &stdout, &stderr)
	if code != 0 {
		t.Fatalf("exit code = %d, want 0 with --require errexit; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
}

func TestRun_InvalidFlagsReportExitTwo(t *testing.T) {
	dir := t.TempDir()
	script := writeScript(t, dir, "script.sh", "#!/bin/bash\nset -euo pipefail\necho hi\n")

	tests := [][]string{
		{"--format", "xml", script},
		{"--require", "bogus", script},
		{"--shell", "zsh", script},
	}
	for _, args := range tests {
		var stdout, stderr bytes.Buffer
		code := run(args, &stdout, &stderr)
		if code != 2 {
			t.Fatalf("args=%v: exit code = %d, want 2; stderr=%q", args, code, stderr.String())
		}
	}
}

func TestRun_DirectoryWalk(t *testing.T) {
	dir := t.TempDir()
	writeScript(t, dir, "a.sh", "#!/bin/bash\nset -euo pipefail\necho hi\n")
	writeScript(t, dir, "b.sh", "#!/bin/bash\necho hi\n")
	writeScript(t, dir, "notashell.py", "print('hi')\n")

	var stdout, stderr bytes.Buffer
	code := run([]string{dir}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "b.sh") {
		t.Fatalf("expected finding for b.sh, got: %s", stdout.String())
	}
	if strings.Contains(stdout.String(), "notashell.py") {
		t.Fatalf("did not expect a .py file to be linted: %s", stdout.String())
	}
}

func TestRun_Stdin(t *testing.T) {
	// Stdin has no path to detect a shebang's absence-vs-extension from
	// meaningfully, so --shell is required for it to be linted rather than
	// skipped.
	oldStdin := os.Stdin
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	defer func() { os.Stdin = oldStdin }()
	os.Stdin = r
	go func() {
		_, _ = w.Write([]byte("echo hi\n"))
		w.Close()
	}()

	var stdout, stderr bytes.Buffer
	code := run([]string{"--shell", "bash", "-"}, &stdout, &stderr)
	if code != 1 {
		t.Fatalf("exit code = %d, want 1; stdout=%q stderr=%q", code, stdout.String(), stderr.String())
	}
	if !strings.Contains(stdout.String(), "-:") {
		t.Fatalf("expected finding attributed to stdin (\"-\"), got: %s", stdout.String())
	}
}
