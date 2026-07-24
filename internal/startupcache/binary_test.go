package startupcache

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

func TestWriteWithStatsCreatesPrivateTestCacheDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not available on Windows")
	}
	dir := t.TempDir()
	entry := Entry{Version: Version, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}

	if _, err := WriteWithStats(&entry, SubdirTest); err != nil {
		t.Fatalf("WriteWithStats() error = %v", err)
	}
	info, err := os.Stat(filepath.Join(dir, ".glade", "test"))
	if err != nil {
		t.Fatalf("Stat() cache directory error = %v", err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("cache directory permissions = %04o, want %04o", got, want)
	}
}

func TestBuildManifestWithSourceDigestsUsesAllOrNothingApexOverlay(t *testing.T) {
	root := t.TempDir()
	dependencyRoot := t.TempDir()
	mainApex := filepath.Join(root, "Main.cls")
	dependencyApex := filepath.Join(dependencyRoot, "Dependency.cls")
	objectMetadata := filepath.Join(root, "Thing__c.object-meta.xml")
	writeStartupCacheTestFile(t, mainApex, "public class Main {}\n")
	writeStartupCacheTestFile(t, dependencyApex, "public class Dependency {}\n")
	writeStartupCacheTestFile(t, objectMetadata, "<CustomObject/>\n")
	dependency := project.Project{Root: dependencyRoot, Namespace: "dep", ApexFiles: []string{dependencyApex}}
	p := project.Project{
		Root:        root,
		ApexFiles:   []string{mainApex},
		ObjectFiles: []string{objectMetadata},
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace: "dep",
			Status:    "loaded",
			Project:   &dependency,
		}},
	}
	index, complete := typesys.BuildWithArtifacts(p, schema.Schema{})
	_, incomplete := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{mainApex}}, schema.Schema{})

	reads := make(map[string]int)
	readFile := func(path string) ([]byte, error) {
		reads[filepath.Clean(path)]++
		return os.ReadFile(path)
	}
	fromSnapshot, err := buildManifestWithSourceDigests(root, p, true, complete.SourceDigests, readFile)
	if err != nil {
		t.Fatal(err)
	}
	if reads[mainApex] != 0 || reads[dependencyApex] != 0 || reads[objectMetadata] != 1 {
		t.Fatalf("complete overlay reads = %#v, want Apex 0/0 and non-Apex 1", reads)
	}
	if fromDisk := BuildManifest(root, p, index); fmt.Sprintf("%#v", fromSnapshot) != fmt.Sprintf("%#v", fromDisk) {
		t.Fatalf("snapshot manifest differs from disk manifest:\n snapshot=%#v\n disk=%#v", fromSnapshot, fromDisk)
	}

	clear(reads)
	fromFallback, err := buildManifestWithSourceDigests(root, p, true, incomplete.SourceDigests, readFile)
	if err != nil {
		t.Fatal(err)
	}
	if reads[mainApex] != 1 || reads[dependencyApex] != 1 || reads[objectMetadata] != 1 {
		t.Fatalf("incomplete overlay reads = %#v, want all-disk 1/1/1", reads)
	}
	if fmt.Sprintf("%#v", fromFallback) != fmt.Sprintf("%#v", fromSnapshot) {
		t.Fatalf("fallback manifest differs from complete manifest:\n fallback=%#v\n complete=%#v", fromFallback, fromSnapshot)
	}
}

func TestFreshManifestWithSourceDigestsUsesExactApexReadCounts(t *testing.T) {
	root := t.TempDir()
	first := filepath.Join(root, "force-app", "main", "default", "classes", "First.cls")
	second := filepath.Join(root, "force-app", "main", "default", "classes", "Second.cls")
	writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeStartupCacheTestFile(t, first, "public class First {}\n")
	writeStartupCacheTestFile(t, second, "public class Second {}\n")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index, complete := typesys.BuildWithArtifacts(p, schema.Schema{})
	_, incomplete := typesys.BuildWithArtifacts(project.Project{Root: root, ApexFiles: []string{first}}, schema.Schema{})
	manifest := BuildManifestWithSourceDigests(root, p, index, complete.SourceDigests)

	reads := make(map[string]int)
	readFile := func(path string) ([]byte, error) {
		reads[filepath.Clean(path)]++
		return os.ReadFile(path)
	}
	if !freshManifestWithSourceDigests(Version, root, currentPlatformABI(), manifest, root, Version, complete.SourceDigests, readFile) {
		t.Fatal("complete snapshot manifest is stale")
	}
	if reads[first] != 0 || reads[second] != 0 {
		t.Fatalf("complete snapshot Apex reads = %d/%d, want 0/0", reads[first], reads[second])
	}

	clear(reads)
	if !freshManifestWithSourceDigests(Version, root, currentPlatformABI(), manifest, root, Version, incomplete.SourceDigests, readFile) {
		t.Fatal("all-disk fallback manifest is stale")
	}
	if reads[first] != 1 || reads[second] != 1 {
		t.Fatalf("incomplete snapshot Apex reads = %d/%d, want all-disk 1/1", reads[first], reads[second])
	}
}

func TestSplitCacheRuntimeKeyRoundTripsAndKeylessEntryRemainsReadable(t *testing.T) {
	root := t.TempDir()
	writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[]}`)
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	entry := NewEntry(root, p, typesys.Index{}, storage.NewOrgState(), CompiledRuntime{})
	entry.RuntimeABI = "runtime-v5"
	entry.RuntimeKey = "runtime-key"
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatal(err)
	}
	got, err := Read(root, SubdirTest)
	if err != nil || got == nil {
		t.Fatalf("Read() = %#v, %v", got, err)
	}
	if got.RuntimeKey != entry.RuntimeKey {
		t.Fatalf("RuntimeKey = %q, want %q", got.RuntimeKey, entry.RuntimeKey)
	}
	header, err := readTestCacheHeader(filepath.Join(root, ".glade", "test", stateHeaderFile))
	if err != nil {
		t.Fatal(err)
	}
	if header.RuntimeKey != entry.RuntimeKey {
		t.Fatalf("header RuntimeKey = %q, want %q", header.RuntimeKey, entry.RuntimeKey)
	}

	entry.RuntimeKey = ""
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatal(err)
	}
	got, err = Read(root, SubdirTest)
	if err != nil || got == nil || got.RuntimeKey != "" {
		t.Fatalf("keyless Read() = %#v, %v", got, err)
	}
}

func TestReadFreshRuntimeWithSourceDigestsValidatesKeyBeforePayload(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app", "main", "default", "classes", "Cached.cls")
	writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeStartupCacheTestFile(t, classPath, "public class Cached {}\n")
	p, err := project.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	index, artifacts := typesys.BuildWithArtifacts(p, schema.Schema{})
	entry := NewEntryWithSourceDigests(root, p, index, artifacts.SourceDigests, storage.NewOrgState(), CompiledRuntime{})
	entry.RuntimeABI = "runtime-v5"
	entry.RuntimeKey = "runtime-key"
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatal(err)
	}

	apexReads := 0
	manifestReads := 0
	readFile := func(path string) ([]byte, error) {
		manifestReads++
		if filepath.Clean(path) == classPath {
			apexReads++
		}
		return os.ReadFile(path)
	}
	got, err := readFreshRuntimeWithSourceDigests(root, SubdirTest, Version, "runtime-v5", "runtime-key", artifacts.SourceDigests, readFile)
	if err != nil || got == nil {
		t.Fatalf("fresh read = %#v, %v", got, err)
	}
	if apexReads != 0 {
		t.Fatalf("fresh read Apex reads = %d, want 0", apexReads)
	}
	if manifestReads == 0 {
		t.Fatal("eligible header did not validate non-Apex manifest inputs")
	}

	header, err := readTestCacheHeader(filepath.Join(root, ".glade", "test", stateHeaderFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(filepath.Join(root, ".glade", "test", header.PayloadFile)); err != nil {
		t.Fatal(err)
	}
	manifestReads = 0
	got, err = readFreshRuntimeWithSourceDigests(root, SubdirTest, Version, "runtime-v5", "wrong-key", artifacts.SourceDigests, readFile)
	if err != nil || got != nil {
		t.Fatalf("mismatched key should reject before missing payload: got=%#v err=%v", got, err)
	}
	if manifestReads != 0 {
		t.Fatalf("mismatched key manifest reads = %d, want 0", manifestReads)
	}
	if _, err := readFreshRuntimeWithSourceDigests(root, SubdirTest, Version, "runtime-v5", "runtime-key", artifacts.SourceDigests, readFile); err == nil {
		t.Fatal("exact key did not reach missing payload")
	}
	got, err = readFreshRuntimeWithSourceDigests(root, SubdirTest, Version, "runtime-v5", "", artifacts.SourceDigests, readFile)
	if err != nil || got != nil {
		t.Fatalf("keyless lookup should fail closed before payload: got=%#v err=%v", got, err)
	}
}

func TestWriteWithStatsRestrictsExistingTestCacheDirectory(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission bits are not available on Windows")
	}
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, ".glade", "test")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	entry := Entry{Version: Version, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}

	if _, err := WriteWithStats(&entry, SubdirTest); err != nil {
		t.Fatalf("WriteWithStats() error = %v", err)
	}
	info, err := os.Stat(cacheDir)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o700); got != want {
		t.Fatalf("cache directory permissions = %04o, want %04o", got, want)
	}
}

func TestWriteWithStatsRejectsSymlinkTestCacheDirectoryWithoutChangingTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink and permission behavior is not available on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(t.TempDir(), "target")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	cacheDir := filepath.Join(dir, ".glade", "test")
	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(target, cacheDir); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	entry := Entry{Version: Version, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}

	if _, err := WriteWithStats(&entry, SubdirTest); err == nil {
		t.Fatal("WriteWithStats() error = nil, want symlink rejection")
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o755); got != want {
		t.Fatalf("symlink target permissions = %04o, want unchanged %04o", got, want)
	}
}

func TestWriteWithStatsRejectsSymlinkedCacheAncestorWithoutChangingTarget(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink and permission behavior is not available on Windows")
	}
	dir := t.TempDir()
	externalRoot := t.TempDir()
	externalCache := filepath.Join(externalRoot, "test")
	if err := os.Mkdir(externalCache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(externalRoot, filepath.Join(dir, ".glade")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	entry := Entry{Version: Version, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}

	if _, err := WriteWithStats(&entry, SubdirTest); err == nil {
		t.Fatal("WriteWithStats() error = nil, want symlinked ancestor rejection")
	}
	info, err := os.Stat(externalCache)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o755); got != want {
		t.Fatalf("external cache permissions = %04o, want unchanged %04o", got, want)
	}
}

func TestWriteWithStatsRejectsInProjectSymlinkedCacheAncestor(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink and permission behavior is not available on Windows")
	}
	dir := t.TempDir()
	target := filepath.Join(dir, "force-app")
	if err := os.Mkdir(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "sentinel"), []byte("unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("force-app", filepath.Join(dir, ".glade")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	entry := Entry{Version: Version, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}

	if _, err := WriteWithStats(&entry, SubdirTest); err == nil {
		t.Fatal("WriteWithStats() error = nil, want in-project ancestor symlink rejection")
	}
	assertSentinelDirectoryUnchanged(t, target, 0o755)
}

func TestWriteWithStatsRejectsInProjectSymlinkedCacheLeaf(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink and permission behavior is not available on Windows")
	}
	dir := t.TempDir()
	gladeDir := filepath.Join(dir, ".glade")
	target := filepath.Join(gladeDir, "sibling")
	if err := os.MkdirAll(target, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(target, "sentinel"), []byte("unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink("sibling", filepath.Join(gladeDir, "test")); err != nil {
		t.Skipf("symlinks unavailable: %v", err)
	}
	entry := Entry{Version: Version, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}

	if _, err := WriteWithStats(&entry, SubdirTest); err == nil {
		t.Fatal("WriteWithStats() error = nil, want in-project leaf symlink rejection")
	}
	assertSentinelDirectoryUnchanged(t, target, 0o755)
}

func TestOpenPrivateTestCacheDirRejectsComponentSwap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix rename behavior is not available on Windows")
	}
	dir := t.TempDir()
	gladeDir := filepath.Join(dir, ".glade")
	cacheDir := filepath.Join(gladeDir, "test")
	originalDir := filepath.Join(gladeDir, "original-test")
	replacementDir := filepath.Join(gladeDir, "replacement")
	if err := os.MkdirAll(cacheDir, 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(replacementDir, 0o700); err != nil {
		t.Fatal(err)
	}
	hook := func(component string) error {
		if component != "test" {
			return nil
		}
		if err := os.Rename(cacheDir, originalDir); err != nil {
			return err
		}
		return os.Rename(replacementDir, cacheDir)
	}

	root, err := openPrivateTestCacheDirAfterLstat(dir, SubdirTest, hook)
	if root != nil {
		_ = root.Close()
	}
	if err == nil {
		t.Fatal("openPrivateTestCacheDirAfterLstat() error = nil, want component identity mismatch")
	}
	if _, err := os.Stat(originalDir); err != nil {
		t.Fatalf("original directory missing after swap: %v", err)
	}
	if _, err := os.Stat(cacheDir); err != nil {
		t.Fatalf("replacement directory missing after swap: %v", err)
	}
}

func assertSentinelDirectoryUnchanged(t *testing.T, directory string, wantMode os.FileMode) {
	t.Helper()
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "sentinel" {
		t.Fatalf("directory contents changed: %v", entries)
	}
	data, err := os.ReadFile(filepath.Join(directory, "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "unchanged\n" {
		t.Fatalf("sentinel changed: %q", data)
	}
	info, err := os.Stat(directory)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != wantMode {
		t.Fatalf("directory permissions = %04o, want unchanged %04o", got, wantMode)
	}
}

func TestWriteWithStatsRejectsNonDirectoryTestCachePath(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, ".glade", "test")
	if err := os.MkdirAll(filepath.Dir(cacheDir), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(cacheDir, []byte("not a directory\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := Entry{Version: Version, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}

	if _, err := WriteWithStats(&entry, SubdirTest); err == nil {
		t.Fatal("WriteWithStats() error = nil, want non-directory rejection")
	}
}

func TestWriteWithStatsContinuesInOpenedCacheAfterPathSwap(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix symlink and rename behavior is not available on Windows")
	}
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, ".glade", "test")
	displacedCache := filepath.Join(dir, ".glade", "opened-test")
	externalCache := filepath.Join(t.TempDir(), "external")
	if err := os.Mkdir(externalCache, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(externalCache, "sentinel"), []byte("unchanged\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	entry := Entry{Version: Version, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}
	hook := func() error {
		if err := os.Rename(cacheDir, displacedCache); err != nil {
			return err
		}
		return os.Symlink(externalCache, cacheDir)
	}

	if _, err := writeSplitTestCacheWithStatsAfterRootOpened(&entry, SubdirTest, hook); err != nil {
		t.Fatalf("writeSplitTestCacheWithStatsAfterRootOpened() error = %v", err)
	}
	entries, err := os.ReadDir(externalCache)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "sentinel" {
		t.Fatalf("external cache contents changed: %v", entries)
	}
	sentinel, err := os.ReadFile(filepath.Join(externalCache, "sentinel"))
	if err != nil {
		t.Fatal(err)
	}
	if string(sentinel) != "unchanged\n" {
		t.Fatalf("external sentinel changed: %q", sentinel)
	}
	info, err := os.Stat(externalCache)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := info.Mode().Perm(), os.FileMode(0o755); got != want {
		t.Fatalf("external cache permissions = %04o, want unchanged %04o", got, want)
	}
	headerData, err := os.ReadFile(filepath.Join(displacedCache, stateHeaderFile))
	if err != nil {
		t.Fatalf("read header from opened cache: %v", err)
	}
	var header testCacheHeader
	if err := json.Unmarshal(headerData, &header); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(displacedCache, header.PayloadFile)); err != nil {
		t.Fatalf("stat payload in opened cache: %v", err)
	}
}

func TestLegacyGobRoundTrip(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		BuiltAt:     "2026-06-09T00:00:00Z",
		Manifest: Manifest{
			ProjectRoot: dir,
			Files:       []File{{Path: "classes/Foo.cls", Size: 12, ModTime: 1}},
		},
		Org: storage.OrgState{
			APIVersion: "64.0",
			Objects: map[string]storage.ObjectState{
				"Account": {
					Definition: storage.ObjectDefinition{
						APIName: "Account",
						Fields: map[string]storage.Field{
							"Name": {APIName: "Name", Type: storage.FieldString},
						},
					},
				},
			},
		},
		Runtime: CompiledRuntime{
			Methods: map[string]vm.Method{
				"Foo.bar": {Name: "bar", ClassName: "Foo"},
			},
		},
	}
	if err := writeLegacyGob(&entry, SubdirTest); err != nil {
		t.Fatalf("writeLegacyGob() error = %v", err)
	}
	gobPath := filepath.Join(dir, ".glade", "test", stateGobFile)
	if _, err := os.Stat(gobPath); err != nil {
		t.Fatalf("gob file missing: %v", err)
	}
	got, err := Read(dir, SubdirTest)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got == nil {
		t.Fatal("Read() = nil")
	}
	if got.Version != entry.Version {
		t.Fatalf("version = %d, want %d", got.Version, entry.Version)
	}
	if got.Runtime.Methods["Foo.bar"].Name != "bar" {
		t.Fatalf("method name = %q", got.Runtime.Methods["Foo.bar"].Name)
	}
	if got.Org.Objects["Account"].Definition.APIName != "Account" {
		t.Fatalf("org object api name = %q", got.Org.Objects["Account"].Definition.APIName)
	}
}

func TestClearRemovesGob(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{Version: Version, ProjectRoot: dir}
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	headerPath := filepath.Join(dir, ".glade", "test", stateHeaderFile)
	headerData, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var header testCacheHeader
	if err := json.Unmarshal(headerData, &header); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if err := Clear(dir, SubdirTest); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".glade", "test", stateGobFile)); !os.IsNotExist(err) {
		t.Fatalf("gob still present after Clear(): %v", err)
	}
	if _, err := os.Stat(headerPath); !os.IsNotExist(err) {
		t.Fatalf("header still present after Clear(): %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".glade", "test", header.PayloadFile)); !os.IsNotExist(err) {
		t.Fatalf("payload still present after Clear(): %v", err)
	}
}

func TestFreshRejectsMissingManifestProjectFile(t *testing.T) {
	dir := t.TempDir()
	classPath := filepath.Join(dir, "classes", "Foo.cls")
	if err := os.MkdirAll(filepath.Dir(classPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(classPath, []byte("public class Foo {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	fp, ok := statFile(dir, classPath)
	if !ok {
		t.Fatal("statFile() did not fingerprint class file")
	}
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		Manifest: Manifest{
			ProjectRoot: dir,
			Files:       []File{fp},
		},
	}
	if err := os.Remove(classPath); err != nil {
		t.Fatalf("Remove() error = %v", err)
	}
	if Fresh(&entry, dir, Version) {
		t.Fatal("Fresh() = true, want false for missing manifest project file")
	}
}

func TestFreshRejectsExactInvalidationMatrix(t *testing.T) {
	t.Run("same-size content change with preserved mtime", func(t *testing.T) {
		root, classPath, entry := newExactInvalidationFixture(t, nil)
		info, err := os.Stat(classPath)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		writeStartupCacheTestFile(t, classPath, "public class Exact { Integer value = 2; }\n")
		if err := os.Chtimes(classPath, info.ModTime(), info.ModTime()); err != nil {
			t.Fatalf("Chtimes() error = %v", err)
		}
		if Fresh(&entry, root, Version) {
			t.Fatal("Fresh() = true after same-size source mutation with preserved mtime")
		}
	})

	t.Run("add nested project file", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, nil)
		writeStartupCacheTestFile(t, filepath.Join(root, "force-app", "main", "default", "classes", "nested", "Added.cls"), "public class Added {}\n")
		assertStartupCacheStale(t, &entry, root, "nested source addition")
	})

	t.Run("delete nested project file", func(t *testing.T) {
		root, classPath, entry := newExactInvalidationFixture(t, nil)
		if err := os.Remove(classPath); err != nil {
			t.Fatalf("Remove() error = %v", err)
		}
		assertStartupCacheStale(t, &entry, root, "nested source deletion")
	})

	t.Run("rename nested project file", func(t *testing.T) {
		root, classPath, entry := newExactInvalidationFixture(t, nil)
		renamed := filepath.Join(filepath.Dir(classPath), "Renamed.cls")
		if err := os.Rename(classPath, renamed); err != nil {
			t.Fatalf("Rename() error = %v", err)
		}
		assertStartupCacheStale(t, &entry, root, "nested source rename")
	})

	t.Run("branch-like package replacement", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, nil)
		packageRoot := filepath.Join(root, "force-app")
		info, err := os.Stat(packageRoot)
		if err != nil {
			t.Fatalf("Stat() error = %v", err)
		}
		if err := os.Rename(packageRoot, filepath.Join(root, "old-force-app")); err != nil {
			t.Fatalf("Rename() old package error = %v", err)
		}
		writeStartupCacheTestFile(t, filepath.Join(packageRoot, "main", "default", "classes", "Branch.cls"), "public class Branch {}\n")
		if err := os.Chtimes(packageRoot, info.ModTime(), info.ModTime()); err != nil {
			t.Fatalf("Chtimes() package root error = %v", err)
		}
		assertStartupCacheStale(t, &entry, root, "branch-like package replacement")
	})

	t.Run("API version change", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, nil)
		writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)
		assertStartupCacheStale(t, &entry, root, "source API version change")
	})

	t.Run("effective config addition", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, nil)
		writeStartupCacheTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  defaultNamespace: exactns\n")
		assertStartupCacheStale(t, &entry, root, "glade.yml addition")
	})

	t.Run("project-root redirect config change", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, func(root string) {
			writeStartupCacheTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  root: nested\n# branch a\n")
			writeStartupCacheTestFile(t, filepath.Join(root, "nested", "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeStartupCacheTestFile(t, filepath.Join(root, "nested", "force-app", "main", "default", "classes", "Redirected.cls"), "public class Redirected {}\n")
		})
		configPath := filepath.Join(root, "glade.yml")
		info, err := os.Stat(configPath)
		if err != nil {
			t.Fatalf("Stat() redirect config error = %v", err)
		}
		writeStartupCacheTestFile(t, configPath, "project:\n  root: nested\n# branch b\n")
		if err := os.Chtimes(configPath, info.ModTime(), info.ModTime()); err != nil {
			t.Fatalf("Chtimes() redirect config error = %v", err)
		}
		assertStartupCacheStale(t, &entry, root, "project-root redirect config change")
	})

	t.Run("namespace remap change", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, func(root string) {
			writeStartupCacheTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  namespaceRemaps: [\"BasePkg:stagepkg\"]\n")
		})
		writeStartupCacheTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  namespaceRemaps: [\"BasePkg:otherpkg\"]\n")
		assertStartupCacheStale(t, &entry, root, "namespace remap change")
	})

	t.Run("dependency project change", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, func(root string) {
			writeStartupCacheTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  managedPackageDependencies: [\"dep:dependencies/dep\"]\n")
			writeStartupCacheTestFile(t, filepath.Join(root, "dependencies", "dep", "sfdx-project.json"), `{"namespace":"dep","packageDirectories":[{"path":"force-app","default":true}]}`)
			writeStartupCacheTestFile(t, filepath.Join(root, "dependencies", "dep", "force-app", "main", "default", "classes", "Dependency.cls"), "global class Dependency { global static Integer value = 1; }\n")
		})
		writeStartupCacheTestFile(t, filepath.Join(root, "dependencies", "dep", "force-app", "main", "default", "classes", "Dependency.cls"), "global class Dependency { global static Integer value = 2; }\n")
		assertStartupCacheStale(t, &entry, root, "dependency source change")
	})

	t.Run("dependency artifact change", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, func(root string) {
			writeStartupCacheTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  managedPackageDependencies: [\"pkg:artifact:packages/pkg.glade-package.json:1.0\"]\n")
			writeStartupCacheTestFile(t, filepath.Join(root, "packages", "pkg.glade-package.json"), `{"namespace":"pkg","version":"1.0","sourceHash":"aaa"}`)
		})
		artifact := filepath.Join(root, "packages", "pkg.glade-package.json")
		info, err := os.Stat(artifact)
		if err != nil {
			t.Fatalf("Stat() artifact error = %v", err)
		}
		writeStartupCacheTestFile(t, artifact, `{"namespace":"pkg","version":"1.0","sourceHash":"bbb"}`)
		if err := os.Chtimes(artifact, info.ModTime(), info.ModTime()); err != nil {
			t.Fatalf("Chtimes() artifact error = %v", err)
		}
		assertStartupCacheStale(t, &entry, root, "artifact content change")
	})

	t.Run("package shim change", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, func(root string) {
			writeStartupCacheTestFile(t, filepath.Join(root, "glade.yml"), "project:\n  packageShims: [\"pkg:test-support/package-shims/pkg\"]\n")
			writeStartupCacheTestFile(t, filepath.Join(root, "test-support", "package-shims", "pkg", "sfdx-project.json"), `{"packageDirectories":[{"path":"classes","default":true}]}`)
			writeStartupCacheTestFile(t, filepath.Join(root, "test-support", "package-shims", "pkg", "classes", "Gateway.cls"), "global class Gateway { global static Integer value = 1; }\n")
		})
		writeStartupCacheTestFile(t, filepath.Join(root, "test-support", "package-shims", "pkg", "classes", "Gateway.cls"), "global class Gateway { global static Integer value = 2; }\n")
		assertStartupCacheStale(t, &entry, root, "package shim source change")
	})

	t.Run("CumulusCI org feature input change", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, func(root string) {
			writeStartupCacheTestFile(t, filepath.Join(root, "cumulusci.yml"), "orgs:\n  scratch:\n    config_file: config/cci-scratch.json\n")
			writeStartupCacheTestFile(t, filepath.Join(root, "config", "cci-scratch.json"), `{"features":["Communities"]}`)
		})
		writeStartupCacheTestFile(t, filepath.Join(root, "config", "cci-scratch.json"), `{"features":["PersonAccounts"]}`)
		assertStartupCacheStale(t, &entry, root, "CumulusCI feature input change")
	})

	t.Run("runtime-discovered input families", func(t *testing.T) {
		type mutationCase struct {
			name   string
			setup  func(t *testing.T, root, classPath string)
			mutate func(t *testing.T, root, classPath string)
		}
		notificationPath := func(root string) string {
			return filepath.Join(root, "force-app", "main", "default", "notificationtypes", "Cache_Notice.notiftype-meta.xml")
		}
		dataPath := func(root string) string {
			return filepath.Join(root, "fixtures", "data", "CacheShape.json")
		}
		apexMeta := func(apiVersion string) string {
			return `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>` + apiVersion + `</apiVersion></ApexClass>`
		}
		notification := func(label string) string {
			return `<CustomNotificationType xmlns="http://soap.sforce.com/2006/04/metadata"><customNotifTypeName>` + label + `</customNotifTypeName></CustomNotificationType>`
		}
		data := func(field string) string {
			return `{"records":{"CacheShape__c":[{"` + field + `":"value"}]}}`
		}
		cases := []mutationCase{
			{
				name: "add Apex metadata sidecar",
				mutate: func(t *testing.T, _, classPath string) {
					writeStartupCacheTestFile(t, classPath+"-meta.xml", apexMeta("61.0"))
				},
			},
			{
				name: "edit Apex metadata sidecar",
				setup: func(t *testing.T, _, classPath string) {
					writeStartupCacheTestFile(t, classPath+"-meta.xml", apexMeta("61.0"))
				},
				mutate: func(t *testing.T, _, classPath string) {
					writeStartupCacheTestFile(t, classPath+"-meta.xml", apexMeta("62.0"))
				},
			},
			{
				name: "delete Apex metadata sidecar",
				setup: func(t *testing.T, _, classPath string) {
					writeStartupCacheTestFile(t, classPath+"-meta.xml", apexMeta("61.0"))
				},
				mutate: func(t *testing.T, _, classPath string) {
					if err := os.Remove(classPath + "-meta.xml"); err != nil {
						t.Fatalf("Remove(Apex metadata sidecar) error = %v", err)
					}
				},
			},
			{
				name: "add notification type",
				mutate: func(t *testing.T, root, _ string) {
					writeStartupCacheTestFile(t, notificationPath(root), notification("Added Label"))
				},
			},
			{
				name: "edit notification type",
				setup: func(t *testing.T, root, _ string) {
					writeStartupCacheTestFile(t, notificationPath(root), notification("Before Label"))
				},
				mutate: func(t *testing.T, root, _ string) {
					writeStartupCacheTestFile(t, notificationPath(root), notification("After Label"))
				},
			},
			{
				name: "delete notification type",
				setup: func(t *testing.T, root, _ string) {
					writeStartupCacheTestFile(t, notificationPath(root), notification("Deleted Label"))
				},
				mutate: func(t *testing.T, root, _ string) {
					if err := os.Remove(notificationPath(root)); err != nil {
						t.Fatalf("Remove(notification type) error = %v", err)
					}
				},
			},
			{
				name: "add project data JSON",
				mutate: func(t *testing.T, root, _ string) {
					writeStartupCacheTestFile(t, dataPath(root), data("Added__c"))
				},
			},
			{
				name: "edit project data JSON",
				setup: func(t *testing.T, root, _ string) {
					writeStartupCacheTestFile(t, dataPath(root), data("Before__c"))
				},
				mutate: func(t *testing.T, root, _ string) {
					writeStartupCacheTestFile(t, dataPath(root), data("After___c"))
				},
			},
			{
				name: "delete project data JSON",
				setup: func(t *testing.T, root, _ string) {
					writeStartupCacheTestFile(t, dataPath(root), data("Deleted__c"))
				},
				mutate: func(t *testing.T, root, _ string) {
					if err := os.Remove(dataPath(root)); err != nil {
						t.Fatalf("Remove(project data JSON) error = %v", err)
					}
				},
			},
		}
		for _, tc := range cases {
			t.Run(tc.name, func(t *testing.T) {
				root, classPath, entry := newExactInvalidationFixture(t, func(root string) {
					if tc.setup != nil {
						tc.setup(t, root, filepath.Join(root, "force-app", "main", "default", "classes", "Exact.cls"))
					}
				})
				tc.mutate(t, root, classPath)
				assertStartupCacheStale(t, &entry, root, tc.name)
			})
		}
	})

	t.Run("cache version mismatch", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, nil)
		entry.Version--
		assertStartupCacheStale(t, &entry, root, "cache version mismatch")
	})

	t.Run("manifest schema mismatch", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, nil)
		entry.Manifest.SchemaVersion = 99
		assertStartupCacheStale(t, &entry, root, "manifest schema mismatch")
	})

	t.Run("runtime ABI mismatch", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, nil)
		entry.RuntimeABI = "old-runtime-abi"
		if FreshRuntime(&entry, root, Version, "current-runtime-abi") {
			t.Fatal("FreshRuntime() = true for runtime ABI mismatch")
		}
	})

	t.Run("platform ABI mismatch", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, nil)
		entry.PlatformABI = "foreign-platform-abi"
		assertStartupCacheStale(t, &entry, root, "platform ABI mismatch")
	})

	t.Run("unreadable tracked input", func(t *testing.T) {
		if runtime.GOOS == "windows" {
			t.Skip("Unix permission bits are not available on Windows")
		}
		root, classPath, entry := newExactInvalidationFixture(t, nil)
		if err := os.Chmod(classPath, 0); err != nil {
			t.Fatalf("Chmod() error = %v", err)
		}
		t.Cleanup(func() { _ = os.Chmod(classPath, 0o644) })
		assertStartupCacheStale(t, &entry, root, "unreadable tracked source")
	})

	t.Run("incomplete manifest", func(t *testing.T) {
		root, _, entry := newExactInvalidationFixture(t, nil)
		entry.Manifest.Files = nil
		assertStartupCacheStale(t, &entry, root, "incomplete manifest file set")
	})
}

func TestBuildManifestTracksOnlyRuntimeConsumerInputs(t *testing.T) {
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app", "main", "default", "classes", "Exact.cls")
	triggerPath := filepath.Join(root, "force-app", "main", "default", "triggers", "Exact.trigger")
	writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeStartupCacheTestFile(t, classPath, "public class Exact {}\n")
	writeStartupCacheTestFile(t, classPath+"-meta.xml", `<ApexClass><apiVersion>61.0</apiVersion></ApexClass>`)
	writeStartupCacheTestFile(t, triggerPath, "trigger Exact on Account (before insert) {}\n")
	writeStartupCacheTestFile(t, triggerPath+"-meta.xml", `<ApexTrigger><apiVersion>61.0</apiVersion></ApexTrigger>`)
	writeStartupCacheTestFile(t, filepath.Join(root, "metadata", "Exact.notiftype"), `<CustomNotificationType/>`)
	writeStartupCacheTestFile(t, filepath.Join(root, "fixtures", "data", "Shape.JSON"), `{"records":{}}`)
	writeStartupCacheTestFile(t, filepath.Join(root, "fixtures", "Shape.json"), `{"ignored":true}`)
	writeStartupCacheTestFile(t, filepath.Join(root, ".hidden", "data", "Hidden.json"), `{"ignored":true}`)
	writeStartupCacheTestFile(t, filepath.Join(root, "node_modules", "data", "Dependency.json"), `{"ignored":true}`)
	writeStartupCacheTestFile(t, filepath.Join(root, "fixtures", "data", "README.txt"), "ignored\n")

	p, err := project.Load(root)
	if err != nil {
		t.Fatalf("project.Load() error = %v", err)
	}
	manifest := BuildManifest(root, p, typesys.Index{})
	if !manifest.Complete {
		t.Fatalf("manifest is incomplete: %#v", manifest)
	}
	tracked := make(map[string]bool, len(manifest.Files))
	for _, file := range manifest.Files {
		tracked[file.Path] = true
	}
	for _, path := range []string{
		"force-app/main/default/classes/Exact.cls-meta.xml",
		"force-app/main/default/triggers/Exact.trigger-meta.xml",
		"metadata/Exact.notiftype",
		"fixtures/data/Shape.JSON",
	} {
		if !tracked[path] {
			t.Errorf("runtime consumer input %q is not tracked: files=%#v", path, manifest.Files)
		}
	}
	for _, path := range []string{
		"fixtures/Shape.json",
		".hidden/data/Hidden.json",
		"node_modules/data/Dependency.json",
		"fixtures/data/README.txt",
	} {
		if tracked[path] {
			t.Errorf("unconsumed runtime file %q is tracked: files=%#v", path, manifest.Files)
		}
	}
}

func newExactInvalidationFixture(t *testing.T, setup func(root string)) (string, string, Entry) {
	t.Helper()
	root := t.TempDir()
	classPath := filepath.Join(root, "force-app", "main", "default", "classes", "Exact.cls")
	writeStartupCacheTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"64.0"}`)
	writeStartupCacheTestFile(t, classPath, "public class Exact { Integer value = 1; }\n")
	if setup != nil {
		setup(root)
	}
	loaded, err := project.Load(root)
	if err != nil {
		t.Fatalf("project.Load() error = %v", err)
	}
	entry := NewEntry(root, loaded, typesys.Index{Project: typesys.ProjectInfo{
		Root:             root,
		Namespace:        loaded.Namespace,
		SourceAPIVersion: loaded.SourceAPIVersion,
	}}, storage.NewOrgState(), CompiledRuntime{})
	entry.RuntimeABI = "current-runtime-abi"
	if !Fresh(&entry, root, Version) {
		t.Fatal("Fresh() = false before mutation")
	}
	return root, classPath, entry
}

func assertStartupCacheStale(t *testing.T, entry *Entry, root, reason string) {
	t.Helper()
	if Fresh(entry, root, Version) {
		t.Fatalf("Fresh() = true after %s", reason)
	}
}

func writeStartupCacheTestFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("MkdirAll(%q) error = %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("WriteFile(%q) error = %v", path, err)
	}
}

func TestBuildManifestTracksLoadedDependencyProjectFiles(t *testing.T) {
	root := t.TempDir()
	consumerRoot := filepath.Join(root, "consumer")
	depRoot := filepath.Join(root, "dependency")
	consumerClass := filepath.Join(consumerRoot, "force-app", "main", "default", "classes", "Consumer.cls")
	depField := filepath.Join(depRoot, "force-app", "main", "default", "objects", "Contact", "fields", "ExternalID__c.field-meta.xml")
	depConfig := filepath.Join(depRoot, "sfdx-project.json")
	consumerConfig := filepath.Join(consumerRoot, "sfdx-project.json")
	consumerGlade := filepath.Join(consumerRoot, "glade.yml")
	for _, path := range []string{consumerClass, depField, depConfig, consumerConfig, consumerGlade} {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatalf("MkdirAll(%s) error = %v", path, err)
		}
	}
	if err := os.WriteFile(consumerClass, []byte("public class Consumer {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(depField, []byte("<CustomField><externalId>true</externalId></CustomField>\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(depConfig, []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(consumerConfig, []byte(`{"packageDirectories":[{"path":"force-app","default":true}]}`), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	if err := os.WriteFile(consumerGlade, []byte("project:\n  managedPackageDependencies: [\"depns:../dependency\"]\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	consumer, err := project.Load(consumerRoot)
	if err != nil {
		t.Fatalf("project.Load() error = %v", err)
	}

	manifest := BuildManifest(consumerRoot, consumer, typesys.Index{})
	if !manifestHasFile(manifest.Files, "../dependency/force-app/main/default/objects/Contact/fields/ExternalID__c.field-meta.xml") {
		t.Fatalf("dependency field missing from manifest files: %#v", manifest.Files)
	}
	if !manifestHasFile(manifest.ConfigFiles, "../dependency/sfdx-project.json") {
		t.Fatalf("dependency config missing from manifest config files: %#v", manifest.ConfigFiles)
	}
	if !manifestHasDirectory(manifest.PackageRoots, "../dependency/force-app") {
		t.Fatalf("dependency package root missing from manifest package roots: %#v", manifest.PackageRoots)
	}
	entry := NewEntry(consumerRoot, consumer, typesys.Index{}, storage.NewOrgState(), CompiledRuntime{})
	if !Fresh(&entry, consumerRoot, Version) {
		t.Fatal("Fresh() = false, want true before dependency edit")
	}
	if err := os.WriteFile(depField, []byte("<CustomField><externalId>false</externalId></CustomField>\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() edit error = %v", err)
	}
	if Fresh(&entry, consumerRoot, Version) {
		t.Fatal("Fresh() = true, want false after dependency field edit")
	}
}

func TestFreshRejectsLoadedDirectDependencyDataMutations(t *testing.T) {
	dataQuery := func(recordTypeName string) string {
		return `{"query":"SELECT Id FROM RecordType WHERE SObjectType = 'pkg__Product__c' AND Name = '` + recordTypeName + `'"}`
	}
	type mutationCase struct {
		name    string
		initial string
		mutate  func(t *testing.T, path string)
		present bool
	}
	cases := []mutationCase{
		{
			name: "add",
			mutate: func(t *testing.T, path string) {
				writeStartupCacheTestFile(t, path, dataQuery("Added Plan"))
			},
			present: true,
		},
		{
			name:    "edit",
			initial: dataQuery("Before Plan"),
			mutate: func(t *testing.T, path string) {
				writeStartupCacheTestFile(t, path, dataQuery("After Plan"))
			},
			present: true,
		},
		{
			name:    "delete",
			initial: dataQuery("Deleted Plan"),
			mutate: func(t *testing.T, path string) {
				if err := os.Remove(path); err != nil {
					t.Fatalf("Remove(dependency data) error = %v", err)
				}
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			consumerRoot := filepath.Join(root, "consumer")
			dependencyRoot := filepath.Join(root, "dependency")
			dataPath := filepath.Join(dependencyRoot, "fixtures", "data", "RecordTypes.json")
			writeStartupCacheTestFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
			writeStartupCacheTestFile(t, filepath.Join(consumerRoot, "glade.yml"), "project:\n  managedPackageDependencies: [\"pkg:../dependency\"]\n")
			writeStartupCacheTestFile(t, filepath.Join(consumerRoot, "force-app", "main", "default", "classes", "Consumer.cls"), "public class Consumer {}\n")
			writeStartupCacheTestFile(t, filepath.Join(dependencyRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"pkg"}`)
			writeStartupCacheTestFile(t, filepath.Join(dependencyRoot, "force-app", "main", "default", "objects", "Product__c", "Product__c.object-meta.xml"), `<CustomObject><label>Product</label></CustomObject>`)
			writeStartupCacheTestFile(t, filepath.Join(dependencyRoot, "fixtures", "RecordTypes.json"), dataQuery("Ignored Plan"))
			writeStartupCacheTestFile(t, filepath.Join(dependencyRoot, "metadata", "Ignored.notiftype"), `<CustomNotificationType/>`)
			if tc.initial != "" {
				writeStartupCacheTestFile(t, dataPath, tc.initial)
			}

			consumer, err := project.Load(consumerRoot)
			if err != nil {
				t.Fatalf("project.Load() error = %v", err)
			}
			entry := NewEntry(consumerRoot, consumer, typesys.Index{}, storage.NewOrgState(), CompiledRuntime{})
			dependencyDataManifestPath := "../dependency/fixtures/data/RecordTypes.json"
			if got := manifestHasFile(entry.Manifest.Files, dependencyDataManifestPath); got != (tc.initial != "") {
				t.Fatalf("initial dependency data tracked = %v, want %v: files=%#v", got, tc.initial != "", entry.Manifest.Files)
			}
			for _, path := range []string{"../dependency/fixtures/RecordTypes.json", "../dependency/metadata/Ignored.notiftype"} {
				if manifestHasFile(entry.Manifest.Files, path) {
					t.Fatalf("unconsumed dependency input %q is tracked: files=%#v", path, entry.Manifest.Files)
				}
			}
			if !Fresh(&entry, consumerRoot, Version) {
				t.Fatal("Fresh() = false before dependency data mutation")
			}

			tc.mutate(t, dataPath)
			if Fresh(&entry, consumerRoot, Version) {
				t.Fatalf("Fresh() = true after direct dependency data %s", tc.name)
			}
			current, err := project.Load(consumerRoot)
			if err != nil {
				t.Fatalf("project.Load() after mutation error = %v", err)
			}
			currentManifest := BuildManifest(consumerRoot, current, typesys.Index{})
			if got := manifestHasFile(currentManifest.Files, dependencyDataManifestPath); got != tc.present {
				t.Fatalf("current dependency data tracked = %v, want %v: files=%#v", got, tc.present, currentManifest.Files)
			}
		})
	}
}

func TestBuildManifestDoesNotTrackTransitiveDependencyData(t *testing.T) {
	root := t.TempDir()
	consumerRoot := filepath.Join(root, "consumer")
	directRoot := filepath.Join(root, "direct")
	transitiveRoot := filepath.Join(root, "transitive")
	writeStartupCacheTestFile(t, filepath.Join(consumerRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	writeStartupCacheTestFile(t, filepath.Join(consumerRoot, "glade.yml"), "project:\n  managedPackageDependencies: [\"direct:../direct\"]\n")
	writeStartupCacheTestFile(t, filepath.Join(consumerRoot, "force-app", "main", "default", "classes", "Consumer.cls"), "public class Consumer {}\n")
	writeStartupCacheTestFile(t, filepath.Join(directRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"direct"}`)
	writeStartupCacheTestFile(t, filepath.Join(directRoot, "glade.yml"), "project:\n  managedPackageDependencies: [\"nested:../transitive\"]\n")
	writeStartupCacheTestFile(t, filepath.Join(directRoot, "force-app", "main", "default", "classes", "Direct.cls"), "global class Direct {}\n")
	writeStartupCacheTestFile(t, filepath.Join(transitiveRoot, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}],"namespace":"nested"}`)
	writeStartupCacheTestFile(t, filepath.Join(transitiveRoot, "force-app", "main", "default", "classes", "Nested.cls"), "global class Nested {}\n")
	writeStartupCacheTestFile(t, filepath.Join(transitiveRoot, "fixtures", "data", "RecordTypes.json"), `{"query":"SELECT Id FROM RecordType WHERE SObjectType = 'nested__Thing__c' AND Name = 'Nested Plan'"}`)

	consumer, err := project.Load(consumerRoot)
	if err != nil {
		t.Fatalf("project.Load() error = %v", err)
	}
	manifest := BuildManifest(consumerRoot, consumer, typesys.Index{})
	if manifestHasFile(manifest.Files, "../transitive/fixtures/data/RecordTypes.json") {
		t.Fatalf("transitive dependency data is tracked: files=%#v", manifest.Files)
	}
	if !manifestHasFile(manifest.Files, "../transitive/force-app/main/default/classes/Nested.cls") {
		t.Fatalf("transitive dependency source is missing from manifest closure: files=%#v", manifest.Files)
	}
}

func TestBuildManifestCanonicalizesSemanticPackageDirectoryOrder(t *testing.T) {
	root := t.TempDir()
	for _, name := range []string{"first", "second"} {
		if err := os.MkdirAll(filepath.Join(root, name), 0o755); err != nil {
			t.Fatalf("MkdirAll(%q) error = %v", name, err)
		}
	}
	first := project.Project{
		Root: root,
		PackageDirectories: []project.PackageDirectory{
			{Path: "first", Default: true, Dependencies: []project.PackageDependency{{Package: "base", VersionNumber: "1.0"}, {Package: "common", VersionNumber: "2.0"}}},
			{Path: "second", Package: "secondary"},
		},
	}
	second := first
	second.PackageDirectories = []project.PackageDirectory{
		{Path: "second", Package: "secondary"},
		{Path: "first", Default: true, Dependencies: []project.PackageDependency{{Package: "common", VersionNumber: "2.0"}, {Package: "base", VersionNumber: "1.0"}}},
	}

	firstManifest := BuildManifest(root, first, typesys.Index{})
	secondManifest := BuildManifest(root, second, typesys.Index{})
	if firstManifest.ProjectDigest == "" || firstManifest.ProjectDigest != secondManifest.ProjectDigest {
		t.Fatalf("project digests differ for reordered semantics: first=%q second=%q", firstManifest.ProjectDigest, secondManifest.ProjectDigest)
	}
}

func manifestHasFile(files []File, path string) bool {
	for _, file := range files {
		if file.Path == path {
			return true
		}
	}
	return false
}

func manifestHasDirectory(dirs []Directory, path string) bool {
	for _, dir := range dirs {
		if dir.Path == path {
			return true
		}
	}
	return false
}

func currentStartupCacheTestEntry(t *testing.T, root string) Entry {
	t.Helper()
	loaded, err := project.Load(root)
	if err != nil {
		t.Fatalf("project.Load() error = %v", err)
	}
	return NewEntry(root, loaded, typesys.Index{}, storage.NewOrgState(), CompiledRuntime{})
}

func TestSplitCacheWritesHeaderAndHashedPayload(t *testing.T) {
	dir := t.TempDir()
	classPath := filepath.Join(dir, "classes", "Foo.cls")
	if err := os.MkdirAll(filepath.Dir(classPath), 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	if err := os.WriteFile(classPath, []byte("public class Foo {}\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	entry := currentStartupCacheTestEntry(t, dir)
	entry.BuiltAt = "2026-06-16T00:00:00Z"
	entry.RuntimeABI = "test-runtime-abi"
	entry.Org = storage.OrgState{
		APIVersion: "64.0",
		Objects: map[string]storage.ObjectState{
			"Account": {Definition: storage.ObjectDefinition{APIName: "Account"}},
		},
	}
	entry.Runtime = CompiledRuntime{Methods: map[string]vm.Method{"Foo.bar": {Name: "bar", ClassName: "Foo"}}}

	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	headerPath := filepath.Join(dir, ".glade", "test", stateHeaderFile)
	headerData, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("header missing: %v", err)
	}
	var header testCacheHeader
	if err := json.Unmarshal(headerData, &header); err != nil {
		t.Fatalf("header json error = %v", err)
	}
	if header.RuntimeABI != entry.RuntimeABI {
		t.Fatalf("header runtime ABI = %q, want %q", header.RuntimeABI, entry.RuntimeABI)
	}
	if header.PlatformABI != entry.PlatformABI || header.PlatformABI == "" {
		t.Fatalf("header platform ABI = %q, want %q", header.PlatformABI, entry.PlatformABI)
	}
	if header.PayloadFile == "" || header.PayloadSHA256 == "" || header.PayloadSize <= 0 {
		t.Fatalf("header missing payload fields: %#v", header)
	}
	if _, err := os.Stat(filepath.Join(dir, ".glade", "test", header.PayloadFile)); err != nil {
		t.Fatalf("payload missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".glade", "test", stateGobFile)); !os.IsNotExist(err) {
		t.Fatalf("legacy gob should not be written for split cache: %v", err)
	}

	got, err := Read(dir, SubdirTest)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got == nil || got.RuntimeABI != entry.RuntimeABI || got.Runtime.Methods["Foo.bar"].Name != "bar" {
		t.Fatalf("Read() = %#v", got)
	}
}

func TestSplitCacheReadStatsSeparateValidationAndDecode(t *testing.T) {
	dir := t.TempDir()
	entry := currentStartupCacheTestEntry(t, dir)
	entry.BuiltAt = "2026-07-09T00:00:00Z"
	entry.RuntimeABI = "read-stats-abi"
	entry.Runtime = CompiledRuntime{Methods: map[string]vm.Method{"Stats.read": {Name: "read", ClassName: "Stats"}}}
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() error = %v", err)
	}

	got, stats, err := ReadWithStats(dir, SubdirTest)
	if err != nil {
		t.Fatalf("ReadWithStats() error = %v", err)
	}
	if got == nil || got.Runtime.Methods["Stats.read"].Name != "read" {
		t.Fatalf("ReadWithStats() = %#v", got)
	}
	if stats.ValidationNS <= 0 || stats.DecodeNS <= 0 {
		t.Fatalf("read stats = %#v, want separate positive validation and decode", stats)
	}
}

func TestSplitCacheWriteStatsRecordEncode(t *testing.T) {
	dir := t.TempDir()
	entry := currentStartupCacheTestEntry(t, dir)
	entry.BuiltAt = "2026-07-09T00:00:00Z"
	entry.Runtime = CompiledRuntime{Methods: map[string]vm.Method{"Stats.write": {Name: "write", ClassName: "Stats"}}}
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	cacheDir := filepath.Join(dir, ".glade", "test")
	normalHeader, err := os.ReadFile(filepath.Join(cacheDir, stateHeaderFile))
	if err != nil {
		t.Fatal(err)
	}
	var header testCacheHeader
	if err := json.Unmarshal(normalHeader, &header); err != nil {
		t.Fatal(err)
	}
	normalPayload, err := os.ReadFile(filepath.Join(cacheDir, header.PayloadFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := Clear(dir, SubdirTest); err != nil {
		t.Fatal(err)
	}

	stats, err := WriteWithStats(&entry, SubdirTest)
	if err != nil {
		t.Fatalf("WriteWithStats() error = %v", err)
	}
	if stats.EncodeNS <= 0 {
		t.Fatalf("write stats = %#v, want positive encode duration", stats)
	}
	measuredHeader, err := os.ReadFile(filepath.Join(cacheDir, stateHeaderFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(measuredHeader, &header); err != nil {
		t.Fatal(err)
	}
	measuredPayload, err := os.ReadFile(filepath.Join(cacheDir, header.PayloadFile))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(measuredHeader, normalHeader) || !bytes.Equal(measuredPayload, normalPayload) {
		t.Fatal("WriteWithStats changed split-cache bytes")
	}
	got, err := Read(dir, SubdirTest)
	if err != nil || got == nil || got.Runtime.Methods["Stats.write"].Name != "write" {
		t.Fatalf("Read() = %#v, %v", got, err)
	}
}

func TestSplitCacheReusesExistingPayloadOnIdenticalRewrite(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		BuiltAt:     "2026-06-16T00:00:00Z",
		Manifest:    Manifest{ProjectRoot: dir},
		Runtime: CompiledRuntime{
			Methods: map[string]vm.Method{"Foo.one": {Name: "one", ClassName: "Foo"}},
		},
	}
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() first error = %v", err)
	}
	headerPath := filepath.Join(dir, ".glade", "test", stateHeaderFile)
	headerData, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("ReadFile() first header error = %v", err)
	}
	var first testCacheHeader
	if err := json.Unmarshal(headerData, &first); err != nil {
		t.Fatalf("Unmarshal() first header error = %v", err)
	}
	payloadPath := filepath.Join(dir, ".glade", "test", first.PayloadFile)
	oldTime := time.Unix(1000, 0)
	if err := os.Chtimes(payloadPath, oldTime, oldTime); err != nil {
		t.Fatalf("Chtimes() error = %v", err)
	}

	entry.BuiltAt = "2026-06-16T00:01:00Z"
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() second error = %v", err)
	}
	headerData, err = os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("ReadFile() second header error = %v", err)
	}
	var second testCacheHeader
	if err := json.Unmarshal(headerData, &second); err != nil {
		t.Fatalf("Unmarshal() second header error = %v", err)
	}
	if second.PayloadFile != first.PayloadFile || second.PayloadSHA256 != first.PayloadSHA256 || second.PayloadSize != first.PayloadSize {
		t.Fatalf("payload changed on identical rewrite: first=%#v second=%#v", first, second)
	}
	info, err := os.Stat(payloadPath)
	if err != nil {
		t.Fatalf("Stat() payload error = %v", err)
	}
	if !info.ModTime().Equal(oldTime) {
		t.Fatalf("payload modtime = %s, want unchanged %s", info.ModTime(), oldTime)
	}
}

func TestFreshRuntimeRejectsABIMismatch(t *testing.T) {
	dir := t.TempDir()
	entry := currentStartupCacheTestEntry(t, dir)
	entry.RuntimeABI = "old-runtime-abi"
	if FreshRuntime(&entry, dir, Version, "new-runtime-abi") {
		t.Fatal("FreshRuntime() = true, want false for runtime ABI mismatch")
	}
	if !FreshRuntime(&entry, dir, Version, "old-runtime-abi") {
		t.Fatal("FreshRuntime() = false, want true for matching runtime ABI")
	}
	entry.RuntimeABI = ""
	if FreshRuntime(&entry, dir, Version, "old-runtime-abi") {
		t.Fatal("FreshRuntime() = true, want false for missing runtime ABI")
	}
}

func TestSplitCacheRejectsStaleHeaderWithoutPayloadDecode(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, ".glade", "test")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	header := testCacheHeader{
		FormatVersion: testCacheFormatVersion,
		Version:       Version,
		ProjectRoot:   dir,
		BuiltAt:       "2026-06-16T00:00:00Z",
		Manifest: Manifest{
			ProjectRoot: filepath.Join(dir, "different-root"),
		},
		PayloadFile:   "payload-is-not-a-file",
		PayloadSHA256: "bad",
		PayloadSize:   1,
	}
	data, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, stateHeaderFile), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, err := Read(dir, SubdirTest)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got != nil {
		t.Fatalf("Read() = %#v, want nil for stale header", got)
	}
}

func TestSplitCacheFreshHeaderMissingPayloadReturnsError(t *testing.T) {
	dir := t.TempDir()
	cacheDir := filepath.Join(dir, ".glade", "test")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatalf("MkdirAll() error = %v", err)
	}
	entry := currentStartupCacheTestEntry(t, dir)
	header := testCacheHeader{
		FormatVersion: testCacheFormatVersion,
		Version:       Version,
		ProjectRoot:   dir,
		BuiltAt:       "2026-06-16T00:00:00Z",
		PlatformABI:   entry.PlatformABI,
		Manifest:      entry.Manifest,
		PayloadFile:   "startup.payload.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.gob",
		PayloadSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
		PayloadSize:   1,
	}
	data, err := json.Marshal(header)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, stateHeaderFile), data, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, err := Read(dir, SubdirTest)
	if err == nil {
		t.Fatal("Read() error = nil, want missing payload error")
	}
	if got != nil {
		t.Fatalf("Read() = %#v, want nil for missing payload", got)
	}
}

func TestSplitCacheRejectsPayloadHashMismatch(t *testing.T) {
	dir := t.TempDir()
	entry := currentStartupCacheTestEntry(t, dir)
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	headerPath := filepath.Join(dir, ".glade", "test", stateHeaderFile)
	headerData, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("ReadFile() error = %v", err)
	}
	var header testCacheHeader
	if err := json.Unmarshal(headerData, &header); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	cacheDir := filepath.Join(dir, ".glade", "test")
	originalPayloadPath := filepath.Join(cacheDir, header.PayloadFile)
	fakeHash := "0000000000000000000000000000000000000000000000000000000000000000"
	fakePayloadFile := payloadFilePrefix + fakeHash + payloadFileSuffix
	payloadBytes, err := os.ReadFile(originalPayloadPath)
	if err != nil {
		t.Fatalf("ReadFile() payload error = %v", err)
	}
	if err := os.WriteFile(filepath.Join(cacheDir, fakePayloadFile), payloadBytes, 0o644); err != nil {
		t.Fatalf("WriteFile() fake payload error = %v", err)
	}
	header.PayloadSHA256 = fakeHash
	header.PayloadFile = fakePayloadFile
	headerData, err = json.Marshal(header)
	if err != nil {
		t.Fatalf("Marshal() error = %v", err)
	}
	if err := os.WriteFile(headerPath, headerData, 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}
	got, err := Read(dir, SubdirTest)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got != nil {
		t.Fatalf("Read() = %#v, want nil for hash mismatch", got)
	}
}

func TestLegacyGobFallbackWhenHeaderMissing(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		Manifest:    Manifest{ProjectRoot: dir},
		Runtime:     CompiledRuntime{Methods: map[string]vm.Method{"Foo.bar": {Name: "bar", ClassName: "Foo"}}},
	}
	if err := writeLegacyGob(&entry, SubdirTest); err != nil {
		t.Fatalf("writeLegacyGob() error = %v", err)
	}
	got, err := Read(dir, SubdirTest)
	if err != nil {
		t.Fatalf("Read() error = %v", err)
	}
	if got == nil || got.Runtime.Methods["Foo.bar"].Name != "bar" {
		t.Fatalf("legacy Read() = %#v", got)
	}
}

func TestSplitCacheMalformedHeaderDoesNotFallBackToLegacyGob(t *testing.T) {
	tests := []struct {
		name   string
		mutate func(*testCacheHeader)
		raw    []byte
	}{
		{name: "malformed JSON", raw: []byte("{not json\n")},
		{name: "format version", mutate: func(header *testCacheHeader) { header.FormatVersion++ }},
		{name: "manifest schema", mutate: func(header *testCacheHeader) { header.Manifest.SchemaVersion++ }},
		{name: "platform ABI", mutate: func(header *testCacheHeader) { header.PlatformABI = "foreign-platform-abi" }},
		{name: "incomplete manifest", mutate: func(header *testCacheHeader) { header.Manifest.Complete = false }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			entry := currentStartupCacheTestEntry(t, dir)
			entry.Runtime = CompiledRuntime{Methods: map[string]vm.Method{"Legacy.method": {Name: "method", ClassName: "Legacy"}}}
			if err := writeLegacyGob(&entry, SubdirTest); err != nil {
				t.Fatalf("writeLegacyGob() error = %v", err)
			}
			header := testCacheHeader{
				FormatVersion: testCacheFormatVersion,
				Version:       entry.Version,
				ProjectRoot:   entry.ProjectRoot,
				BuiltAt:       entry.BuiltAt,
				PlatformABI:   entry.PlatformABI,
				RuntimeABI:    entry.RuntimeABI,
				Manifest:      entry.Manifest,
				PayloadFile:   "startup.payload.aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa.gob",
				PayloadSHA256: "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa",
				PayloadSize:   1,
			}
			if tc.mutate != nil {
				tc.mutate(&header)
			}
			data := tc.raw
			if data == nil {
				var err error
				data, err = json.Marshal(header)
				if err != nil {
					t.Fatalf("Marshal() error = %v", err)
				}
			}
			headerPath := filepath.Join(dir, ".glade", "test", stateHeaderFile)
			if err := os.WriteFile(headerPath, data, 0o644); err != nil {
				t.Fatalf("WriteFile() error = %v", err)
			}

			got, err := Read(dir, SubdirTest)
			if got != nil {
				t.Fatalf("Read() = %#v, %v; corrupt header fell back to legacy gob", got, err)
			}
		})
	}
}

func TestSplitCachePrunesOldPayloadsOnRewrite(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		Manifest:    Manifest{ProjectRoot: dir},
		Runtime:     CompiledRuntime{Methods: map[string]vm.Method{"Foo.one": {Name: "one", ClassName: "Foo"}}},
	}
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() first error = %v", err)
	}
	headerPath := filepath.Join(dir, ".glade", "test", stateHeaderFile)
	headerData, err := os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("ReadFile() first header error = %v", err)
	}
	var first testCacheHeader
	if err := json.Unmarshal(headerData, &first); err != nil {
		t.Fatalf("Unmarshal() first header error = %v", err)
	}
	firstPayloadPath := filepath.Join(dir, ".glade", "test", first.PayloadFile)
	if _, err := os.Stat(firstPayloadPath); err != nil {
		t.Fatalf("first payload missing: %v", err)
	}

	entry.Runtime.Methods["Foo.two"] = vm.Method{Name: "two", ClassName: "Foo"}
	if err := Write(&entry, SubdirTest); err != nil {
		t.Fatalf("Write() second error = %v", err)
	}
	headerData, err = os.ReadFile(headerPath)
	if err != nil {
		t.Fatalf("ReadFile() second header error = %v", err)
	}
	var second testCacheHeader
	if err := json.Unmarshal(headerData, &second); err != nil {
		t.Fatalf("Unmarshal() second header error = %v", err)
	}
	secondPayloadPath := filepath.Join(dir, ".glade", "test", second.PayloadFile)
	if _, err := os.Stat(secondPayloadPath); err != nil {
		t.Fatalf("second payload missing: %v", err)
	}
	if first.PayloadFile != second.PayloadFile {
		if _, err := os.Stat(firstPayloadPath); !os.IsNotExist(err) {
			t.Fatalf("old payload still present after rewrite: %v", err)
		}
	}
}

func TestDAPCacheStillUsesJSONStateFile(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{Version: DAPCacheVersion, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}
	if err := Write(&entry, SubdirDAP); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".glade", "dap", stateFile)); err != nil {
		t.Fatalf("DAP startup.json missing: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, ".glade", "dap", stateHeaderFile)); !os.IsNotExist(err) {
		t.Fatalf("DAP split header should not exist: %v", err)
	}
}

func TestClearRemovesDAPJSONStateFile(t *testing.T) {
	dir := t.TempDir()
	entry := Entry{Version: DAPCacheVersion, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}
	if err := Write(&entry, SubdirDAP); err != nil {
		t.Fatalf("Write() error = %v", err)
	}
	statePath := filepath.Join(dir, ".glade", "dap", stateFile)
	if _, err := os.Stat(statePath); err != nil {
		t.Fatalf("DAP startup.json missing before Clear(): %v", err)
	}
	if err := Clear(dir, SubdirDAP); err != nil {
		t.Fatalf("Clear() error = %v", err)
	}
	if _, err := os.Stat(statePath); !os.IsNotExist(err) {
		t.Fatalf("DAP startup.json still present after Clear(): %v", err)
	}
}
