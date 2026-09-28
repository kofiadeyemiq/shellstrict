// Command fetchcorpus downloads the third-party scripts listed in
// testdata/corpus.tsv into testdata/corpus/ (gitignored), verifying each
// against its manifest sha256. Run it with:
//
//	go run ./internal/fetchcorpus
//
// before running the corpus evidence test (see corpus_test.go), or let that
// test invoke the same logic itself.
package main

import (
	"fmt"
	"os"

	"github.com/__GH_OWNER__/shellstrict/internal/corpus"
)

func main() {
	manifest := "testdata/corpus.tsv"
	dest := "testdata/corpus"
	if len(os.Args) > 1 {
		manifest = os.Args[1]
	}
	if len(os.Args) > 2 {
		dest = os.Args[2]
	}

	entries, paths, err := corpus.FetchAll(manifest, dest)
	if err != nil {
		fmt.Fprintf(os.Stderr, "fetchcorpus: %v\n", err)
		os.Exit(1)
	}
	for _, e := range entries {
		fmt.Printf("%s -> %s (%s, sha256 verified)\n", e.Name, paths[e.Name], e.License)
	}
	fmt.Printf("%d file(s) fetched and verified into %s\n", len(entries), dest)
}
