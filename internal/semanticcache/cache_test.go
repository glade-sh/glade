package semanticcache

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/diagnostic"
	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/typesys"
)

func TestCacheExactIdentityReturnsCompleteResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.json")
	identity := testIdentity()
	want := analyzedTestResult()
	if len(want.Types) <= 200 {
		t.Fatalf("fixture exported %d types, want more than the normal CLI JSON limit", len(want.Types))
	}

	if err := storeTestPath(path, identity, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadTestPath(path, identity)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip changed result\nwant: %#v\n got: %#v", want, got)
	}
}

func TestEnvelopePreservesNilAndEmptyCollections(t *testing.T) {
	for _, tc := range []struct {
		name   string
		result sema.Result
	}{
		{name: "nil", result: sema.Result{}},
		{name: "empty", result: sema.Result{
			Diagnostics: []diagnostic.Diagnostic{},
			Types:       map[string]sema.TypeReference{},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "semantic.json")
			if err := storeTestPath(path, testIdentity(), tc.result); err != nil {
				t.Fatal(err)
			}
			got, err := loadTestPath(path, testIdentity())
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(got, tc.result) {
				t.Fatalf("collection shape changed\nwant: %#v\n got: %#v", tc.result, got)
			}
		})
	}
}

func TestCacheIdentityAndOptionMissReasons(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.json")
	identity := testIdentity()
	if err := storeTestPath(path, identity, smallTestResult()); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name   string
		change func(*Identity)
		reason MissReason
	}{
		{name: "Apex digest", change: func(id *Identity) { id.ProjectContentSHA256 = "changed" }, reason: MissIdentityMismatch},
		{name: "schema digest", change: func(id *Identity) { id.SchemaContentSHA256 = "changed" }, reason: MissIdentityMismatch},
		{name: "dependency digest", change: func(id *Identity) { id.DependencySHA256 = "changed" }, reason: MissIdentityMismatch},
		{name: "semantic ABI", change: func(id *Identity) { id.SemanticABI = "changed" }, reason: MissIdentityMismatch},
		{name: "platform ABI", change: func(id *Identity) { id.PlatformABI = "changed" }, reason: MissIdentityMismatch},
		{name: "options fingerprint", change: func(id *Identity) { id.OptionsFingerprint = "changed" }, reason: MissOptionsMismatch},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			expected := identity
			tc.change(&expected)
			got, err := loadTestPath(path, expected)
			if !reflect.DeepEqual(got, sema.Result{}) {
				t.Fatalf("miss returned a partial result: %#v", got)
			}
			assertMissReason(t, err, tc.reason)
		})
	}
}

func TestCacheAbsentFileReportsAbsent(t *testing.T) {
	got, err := loadTestPath(filepath.Join(t.TempDir(), "missing.json"), testIdentity())
	if !reflect.DeepEqual(got, sema.Result{}) {
		t.Fatalf("miss returned a partial result: %#v", got)
	}
	assertMissReason(t, err, MissAbsent)
}

func TestCacheRejectsIncompleteIdentity(t *testing.T) {
	complete := testIdentity()
	tests := []struct {
		name  string
		clear func(*Identity)
	}{
		{name: "project digest", clear: func(id *Identity) { id.ProjectContentSHA256 = "" }},
		{name: "schema digest", clear: func(id *Identity) { id.SchemaContentSHA256 = "" }},
		{name: "dependency digest", clear: func(id *Identity) { id.DependencySHA256 = "" }},
		{name: "semantic ABI", clear: func(id *Identity) { id.SemanticABI = "" }},
		{name: "platform ABI", clear: func(id *Identity) { id.PlatformABI = "" }},
		{name: "options fingerprint", clear: func(id *Identity) { id.OptionsFingerprint = "" }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			identity := complete
			tc.clear(&identity)
			if err := storeTestPath(filepath.Join(t.TempDir(), "semantic.json"), identity, smallTestResult()); err == nil {
				t.Fatal("Store succeeded with an incomplete semantic identity")
			}
		})
	}
}

func TestCacheIncompleteExpectedIdentityFailsClosed(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.json")
	identity := testIdentity()
	identity.ProjectContentSHA256 = ""
	payload := checksumPayload{
		Version:  EnvelopeVersion,
		Identity: identity,
		Result:   sema.SnapshotResult(smallTestResult()),
	}
	checksum, err := payloadChecksum(payload)
	if err != nil {
		t.Fatal(err)
	}
	data, err := json.Marshal(diskEnvelope{
		Version:        payload.Version,
		Identity:       payload.Identity,
		Result:         payload.Result,
		ChecksumSHA256: checksum,
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data, 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := loadTestPath(path, identity)
	if !reflect.DeepEqual(got, sema.Result{}) {
		t.Fatalf("incomplete proof returned a result: %#v", got)
	}
	assertMissReason(t, err, MissIdentityMismatch)
}

func TestEnvelopeTruncatedFileReportsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.json")
	if err := storeTestPath(path, testIdentity(), smallTestResult()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, data[:len(data)/2], 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := loadTestPath(path, testIdentity())
	if !reflect.DeepEqual(got, sema.Result{}) {
		t.Fatalf("corrupt file returned a partial result: %#v", got)
	}
	assertMissReason(t, err, MissCorrupt)
}

func TestEnvelopeOversizedFileReportsCorruptWithoutReadingIt(t *testing.T) {
	projectRoot := t.TempDir()
	cacheDir := filepath.Join(projectRoot, "cache")
	if err := os.Mkdir(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cacheDir, "semantic.json")
	file, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := file.Truncate(maxEnvelopeBytes + 1); err != nil {
		_ = file.Close()
		t.Fatal(err)
	}
	if err := file.Close(); err != nil {
		t.Fatal(err)
	}

	got, err := Load(projectRoot, "cache/semantic.json", testIdentity())
	if !reflect.DeepEqual(got, sema.Result{}) {
		t.Fatalf("oversized file returned a partial result: %#v", got)
	}
	assertMissReason(t, err, MissCorrupt)
}

func TestEnvelopeChecksumFailureReportsCorrupt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.json")
	if err := storeTestPath(path, testIdentity(), smallTestResult()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	mutated := bytes.Replace(data, []byte("test diagnostic"), []byte("changed diagnostic"), 1)
	if bytes.Equal(mutated, data) {
		t.Fatal("fixture diagnostic was not present in envelope")
	}
	if err := os.WriteFile(path, mutated, 0o600); err != nil {
		t.Fatal(err)
	}

	_, err = loadTestPath(path, testIdentity())
	assertMissReason(t, err, MissCorrupt)
}

func TestEnvelopeUnknownVersionReportsUnsupported(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.json")
	if err := os.WriteFile(path, []byte(`{"version":999}`), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := loadTestPath(path, testIdentity())
	if !reflect.DeepEqual(got, sema.Result{}) {
		t.Fatalf("unsupported file returned a partial result: %#v", got)
	}
	assertMissReason(t, err, MissUnsupportedVersion)
}

func TestConcurrentReadersObserveOnlyCompleteEnvelopes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.json")
	identity := testIdentity()
	first := smallTestResult()
	first.Summary.Types = 101
	second := smallTestResult()
	second.Summary.Types = 202
	if err := storeTestPath(path, identity, first); err != nil {
		t.Fatal(err)
	}

	const readerCount = 4
	const writes = 24
	start := make(chan struct{})
	errs := make(chan error, readerCount)
	var readers sync.WaitGroup
	for range readerCount {
		readers.Add(1)
		go func() {
			defer readers.Done()
			<-start
			for range writes {
				got, err := loadTestPath(path, identity)
				if err != nil {
					errs <- err
					return
				}
				if got.Summary.Types != first.Summary.Types && got.Summary.Types != second.Summary.Types {
					errs <- errors.New("reader observed a partial semantic result")
					return
				}
			}
		}()
	}
	close(start)
	for i := range writes {
		value := first
		if i%2 == 1 {
			value = second
		}
		if err := storeTestPath(path, identity, value); err != nil {
			t.Fatal(err)
		}
	}
	readers.Wait()
	close(errs)
	for err := range errs {
		t.Error(err)
	}
}

func TestCacheFilePermissionsArePrivate(t *testing.T) {
	path := filepath.Join(t.TempDir(), "nested", "semantic.json")
	if err := storeTestPath(path, testIdentity(), smallTestResult()); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("cache mode = %#o, want 0600", got)
	}
	dirInfo, err := os.Stat(filepath.Dir(path))
	if err != nil {
		t.Fatal(err)
	}
	if got := dirInfo.Mode().Perm(); got != 0o700 {
		t.Fatalf("cache directory mode = %#o, want 0700", got)
	}
}

func TestStoreRejectsSymlinkedCacheDirectory(t *testing.T) {
	projectRoot := t.TempDir()
	external := t.TempDir()
	if err := os.WriteFile(filepath.Join(external, "sentinel"), []byte("unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(external, filepath.Join(projectRoot, "cache")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}

	err := Store(projectRoot, "cache/semantic.json", testIdentity(), smallTestResult())
	if err == nil {
		t.Fatal("Store succeeded through a symlinked cache directory")
	}
	if _, err := os.Stat(filepath.Join(external, "semantic.json")); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("redirected semantic cache file exists: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(external, "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "unchanged\n" {
		t.Fatalf("external sentinel changed: %q", data)
	}
}

func TestLoadRejectsSymlinkedCachePathsAsCorrupt(t *testing.T) {
	for _, tc := range []struct {
		name string
		link func(t *testing.T, projectRoot, target string)
	}{
		{
			name: "directory",
			link: func(t *testing.T, projectRoot, target string) {
				t.Helper()
				if err := os.Symlink(filepath.Base(target), filepath.Join(projectRoot, "cache")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			},
		},
		{
			name: "file",
			link: func(t *testing.T, projectRoot, target string) {
				t.Helper()
				cacheDir := filepath.Join(projectRoot, "cache")
				if err := os.Mkdir(cacheDir, 0o700); err != nil {
					t.Fatal(err)
				}
				if err := os.Symlink(filepath.Join("..", filepath.Base(target), "semantic.json"), filepath.Join(cacheDir, "semantic.json")); err != nil {
					t.Skipf("symlinks unavailable: %v", err)
				}
			},
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			projectRoot := t.TempDir()
			target := filepath.Join(projectRoot, "real")
			if err := Store(projectRoot, "real/semantic.json", testIdentity(), smallTestResult()); err != nil {
				t.Fatal(err)
			}
			tc.link(t, projectRoot, target)

			got, err := Load(projectRoot, "cache/semantic.json", testIdentity())
			if !reflect.DeepEqual(got, sema.Result{}) {
				t.Fatalf("symlinked cache path returned a result: %#v", got)
			}
			assertMissReason(t, err, MissCorrupt)
		})
	}
}

func TestOpenPrivateCacheDirRejectsComponentSwap(t *testing.T) {
	projectRoot := t.TempDir()
	cacheDir := filepath.Join(projectRoot, "cache")
	displaced := filepath.Join(projectRoot, "displaced-cache")
	replacement := filepath.Join(projectRoot, "replacement")
	if err := os.Mkdir(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(replacement, 0o700); err != nil {
		t.Fatal(err)
	}
	hook := func(component string) error {
		if component != "cache" {
			return nil
		}
		if err := os.Rename(cacheDir, displaced); err != nil {
			return err
		}
		return os.Rename(replacement, cacheDir)
	}

	root, err := openPrivateCacheDirAfterLstat(projectRoot, "cache", false, hook)
	if root != nil {
		_ = root.Close()
	}
	if err == nil {
		t.Fatal("openPrivateCacheDirAfterLstat succeeded after a component swap")
	}
}

func TestEnvelopeRoundTripPreservesCanonicalSemanticResult(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.json")
	identity := testIdentity()
	want := analyzedTestResult()
	if err := storeTestPath(path, identity, want); err != nil {
		t.Fatal(err)
	}
	got, err := loadTestPath(path, identity)
	if err != nil {
		t.Fatal(err)
	}

	wantCanonical := canonicalResult(t, want)
	gotCanonical := canonicalResult(t, got)
	if string(gotCanonical) != string(wantCanonical) {
		t.Fatalf("canonical result changed\nwant: %s\n got: %s", wantCanonical, gotCanonical)
	}
}

func TestEnvelopeDecodedMutationDoesNotAffectOriginalOrLaterLoad(t *testing.T) {
	path := filepath.Join(t.TempDir(), "semantic.json")
	identity := testIdentity()
	original := smallTestResult()
	if err := storeTestPath(path, identity, original); err != nil {
		t.Fatal(err)
	}
	first, err := loadTestPath(path, identity)
	if err != nil {
		t.Fatal(err)
	}
	first.Diagnostics[0].Message = "mutated"
	first.Diagnostics[0].Range.Start.Line = 99
	first.Types["account"] = sema.TypeReference{Name: "Mutated"}

	if original.Diagnostics[0].Message == "mutated" || original.Diagnostics[0].Range.Start.Line == 99 {
		t.Fatalf("decoded diagnostic aliases original: %#v", original.Diagnostics[0])
	}
	if original.Types["account"].Name == "Mutated" {
		t.Fatalf("decoded types alias original: %#v", original.Types)
	}
	again, err := loadTestPath(path, identity)
	if err != nil {
		t.Fatal(err)
	}
	if again.Diagnostics[0].Message == "mutated" || again.Diagnostics[0].Range.Start.Line == 99 {
		t.Fatalf("decoded diagnostic aliases later load: %#v", again.Diagnostics[0])
	}
	if again.Types["account"].Name == "Mutated" {
		t.Fatalf("decoded types alias later load: %#v", again.Types)
	}
}

func TestClearRemovesOnlyVerifiedSemanticCacheEntries(t *testing.T) {
	root := t.TempDir()
	const relativePath = ".glade/semantic/result.json"
	if err := Store(root, relativePath, testIdentity(), smallTestResult()); err != nil {
		t.Fatal(err)
	}
	if err := Clear(root); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root, relativePath, testIdentity()); !errors.Is(err, ErrMiss) {
		t.Fatalf("Load() after Clear = %v, want miss", err)
	}
}

func storeTestPath(path string, identity Identity, result sema.Result) error {
	parent := filepath.Dir(path)
	return Store(filepath.Dir(parent), filepath.Join(filepath.Base(parent), filepath.Base(path)), identity, result)
}

func loadTestPath(path string, identity Identity) (sema.Result, error) {
	parent := filepath.Dir(path)
	return Load(filepath.Dir(parent), filepath.Join(filepath.Base(parent), filepath.Base(path)), identity)
}

func testIdentity() Identity {
	return Identity{
		ProjectContentSHA256: "project",
		SchemaContentSHA256:  "schema",
		DependencySHA256:     "dependency",
		SemanticABI:          sema.SemanticABI,
		PlatformABI:          "platform",
		OptionsFingerprint: sema.AnalyzeOptionsFingerprint(sema.AnalyzeOptions{
			Diagnostics:                    true,
			ExportTypes:                    true,
			SuppressPerformanceDiagnostics: true,
		}),
	}
}

var (
	analyzedResultOnce sync.Once
	analyzedResult     sema.Result
)

func analyzedTestResult() sema.Result {
	analyzedResultOnce.Do(func() {
		analyzedResult = sema.AnalyzeWithOptions(typesys.Index{
			Project: typesys.ProjectInfo{
				Root:             "/workspace",
				Namespace:        "sample",
				SourceAPIVersion: "65.0",
			},
			Diagnostics: []diagnostic.Diagnostic{{
				Severity: diagnostic.Error,
				Code:     "GLADETEST001",
				Message:  "test diagnostic",
				File:     "Example.cls",
				Range: &diagnostic.Range{
					Start: diagnostic.Position{Line: 2, Column: 3, Offset: 4},
					End:   diagnostic.Position{Line: 2, Column: 8, Offset: 9},
				},
				Excerpt: "missing",
			}},
		}, sema.AnalyzeOptions{
			Diagnostics:                    true,
			ExportTypes:                    true,
			SuppressPerformanceDiagnostics: true,
		})
	})
	return sema.SnapshotResult(analyzedResult).Result()
}

func smallTestResult() sema.Result {
	return sema.Result{
		Project: typesys.ProjectInfo{
			Root:             "/workspace",
			Namespace:        "sample",
			SourceAPIVersion: "65.0",
		},
		Summary: sema.Summary{
			Types:       2,
			Diagnostics: 1,
		},
		Diagnostics: []diagnostic.Diagnostic{{
			Severity: diagnostic.Error,
			Code:     "GLADETEST001",
			Message:  "test diagnostic",
			File:     "Example.cls",
			Range: &diagnostic.Range{
				Start: diagnostic.Position{Line: 2, Column: 3, Offset: 4},
				End:   diagnostic.Position{Line: 2, Column: 8, Offset: 9},
			},
			Excerpt: "missing",
		}},
		Types: map[string]sema.TypeReference{
			"account": {Name: "Account", Kind: sema.TypeSchema},
			"example": {Name: "Example", Kind: sema.TypeApex, Source: "Example.cls"},
		},
	}
}

func canonicalResult(t *testing.T, result sema.Result) []byte {
	t.Helper()
	diagnostics := append([]diagnostic.Diagnostic(nil), result.Diagnostics...)
	sort.Slice(diagnostics, func(i, j int) bool {
		left, _ := json.Marshal(diagnostics[i])
		right, _ := json.Marshal(diagnostics[j])
		return string(left) < string(right)
	})
	type exportedType struct {
		Key       string             `json:"key"`
		Reference sema.TypeReference `json:"reference"`
	}
	types := make([]exportedType, 0, len(result.Types))
	for key, reference := range result.Types {
		types = append(types, exportedType{Key: key, Reference: reference})
	}
	sort.Slice(types, func(i, j int) bool { return types[i].Key < types[j].Key })
	data, err := json.Marshal(struct {
		Project     typesys.ProjectInfo     `json:"project"`
		Summary     sema.Summary            `json:"summary"`
		Diagnostics []diagnostic.Diagnostic `json:"diagnostics"`
		Types       []exportedType          `json:"types"`
	}{
		Project:     result.Project,
		Summary:     result.Summary,
		Diagnostics: diagnostics,
		Types:       types,
	})
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func assertMissReason(t *testing.T, err error, want MissReason) {
	t.Helper()
	if err == nil {
		t.Fatalf("Load succeeded, want %s miss", want)
	}
	if !errors.Is(err, ErrMiss) {
		t.Fatalf("Load error %v does not match ErrMiss", err)
	}
	var miss *MissError
	if !errors.As(err, &miss) {
		t.Fatalf("Load error %T is not *MissError", err)
	}
	if miss.Reason != want {
		t.Fatalf("miss reason = %q, want %q", miss.Reason, want)
	}
}
