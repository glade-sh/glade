package testdaemon

import (
	"context"
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/testreport"
	"github.com/glade-sh/glade/internal/watch"
)

// BenchmarkDaemonBodyEdit runs every test after an equivalent one-line body
// edit, including the watcher's update and warm. The digest checks the full
// set of test outcomes against the initial run on every iteration.
func BenchmarkDaemonBodyEdit(b *testing.B) {
	for _, size := range []struct {
		name     string
		services int
		tests    int
	}{{"medium", 100, 100}, {"large", 300, 300}} {
		b.Run(size.name, func(b *testing.B) {
			root, _ := writeDaemonBenchmarkProject(b, size.services)
			classes := filepath.Join(root, "force-app/main/default/classes")
			for i := range size.services {
				path := filepath.Join(classes, fmt.Sprintf("BenchService%03d.cls", i))
				content, err := os.ReadFile(path)
				if err != nil {
					b.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(strings.ReplaceAll(string(content), `"`, `'`)), 0o644); err != nil {
					b.Fatal(err)
				}
			}
			var tests strings.Builder
			tests.WriteString("@IsTest private class BenchAllTest {\n")
			for i := range size.tests {
				fmt.Fprintf(&tests, "@IsTest static void test%03d() { System.assertEquals(1, BenchService%03d.add1(0)); }\n", i, i%size.services)
			}
			tests.WriteString("}\n")
			path := filepath.Join(classes, "BenchAllTest.cls")
			if err := os.WriteFile(path, []byte(tests.String()), 0o644); err != nil {
				b.Fatal(err)
			}
			if err := os.WriteFile(path+"-meta.xml", []byte(`<ApexClass xmlns="http://soap.sforce.com/2006/04/metadata"><apiVersion>62.0</apiVersion><status>Active</status></ApexClass>`), 0o644); err != nil {
				b.Fatal(err)
			}
			path = filepath.Join(classes, fmt.Sprintf("BenchService%03d.cls", size.services-1))
			original, err := os.ReadFile(path)
			if err != nil {
				b.Fatal(err)
			}
			edited := strings.Replace(string(original), "return value + 1;", "return 1 + value;", 1)
			if edited == string(original) {
				b.Fatal("fixture body not found")
			}
			d, err := New(root)
			if err != nil {
				b.Fatal(err)
			}
			apextest.InvalidateRuntimeCaches()
			if err := d.Warm(); err != nil {
				b.Fatal(err)
			}
			request := NewRunRequestV1(apextest.Options{NoDiskCache: true, Parallelism: 1}, "", 0, 0, false)
			prime, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil)
			if err != nil || prime.Summary().Passed != size.tests+1 {
				b.Fatalf("prime: %+v, %v, %s", prime.Summary(), err, runProblems(prime))
			}
			want := bodyEditOutcomeDigest(prime)
			b.Logf("outcome digest: %x", want)
			scope := d.WatchScopeSnapshot(root)
			b.ReportAllocs()
			b.ResetTimer()
			for i := range b.N {
				b.StopTimer()
				content := original
				if i%2 == 0 {
					content = []byte(edited)
				}
				if err := os.WriteFile(path, content, 0o644); err != nil {
					b.Fatal(err)
				}
				b.StartTimer()
				if exact, err := d.TryUpdateChanges(context.Background(), []watch.Change{{Path: path, Op: watch.ChangeModified, Kind: watch.FileKindApexClass}}, scope); err != nil || !exact {
					b.Fatalf("watch update: exact=%v error=%v", exact, err)
				}
				apextest.InvalidateRuntimeCaches()
				if err := d.Warm(); err != nil {
					b.Fatal(err)
				}
				if stats := d.semaState.LastStats(); !stats.Incremental || stats.Reused == 0 || stats.Rechecked == 0 || stats.Rechecked >= stats.Types {
					b.Fatalf("body edit did not reuse semantic checks: %+v", stats)
				}
				run, _, _, err := d.RunRequestV1(context.Background(), request, nil, nil)
				if err != nil {
					b.Fatal(err)
				}
				if got := bodyEditOutcomeDigest(run); got != want {
					b.Fatalf("outcomes changed: got %x want %x; %s", got, want, runProblems(run))
				}
			}
		})
	}
}

func bodyEditOutcomeDigest(run testreport.Run) [32]byte {
	var outcomes []string
	for _, suite := range run.Suites {
		for _, c := range suite.Cases {
			outcomes = append(outcomes, fmt.Sprintf("%s\x00%s\x00%s\x00%s\x00%#v", suite.Name, c.ClassName, c.Name, c.Status, c.Problem))
		}
	}
	sort.Strings(outcomes)
	return sha256.Sum256([]byte(strings.Join(outcomes, "\n")))
}
