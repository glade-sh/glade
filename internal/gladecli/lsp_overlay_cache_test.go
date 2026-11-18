package gladecli

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/glade-sh/glade/internal/lsp"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/semanticcache"
)

func hashTree(t *testing.T, root string) map[string][sha256.Size]byte {
	t.Helper()
	files := map[string][sha256.Size]byte{}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if os.IsNotExist(err) {
			return nil
		}
		if err != nil || d.IsDir() {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		files[path] = sha256.Sum256(data)
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	return files
}

func overlayLSPHandler(t *testing.T, project string) (*lspProjectLoader, *lsp.Handler) {
	t.Helper()
	l, err := newLSPProjectLoader(project)
	if err != nil {
		t.Fatal(err)
	}
	l.cacheAllowed = true
	h := lsp.NewPendingHandler()
	h.SetRefresher(l.refresh)
	h.SetGenerationValidator(l.valid)
	h.SetBase(l.base())
	return l, h
}

func publishedCount(t *testing.T, notifications []lsp.Notification) int {
	t.Helper()
	if len(notifications) != 1 {
		t.Fatalf("notifications = %#v", notifications)
	}
	params, ok := notifications[0].Params.(lsp.PublishDiagnosticsParams)
	if !ok {
		t.Fatalf("params = %T", notifications[0].Params)
	}
	return len(params.Diagnostics)
}

// Unsaved editor content never reaches a persistent cache: a full overlay
// cycle leaves the project's .glade tree byte-identical.
func TestVolatileGenerationIsNeverPersisted(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	project := filepath.Join(root, "project")
	writeTestProject(t, project)
	file := filepath.Join(project, "force-app/main/classes/Draft.cls")
	saved := "public class Draft { public void run() { Integer x = 1; } }"
	writeTestFile(t, file, saved)
	_, h := overlayLSPHandler(t, project)
	uri := lsp.DocumentURI("file://" + file)
	if got := publishedCount(t, h.DidOpen(lsp.DidOpenTextDocumentParams{TextDocument: lsp.TextDocumentItem{URI: uri, Version: 1, Text: saved}})); got != 0 {
		t.Fatalf("saved diagnostics = %d", got)
	}
	cacheDir := filepath.Join(project, ".glade")
	before := hashTree(t, cacheDir)
	if len(before) == 0 {
		t.Fatal("disk analysis wrote no cache; the comparison would be empty")
	}
	for i, text := range []string{
		"public class Draft { public void run() { Integer x = 'wrong'; } }",
		"public class Draft { public void run() { Integer x = 2; } public void added() {} }",
		"public class Draft { public void run() { Integer x = 'again'; } }",
	} {
		notifications, err := h.DidChange(lsp.DidChangeTextDocumentParams{TextDocument: lsp.VersionedTextDocumentIdentifier{URI: uri, Version: i + 2}, ContentChanges: []lsp.TextDocumentContentChangeEvent{{Text: text}}})
		if err != nil {
			t.Fatal(err)
		}
		publishedCount(t, notifications)
	}
	h.DidClose(lsp.DidCloseTextDocumentParams{TextDocument: lsp.TextDocumentIdentifier{URI: uri}})
	after := hashTree(t, cacheDir)
	if len(after) != len(before) {
		t.Fatalf("cache files changed: before %d, after %d", len(before), len(after))
	}
	for path, digest := range before {
		if after[path] != digest {
			t.Fatalf("cache file changed: %s", path)
		}
	}
}

// After an unsaved edit is saved, the disk generation analyzes exactly like a
// cold build, and the semantic cache entry it writes equals a cold process's.
func TestOverlayThenSaveMatchesColdAnalysis(t *testing.T) {
	root := t.TempDir()
	t.Setenv("XDG_DATA_HOME", filepath.Join(root, "data"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(root, "cache"))
	project := filepath.Join(root, "project")
	writeTestProject(t, project)
	file := filepath.Join(project, "force-app/main/classes/Draft.cls")
	initial := "public class Draft { public void run() { Integer x = 1; } }"
	edited := "public class Draft { public void run() { Integer x = 'wrong'; MathUtil.add(1, 2); } }"
	writeTestFile(t, file, initial)
	_, h := overlayLSPHandler(t, project)
	uri := lsp.DocumentURI("file://" + file)
	h.DidOpen(lsp.DidOpenTextDocumentParams{TextDocument: lsp.TextDocumentItem{URI: uri, Version: 1, Text: initial}})
	notifications, err := h.DidChange(lsp.DidChangeTextDocumentParams{TextDocument: lsp.VersionedTextDocumentIdentifier{URI: uri, Version: 2}, ContentChanges: []lsp.TextDocumentContentChangeEvent{{Text: edited}}})
	if err != nil {
		t.Fatal(err)
	}
	overlayCount := publishedCount(t, notifications)
	if overlayCount == 0 {
		t.Fatal("unsaved edit produced no diagnostics")
	}
	writeTestFile(t, file, edited)
	if err := h.Refresh(context.Background(), []string{file}); err != nil {
		t.Fatal(err)
	}
	notifications, err = h.DidChange(lsp.DidChangeTextDocumentParams{TextDocument: lsp.VersionedTextDocumentIdentifier{URI: uri, Version: 3}, ContentChanges: []lsp.TextDocumentContentChangeEvent{{Text: edited}}})
	if err != nil {
		t.Fatal(err)
	}
	savedCount := publishedCount(t, notifications)
	cold, artifacts := buildLSPTestIndex(t, project)
	want := sema.AnalyzeWithOptions(cold, sema.AnalyzeOptions{Diagnostics: true, ExportTypes: true, BuildArtifacts: artifacts})
	var coldCount int
	for _, d := range want.Diagnostics {
		if d.File == file {
			coldCount++
		}
	}
	if savedCount != coldCount || overlayCount != coldCount {
		t.Fatalf("diagnostics: overlay %d, saved %d, cold %d", overlayCount, savedCount, coldCount)
	}
	warm := semanticEntries(t, project)
	if len(warm) == 0 {
		t.Fatal("saved generation wrote no semantic cache entry")
	}
	if err := os.RemoveAll(filepath.Join(project, ".glade")); err != nil {
		t.Fatal(err)
	}
	cold, artifacts = buildLSPTestIndex(t, project)
	if _, source := cachedLSPAnalysis(cold, artifacts, true, semanticcache.NewManager(semanticcache.Limits{})); source != semanticcache.SourceBuild {
		t.Fatalf("cold source = %q", source)
	}
	coldEntries := semanticEntries(t, project)
	for name, data := range coldEntries {
		if got, ok := warm[name]; !ok || !bytes.Equal(got, data) {
			t.Fatalf("semantic cache entry %s differs from a cold process", name)
		}
	}
}

func semanticEntries(t *testing.T, project string) map[string][]byte {
	t.Helper()
	matches, err := filepath.Glob(filepath.Join(project, ".glade", "semantic", "result-*.json"))
	if err != nil {
		t.Fatal(err)
	}
	sort.Strings(matches)
	entries := map[string][]byte{}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatal(err)
		}
		if !json.Valid(data) {
			t.Fatalf("semantic entry %s is not JSON", path)
		}
		entries[filepath.Base(path)] = data
	}
	return entries
}
