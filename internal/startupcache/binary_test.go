package startupcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

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
