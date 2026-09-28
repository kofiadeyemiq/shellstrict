// Package corpus loads testdata/corpus.tsv (a manifest of third-party
// scripts, each pinned to a commit SHA and a known sha256) and fetches those
// scripts into a local, gitignored cache directory for shellstrict's
// real-world evidence run.
//
// The scripts themselves are not vendored into this repository: several
// carry licenses (GPL-2.0, and MIT/BSD copies without their accompanying
// LICENSE text) that make redistributing them here either a license
// violation or, at minimum, not something to do without the notices they
// require. Fetching them on demand, verified by hash, keeps the evidence
// run honest without shipping copies shellstrict has no right to ship.
package corpus

import (
	"bufio"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Entry is one row of testdata/corpus.tsv.
type Entry struct {
	Name    string // destination filename inside the cache directory
	URL     string // raw URL, pinned to a commit SHA (never a branch name)
	License string
	SHA256  string // expected hex sha256 of the fetched content
}

// LoadManifest reads a corpus.tsv file: a tab-separated file with header
// "name\turl\tlicense\tsha256".
func LoadManifest(path string) ([]Entry, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open manifest: %w", err)
	}
	defer f.Close()

	var entries []Entry
	sc := bufio.NewScanner(f)
	sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	lineNo := 0
	for sc.Scan() {
		lineNo++
		line := sc.Text()
		if lineNo == 1 {
			continue // header
		}
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) != 4 {
			return nil, fmt.Errorf("%s:%d: expected 4 tab-separated fields, got %d", path, lineNo, len(fields))
		}
		entries = append(entries, Entry{
			Name:    fields[0],
			URL:     fields[1],
			License: fields[2],
			SHA256:  strings.ToLower(fields[3]),
		})
	}
	if err := sc.Err(); err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	return entries, nil
}

// FetchAll ensures every entry in the manifest at manifestPath is present
// under destDir with content matching its expected sha256, downloading
// (or re-downloading) whatever is missing or mismatched. It returns the
// manifest entries and the local path each was written to, in manifest
// order. A checksum mismatch — from either an already-cached file or a
// freshly downloaded one — is a hard error: this manifest exists precisely
// so a compromised or moved upstream file is never linted silently.
func FetchAll(manifestPath, destDir string) ([]Entry, map[string]string, error) {
	entries, err := LoadManifest(manifestPath)
	if err != nil {
		return nil, nil, err
	}
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		return nil, nil, fmt.Errorf("create cache dir: %w", err)
	}

	client := &http.Client{Timeout: 30 * time.Second}
	paths := make(map[string]string, len(entries))

	for _, e := range entries {
		dest := filepath.Join(destDir, e.Name)
		if sum, err := sha256File(dest); err == nil && sum == e.SHA256 {
			paths[e.Name] = dest
			continue // already cached and verified
		}

		if err := download(client, e.URL, dest); err != nil {
			return nil, nil, fmt.Errorf("%s: %w", e.Name, err)
		}
		sum, err := sha256File(dest)
		if err != nil {
			return nil, nil, fmt.Errorf("%s: hash after download: %w", e.Name, err)
		}
		if sum != e.SHA256 {
			_ = os.Remove(dest)
			return nil, nil, fmt.Errorf("%s: sha256 mismatch: manifest says %s, downloaded content is %s (upstream file changed, or the manifest entry is stale — re-pin the commit SHA)", e.Name, e.SHA256, sum)
		}
		paths[e.Name] = dest
	}
	return entries, paths, nil
}

func download(client *http.Client, url, dest string) error {
	resp, err := client.Get(url)
	if err != nil {
		return fmt.Errorf("GET %s: %w", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("GET %s: status %s", url, resp.Status)
	}

	tmp := dest + ".download"
	out, err := os.Create(tmp)
	if err != nil {
		return fmt.Errorf("create %s: %w", tmp, err)
	}
	if _, err := io.Copy(out, resp.Body); err != nil {
		out.Close()
		os.Remove(tmp)
		return fmt.Errorf("write %s: %w", dest, err)
	}
	if err := out.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, dest)
}

func sha256File(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
