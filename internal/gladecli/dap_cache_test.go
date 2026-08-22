package gladecli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/startupcache"
)

func TestLoadDAPStartupStateCachesReusesAndInvalidates(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestProject(t, root)

	cachePath := filepath.Join(root, ".glade", "dap", "startup.json")

	_, runtimeOne, _, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if len(runtimeOne.Methods) == 0 {
		t.Fatalf("expected methods in first cached runtime")
	}

	parsedOne := readDAPCacheEntry(t, cachePath)
	if parsedOne.BuiltAt == "" {
		t.Fatal("cache did not record builtAt")
	}
	if len(parsedOne.Runtime.Methods) == 0 {
		t.Fatal("expected compiled runtime methods persisted in DAP cache")
	}

	time.Sleep(2 * time.Millisecond)
	_, runtimeTwo, _, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}
	if len(runtimeTwo.Methods) == 0 {
		t.Fatal("expected methods in reused cached runtime")
	}
	parsedTwo := readDAPCacheEntry(t, cachePath)

	if parsedOne.BuiltAt != parsedTwo.BuiltAt {
		t.Fatalf("cache was rebuilt unexpectedly (builtAt changed from %s to %s)", parsedOne.BuiltAt, parsedTwo.BuiltAt)
	}

	source := filepath.Join(root, "force-app", "main", "classes", "MathUtil.cls")
	contents, err := os.ReadFile(source)
	if err != nil {
		t.Fatalf("read source before edit: %v", err)
	}
	if err := os.WriteFile(source, append(contents, []byte("\n// touch for dap cache invalidation\n")...), 0o644); err != nil {
		t.Fatalf("edit source for invalidation: %v", err)
	}
	time.Sleep(2 * time.Millisecond)
	_, runtimeThree, _, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("reload after edit: %v", err)
	}
	if len(runtimeThree.Methods) == 0 {
		t.Fatal("expected methods in rebuilt cached runtime")
	}
	parsedThree := readDAPCacheEntry(t, cachePath)
	if parsedThree.BuiltAt == "" {
		t.Fatal("rebuilt cache did not record builtAt")
	}
	if len(parsedThree.Runtime.Methods) == 0 {
		t.Fatal("expected compiled runtime methods persisted in rebuilt DAP cache")
	}
	if parsedOne.BuiltAt == parsedThree.BuiltAt || parsedTwo.BuiltAt == parsedThree.BuiltAt {
		t.Fatalf("cache was not rebuilt after source change")
	}
}

func TestDAPCacheKeyIncludesSourceAPIVersion(t *testing.T) {
	root := t.TempDir()
	writeTestProject(t, root)
	projectFile := filepath.Join(root, "sfdx-project.json")
	writeTestFile(t, projectFile, `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"65.0"}`)

	_, _, firstVersion, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatal(err)
	}
	first := readDAPCacheEntry(t, filepath.Join(root, ".glade", "dap", "startup.json"))

	time.Sleep(2 * time.Millisecond)
	writeTestFile(t, projectFile, `{"packageDirectories":[{"path":"force-app","default":true}],"sourceApiVersion":"66.0"}`)
	_, _, secondVersion, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatal(err)
	}
	second := readDAPCacheEntry(t, filepath.Join(root, ".glade", "dap", "startup.json"))

	if firstVersion != "65.0" || secondVersion != "66.0" || first.BuiltAt == second.BuiltAt {
		t.Fatalf("versions = %q, %q; builtAt = %q, %q", firstVersion, secondVersion, first.BuiltAt, second.BuiltAt)
	}
}

func readDAPCacheEntry(t *testing.T, path string) startupcache.Entry {
	t.Helper()

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read DAP cache entry: %v", err)
	}
	var entry startupcache.Entry
	if err := json.Unmarshal(contents, &entry); err != nil {
		t.Fatalf("parse DAP cache entry: %v", err)
	}
	return entry
}
