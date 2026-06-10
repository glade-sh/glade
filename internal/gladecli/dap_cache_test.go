package gladecli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/startupcache"
)

func TestLoadDAPStartupStateCachesAndReusesState(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestProject(t, root)

	cachePath := filepath.Join(root, ".glade", "dap", "startup.json")

	_, runtimeOne, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("first load: %v", err)
	}
	if len(runtimeOne.Methods) == 0 {
		t.Fatalf("expected methods in first cached runtime")
	}

	cacheOne, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file missing after first load: %v", err)
	}
	var parsedOne startupcache.Entry
	if err := json.Unmarshal(cacheOne, &parsedOne); err != nil {
		t.Fatalf("parse first cache: %v", err)
	}
	if parsedOne.BuiltAt == "" {
		t.Fatal("cache did not record builtAt")
	}

	time.Sleep(2 * time.Millisecond)
	_, _, err = loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("second load: %v", err)
	}

	cacheTwo, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file missing after second load: %v", err)
	}
	var parsedTwo startupcache.Entry
	if err := json.Unmarshal(cacheTwo, &parsedTwo); err != nil {
		t.Fatalf("parse second cache: %v", err)
	}

	if parsedOne.BuiltAt != parsedTwo.BuiltAt {
		t.Fatalf("cache was rebuilt unexpectedly (builtAt changed from %s to %s)", parsedOne.BuiltAt, parsedTwo.BuiltAt)
	}
}

func TestLoadDAPStartupStateInvalidatesCacheOnProjectChange(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestProject(t, root)

	cachePath := filepath.Join(root, ".glade", "dap", "startup.json")
	_, _, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("initial load: %v", err)
	}

	cacheBefore, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file missing after initial load: %v", err)
	}
	var parsedBefore startupcache.Entry
	if err := json.Unmarshal(cacheBefore, &parsedBefore); err != nil {
		t.Fatalf("parse initial cache: %v", err)
	}
	if parsedBefore.BuiltAt == "" {
		t.Fatal("initial builtAt empty")
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
	_, _, err = loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("reload after edit: %v", err)
	}

	cacheAfter, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file missing after reload: %v", err)
	}
	var parsedAfter startupcache.Entry
	if err := json.Unmarshal(cacheAfter, &parsedAfter); err != nil {
		t.Fatalf("parse reloaded cache: %v", err)
	}
	if parsedBefore.BuiltAt == parsedAfter.BuiltAt {
		t.Fatalf("cache was not rebuilt after source change")
	}
}

func TestLoadDAPStartupStateCachesCompiledRuntime(t *testing.T) {
	t.Parallel()

	root := t.TempDir()
	writeTestProject(t, root)

	_, runtime, err := loadDAPStartupState(root)
	if err != nil {
		t.Fatalf("load startup state: %v", err)
	}
	if len(runtime.Methods) == 0 {
		t.Fatal("expected compiled runtime methods in DAP startup state")
	}

	cachePath := filepath.Join(root, ".glade", "dap", "startup.json")
	data, err := os.ReadFile(cachePath)
	if err != nil {
		t.Fatalf("cache file missing: %v", err)
	}
	var parsed startupcache.Entry
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatalf("parse cache: %v", err)
	}
	if len(parsed.Runtime.Methods) == 0 {
		t.Fatal("expected compiled runtime methods persisted in DAP cache")
	}
}
