package apextest

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/debug"
	"strings"
	"testing"
)

// Exercise repeated schema reflection, DML and relationship JSON across isolated
// test methods using the same owned helper as the native conformance matrix.
func BenchmarkSchemaReaderProject(b *testing.B) {
	for _, percent := range []int{50, -1} {
		b.Run(fmt.Sprintf("GOGC_%d", percent), func(b *testing.B) {
			oldLimit := debug.SetMemoryLimit(2 << 30)
			oldGC := debug.SetGCPercent(percent)
			defer debug.SetMemoryLimit(oldLimit)
			defer debug.SetGCPercent(oldGC)
			root := b.TempDir()
			fixture := "testdata/conformance/folded_schema_names/project"
			err := filepath.WalkDir(fixture, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				rel, err := filepath.Rel(fixture, path)
				if err != nil {
					return err
				}
				data, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				writeBenchmarkTestFile(b, filepath.Join(root, rel), string(data))
				return nil
			})
			if err != nil {
				b.Fatal(err)
			}
			var source strings.Builder
			source.WriteString("@IsTest private class SchemaNameProbeTest {\n")
			for i := range 8 {
				fmt.Fprintf(&source, `@IsTest static void reads%d() {
  for (Integer i = 0; i < 2; i++) {
    System.assert(SchemaNameProbe.observe().contains('"global":"Account"'));
  }
}
`, i)
			}
			source.WriteString("}\n")
			writeBenchmarkTestFile(b, filepath.Join(root, "force-app/main/default/classes/SchemaNameProbeTest.cls"), source.String())
			index := benchmarkLoadTestIndex(b, root)
			runProject := func() {
				run := Run(index, Options{NoDiskCache: true, Parallelism: 1})
				if summary := run.Summary(); summary.Total != 8 || summary.Passed != 8 {
					b.Fatalf("summary = %#v; suites = %#v", summary, run.Suites)
				}
			}
			runProject()
			runtime.GC()
			var before, after runtime.MemStats
			runtime.ReadMemStats(&before)
			b.ReportAllocs()
			b.ResetTimer()
			for range b.N {
				runProject()
			}
			b.StopTimer()
			runtime.ReadMemStats(&after)
			b.ReportMetric(float64(after.NumGC-before.NumGC)/float64(b.N), "GC/op")
		})
	}
}
