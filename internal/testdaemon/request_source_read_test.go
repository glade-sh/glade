package testdaemon

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
)

// writeDaemonRequestFixture creates a small deterministic project and the
// paths whose per-request reads the counting test observes.
func writeDaemonRequestFixture(t *testing.T, serviceCount int) (root string, watchPaths []string, rewrite func()) {
	t.Helper()
	root = t.TempDir()
	contents := map[string]string{}
	write := func(rel, body string) string {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
		contents[path] = body
		watchPaths = append(watchPaths, path)
		return path
	}
	write("sfdx-project.json", `{"sourceApiVersion":"62.0","packageDirectories":[{"path":"force-app","default":true}]}`)
	for i := range serviceCount {
		name := "DaemonService" + string(rune('A'+i))
		write("force-app/main/default/classes/"+name+".cls",
			"public class "+name+" { public static Integer value() { return "+string(rune('0'+i))+"; } }")
		write("force-app/main/default/classes/"+name+".cls-meta.xml",
			`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	}
	write("force-app/main/default/classes/DaemonReadsTest.cls",
		`@IsTest private class DaemonReadsTest { @IsTest static void passes() { System.assertEquals(0, DaemonServiceA.value()); } }`)
	write("force-app/main/default/classes/DaemonReadsTest.cls-meta.xml",
		`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`)
	rewrite = func() {
		for path, body := range contents {
			if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	return root, watchPaths, rewrite
}

// TestDaemonRunRequestReadsEachSourceOnce proves one daemon run request reads
// and hashes every captured source and companion metadata file exactly once,
// reusing that digest at every later validation boundary of the same request.
// Files are rewritten with identical bytes first, so each is racily clean and
// the request right after a save is the measured shape.
func TestDaemonRunRequestReadsEachSourceOnce(t *testing.T) {
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, watchPaths, rewrite := writeDaemonRequestFixture(t, 4)
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
	prime, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	primeSummary := prime.Summary()
	if primeSummary.Total != 1 || primeSummary.Passed != 1 || !primeSummary.OK() {
		t.Fatalf("prime run summary = %#v; suites = %#v", primeSummary, prime.Suites)
	}
	for range 2 {
		rewrite()
		var mu sync.Mutex
		reads := make(map[string]int, len(watchPaths))
		stop := apextest.SetFileDigestReadHookForTesting(func(path string) {
			mu.Lock()
			reads[path]++
			mu.Unlock()
		})
		run, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil)
		stop()
		if err != nil {
			t.Fatal(err)
		}
		var unexpected []string
		for _, path := range watchPaths {
			if strings.HasSuffix(path, "sfdx-project.json") {
				// The manifest is not a digest-validated source; project load
				// reads it through its own path.
				continue
			}
			if got := reads[path]; got != 1 {
				unexpected = append(unexpected, fmt.Sprintf("%s: %d reads", filepath.Base(path), got))
			}
		}
		if len(unexpected) != 0 {
			t.Fatalf("one run request read and hashed files more than once, want exactly 1 read per file:\n%s", strings.Join(unexpected, "\n"))
		}
		if summary := run.Summary(); summary.Total != 1 || summary.Passed != 1 || !summary.OK() {
			t.Fatalf("warm run summary = %#v, want the prime run's %#v", summary, primeSummary)
		}
	}
}
