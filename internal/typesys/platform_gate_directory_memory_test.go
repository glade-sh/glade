package typesys

import (
	"fmt"
	"hash/fnv"
	"os"
	"os/exec"
	"runtime"
	"sync"
	"testing"
)

func directoryDigest(dir *gatedPlatformStubDirectory) uint64 {
	h := fnv.New64a()
	for i := range dir.entries {
		e := &dir.entries[i]
		fmt.Fprintf(h, "%s\x00%s\x00%s\x00%s\x00%d\n", e.namespace, e.name, e.runtime, e.kind, e.bit)
	}
	return h.Sum64()
}

func TestGatedDirectoryConcurrentSymbol(t *testing.T) {
	dir := buildGatedPlatformStubDirectory(standardPlatformSymbolSpecList(false))
	if len(dir.entries) < 5000 {
		t.Fatalf("directory has %d entries", len(dir.entries))
	}
	t.Logf("directory entries=%d digest=%016x", len(dir.entries), directoryDigest(dir))
	for _, index := range []int{0, len(dir.entries) / 2, len(dir.entries) - 1} {
		const workers = 16
		got := make([]*TypeSymbol, workers)
		var wg sync.WaitGroup
		for i := range got {
			wg.Add(1)
			go func(i int) {
				defer wg.Done()
				got[i] = dir.symbol(index)
			}(i)
		}
		wg.Wait()
		for i, symbol := range got {
			if symbol != got[0] || symbol.Name != dir.entries[index].name || symbol.Namespace != dir.entries[index].namespace {
				t.Fatalf("entry %d worker %d returned a different symbol", index, i)
			}
		}
	}
	if _, ok := dir.byRuntime["system.string"]; ok {
		t.Fatal("directory contains an ungated type")
	}
}

var benchmarkGatedDirectory *gatedPlatformStubDirectory

func BenchmarkGatedDirectoryBuild(b *testing.B) {
	specs := standardPlatformSymbolSpecList(false)
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		benchmarkGatedDirectory = buildGatedPlatformStubDirectory(specs)
	}
}

func TestGatedDirectoryRetainedHeap(t *testing.T) {
	if os.Getenv("GLADE_DIRECTORY_HEAP_CHILD") == "" {
		cmd := exec.Command(os.Args[0], "-test.run=^TestGatedDirectoryRetainedHeap$", "-test.v")
		cmd.Env = append(os.Environ(), "GLADE_DIRECTORY_HEAP_CHILD=1")
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("heap subprocess: %v\n%s", err, out)
		}
		t.Logf("%s", out)
		return
	}
	specs := standardPlatformSymbolSpecList(false)
	runtime.GC()
	var before, after runtime.MemStats
	runtime.ReadMemStats(&before)
	dir := buildGatedPlatformStubDirectory(specs)
	runtime.GC()
	runtime.ReadMemStats(&after)
	t.Logf("directory retained=%d entries=%d digest=%016x", int64(after.HeapAlloc)-int64(before.HeapAlloc), len(dir.entries), directoryDigest(dir))
	runtime.KeepAlive(specs)
	runtime.KeepAlive(dir)
}
