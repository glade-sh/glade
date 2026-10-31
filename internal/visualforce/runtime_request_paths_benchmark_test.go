package visualforce_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/glade-sh/glade/internal/apextest"
	"github.com/glade-sh/glade/internal/typesys"
	"github.com/glade-sh/glade/internal/vm"
)

// Compare the registration operations used by the former V04 and V13 paths
// with the shared request template on the same four owned runtime rows.
func BenchmarkVFRequestRegistrationPaths(b *testing.B) {
	for _, family := range []string{"v04", "v13"} {
		var table struct {
			Cases []vfRegistrationBenchmarkRow `json:"cases"`
		}
		data, err := os.ReadFile("testdata/" + family + "_salesforce.json")
		if err != nil {
			b.Fatal(err)
		}
		if err := json.Unmarshal(data, &table); err != nil {
			b.Fatal(err)
		}
		var rows []vfRegistrationBenchmarkRow
		for _, row := range table.Cases {
			if row.Kind == "runtime" {
				rows = append(rows, row)
			}
		}
		if len(rows) < 4 {
			b.Fatal("missing owned runtime rows")
		}
		for _, api := range []string{"59.0", "67.0"} {
			b.Run(family+"/"+api, func(b *testing.B) {
				indexes := make([]typesys.Index, 0, 4)
				for _, row := range rows[:4] {
					indexes = append(indexes, vfRegistrationBenchmarkIndex(b, row.Inputs[api].Files))
				}
				for _, index := range indexes {
					if _, err := apextest.NewProjectRuntimeForRequest(index); err != nil {
						b.Fatal(err)
					}
				}
				b.Run("uncached-registration", func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if err := apextest.RegisterUncachedProjectRuntimeForRequest(vm.New(nil), indexes[i%len(indexes)]); err != nil {
							b.Fatal(err)
						}
					}
				})
				b.Run("cached-registration", func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if err := apextest.RegisterProjectRuntimeForRequest(vm.New(nil), indexes[i%len(indexes)]); err != nil {
							b.Fatal(err)
						}
					}
				})
				b.Run("shared-request", func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						if _, err := apextest.NewProjectRuntimeForRequest(indexes[i%len(indexes)]); err != nil {
							b.Fatal(err)
						}
					}
				})
			})
		}
	}
}

// Use -benchtime=4x: the fifth owned index warms only the common prefix and
// the four timed indices are distinct cold project runtimes.
func BenchmarkVFColdRequestRegistrationPaths(b *testing.B) {
	for _, family := range []string{"v04", "v13"} {
		var table struct {
			Cases []vfRegistrationBenchmarkRow `json:"cases"`
		}
		data, err := os.ReadFile("testdata/" + family + "_salesforce.json")
		if err != nil {
			b.Fatal(err)
		}
		if err := json.Unmarshal(data, &table); err != nil {
			b.Fatal(err)
		}
		var rows []vfRegistrationBenchmarkRow
		for _, row := range table.Cases {
			if row.Kind == "runtime" {
				rows = append(rows, row)
			}
		}
		if len(rows) < 5 {
			b.Fatal("missing owned runtime rows")
		}
		for _, api := range []string{"59.0", "67.0"} {
			for _, mode := range []string{"uncached-registration", "cached-registration", "shared-request"} {
				b.Run(family+"/"+api+"/"+mode, func(b *testing.B) {
					b.StopTimer()
					apextest.InvalidateRuntimeCaches()
					indexes := make([]typesys.Index, 0, 5)
					for _, row := range rows[:5] {
						indexes = append(indexes, vfRegistrationBenchmarkIndex(b, row.Inputs[api].Files))
					}
					if _, err := apextest.NewProjectRuntimeForRequest(indexes[4]); err != nil {
						b.Fatal(err)
					}
					b.ReportAllocs()
					b.StartTimer()
					for i := 0; i < b.N; i++ {
						index := indexes[i%4]
						switch mode {
						case "uncached-registration":
							if err := apextest.RegisterUncachedProjectRuntimeForRequest(vm.New(nil), index); err != nil {
								b.Fatal(err)
							}
						case "cached-registration":
							if err := apextest.RegisterProjectRuntimeForRequest(vm.New(nil), index); err != nil {
								b.Fatal(err)
							}
						case "shared-request":
							if _, err := apextest.NewProjectRuntimeForRequest(index); err != nil {
								b.Fatal(err)
							}
						}
					}
				})
			}
		}
	}
}
