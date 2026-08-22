package playground

import "testing"

func TestCacheStoresAndLoadsRunResult(t *testing.T) {
	cache := NewResultCache(t.TempDir())
	key := CacheKey{
		WorkspaceHash: "workspace",
		AnonymousBody: "System.debug('x');",
		SeedHash:      "seed",
		LimitMode:     "permissive",
		RunMode:       "scratch",
		Version:       "test",
	}.String()

	result := RunResult{
		RunID:  "run-1",
		Status: RunStatusPass,
		Logs:   []string{"x"},
	}
	if err := cache.Store(key, result); err != nil {
		t.Fatalf("Store() error = %v", err)
	}

	got, ok, err := cache.Load(key)
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if !ok || got.RunID != "run-1" || len(got.Logs) != 1 || got.Logs[0] != "x" {
		t.Fatalf("Load() = %#v, %v", got, ok)
	}

	latest, ok, err := cache.Latest()
	if err != nil {
		t.Fatalf("Latest() error = %v", err)
	}
	if !ok || latest.RunID != "run-1" {
		t.Fatalf("Latest() = %#v, %v", latest, ok)
	}
}

func TestCacheKeyChangesWithAnonymousBody(t *testing.T) {
	base := CacheKey{WorkspaceHash: "workspace", SeedHash: "seed", LimitMode: "permissive", RunMode: "scratch", Version: "test"}
	first := base
	first.AnonymousBody = "System.debug('a');"
	second := base
	second.AnonymousBody = "System.debug('b');"
	if first.String() == second.String() {
		t.Fatalf("cache key did not change")
	}
}

func TestCacheKeyChangesWithSourceAPIVersion(t *testing.T) {
	first := CacheKey{WorkspaceHash: "workspace", SourceAPIVersion: "65.0"}
	second := first
	second.SourceAPIVersion = "66.0"
	if first.String() == second.String() {
		t.Fatal("cache key did not change")
	}
}
