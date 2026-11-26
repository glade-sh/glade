package apextest

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Benchmark the captured one-parent/two-share and two-parent/four-share
// subscriber projects through the test runner, including event delivery.
func BenchmarkEventSubscriberParentCounts(b *testing.B) {
	for _, fixture := range []string{"event_trigger_parent_visibility", "event_subscriber_unrelated_visibility"} {
		b.Run(fixture, func(b *testing.B) {
			root := b.TempDir()
			source := filepath.Join("testdata", "conformance", fixture, "project")
			err := filepath.WalkDir(source, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				raw, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(source, path)
				if err != nil {
					return err
				}
				text := string(raw)
				if strings.HasSuffix(name, "ShareEventProbeTest.cls") {
					text = strings.ReplaceAll(text, "System.assert(false, 'OUT:' + run(true));", "System.assert(run(true).contains('trig=parent'));")
					text = strings.ReplaceAll(text, "System.assert(false, 'OUT:' + run(false));", "System.assert(run(false).contains('trig=parent'));")
				}
				writeBenchmarkTestFile(b, filepath.Join(root, name), text)
				return nil
			})
			if err != nil {
				b.Fatal(err)
			}
			index := benchmarkLoadTestIndex(b, root)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				run := Run(index, Options{SelectedClasses: []string{"ShareEventProbeTest"}, NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != 2 || summary.Passed != 2 {
					b.Fatalf("summary = %#v", summary)
				}
			}
		})
	}
}
