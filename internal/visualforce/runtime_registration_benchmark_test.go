package visualforce_test

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/project"
	"github.com/glade-sh/glade/internal/schema"
	"github.com/glade-sh/glade/internal/server"
	"github.com/glade-sh/glade/internal/storage"
	"github.com/glade-sh/glade/internal/typesys"
)

type vfRegistrationBenchmarkRow struct {
	Kind   string `json:"kind"`
	Inputs map[string]struct {
		Files map[string]string `json:"files"`
	} `json:"inputs"`
}

// Measure the real per-row server registration using captured controller inputs.
// Source discovery/indexing stays outside the timer. Conformance assertions and
// their full native inputs remain unchanged.
func BenchmarkVFRowRuntimeRegistration(b *testing.B) {
	benchmarkVFRegistration(b, false)
}

// One operation registers every runtime row, with only one index retained at a
// time. Divide B/op and allocs/op by rows/op for the family-wide per-row cost.
func BenchmarkVFAllRuntimeRowRegistration(b *testing.B) {
	benchmarkVFRegistration(b, true)
}

func benchmarkVFRegistration(b *testing.B, all bool) {
	for _, family := range []string{"v04", "v13"} {
		var table struct {
			Cases []vfRegistrationBenchmarkRow `json:"cases"`
		}
		raw, err := os.ReadFile("testdata/" + family + "_salesforce.json")
		if err != nil {
			b.Fatal(err)
		}
		if err := json.Unmarshal(raw, &table); err != nil {
			b.Fatal(err)
		}
		var rows []vfRegistrationBenchmarkRow
		for _, row := range table.Cases {
			if row.Kind == "runtime" {
				rows = append(rows, row)
			}
		}
		want := 100
		if family == "v13" {
			want = 122
		}
		if len(rows) != want {
			b.Fatalf("%s: runtime rows %d, want %d", family, len(rows), want)
		}
		for _, api := range []string{"59.0", "67.0"} {
			b.Run(family+"/"+api, func(b *testing.B) {
				var indexes []typesys.Index
				for _, row := range rows[:4] {
					index := vfRegistrationBenchmarkIndex(b, row.Inputs[api].Files)
					indexes = append(indexes, index)
					if _, err := apextest.NewProjectRuntimeForRequest(index); err != nil {
						b.Fatal(err)
					}
				}
				register := func(index typesys.Index) {
					apextest.InvalidateRuntimeCaches()
					org := storage.NewOrgState()
					host := server.New(&org)
					host.SetProjectIndex(index)
				}
				b.ReportAllocs()
				b.ResetTimer()
				for i := 0; i < b.N; i++ {
					if !all {
						register(indexes[i%len(indexes)])
						continue
					}
					for _, row := range rows {
						b.StopTimer()
						index := vfRegistrationBenchmarkIndex(b, row.Inputs[api].Files)
						b.StartTimer()
						register(index)
					}
				}
				if all {
					b.ReportMetric(float64(len(rows)), "rows/op")
				}
			})
		}
	}
}

func vfRegistrationBenchmarkIndex(b *testing.B, files map[string]string) typesys.Index {
	b.Helper()
	root := b.TempDir()
	for path, source := range files {
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
			b.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			b.Fatal(err)
		}
	}
	p, err := project.Load(root)
	if err != nil {
		b.Fatal(err)
	}
	sch, err := schema.LoadProject(p)
	if err != nil {
		b.Fatal(err)
	}
	return typesys.Build(p, sch)
}
