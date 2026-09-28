package corpus

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeManifest(t *testing.T, dir string, rows [][4]string) string {
	t.Helper()
	var sb strings.Builder
	sb.WriteString("name\turl\tlicense\tsha256\n")
	for _, r := range rows {
		fmt.Fprintf(&sb, "%s\t%s\t%s\t%s\n", r[0], r[1], r[2], r[3])
	}
	path := filepath.Join(dir, "corpus.tsv")
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func sha256Hex(s string) string {
	h := sha256.Sum256([]byte(s))
	return hex.EncodeToString(h[:])
}

func TestLoadManifest(t *testing.T) {
	dir := t.TempDir()
	path := writeManifest(t, dir, [][4]string{
		{"a.sh", "https://example.com/a.sh", "MIT", "abc123"},
		{"b.sh", "https://example.com/b.sh", "Apache-2.0", "def456"},
	})

	entries, err := LoadManifest(path)
	if err != nil {
		t.Fatalf("LoadManifest: %v", err)
	}
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if entries[0].Name != "a.sh" || entries[0].License != "MIT" || entries[0].SHA256 != "abc123" {
		t.Fatalf("entry 0 = %+v", entries[0])
	}
}

func TestLoadManifest_MalformedRow(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "corpus.tsv")
	if err := os.WriteFile(path, []byte("name\turl\tlicense\tsha256\nonly-two\tfields\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadManifest(path); err == nil {
		t.Fatal("expected an error for a malformed row")
	}
}

func TestFetchAll_DownloadsAndVerifies(t *testing.T) {
	const content = "#!/bin/bash\necho hi\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	manifest := writeManifest(t, dir, [][4]string{
		{"script.sh", srv.URL + "/script.sh", "MIT", sha256Hex(content)},
	})
	dest := filepath.Join(dir, "cache")

	entries, paths, err := FetchAll(manifest, dest)
	if err != nil {
		t.Fatalf("FetchAll: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	got, err := os.ReadFile(paths["script.sh"])
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != content {
		t.Fatalf("downloaded content = %q, want %q", got, content)
	}
}

func TestFetchAll_ChecksumMismatchErrors(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "actual content")
	}))
	defer srv.Close()

	dir := t.TempDir()
	manifest := writeManifest(t, dir, [][4]string{
		{"script.sh", srv.URL + "/script.sh", "MIT", sha256Hex("different content")},
	})
	dest := filepath.Join(dir, "cache")

	_, _, err := FetchAll(manifest, dest)
	if err == nil {
		t.Fatal("expected a checksum mismatch error")
	}
	if !strings.Contains(err.Error(), "sha256 mismatch") {
		t.Fatalf("error = %v, want it to mention sha256 mismatch", err)
	}
	if _, statErr := os.Stat(filepath.Join(dest, "script.sh")); !os.IsNotExist(statErr) {
		t.Fatal("expected the mismatched download to be removed, not left on disk")
	}
}

func TestFetchAll_CachedFileSkipsReDownload(t *testing.T) {
	const content = "cached content\n"
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	manifest := writeManifest(t, dir, [][4]string{
		{"script.sh", srv.URL + "/script.sh", "MIT", sha256Hex(content)},
	})
	dest := filepath.Join(dir, "cache")

	if _, _, err := FetchAll(manifest, dest); err != nil {
		t.Fatalf("first FetchAll: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests after first fetch = %d, want 1", requests)
	}
	if _, _, err := FetchAll(manifest, dest); err != nil {
		t.Fatalf("second FetchAll: %v", err)
	}
	if requests != 1 {
		t.Fatalf("requests after second fetch = %d, want still 1 (cache hit)", requests)
	}
}

func TestFetchAll_StaleCacheIsRefetched(t *testing.T) {
	const content = "fresh content\n"
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, content)
	}))
	defer srv.Close()

	dir := t.TempDir()
	manifest := writeManifest(t, dir, [][4]string{
		{"script.sh", srv.URL + "/script.sh", "MIT", sha256Hex(content)},
	})
	dest := filepath.Join(dir, "cache")
	if err := os.MkdirAll(dest, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "script.sh"), []byte("stale, wrong content\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, paths, err := FetchAll(manifest, dest); err != nil {
		t.Fatalf("FetchAll: %v", err)
	} else {
		got, _ := os.ReadFile(paths["script.sh"])
		if string(got) != content {
			t.Fatalf("content after refetch = %q, want %q", got, content)
		}
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1 (the stale file should trigger exactly one re-download)", requests)
	}
}

func TestFetchAll_HTTPErrorIsReported(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "not found", http.StatusNotFound)
	}))
	defer srv.Close()

	dir := t.TempDir()
	manifest := writeManifest(t, dir, [][4]string{
		{"script.sh", srv.URL + "/script.sh", "MIT", "irrelevant"},
	})

	_, _, err := FetchAll(manifest, filepath.Join(dir, "cache"))
	if err == nil {
		t.Fatal("expected an error for a 404 response")
	}
}
