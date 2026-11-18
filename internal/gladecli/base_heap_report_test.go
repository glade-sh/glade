//go:build heapreport

package gladecli

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/storage"
)

// TestBaseHeapReport loads a small synthetic project through the local test
// route and logs the heap the process retains afterwards. It measures; it does
// not assert. Run it with -tags heapreport, GLADE_HEAP_REPORT=1 and -v, in a fresh process per
// sample (-count=1 -run TestBaseHeapReport).
func TestBaseHeapReport(t *testing.T) {
	if os.Getenv("GLADE_HEAP_REPORT") == "" {
		t.Skip("set GLADE_HEAP_REPORT=1 to report retained heap")
	}
	storage.EnableStandardCatalogRelease()
	root := t.TempDir()
	writeTestFile(t, filepath.Join(root, "sfdx-project.json"), `{"packageDirectories":[{"path":"force-app","default":true}]}`)
	classes := filepath.Join(root, "force-app", "main", "default", "classes")
	writeTestFile(t, filepath.Join(classes, "HeapProbeTest.cls"), `@IsTest private class HeapProbeTest {
	@IsTest static void probes() {
		Account a = new Account(Name = 'x');
		insert a;
		System.assertEquals(1, [SELECT COUNT() FROM Account]);
	}
}`)
	writeTestFile(t, filepath.Join(classes, "HeapProbeTest.cls-meta.xml"), `<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>67.0</apiVersion><status>Active</status></ApexClass>`)
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()
	var stdout, stderr bytes.Buffer
	run, err := runTest(ctx, []string{"--project", root, "--no-progress", "--no-serve", "--no-cache", "--parallelism", "1"}, &stdout, &stderr)
	if err != nil {
		t.Fatalf("run: %v; stdout=%s stderr=%s", err, &stdout, &stderr)
	}
	if got := run.Summary(); got.Total != 1 || got.Passed != 1 {
		t.Fatalf("summary = %#v", got)
	}
	runtime.GC()
	runtime.GC()
	var stats runtime.MemStats
	runtime.ReadMemStats(&stats)
	t.Logf("HEAPREPORT heap_inuse_mib=%.1f heap_alloc_mib=%.1f sys_mib=%.1f", float64(stats.HeapInuse)/(1<<20), float64(stats.HeapAlloc)/(1<<20), float64(stats.Sys)/(1<<20))
}
