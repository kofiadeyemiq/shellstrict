# shellstrict

A linter for two bash/sh patterns ShellCheck does not check: missing strict-mode flags, and `mktemp` results with no trap-based cleanup.

```sh
$ shellstrict deploy.sh
deploy.sh:9:1: tempfile-cleanup: mktemp result assigned to $tmp has no EXIT trap that cleans it up
deploy.sh:10:1: strict-mode: missing errexit, nounset, pipefail before this command (required: errexit, nounset, pipefail)
```

## Why

ShellCheck is thorough, but two common failure modes are out of its scope by design:

- A script that never runs `set -e` (or `-u`, or `-o pipefail`), so a failing command midway through is silently ignored and the script carries on with stale or missing data.
- A script that creates a temp file or directory with `mktemp` and never arranges to remove it, leaking files into `/tmp` on every run.

This isn't an oversight ShellCheck is going to fix: [koalaman/shellcheck#409](https://github.com/koalaman/shellcheck/issues/409) has asked for a `set -e` check since 2015, and [PR #2237](https://github.com/koalaman/shellcheck/pull/2237), which attempted one, was never merged. As of ShellCheck 0.11.0, `shellcheck --list-optional` lists twelve opt-in checks (`add-default-case`, `avoid-negated-conditions`, `avoid-nullary-conditions`, `check-extra-masked-returns`, `check-set-e-suppressed`, `check-unassigned-uppercase`, `deprecate-which`, `quote-safe-variables`, `require-double-brackets`, `require-variable-braces`, `useless-use-of-cat`) and none of them, default or optional, covers either pattern.

shellstrict does not replace ShellCheck. Run both; they don't overlap.

Incumbent checked: ShellCheck 0.11.0, default checks and every optional check (`--enable=all`).

Measured: `shellcheck --enable=all examples/gap-demo.sh` against a script with no strict-mode flags and an uncleaned `mktemp`:

```
In examples/gap-demo.sh line 10:
echo "working in $tmp"
                 ^--^ SC2250 (style): Prefer putting braces around variable references even when not strictly required.
...
In examples/gap-demo.sh line 11:
cp "$config_file" "$tmp"
    ^----------^ SC2154 (warning): config_file is referenced but not assigned.
    ^----------^ SC2250 (style): Prefer putting braces around variable references even when not strictly required.
                   ^--^ SC2250 (style): Prefer putting braces around variable references even when not strictly required.
...
In examples/gap-demo.sh line 12:
process "$tmp"
         ^--^ SC2250 (style): Prefer putting braces around variable references even when not strictly required.
```

Brace style and one unrelated unset variable — nothing about the missing `set -euo pipefail` or the leaked temp file. `shellstrict examples/gap-demo.sh` on the same file:

```
examples/gap-demo.sh:9:1: tempfile-cleanup: mktemp result assigned to $tmp has no EXIT trap that cleans it up
examples/gap-demo.sh:10:1: strict-mode: missing errexit, nounset, pipefail before this command (required: errexit, nounset, pipefail)
```

To check the noise floor on real code rather than one demo script, shellstrict is also run against a corpus of 12 install/completion scripts from well-known projects (provenance in `testdata/README.md`; fetched on demand, not vendored — see [Development](#development)). Measured: `SHELLSTRICT_CORPUS=1 go test -run TestCorpus -v .`, 12 files — 9 `strict-mode` findings, 0 `tempfile-cleanup` findings, 1 file that could not be parsed (exit 2). Every finding was read in its source context and classified by hand: all 9 `strict-mode` findings are true positives (the named flags really are unset at the reported line). The corpus's one real `mktemp` user (`rustup-init.sh`) routes it through a wrapper function, which is a known `tempfile-cleanup` blind spot — see [Limitations](#limitations) — so the 0 there is a false negative, not evidence the check has nothing to find. The one parse failure is `git`'s own `git-completion.bash`, which mixes bash and zsh syntax behind a runtime check; see Limitations for why that's expected.

## Installation

```sh
go install github.com/__GH_OWNER__/shellstrict/cmd/shellstrict@latest
```

Go 1.26+. One dependency: `mvdan.cc/sh/v3`.

## Usage

### Lint one or more files

```sh
shellstrict script.sh other.sh
```

### Lint a directory

```sh
shellstrict .
```

Directories are walked recursively. Every regular file is checked; files that are not recognized as bash or sh (see [How it works](#how-it-works)) are skipped without comment, the same as passing them individually would do.

### Lint stdin

```sh
curl -fsSL https://example.com/install.sh | shellstrict --shell bash -
```

Stdin has no path to read a file extension from, so pass `--shell` explicitly — otherwise a script with no shebang read from stdin cannot be identified and is skipped.

### Machine-readable output

```sh
shellstrict --format json script.sh
```

### Silence a specific finding

```sh
tmp=$(mktemp) # shellstrict disable=tempfile-cleanup
```

Put `# shellstrict disable=<rule>` on its own line (no code before the `#`) to disable a rule for the whole file, or as a trailing comment on the line a finding is reported on to disable it just there. `<rule>` is `strict-mode`, `tempfile-cleanup`, or `all`; a comma-separated list disables more than one at once.

### Loosen or tighten strict-mode

```sh
shellstrict --require errexit,pipefail script.sh   # drop the nounset check
shellstrict --require errexit script.sh             # only check for -e
```

`set -e` is contested — see [BashFAQ/105](https://mywiki.wooledge.org/BashFAQ/105) for the well-known list of cases where `errexit` silently does not fire (a command inside `&&`/`||`, in a pipeline other than the last stage without `pipefail`, in a command substitution used in a condition, and others). shellstrict has no opinion on whether your script should use `set -e`; it only checks that the flags you *do* want are present before the first place they'd matter. If your team has decided against `errexit`, run with `--require nounset,pipefail` or narrower, or disable the rule outright with a file-wide directive.

## Reference

### Flags

| Name | Type | Default | Description |
| --- | --- | --- | --- |
| `--require` | string | all three for bash scripts; `errexit,nounset` for sh scripts | Comma-separated subset of `errexit`, `nounset`, `pipefail` to check for. |
| `--shell` | string | detected from shebang / extension | Force shell detection to `bash` or `sh`, overriding the shebang. Needed for stdin. |
| `--format` | string | `text` | `text` (`file:line:col: rule: message`, one per line) or `json` (an array of the same fields). |

### Exit codes

| Code | Meaning |
| --- | --- |
| `0` | No findings. |
| `1` | At least one finding, every file analyzed cleanly. |
| `2` | A file could not be read or parsed as shell, or the flags were invalid. |

### Rules

| Rule | Checks |
| --- | --- |
| `strict-mode` | Reports the first command in the script (skipping function definitions and bare assignments) that runs before all of `--require`'s flags are enabled, and names which are still missing at that point. |
| `tempfile-cleanup` | Reports every `VAR=$(mktemp ...)` (or backtick, `-d`, `-t`, inside a function) assignment whose variable is never referenced by an `EXIT` trap, directly or through one level of function-call indirection (`trap cleanup EXIT` where `cleanup` removes it). |

## How it works

shellstrict parses each script with [`mvdan.cc/sh/v3/syntax`](https://pkg.go.dev/mvdan.cc/sh/v3/syntax) rather than matching text with regular expressions. A regex-based version of either check has an unacceptable false-positive rate: `# set -e` in a comment, `"set -e"` inside a string, or `mktemp` appearing as a substring of a longer command name would all need to be excluded by hand, and a real parser already gets that for free by construction. The dependency also gives `strict-mode` a real, ordered walk of top-level statements (so it can tell that a `set -e` after the first failing command is too late) and gives `tempfile-cleanup` a way to look inside a single-quoted trap action as if it were a small script of its own, since that's exactly what the shell does with it when the signal fires.

Shell family (bash vs. sh) is read from the shebang line — including through `#!/usr/bin/env bash` and `#!/usr/bin/env -S bash -e` — or from a `.sh`/`.bash` extension when there is no shebang at all, or forced with `--shell`. A file with neither a recognized shebang nor a recognized extension is skipped, the same way a `.py` or `.zsh` file is: shellstrict only understands bash and POSIX sh.

`pipefail` is a bash/ksh/zsh extension that POSIX sh (as dash, `/bin/sh` on Debian and Ubuntu, implements it) does not support at all, so a sh script defaults to checking for `errexit` and `nounset` only, not `pipefail`. Pass `--require` explicitly to override this.

A file with no shebang whose every top-level statement is a function definition is treated as a library meant to be sourced, not executed, and `strict-mode` is skipped for it by default: `set -e`/`set -u` inside a sourced file changes the *calling* shell's options too, which a library has no business doing unilaterally. `tempfile-cleanup` still applies to such files, since a leaked temp file is a real problem regardless of who calls the function that creates it.

## Limitations

- `strict-mode` only looks at unconditional, top-level `set` statements and the shebang line. `if some_check; then set -e; fi`, or a flag set inside a function that runs before other top-level code, is not tracked.
- Once a required flag is seen as enabled, `strict-mode` does not notice a later `set +e` turning it back off — it checks that the flags are on *before* the first risky command, not that they stay on for the rest of the script.
- `set -o` with the option name glued directly onto other short flags in an unusual order (`-oeu`, option before the flags it's grouped with) is not parsed; the common orderings (`-euo pipefail`, `-eu -o pipefail`, `-uoerrexit`) are.
- `tempfile-cleanup` only recognizes a direct `VAR=$(mktemp ...)` assignment. A wrapper like `VAR=$(ensure mktemp -d)` — a real pattern found while building the test corpus, in `rustup-init.sh` — is not seen, because the literal command in the substitution is `ensure`, not `mktemp`.
- `tempfile-cleanup` resolves at most one level of function-call indirection in a trap's action (`trap cleanup EXIT` where `cleanup` itself calls a second function is not followed further), and matches purely by variable name across the whole file — two unrelated variables that happen to share a name are not distinguished.
- `# shellstrict disable=` directives are recognized by a plain-text scan of each line, not by parsing where a comment attaches in the AST. A `#` inside a quoted string is not distinguished from a real comment marker, so a directive-shaped string literal would be misread. This has not come up in either the synthetic tests or the real-world corpus, but it is a known gap.
- Scripts are always parsed as bash syntax (a superset permissive enough for sh scripts too), regardless of whether the shebang says `bash` or `sh`. A script whose shebang says `sh` but that actually uses a bash-only construct will parse and be checked, rather than being rejected as invalid sh.
- Not a shell parser for every dialect: a script that mixes bash and zsh-only syntax behind a runtime `$ZSH_VERSION` check (`contrib/completion/git-completion.bash` in git's own repository does this) fails to parse and is reported as exit code 2, because the syntax is evaluated statically regardless of the runtime guard. ShellCheck also produces related warnings on the same construct when run as `-s bash`, for what it's worth — it's a genuinely unusual file, not a parser bug specific to shellstrict's dependency.
- Unquoted `$@`/`$*` is out of scope on purpose — ShellCheck's SC2068/SC2086 already cover it well.

## Compatibility

Run in CI: Go 1.26, on `ubuntu-latest`. Manually run during development against ShellCheck 0.11.0 (Homebrew build) on macOS/arm64.

## Development

```sh
go build ./...
go vet ./...
go test -race -count=1 ./...
gofmt -l .
go run ./cmd/shellstrict examples/gap-demo.sh
```

`go test` above does not touch the network. The real-world corpus test (`TestCorpus`, in `corpus_test.go`) is opt-in and skips loudly unless asked for, since it fetches third-party scripts — listed with their pinned commit SHA and expected sha256 in `testdata/corpus.tsv`, and their licenses in `testdata/README.md` — into the gitignored `testdata/corpus/`, rather than vendoring them into this repository:

```sh
SHELLSTRICT_CORPUS=1 go test -run TestCorpus -v .
```

`go run ./internal/fetchcorpus` fetches (and sha256-verifies) the same files into `testdata/corpus/` on its own, without running the test, if you want to inspect them directly.

## License

MIT — see [LICENSE](LICENSE).
