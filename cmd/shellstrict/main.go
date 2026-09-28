// Command shellstrict lints bash and sh scripts for two patterns ShellCheck
// does not check: missing strict-mode flags and mktemp results with no
// trap-based cleanup. Run `shellstrict -h` for usage.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/kofiadeyemiq/shellstrict"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("shellstrict", flag.ContinueOnError)
	fs.SetOutput(stderr)
	var (
		requireFlag = fs.String("require", "", "comma-separated strict-mode flags to check for: errexit,nounset,pipefail (default: all three for bash scripts; errexit,nounset for sh scripts, since POSIX sh has no pipefail)")
		shellFlag   = fs.String("shell", "", "force shell detection instead of reading the shebang: bash or sh")
		format      = fs.String("format", "text", "output format: text or json")
	)
	fs.Usage = func() {
		fmt.Fprintf(stderr, `shellstrict: lint bash/sh scripts for strict-mode and mktemp-cleanup gaps.

ShellCheck complement, not a replacement — see the README for why ShellCheck
does not (and, per its maintainer, will not) cover these two checks.

Usage:
  shellstrict [flags] <file|dir>...
  shellstrict [flags] -          (read one script from stdin)

Flags:
`)
		fs.PrintDefaults()
	}
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() == 0 {
		fs.Usage()
		return 2
	}
	if *format != "text" && *format != "json" {
		fmt.Fprintf(stderr, "shellstrict: invalid --format %q (want text or json)\n", *format)
		return 2
	}
	if *shellFlag != "" && *shellFlag != "bash" && *shellFlag != "sh" {
		fmt.Fprintf(stderr, "shellstrict: invalid --shell %q (want bash or sh)\n", *shellFlag)
		return 2
	}

	// require stays nil (use the library's per-shell default) unless the
	// user explicitly passed --require.
	var require []string
	if *requireFlag != "" {
		require = []string{}
		for _, r := range strings.Split(*requireFlag, ",") {
			r = strings.TrimSpace(r)
			if r == "" {
				continue
			}
			if r != "errexit" && r != "nounset" && r != "pipefail" {
				fmt.Fprintf(stderr, "shellstrict: invalid --require value %q (want errexit, nounset, pipefail)\n", r)
				return 2
			}
			require = append(require, r)
		}
	}
	opts := shellstrict.Options{Require: require, Shell: *shellFlag}

	paths, err := expandArgs(fs.Args())
	if err != nil {
		fmt.Fprintf(stderr, "shellstrict: %v\n", err)
		return 2
	}

	var (
		results  []shellstrict.Result
		hadError bool
		hadFinds bool
	)
	for _, p := range paths {
		var src []byte
		var readErr error
		if p == "-" {
			src, readErr = io.ReadAll(os.Stdin)
		} else {
			src, readErr = os.ReadFile(p)
		}
		if readErr != nil {
			fmt.Fprintf(stderr, "shellstrict: %s: %v\n", p, readErr)
			hadError = true
			continue
		}
		res := shellstrict.Lint(p, src, opts)
		if res.Err != nil {
			fmt.Fprintf(stderr, "shellstrict: %s: %v\n", p, res.Err)
			hadError = true
			continue
		}
		if len(res.Findings) > 0 {
			hadFinds = true
		}
		results = append(results, res)
	}

	if *format == "json" {
		printJSON(stdout, results)
	} else {
		printText(stdout, results)
	}

	switch {
	case hadError:
		return 2
	case hadFinds:
		return 1
	default:
		return 0
	}
}

// expandArgs turns files and directories into a flat, sorted list of file
// paths. A directory is walked recursively; every regular file is handed to
// Lint, which skips anything it does not recognise as bash/sh. "-" (stdin)
// passes through unchanged.
func expandArgs(args []string) ([]string, error) {
	var out []string
	for _, a := range args {
		if a == "-" {
			out = append(out, a)
			continue
		}
		info, err := os.Stat(a)
		if err != nil {
			return nil, err
		}
		if !info.IsDir() {
			out = append(out, a)
			continue
		}
		err = filepath.WalkDir(a, func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if d.IsDir() {
				if d.Name() == ".git" {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type().IsRegular() {
				out = append(out, p)
			}
			return nil
		})
		if err != nil {
			return nil, err
		}
	}
	sort.Strings(out)
	return out, nil
}

func printText(w io.Writer, results []shellstrict.Result) {
	for _, res := range results {
		for _, f := range res.Findings {
			fmt.Fprintln(w, f.String())
		}
	}
}

type jsonFinding struct {
	File    string `json:"file"`
	Line    int    `json:"line"`
	Col     int    `json:"col"`
	Rule    string `json:"rule"`
	Message string `json:"message"`
}

func printJSON(w io.Writer, results []shellstrict.Result) {
	var all []jsonFinding
	for _, res := range results {
		for _, f := range res.Findings {
			all = append(all, jsonFinding(f))
		}
	}
	if all == nil {
		all = []jsonFinding{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	_ = enc.Encode(all)
}
