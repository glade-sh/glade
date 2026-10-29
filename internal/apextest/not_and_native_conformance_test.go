package apextest

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/glade-sh/glade/internal/sema"
	"github.com/glade-sh/glade/internal/startupcache"
	"github.com/glade-sh/glade/internal/vm"
)

// TestNotAndNativeConformance pins the fresh API 62/67 capture, including
// scalar controls for the grouped semi-join, default visibility, and name case.
func TestNotAndNativeConformance(t *testing.T) {
	const fixture = "testdata/conformance/not_and"
	read := func(name string) string {
		t.Helper()
		body, err := os.ReadFile(filepath.Join(fixture, name))
		if err != nil {
			t.Fatal(err)
		}
		return string(body)
	}
	cases := make(map[string]bool)
	for _, line := range strings.Split(strings.TrimSpace(read("cases.tsv")), "\n")[1:] {
		id, _, ok := strings.Cut(line, "\t")
		if !ok || cases[id] {
			t.Fatalf("invalid case: %q", line)
		}
		cases[id] = true
	}
	if len(cases) != 20 {
		t.Fatalf("expected 20 captured cases, got %d", len(cases))
	}
	native := make(map[string]map[string]any)
	lines := strings.Split(strings.TrimSpace(read("org.tsv")), "\n")
	if lines[0] != "source_api\troute\tcase_id\tobservation" || len(lines) != 121 {
		t.Fatal("incomplete native matrix")
	}
	for _, line := range lines[1:] {
		row := strings.SplitN(line, "\t", 4)
		if len(row) != 4 || (row[0] != "62.0" && row[0] != "67.0") ||
			(row[1] != "anonymous" && row[1] != "isTest_explicit" && row[1] != "isTest_default") || !cases[row[2]] {
			t.Fatalf("invalid native row: %q", line)
		}
		key := row[0] + "/" + row[1]
		if native[key] == nil {
			native[key] = make(map[string]any)
		}
		if _, duplicate := native[key][row[2]]; duplicate {
			t.Fatalf("duplicate native cell: %s/%s", key, row[2])
		}
		var value any
		if err := json.Unmarshal([]byte(row[3]), &value); err != nil {
			t.Fatal(err)
		}
		native[key][row[2]] = value
	}
	for key, observations := range native {
		if len(observations) != len(cases) {
			t.Fatalf("incomplete native cell: %s", key)
		}
	}
	assertObservation := func(t *testing.T, payload, key string) {
		t.Helper()
		var got map[string]any
		if err := json.Unmarshal([]byte(payload), &got); err != nil {
			t.Fatalf("invalid observation %q: %v", payload, err)
		}
		if len(got) != len(cases) {
			t.Fatalf("observed %d cases, expected %d", len(got), len(cases))
		}
		for id, want := range native[key] {
			t.Run(id, func(t *testing.T) {
				if !reflect.DeepEqual(got[id], want) {
					t.Fatalf("native=%v; Glade=%v", want, got[id])
				}
			})
		}
	}
	t.Cleanup(EnableDiskCacheForTesting())
	t.Cleanup(InvalidateRuntimeCaches)
	for _, api := range []string{"62.0", "67.0"} {
		t.Run(api, func(t *testing.T) {
			root := t.TempDir()
			project := filepath.Join(fixture, "project")
			if err := filepath.WalkDir(project, func(path string, entry os.DirEntry, err error) error {
				if err != nil || entry.IsDir() {
					return err
				}
				body, err := os.ReadFile(path)
				if err != nil {
					return err
				}
				name, err := filepath.Rel(project, path)
				if err != nil {
					return err
				}
				writeFile(t, filepath.Join(root, name), strings.ReplaceAll(string(body), "62.0", api))
				return nil
			}); err != nil {
				t.Fatal(err)
			}
			index := loadTestIndex(t, root)
			analysis := sema.Analyze(index)
			if index.HasErrors() || analysis.HasErrors() {
				t.Fatalf("native source compiled; diagnostics=%v %v", index.Diagnostics, analysis.Diagnostics)
			}
			t.Run("anonymous", func(t *testing.T) {
				program, err := vm.CompileAnonymousWithOptions(read("anonymous.apex"), vm.CompileOptions{APIVersion: api})
				if err != nil {
					t.Fatal(err)
				}
				org := orgFromIndex(index)
				org.APIVersion = api
				runner := newConformanceRunner(t, index, conformanceRunnerOptions{LinkProject: true, Org: &org})
				result, err := runner.execute(program)
				if err != nil {
					t.Fatal(err)
				}
				var payloads []string
				for _, line := range result.Debug {
					if _, payload, ok := strings.Cut(line, "NOTAND|"); ok {
						payloads = append(payloads, payload)
					}
				}
				if len(payloads) != 1 {
					t.Fatalf("expected one observation, got %d", len(payloads))
				}
				assertObservation(t, payloads[0], api+"/anonymous")
			})
			for _, route := range []struct{ name, class string }{
				{"isTest_explicit", "NotAndExplicitTest"},
				{"isTest_default", "NotAndDefaultTest"},
			} {
				t.Run(route.name, func(t *testing.T) {
					InvalidateRuntimeCaches()
					// Each modifier twin starts cold. The uncached run comes last
					// so a valid disk entry exists when NoDiskCache is exercised.
					if err := os.RemoveAll(filepath.Join(root, startupcache.SubdirTest)); err != nil {
						t.Fatal(err)
					}
					for _, mode := range []string{"cache_cold", "memory_warm", "disk_restored", "uncached"} {
						t.Run(mode, func(t *testing.T) {
							if mode == "disk_restored" || mode == "uncached" {
								InvalidateRuntimeCaches()
							}
							ResetPerfCounters()
							report := Run(index, Options{SelectedClasses: []string{route.class}, SelectedMethod: "observe", NoDiskCache: mode == "uncached", Parallelism: 1, PerfCounters: true})
							var misses, memoryHits, diskHits uint64
							switch mode {
							case "cache_cold", "uncached":
								misses = 1
							case "memory_warm":
								memoryHits = 1
							case "disk_restored":
								diskHits = 1
							}
							phases := SnapshotPerfCounters().Phases
							if phases.CacheMisses != misses || phases.MemoryCacheHits != memoryHits || phases.DiskCacheHits != diskHits {
								t.Fatalf("unexpected cache route: misses=%d memory=%d disk=%d; want %d/%d/%d", phases.CacheMisses, phases.MemoryCacheHits, phases.DiskCacheHits, misses, memoryHits, diskHits)
							}
							if len(report.Suites) != 1 || len(report.Suites[0].Cases) != 1 {
								t.Fatalf("expected one observation: %+v", report.Summary())
							}
							result := report.Suites[0].Cases[0]
							if result.Problem == nil || !strings.HasSuffix(result.Problem.Type, "AssertException") {
								t.Fatalf("missing assertion transport: %+v", result)
							}
							_, payload, ok := strings.Cut(result.Problem.Message, "NOTAND|")
							if !ok {
								t.Fatal("missing native observation marker")
							}
							assertObservation(t, payload, api+"/"+route.name)
						})
					}
				})
			}
		})
	}
}
