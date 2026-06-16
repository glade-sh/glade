package startupcache

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/storage"
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
	if got == nil || got.Runtime.Methods["Foo.bar"].Name != "bar" {
		t.Fatalf("Read() = %#v", got)
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
