package server

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
)

func TestRuntimeAssetDigestCache(t *testing.T) {
	var cache runtimeAssetDigestCache
	var calls atomic.Int32
	digest := func(b []byte) string {
		calls.Add(1)
		return fmt.Sprintf("digest:%s", b)
	}
	if got := cache.validator("shell/a", []byte("first"), digest); got != "digest:first" {
		t.Fatal(got)
	}
	var wg sync.WaitGroup
	for i := 0; i < 32; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if got := cache.validator("shell/a", []byte("first"), digest); got != "digest:first" {
				t.Error(got)
			}
		}()
	}
	wg.Wait()
	if got := calls.Load(); got != 1 {
		t.Fatalf("unchanged content hashed %d times, want 1", got)
	}
	if got := cache.validator("shell/a", []byte("other"), digest); got != "digest:other" {
		t.Fatal(got)
	}
	if got := cache.validator("shell/b", []byte("first"), digest); got != "digest:first" {
		t.Fatal(got)
	}
	if got := calls.Load(); got != 3 {
		t.Fatalf("changed content or path missed: %d hashes", got)
	}
}
