package startupcache

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
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

func TestBuildManifestTracksLoadedDependencyProjectFiles(t *testing.T) {
	root := t.TempDir()
	consumerRoot := filepath.Join(root, "consumer")
	depRoot := filepath.Join(root, "dependency")
	consumerClass := filepath.Join(consumerRoot, "force-app", "main", "default", "classes", "Consumer.cls")
	depField := filepath.Join(depRoot, "force-app", "main", "default", "objects", "Contact", "fields", "ExternalID__c.field-meta.xml")
	depConfig := filepath.Join(depRoot, "sfdx-project.json")
	for _, path := range []string{consumerClass, depField, depConfig} {
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
	consumer := project.Project{
		Root:               consumerRoot,
		PackageDirectories: []project.PackageDirectory{{Path: "force-app", Default: true}},
		ApexFiles:          []string{consumerClass},
		ManagedPackageDependencies: []project.ManagedPackageDependency{{
			Namespace:  "depns",
			SourceRoot: depRoot,
			Status:     "loaded",
			Project: &project.Project{
				Root:               depRoot,
				PackageDirectories: []project.PackageDirectory{{Path: "force-app", Default: true}},
				FieldFiles:         []string{depField},
			},
		}},
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
	entry := Entry{Version: Version, ProjectRoot: consumerRoot, Manifest: manifest}
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

func TestSplitCacheWritesHeaderAndHashedPayload(t *testing.T) {
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
		BuiltAt:     "2026-06-16T00:00:00Z",
		RuntimeABI:  "test-runtime-abi",
		Manifest: Manifest{
			ProjectRoot: dir,
			Files:       []File{fp},
		},
		Org: storage.OrgState{
			APIVersion: "64.0",
			Objects: map[string]storage.ObjectState{
				"Account": {Definition: storage.ObjectDefinition{APIName: "Account"}},
			},
		},
		Runtime: CompiledRuntime{
			Methods: map[string]vm.Method{"Foo.bar": {Name: "bar", ClassName: "Foo"}},
		},
	}

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
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		BuiltAt:     "2026-07-09T00:00:00Z",
		RuntimeABI:  "read-stats-abi",
		Manifest:    Manifest{ProjectRoot: dir},
		Runtime: CompiledRuntime{
			Methods: map[string]vm.Method{"Stats.read": {Name: "read", ClassName: "Stats"}},
		},
	}
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
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		BuiltAt:     "2026-07-09T00:00:00Z",
		Manifest:    Manifest{ProjectRoot: dir},
		Runtime: CompiledRuntime{
			Methods: map[string]vm.Method{"Stats.write": {Name: "write", ClassName: "Stats"}},
		},
	}
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
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		RuntimeABI:  "old-runtime-abi",
		Manifest:    Manifest{ProjectRoot: dir},
	}
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
	header := testCacheHeader{
		FormatVersion: testCacheFormatVersion,
		Version:       Version,
		ProjectRoot:   dir,
		BuiltAt:       "2026-06-16T00:00:00Z",
		Manifest:      Manifest{ProjectRoot: dir},
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
	entry := Entry{Version: Version, ProjectRoot: dir, Manifest: Manifest{ProjectRoot: dir}}
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
	dir := t.TempDir()
	entry := Entry{
		Version:     Version,
		ProjectRoot: dir,
		Manifest:    Manifest{ProjectRoot: dir},
		Runtime:     CompiledRuntime{Methods: map[string]vm.Method{"Legacy.method": {Name: "method", ClassName: "Legacy"}}},
	}
	if err := writeLegacyGob(&entry, SubdirTest); err != nil {
		t.Fatalf("writeLegacyGob() error = %v", err)
	}
	headerPath := filepath.Join(dir, ".glade", "test", stateHeaderFile)
	if err := os.WriteFile(headerPath, []byte("{not json\n"), 0o644); err != nil {
		t.Fatalf("WriteFile() error = %v", err)
	}

	got, err := Read(dir, SubdirTest)
	if got != nil {
		t.Fatalf("Read() = %#v, %v; malformed header fell back to legacy gob", got, err)
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
