# Corpus provenance

`corpus.tsv` lists the third-party scripts shellstrict's real-world evidence
run (`TestCorpus` in `../corpus_test.go`, and the "Why" section of the main
README) is measured against. They are **not** vendored into this
repository: this repo is MIT-licensed, but git's own
`contrib/completion/git-completion.bash` is GPL-2.0, and simply copying the
MIT/BSD-licensed scripts here without the copyright and permission notice
their licenses require (those notices live in each project's own `LICENSE`
file, not in the script) would not satisfy those licenses either. Instead,
`corpus.tsv` pins each file to an exact commit SHA and a sha256 of its
content, and `internal/corpus` fetches and verifies them on demand into the
gitignored `corpus/` directory:

```sh
go run ./internal/fetchcorpus
```

Each `corpus.tsv` row is `name`, `url` (a `raw.githubusercontent.com` URL
pinned to a 40-character commit SHA — never a branch name, so the content it
points at cannot change under us), `license`, and `sha256` (the exact hash
`fetchcorpus` — and `TestCorpus`, which calls the same code — verifies the
download against before shellstrict is run on it). Licenses below are as
declared by each project's own `LICENSE` file, fetched and read directly
while building this manifest (`git-completion.bash`'s GPL-2.0 is stated in
the file's own header instead):

| File | Source | License |
| --- | --- | --- |
| `bats-install.sh` | github.com/bats-core/bats-core `install.sh` | MIT |
| `docker-install.sh` | github.com/docker/docker-install `install.sh` | Apache-2.0 |
| `fzf-install.sh` | github.com/junegunn/fzf `install` | MIT |
| `git-completion.bash` | github.com/git/git `contrib/completion/git-completion.bash` | GPL-2.0 |
| `homebrew-install.sh` | github.com/Homebrew/install `install.sh` | BSD-2-Clause |
| `k8s-hack-lib-init.sh` | github.com/kubernetes/kubernetes `hack/lib/init.sh` | Apache-2.0 |
| `k8s-verify-gofmt.sh` | github.com/kubernetes/kubernetes `hack/verify-gofmt.sh` | Apache-2.0 |
| `nvm-install.sh` | github.com/nvm-sh/nvm `install.sh` | MIT |
| `ohmyzsh-install.sh` | github.com/ohmyzsh/ohmyzsh `tools/install.sh` | MIT |
| `pyenv.sh` | github.com/pyenv/pyenv `libexec/pyenv` | MIT |
| `rbenv.sh` | github.com/rbenv/rbenv `libexec/rbenv` | MIT |
| `rustup-init.sh` | github.com/rust-lang/rustup `rustup-init.sh` | MIT OR Apache-2.0 |

See `corpus.tsv` for the exact pinned commit SHA behind each row.

These scripts exist only as read-only lint targets for shellstrict's own
test/evidence run; none of their code is reused by shellstrict itself, and
none of it is committed to this repository — only the manifest is.
