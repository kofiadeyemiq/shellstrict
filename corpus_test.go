package shellstrict

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/__GH_OWNER__/shellstrict/internal/corpus"
)

// TestCorpus is shellstrict's real-world evidence run: it lints a set of
// third-party scripts (see testdata/corpus.tsv for provenance) and reports
// the finding counts described in the README's "Why" section.
//
// It needs network access to fetch those scripts, so it only runs when
// explicitly requested. Every other test in this repository runs with
// plain `go test`; this one does not, on purpose.
func TestCorpus(t *testing.T) {
	if os.Getenv("SHELLSTRICT_CORPUS") != "1" {
		t.Skip("SKIPPED: the real-world corpus test needs network access and is opt-in.\n" +
			"Run it with:\n" +
			"    SHELLSTRICT_CORPUS=1 go test -run TestCorpus -v .\n" +
			"(or `go run ./internal/fetchcorpus` first, to fetch testdata/corpus/ once, then re-run either way)")
	}

	entries, paths, err := corpus.FetchAll("testdata/corpus.tsv", "testdata/corpus")
	if err != nil {
		t.Fatalf("fetch corpus: %v", err)
	}
	if len(entries) == 0 {
		t.Fatal("corpus manifest is empty")
	}

	var strictModeFindings, tempfileFindings, parseErrors int
	for _, e := range entries {
		path := paths[e.Name]
		src, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("%s: %v", e.Name, err)
		}
		res := Lint(filepath.ToSlash(path), src, Options{})
		if res.Err != nil {
			t.Logf("%s: could not analyse: %v", e.Name, res.Err)
			parseErrors++
			continue
		}
		for _, f := range res.Findings {
			t.Logf("%s", f.String())
			switch f.Rule {
			case RuleStrictMode:
				strictModeFindings++
			case RuleTempfileCleanup:
				tempfileFindings++
			}
		}
	}

	t.Logf("corpus summary: %d files, %d strict-mode findings, %d tempfile-cleanup findings, %d parse errors",
		len(entries), strictModeFindings, tempfileFindings, parseErrors)

	// These are the counts recorded in the README, verified by hand
	// (every finding read in context and classified true/false positive).
	// A pinned commit SHA means the fetched content never changes, so this
	// is a real regression check, not just a report: if it fails, either a
	// manifest entry was re-pinned to different content, or a code change
	// altered detection behavior — both worth looking at.
	const wantStrictMode = 9
	const wantTempfile = 0
	const wantParseErrors = 1
	if strictModeFindings != wantStrictMode {
		t.Errorf("strict-mode findings = %d, want %d", strictModeFindings, wantStrictMode)
	}
	if tempfileFindings != wantTempfile {
		t.Errorf("tempfile-cleanup findings = %d, want %d", tempfileFindings, wantTempfile)
	}
	if parseErrors != wantParseErrors {
		t.Errorf("parse errors = %d, want %d", parseErrors, wantParseErrors)
	}
}
