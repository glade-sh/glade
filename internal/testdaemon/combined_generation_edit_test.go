package testdaemon

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
)

func TestDaemonSameSizeMidRequestEditsReloadAndConcurrentRequests(t *testing.T) {
	restore := apextest.EnableDiskCacheForTesting()
	t.Cleanup(restore)
	t.Cleanup(apextest.InvalidateRuntimeCaches)
	root, _, rewrite := writeDaemonRequestFixture(t, 4)
	source := filepath.Join(root, "force-app/main/default/classes/DaemonServiceA.cls")
	metadata := source + "-meta.xml"
	d, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	opts := apextest.Options{Parallelism: 1}
	if summary := d.RunOptions(opts).Summary(); summary.Passed != 1 {
		t.Fatalf("prime: %+v", summary)
	}
	apextest.WaitForRuntimeCacheWork()
	rewrite()
	ready, release := make(chan struct{}), make(chan struct{})
	var once sync.Once
	stop := apextest.SetFileDigestReadHookForTesting(func(path string) {
		if path == source {
			once.Do(func() { close(ready); <-release })
		}
	})
	done := make(chan testreport.Run, 1)
	go func() { done <- d.RunOptions(opts) }()
	select {
	case <-ready:
	case <-time.After(time.Minute):
		close(release)
		<-done
		stop()
		t.Fatal("source digest did not start")
	}
	for _, change := range []struct{ path, old, new string }{
		{source, "return 0;", "return 1;"},
		{metadata, "62.0", "67.0"},
	} {
		original, err := os.ReadFile(change.path)
		if err != nil {
			close(release)
			<-done
			stop()
			t.Fatal(err)
		}
		updated := strings.Replace(string(original), change.old, change.new, 1)
		if updated == string(original) || len(updated) != len(original) {
			close(release)
			<-done
			stop()
			t.Fatal("fixture edit is not same size")
		}
		if err := os.WriteFile(change.path, []byte(updated), 0o644); err != nil {
			close(release)
			<-done
			stop()
			t.Fatal(err)
		}
	}
	close(release)
	inflight := <-done
	apextest.WaitForRuntimeCacheWork()
	stop()
	if summary := inflight.Summary(); summary.Passed == 1 {
		t.Fatalf("in-flight run accepted stale result: %+v", summary)
	}
	if err := d.Reload(); err != nil {
		t.Fatal(err)
	}
	fresh, err := New(root)
	if err != nil {
		t.Fatal(err)
	}
	want := comparableDaemonRun(fresh.RunOptions(apextest.Options{NoDiskCache: true, Parallelism: 1}))
	if summary := want.Summary(); summary.Failed != 1 {
		t.Fatalf("fresh result: %+v", summary)
	}
	if got := comparableDaemonRun(inflight); !reflect.DeepEqual(got, want) {
		t.Fatalf("in-flight request differs from fresh: got %#v want %#v", got, want)
	}
	var wg sync.WaitGroup
	results := make([]testreport.Run, 4)
	for i := range results {
		wg.Add(1)
		go func(i int) { defer wg.Done(); results[i] = comparableDaemonRun(d.RunOptions(opts)) }(i)
	}
	wg.Wait()
	for i, got := range results {
		if !reflect.DeepEqual(got, want) {
			t.Errorf("request %d differs from fresh: got %#v want %#v", i, got, want)
		}
	}
	apextest.WaitForRuntimeCacheWork()
}

func comparableDaemonRun(run testreport.Run) testreport.Run {
	run.DurationMS = 0
	for i := range run.Suites {
		run.Suites[i].DurationMS = 0
		for j := range run.Suites[i].Cases {
			run.Suites[i].Cases[j].DurationMS = 0
		}
	}
	return run
}
